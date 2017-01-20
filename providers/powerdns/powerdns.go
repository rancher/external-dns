package powerdns

import (
	"fmt"
	"os"
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/dmportella/powerdns"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type PdnsProvider struct {
	client *PowerDNS.Client
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

	client, err := PowerDNS.NewClient(url, apiKey)

	if err != nil {
		return fmt.Errorf("Failed to initialize powerdns client : %v", err)
	}

	logrus.Infof("PDNS RECORD '%s'", client.ApiVersion)


	d.client = client

	if d.client.ApiVersion == 0 {
		d.root = utils.UnFqdn(rootDomainName)
	} else {
		d.root = rootDomainName
	}

	_, err = d.client.ListRecords(d.root)
	if err != nil {
		return fmt.Errorf("Failed to list records for '%s': %v", d.root, err)
	}

	logrus.Infof("Configured %s with zone '%s'", d.GetName(), d.root)
	return nil
}

func (d *PdnsProvider) formatRecordName(fqdn string) string {
	if d.client.ApiVersion == 0 {
		return utils.UnFqdn(fqdn)
	} else {
		return fqdn
	}
}

func (*PdnsProvider) GetName() string {
	return "PowerDNS"
}

func (d *PdnsProvider) HealthCheck() error {
	_, err := d.client.ListRecords(d.root)
	return err
}

func (d *PdnsProvider) AddRecord(record utils.DnsRecord) error {
	logrus.Debugf("Called AddRecord with: %v\n", record)
	name := record.Fqdn

	pdnsRRecordSet := PowerDNS.ResourceRecordSet{Name: d.formatRecordName(name), Type:record.Type, ChangeType:"REPLACE", TTL: record.TTL}

	pdnsRRecordSet.Records = []PowerDNS.Record{}

	for _, rec  := range record.Records {
		val := PowerDNS.Record{Name: d.formatRecordName(name), Type:record.Type, Content: rec, Disabled: false, TTL: record.TTL}

		// TXT records MUST be in quotes
		if record.Type == "TXT" {
			val.Content = fmt.Sprintf("\"%s\"", rec)
		}

		pdnsRRecordSet.Records = append(pdnsRRecordSet.Records, val)
	}
	
	slcB, _ := json.MarshalIndent(pdnsRRecordSet, "", "    ")

	logrus.Infof("PDNS RECORD '%s'", string(slcB))

	_, err := d.client.ReplaceRecordSet(d.root, pdnsRRecordSet)
	if err != nil {
		logrus.Errorf("Failed to add Record for %s : %v", name, err)
		return err
	}
	return nil
}

func (d *PdnsProvider) findRecords(record utils.DnsRecord) ([]PowerDNS.Record, error) {
	var records []PowerDNS.Record
	resp, err := d.client.ListRecords(d.root)
	if err != nil {
		return records, fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	name := d.formatRecordName(record.Fqdn)

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

	exists, err := d.client.RecordExists(d.root, d.formatRecordName(record.Fqdn), record.Type);

	if err != nil {
		return fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	if !exists {
		return nil
	}

	err = d.client.DeleteRecordSet(d.root, d.formatRecordName(record.Fqdn), record.Type)

	if err != nil {
		return fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	return nil
}

func (d *PdnsProvider) GetRecords() ([]utils.DnsRecord, error) {
	logrus.Debug("Called GetRecords")
	var records []utils.DnsRecord

	pdnsRecords, err := d.client.ListRecords(d.root)
	if err != nil {
		return records, fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	for _, rec := range pdnsRecords {
		logrus.Debugf("%v\n", rec)
		if rec.Disabled == true {
			continue
		}

		name := rec.Name
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