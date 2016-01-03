package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/cognetoapps/go-pointdns"
	"github.com/rancher/external-dns/dns"
	"os"
	"strings"
)

func init() {
	apiToken := os.Getenv("POINTHQ_TOKEN")
	if len(apiToken) == 0 {
		logrus.Info("POINTHQ_TOKEN is not set, skipping init of PointHQ provider")
		return
	}

	email := os.Getenv("POINTHQ_EMAIL")
	if len(email) == 0 {
		logrus.Info("POINTHQ_EMAIL is not set, skipping init of PointHQ provider")
		return
	}

	pointhqHandler := &PointHQHandler{}
	if err := RegisterProvider("pointhq", pointhqHandler); err != nil {
		logrus.Fatal("Could not register pointhqHandler provider")
	}

	pointhqHandler.root = strings.TrimSuffix(dns.RootDomainName, ".")
	pointhqHandler.client = pointdns.NewClient(email, apiToken)

	zones, err := pointhqHandler.client.Zones()
	if err != nil {
		logrus.Fatalf("Failed to list hosted zones: %v", err)
	}

	found := false
	for _, zone := range zones {
		if zone.Name == pointhqHandler.root {
			found = true
			pointhqHandler.zone = zone
			break
		}
	}

	if !found {
		logrus.Fatalf("Hosted zone %s is missing", pointhqHandler.root)
	}

	logrus.Infof("Configured %s with hosted zone %q ", pointhqHandler.GetName(), pointhqHandler.root)
}

type PointHQHandler struct {
	client *pointdns.PointClient
	root   string
	zone   pointdns.Zone
}

func (*PointHQHandler) GetName() string {
	return "PointHQ"
}

func (d *PointHQHandler) parseName(record dns.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, fmt.Sprintf(".%s.", d.root))

	return name
}

func (d *PointHQHandler) AddRecord(record dns.DnsRecord) error {
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

func (d *PointHQHandler) FindRecords(record dns.DnsRecord) ([]pointdns.Record, error) {
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

func (d *PointHQHandler) UpdateRecord(record dns.DnsRecord) error {
	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *PointHQHandler) RemoveRecord(record dns.DnsRecord) error {
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

func (d *PointHQHandler) GetRecords() ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord

	recordResp, err := d.zone.Records()
	if err != nil {
		return records, fmt.Errorf("PointHQ API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range recordResp {
		var fqdn string
		if rec.Name == "" {
			fqdn = dns.RootDomainName
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
			record := dns.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}
	return records, nil
}
