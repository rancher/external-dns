package providers

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/jgreat/powerdns"
	"github.com/rancher/external-dns/dns"
)

func init() {
	pdnsAPIKey := os.Getenv("POWERDNS_API_KEY")
	if len(pdnsAPIKey) == 0 {
		logrus.Info("POWERDNS_API_KEY is not set, skipping init of PowerDNS provider")
		return
	}

	pdnsURL := os.Getenv("POWERDNS_URL")
	if len(pdnsURL) == 0 {
		logrus.Info("POWERDNS_URL is not set, skipping init of PowerDNS provider")
		return
	}

	pdnsHandler := &PdnsHandler{}
	if err := RegisterProvider("powerdns", pdnsHandler); err != nil {
		logrus.Fatal("Could not register powerdns provider")
	}

	pdnsHandler.root = strings.TrimSuffix(dns.RootDomainName, ".")
	pdnsHandler.client = powerdns.New(pdnsURL, "", pdnsHandler.root, pdnsAPIKey)

	_, err := pdnsHandler.client.GetRecords()
	if err != nil {
		logrus.Fatalf("Failed to list records for %s : %v", pdnsHandler.root, err)
	}

	logrus.Infof("Configured %s with hosted zone %q ", pdnsHandler.GetName(), pdnsHandler.root)
}

// PdnsHandler ...
type PdnsHandler struct {
	client *powerdns.PowerDNS
	root   string
}

// GetName ...
func (*PdnsHandler) GetName() string {
	return "PowerDNS"
}

// dns.DnsRecord.Fqdn has a trailing. | powerdns.Record.Name doesn't
func (d *PdnsHandler) parseName(record dns.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, ".")

	return name
}

// AddRecord ...
func (d *PdnsHandler) AddRecord(record dns.DnsRecord) error {
	logrus.Debugf("Called AddRecord with: %v\n", record)
	name := d.parseName(record)
	_, err := d.client.AddRecord(name, record.Type, record.TTL, record.Records)
	if err != nil {
		logrus.Errorf("Failed to add Record for %s : %v", name, err)
		return err
	}
	return nil
}

func (d *PdnsHandler) findRecords(record dns.DnsRecord) ([]powerdns.Record, error) {
	var records []powerdns.Record
	resp, err := d.client.GetRecords()
	if err != nil {
		return records, fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	name := d.parseName(record)
	// dns.DnsRecord.Fqdn has a trailing. | powerdns.Record.Name doesn't
	logrus.Debugf("Parsed Name is %s\n", name)
	for _, rec := range resp {
		if rec.Name == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

// UpdateRecord ...
func (d *PdnsHandler) UpdateRecord(record dns.DnsRecord) error {
	logrus.Debugf("Called UpdateRecord with: %v\n", record)

	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

// RemoveRecord ... Might be able to do this with out the for loop
func (d *PdnsHandler) RemoveRecord(record dns.DnsRecord) error {
	logrus.Debugf("Called RemoveRecord with: %v\n", record)

	records, err := d.findRecords(record)
	if err != nil {
		return err
	}

	name := d.parseName(record)
	for _, rec := range records {
		_, err := d.client.DeleteRecord(name, record.Type, record.TTL, []string{rec.Content})
		if err != nil {
			return fmt.Errorf("PowerDNS API call has failed: %v", err)
		}
	}

	return nil
}

// GetRecords ...
func (d *PdnsHandler) GetRecords() ([]dns.DnsRecord, error) {
	logrus.Debugln("Called GetRecords")
	var records []dns.DnsRecord

	pdnsRecords, err := d.client.GetRecords()
	if err != nil {
		return records, fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	for _, rec := range pdnsRecords {
		logrus.Debugf("%v\n", rec)
		if rec.Type != "A" {
			continue
		}
		if rec.Disabled == true {
			continue
		}

		name := fmt.Sprintf("%s.", rec.Name)
		// need to combine records with the same name
		found := false
		for i, re := range records {
			if re.Fqdn == name {
				found = true
				cont := append(re.Records, rec.Content)
				records[i] = dns.DnsRecord{
					Fqdn:    name,
					Records: cont,
					Type:    rec.Type,
					TTL:     rec.TTL,
				}
			}
		}
		if !found {
			r := dns.DnsRecord{
				Fqdn:    name,
				Records: []string{rec.Content},
				Type:    rec.Type,
				TTL:     rec.TTL,
			}
			records = append(records, r)
		}
	}
	return records, nil
}
