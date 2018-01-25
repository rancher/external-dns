package azure

import (
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/arm/dns"
	"github.com/Azure/azure-sdk-for-go/arm/examples/helpers"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type ZoneDNSProvider struct {
	zoneClient     dns.ZonesClient
	recordClient   dns.RecordSetsClient
	rootDomainName string
	resourceGroup  string
}

func init() {
	providers.RegisterProvider("azure", &ZoneDNSProvider{})
}

// Initialize Azure DNS provider.
// Return nil in case of success otherwise an error
func (zd *ZoneDNSProvider) Init(rootDomainName string) error {
	zd.rootDomainName = strings.TrimRight(rootDomainName, ".")
	zd.resourceGroup = os.Getenv("AZURE_RESOURCE_GROUP")
	c := map[string]string{
		"AZURE_CLIENT_ID":       os.Getenv("AZURE_CLIENT_ID"),
		"AZURE_CLIENT_SECRET":   os.Getenv("AZURE_CLIENT_SECRET"),
		"AZURE_SUBSCRIPTION_ID": os.Getenv("AZURE_SUBSCRIPTION_ID"),
		"AZURE_TENANT_ID":       os.Getenv("AZURE_TENANT_ID")}

	spt, err := helpers.NewServicePrincipalTokenFromCredentials(
		c,
		azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	zd.zoneClient = dns.NewZonesClient(c["AZURE_SUBSCRIPTION_ID"])
	zd.zoneClient.Authorizer = autorest.NewBearerAuthorizer(spt)
	zd.recordClient = dns.NewRecordSetsClient(c["AZURE_SUBSCRIPTION_ID"])
	zd.recordClient.Authorizer = autorest.NewBearerAuthorizer(spt)

	return nil
}

// Return the name of Azure DNS provider.
func (zd *ZoneDNSProvider) GetName() string {
	return "Azure Zone DNS"
}

// Check health of service, in this case fetch ZoneDNS info.
// Returns nil in case of success otherwise an error.
func (zd *ZoneDNSProvider) HealthCheck() error {
	_, err := zd.zoneClient.Get(zd.resourceGroup, zd.rootDomainName)
	return err
}

// Add a record to Azure DNS.
func (zd *ZoneDNSProvider) AddRecord(record utils.DnsRecord) error {
	return zd.changeRecord(record)
}

// Remove a record from Azure DNS.
func (zd *ZoneDNSProvider) RemoveRecord(record utils.DnsRecord) error {
	_, err := zd.recordClient.Delete(
		zd.resourceGroup,
		zd.rootDomainName,
		strings.Replace(utils.UnFqdn(record.Fqdn), "." + zd.rootDomainName, "", -1),
		dns.RecordType(record.Type),
		"")

	if err != nil {
		return fmt.Errorf(
			"failed to delete %s record named '%s' for DNS zone '%s': %v",
			record.Type,
			strings.TrimRight(record.Fqdn, "."),
			zd.rootDomainName,
			err,
		)
	}
	return nil
}

// Update a record on Azure DNS.
func (zd *ZoneDNSProvider) UpdateRecord(record utils.DnsRecord) error {
	return zd.changeRecord(record)
}

// Fetch and return all records from Azure DNS.
func (zd *ZoneDNSProvider) GetRecords() ([]utils.DnsRecord, error) {
	dnsRecords := []utils.DnsRecord{}
	err := zd.iterateRecords(func(recordSet dns.RecordSet) bool {
		if recordSet.Name == nil || recordSet.Type == nil {
			logrus.Error("Skipping invalid record set with nil name or type.")
			return true
		}
		recordType := strings.TrimLeft(*recordSet.Type, "Microsoft.Network/dnszones/")
		if !supportedRecordType(recordType) {
			return true
		}
		name := formatAzureDNSName(*recordSet.Name, zd.rootDomainName)
		targets := extractAzureTarget(&recordSet)
		if targets == nil {
			logrus.Errorf("Failed to extract target for '%s' with type '%s'.", name, recordType)
			return true
		}

		dnsRecords = append(dnsRecords, utils.DnsRecord{
			Fqdn:    name + ".",
			Records: targets,
			Type:    recordType,
			TTL:     int(*recordSet.TTL),
		})
		return true
	})

	if err != nil {
		return nil, err
	}

	return dnsRecords, err
}

// Fetch all records from Azure DNS.
// Maximum Azure SDK allows to fetch 100 records at once, this method iterate to fetch all of them.
func (zd *ZoneDNSProvider) iterateRecords(callback func(dns.RecordSet) bool) error {
	list, err := zd.recordClient.ListByDNSZone(
		zd.resourceGroup,
		zd.rootDomainName,
		nil,
		"")

	if err != nil {
		return err
	}

	for list.Value != nil && len(*list.Value) > 0 {
		for _, recordSet := range *list.Value {
			if !callback(recordSet) {
				return nil
			}
		}

		list, err = zd.recordClient.ListByDNSZoneNextResults(list)
		if err != nil {
			return err
		}
	}
	return nil
}

// Add/Update a record on Azure DNS, the code is the same for both actions.
func (zd *ZoneDNSProvider) changeRecord(record utils.DnsRecord) error {
	recordSet, err := getRecordSetFromDnsRecordType(record)

	if err != nil {
		return err
	} else {
		_, err = zd.recordClient.CreateOrUpdate(
			zd.resourceGroup,
			zd.rootDomainName,
			strings.Replace(utils.UnFqdn(record.Fqdn), "." + zd.rootDomainName, "", -1),
			dns.RecordType(record.Type),
			recordSet,
			"",
			"")
	}

	if err != nil {
		return fmt.Errorf(
			"failed to update %s record named '%s' for DNS zone '%s': %v",
			record.Type,
			strings.TrimRight(record.Fqdn, "."),
			zd.rootDomainName,
			err,
		)
	}
	return nil
}

// Helper function to check if record is supported or not.
// Returns true if supported otherwise false.
func supportedRecordType(recordType string) bool {
	switch recordType {
	case "A", "AAAA", "CNAME", "TXT":
		return true
	default:
		return false
	}
}

// Helper function to create a new recordSet.
// It depends on the record type.
func getRecordSetFromDnsRecordType(record utils.DnsRecord) (dns.RecordSet, error) {
	recordSetProperties := dns.RecordSetProperties{
		TTL: to.Int64Ptr(int64(record.TTL)),
	}
	recordSet := dns.RecordSet{}

	switch record.Type {
	case "A":
		aRecord := make([]dns.ARecord, len(record.Records))
		for i, recordValue := range record.Records {
			aRecord[i].Ipv4Address = to.StringPtr(recordValue)
		}
		recordSetProperties.ARecords = &aRecord
	case "AAAA":
		aaaaRecord := make([]dns.AaaaRecord, len(record.Records))
		for i, recordValue := range record.Records {
			aaaaRecord[i].Ipv6Address = to.StringPtr(recordValue)
		}
		recordSetProperties.AaaaRecords = &aaaaRecord
	case "TXT":
		txtRecord := make([]dns.TxtRecord, len(record.Records))
		for i, recordValue := range record.Records {
			txtRecord[i].Value = &[]string{recordValue}
		}
		recordSetProperties.TxtRecords = &txtRecord
	case "CNAME":
		recordSetProperties.CnameRecord = &dns.CnameRecord{
			Cname: &record.Records[0],
		}
	default:
		return recordSet, fmt.Errorf(
			"recordtype %s did not match any known record type",
			record.Type)
	}

	recordSet.RecordSetProperties = &recordSetProperties

	return recordSet, nil
}

// Helper function to format Azure DNS name.
func formatAzureDNSName(recordName, zoneName string) string {
	if recordName == "@" {
		return zoneName
	}
	return fmt.Sprintf("%s.%s", recordName, zoneName)
}

// Helper function to extract target from dns.RecordSet into []string.
func extractAzureTarget(recordSet *dns.RecordSet) []string {
	properties := recordSet.RecordSetProperties
	records := []string{}
	if properties == nil {
		return nil
	}

	// Check for A records
	aRecords := properties.ARecords
	if aRecords != nil {
		for _, record := range *aRecords {
			if record.Ipv4Address != nil {
				records = append(records, *record.Ipv4Address)
			}
		}
		return records
	}

	// Check for AAAA records
	aaaaRecords := properties.AaaaRecords
	if aaaaRecords != nil {
		for _, record := range *aaaaRecords {
			if record.Ipv6Address != nil {
				records = append(records, *record.Ipv6Address)
			}
		}
		return records
	}

	// Check for CNAME records
	cnameRecord := properties.CnameRecord
	if cnameRecord != nil && cnameRecord.Cname != nil {
		return append(records, *cnameRecord.Cname)
	}

	// Check for TXT records
	txtRecords := properties.TxtRecords
	if txtRecords != nil {
		for _, record := range *txtRecords {
			if record.Value != nil {
				records = append(records, (*record.Value)[0])
			}
		}
		return records
	}
	return nil
}
