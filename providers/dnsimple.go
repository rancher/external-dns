package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	"github.com/weppos/go-dnsimple/dnsimple"
	"os"
	"strings"
)

func init() {
	apiToken := os.Getenv("DNSIMPLE_TOKEN")
	if len(apiToken) == 0 {
		logrus.Info("DNSIMPLE_TOKEN is not set, skipping init of DNSimple provider")
		return
	}

	email := os.Getenv("DNSIMPLE_EMAIL")
	if len(email) == 0 {
		logrus.Info("DNSIMPLE_EMAIL is not set, skipping init of DNSimple provider")
		return
	}

	dnsimpleHandler := &DNSimpleHandler{}
	if err := RegisterProvider("dnsimple", dnsimpleHandler); err != nil {
		logrus.Fatal("Could not register dnsimple provider")
	}

	dns.SetRootDomain(getDefaultRootDomain())

	dnsimpleHandler.root = strings.TrimSuffix(dns.RootDomainName, ".")
	dnsimpleHandler.client = dnsimple.NewClient(apiToken, email)

	domains, _, err := dnsimpleHandler.client.Domains.List()
	if err != nil {
		logrus.Fatalf("Failed to list hosted zones: %v", err)
	}

	found := false
	for _, domain := range domains {
		if domain.Name == dnsimpleHandler.root {
			found = true
			break
		}
	}

	if !found {
		logrus.Fatalf("Hosted zone %s is missing", dnsimpleHandler.root)
	}

	logrus.Infof("Configured %s with hosted zone %q ", dnsimpleHandler.GetName(), dnsimpleHandler.root)
}

type DNSimpleHandler struct {
	client *dnsimple.Client
	root   string
}

func (*DNSimpleHandler) TestConnection() error {
	return nil
}

func (*DNSimpleHandler) GetName() string {
	return "DNSimple"
}

func (*DNSimpleHandler) GetRootDomain() string {
	return getDefaultRootDomain()
}

func (d *DNSimpleHandler) parseName(record dns.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, fmt.Sprintf(".%s.", d.root))

	return name
}

func (d *DNSimpleHandler) AddRecord(record dns.DnsRecord) error {
	name := d.parseName(record)
	for _, rec := range record.Records {
		recordInput := dnsimple.Record{
			Name:    name,
			TTL:     record.TTL,
			Type:    record.Type,
			Content: rec,
		}
		_, _, err := d.client.Domains.CreateRecord(d.root, recordInput)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleHandler) findRecords(record dns.DnsRecord) ([]dnsimple.Record, error) {
	var records []dnsimple.Record
	resp, _, err := d.client.Domains.ListRecords(d.root, "", "")
	if err != nil {
		return records, fmt.Errorf("DNSimple API call has failed: %v", err)
	}

	name := d.parseName(record)
	for _, rec := range resp {
		if rec.Name == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

func (d *DNSimpleHandler) UpdateRecord(record dns.DnsRecord) error {
	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *DNSimpleHandler) RemoveRecord(record dns.DnsRecord) error {
	records, err := d.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		_, err := d.client.Domains.DeleteRecord(d.root, rec.Id)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleHandler) GetRecords(listOpts ...string) ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord

	recordResp, _, err := d.client.Domains.ListRecords(d.root, "", "")
	if err != nil {
		return records, fmt.Errorf("DNSimple API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range recordResp {
		var fqdn string
		if rec.Name == "" {
			fqdn = dns.RootDomainName
		} else {
			fqdn = fmt.Sprintf("%s.%s", rec.Name, dns.RootDomainName)
		}

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][rec.Type] = rec.TTL
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[rec.Type]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Content)
				recordSet[rec.Type] = recordSlice
			} else {
				recordSet[rec.Type] = []string{rec.Content}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.Type] = []string{rec.Content}
		}
	}

	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			ttl := recordTTLs[fqdn][recordType]
			record := dns.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}
	return records, nil
}
