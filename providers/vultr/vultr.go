package vultr

import (
	"fmt"
	"os"
	"strings"

	"github.com/JamesClonk/vultr/lib"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

// Provider allows DNS to be manipulated on Vultr as an external-dns provider
type Provider struct {
	client *lib.Client
	root   string
}

func init() {
	providers.RegisterProvider("vultr", &Provider{})
}

// Init populates the Provider struct
func (d *Provider) Init(rootDomainName string) error {
	var url, apiKey string
	url = os.Getenv("VULTR_URL")

	if apiKey = os.Getenv("VULTR_API_KEY"); len(apiKey) == 0 {
		return fmt.Errorf("VULTR_API_KEY is not set")
	}

	d.root = utils.UnFqdn(rootDomainName)
	d.client = lib.NewClient(apiKey, &lib.Options{Endpoint: url})

	err := d.HealthCheck()
	if err != nil {
		return fmt.Errorf("Failed to list records for '%s': %v", d.root, err)
	}

	logrus.Infof("Configured %s with zone '%s'", d.GetName(), d.root)
	return nil
}

// GetName returns the name of the provider
func (*Provider) GetName() string {
	return "Vultr"
}

// HealthCheck verifies the API is connected and working
func (d *Provider) HealthCheck() error {
	_, err := d.client.GetDnsRecords(d.root)
	return err
}

// utils.DnsRecord.Fqdn has a trailing. | powerdns.Record.Name doesn't
func (d *Provider) parseName(record utils.DnsRecord) string {
	return utils.UnFqdn(record.Fqdn)
}

// AddRecord adds this DNS record to the domain
func (d *Provider) AddRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called AddRecord with: %v\n", record)
	name := d.parseName(record)
	for _, data := range record.Records {
		if err := d.client.CreateDnsRecord(d.root, name, record.Type, data, 0, record.TTL); err != nil {
			logrus.Errorf("Failed to add Record for %s : %v", name, err)
			return err

		}
	}
	return nil
}

func (d *Provider) findRecords(record utils.DnsRecord) ([]lib.DnsRecord, error) {
	var records []lib.DnsRecord
	resp, err := d.client.GetDnsRecords(d.root)
	if err != nil {
		return records, fmt.Errorf("Vultr API call has failed: %v", err)
	}

	name := d.parseName(record)
	// utils.DnsRecord.Fqdn has a trailing. | powerdns.Record.Name doesn't
	logrus.Debugf("Parsed Name is %s\n", name)
	for _, rec := range resp {
		recName := strings.Join([]string{rec.Name, d.root}, ".")
		if recName == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

// UpdateRecord replaces existing records with this one
func (d *Provider) UpdateRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called UpdateRecord with: %v\n", record)

	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

// RemoveRecord deletes the record from the domain
func (d *Provider) RemoveRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called RemoveRecord with: %v\n", record)

	records, err := d.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := d.client.DeleteDnsRecord(d.root, rec.RecordID); err != nil {
			return fmt.Errorf("Vultr API call has failed: %v", err)
		}
	}

	return nil
}

// GetRecords returns all dns records for the domain
func (d *Provider) GetRecords() ([]utils.DnsRecord, error) {
	logrus.Debug("Called GetRecords")
	var records []utils.DnsRecord

	vultsrRecord, err := d.client.GetDnsRecords(d.root)
	if err != nil {
		return records, fmt.Errorf("Vultr API call has failed: %v", err)
	}

	for _, rec := range vultsrRecord {
		logrus.Debugf("%v\n", rec)

		name := strings.Join([]string{rec.Name, d.root}, ".")
		// need to combine records with the same name
		found := false
		for i, re := range records {
			if re.Fqdn == name {
				found = true
				cont := append(re.Records, rec.Data)
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
				Records: []string{rec.Data},
				Type:    rec.Type,
				TTL:     rec.TTL,
			}
			records = append(records, r)
		}
	}
	return records, nil
}
