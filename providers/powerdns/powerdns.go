package powerdns

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/jgreat/powerdns"
	"github.com/mathuin/external-dns/providers"
	"github.com/mathuin/external-dns/utils"
)

type PdnsProvider struct {
	client *powerdns.PowerDNS
	root   string
}

func init() {
	providers.RegisterProvider("powerdns", &PdnsProvider{})
}

func (d *PdnsProvider) Init(rootDomainName string) error {
	var url, apiKey string
	if url = os.Getenv("POWERDNS_URL"); len(url) == 0 {
		return fmt.Errorf("POWERDNS_URL is not set")
	}

	if apiKey = os.Getenv("POWERDNS_API_KEY"); len(apiKey) == 0 {
		return fmt.Errorf("POWERDNS_API_KEY is not set")
	}

	d.root = utils.UnFqdn(rootDomainName)
	d.client = powerdns.New(url, "", d.root, apiKey)

	_, err := d.client.GetRecords()
	if err != nil {
		return fmt.Errorf("Failed to list records for '%s': %v", d.root, err)
	}

	logrus.Infof("Configured %s with zone '%s'", d.GetName(), d.root)
	return nil
}

func (*PdnsProvider) GetName() string {
	return "PowerDNS"
}

func (d *PdnsProvider) HealthCheck() error {
	_, err := d.client.GetRecords()
	return err
}

// utils.DnsRecord.Fqdn has a trailing. | powerdns.Record.Name doesn't
func (d *PdnsProvider) parseName(record utils.DnsRecord) string {
	return utils.UnFqdn(record.Fqdn)
}

func (d *PdnsProvider) AddRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called AddRecord with: %v\n", record)
	name := d.parseName(record)
	_, err := d.client.AddRecord(name, record.Type, record.TTL, record.Records)
	if err != nil {
		logrus.Errorf("Failed to add Record for %s : %v", name, err)
		return err
	}
	return nil
}

func (d *PdnsProvider) findRecords(record utils.DnsRecord) ([]powerdns.Record, error) {
	var records []powerdns.Record
	resp, err := d.client.GetRecords()
	if err != nil {
		return records, fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	name := d.parseName(record)
	// utils.DnsRecord.Fqdn has a trailing. | powerdns.Record.Name doesn't
	logrus.Debugf("Parsed Name is %s\n", name)
	for _, rec := range resp {
		if rec.Name == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

func (d *PdnsProvider) UpdateRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called UpdateRecord with: %v\n", record)

	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

// RemoveRecord ... Might be able to do this with out the for loop
func (d *PdnsProvider) RemoveRecord(record utils.DnsRecord) error {
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

func (d *PdnsProvider) GetRecords() ([]utils.DnsRecord, error) {
	logrus.Debug("Called GetRecords")
	var records []utils.DnsRecord

	pdnsRecords, err := d.client.GetRecords()
	if err != nil {
		return records, fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	for _, rec := range pdnsRecords {
		logrus.Debugf("%v\n", rec)
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
				records[i] = utils.DnsRecord{
					Fqdn:    name,
					Records: cont,
					Type:    rec.Type,
					TTL:     rec.TTL,
				}
			}
		}
		if !found {
			r := utils.DnsRecord{
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
