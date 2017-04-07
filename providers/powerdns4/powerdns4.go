package powerdns4

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/dmportella/powerdns"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

const (
	providerName = "powerdns4"
)

type PdnsProvider struct {
	client *powerdns.Client
	root   string
}

func init() {
	providers.RegisterProvider("powerdns4", &PdnsProvider{})
}

// Init Initialise the powerdns4 provider.
func (pdnsProvider *PdnsProvider) Init(rootDomainName string) error {
	var url, apiKey string
	if url = os.Getenv("POWERDNS_URL"); len(url) == 0 {
		return fmt.Errorf("POWERDNS_URL is not set")
	}

	if apiKey = os.Getenv("POWERDNS_API_KEY"); len(apiKey) == 0 {
		return fmt.Errorf("POWERDNS_API_KEY is not set")
	}

	pdnsProvider.root = utils.UnFqdn(rootDomainName)

	var client *powerdns.Client
	client, err := powerdns.NewClient(url, apiKey)
	if err != nil {
		return fmt.Errorf("Failed to connect to '%s': %v", pdnsProvider.root, err)
	}
	pdnsProvider.client = client

	logrus.Infof("Configured %s with zone '%s'", pdnsProvider.GetName(), pdnsProvider.root)
	return nil
}

// GetName Returns the name of the dns provider.
func (*PdnsProvider) GetName() string {
	return providerName
}

// HealthCheck Checks that the external-dns provider can talk to powerdns.
func (pdnsProvider *PdnsProvider) HealthCheck() error {
	if _, err := pdnsProvider.client.ListZones(); err != nil {
		return fmt.Errorf("healthcheck failed for '%s' with error '%s'", pdnsProvider.GetName(), err)
	}
	return nil
}

// AddRecord Adds a record to the zone.
func (pdnsProvider *PdnsProvider) AddRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called AddRecord with: %v\n", record)

	return pdnsProvider.UpdateRecord(record)
}

// UpdateRecord Updates a record in the zone.
func (pdnsProvider *PdnsProvider) UpdateRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called UpdateRecord with: %v\n", record)

	rrSet := powerdns.ResourceRecordSet{
		Name:       record.Fqdn,
		Type:       record.Type,
		ChangeType: "REPLACE",
		TTL:        record.TTL,
	}

	records := make([]powerdns.Record, 0)
	for _, rec := range record.Records {
		if record.Type == "TXT" {
			rec = fmt.Sprintf("\"%s\"", rec)
		}

		records = append(records, powerdns.Record{
			Content: rec,
		})
	}

	rrSet.Records = records

	recordID, err := pdnsProvider.client.ReplaceRecordSet(pdnsProvider.root, rrSet)
	if err != nil {
		return fmt.Errorf("Failed to add '%s' record on '%s': %v", record.Fqdn, pdnsProvider.root, err)
	}

	logrus.Infof("Added '%s' record on '%s'", recordID, pdnsProvider.root)

	return nil
}

// RemoveRecord Removes a record in the zone.
func (pdnsProvider *PdnsProvider) RemoveRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called RemoveRecord with: %v\n", record)

	found, err := pdnsProvider.client.RecordExists(pdnsProvider.root, record.Fqdn, record.Type)
	if err != nil {
		return fmt.Errorf("Failed to remove '%s' record on '%s': %v", record.Fqdn, pdnsProvider.root, err)
	}

	if !found {
		return nil
	}

	return pdnsProvider.client.DeleteRecordSet(pdnsProvider.root, record.Fqdn, record.Type)
}

// GetRecords Returns all records in the zone.
func (pdnsProvider *PdnsProvider) GetRecords() ([]utils.DnsRecord, error) {
	logrus.Debugf("Called GetRecords")

	rrSets, err := pdnsProvider.client.ListRecordsAsRRSet(pdnsProvider.root)
	if err != nil {
		return nil, fmt.Errorf("Failed getting records in zone '%s': %v", pdnsProvider.root, err)
	}

	result := make([]utils.DnsRecord, 0)

	for _, rrSet := range rrSets {
		rancherRec := utils.DnsRecord{
			Fqdn:    rrSet.Name,
			Type:    rrSet.Type,
			TTL:     rrSet.TTL,
			Records: []string{},
		}

		records := make([]string, 0)
		for _, rec := range rrSet.Records {
			// Powerdns saves and expects TXT record values to be in quotes which is part of the DNS rfc1464.
			if rancherRec.Type == "TXT" {
				rec.Content = strings.Replace(rec.Content, "\"", "", -1)
			}

			records = append(records, rec.Content)
		}

		rancherRec.Records = records

		result = append(result, rancherRec)
	}

	return result, nil
}
