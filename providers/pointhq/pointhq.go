package pointhq

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/cognetoapps/go-pointdns"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type PointHQProvider struct {
	client *pointdns.PointClient
	root   string
	zone   pointdns.Zone
}

func init() {
	providers.RegisterProvider("pointhq", &PointHQProvider{})
}

func (d *PointHQProvider) Init() error {
	rootDomainName := utils.GetDefaultRootDomain()
	var email, apiToken string
	if email = os.Getenv("POINTHQ_EMAIL"); len(email) == 0 {
		return fmt.Errorf("POINTHQ_EMAIL is not set")
	}

	if apiToken = os.Getenv("POINTHQ_TOKEN"); len(apiToken) == 0 {
		return fmt.Errorf("POINTHQ_TOKEN is not set")
	}

	d.root = utils.UnFqdn(rootDomainName)
	d.client = pointdns.NewClient(email, apiToken)

	zones, err := d.client.Zones()
	if err != nil {
		return fmt.Errorf("Failed to list hosted zones: %v", err)
	}

	found := false
	for _, zone := range zones {
		if zone.Name == d.root {
			found = true
			d.zone = zone
			break
		}
	}

	if !found {
		return fmt.Errorf("Zone for '%s' not found", d.root)
	}

	logrus.Infof("Configured %s with zone '%s'", d.GetName(), d.root)
	return nil
}

func (*PointHQProvider) GetName() string {
	return "PointHQ"
}

func (*PointHQProvider) GetRootDomain() string {
	return utils.GetDefaultRootDomain()
}

func (d *PointHQProvider) HealthCheck() error {
	_, err := d.client.Zones()
	return err
}

func (d *PointHQProvider) parseName(record utils.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, fmt.Sprintf(".%s.", d.root))
	return name
}

func (d *PointHQProvider) AddRecord(record utils.DnsRecord) error {
	name := d.parseName(record)
	for _, rec := range record.Records {
		recordInput := pointdns.Record{
			Name:       name,
			Ttl:        record.TTL,
			RecordType: record.Type,
			Data:       rec,
			ZoneId:     d.zone.Id,
		}
		_, err := d.client.CreateRecord(&recordInput)
		if err != nil {
			return fmt.Errorf("PointHQ API call has failed: %v", err)
		}
	}

	return nil
}

func (d *PointHQProvider) FindRecords(record utils.DnsRecord) ([]pointdns.Record, error) {
	var records []pointdns.Record
	resp, err := d.zone.Records()
	if err != nil {
		return records, fmt.Errorf("PointHQ API call has failed: %v", err)
	}

	for _, rec := range resp {
		if rec.Name == record.Fqdn && rec.RecordType == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

func (d *PointHQProvider) UpdateRecord(record utils.DnsRecord) error {
	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *PointHQProvider) RemoveRecord(record utils.DnsRecord) error {
	records, err := d.FindRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		_, err := rec.Delete()
		if err != nil {
			return fmt.Errorf("PointHQ API call has failed: %v", err)
		}
	}

	return nil
}

func (d *PointHQProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord
	recordResp, err := d.zone.Records()
	if err != nil {
		return records, fmt.Errorf("PointHQ API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range recordResp {
		var fqdn string
		if rec.Name == "" {
			fqdn = d.root + "."
		} else {
			fqdn = rec.Name
		}

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][rec.RecordType] = rec.Ttl
		recordSet, exists := recordMap[fqdn]

		if exists {
			recordSlice, sliceExists := recordSet[rec.RecordType]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Data)
				recordSet[rec.RecordType] = recordSlice
			} else {
				recordSet[rec.RecordType] = []string{rec.Data}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.RecordType] = []string{rec.Data}
		}
	}

	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			ttl := recordTTLs[fqdn][recordType]
			record := utils.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}

	return records, nil
}
