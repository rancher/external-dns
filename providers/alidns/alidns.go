package alidns

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	api "github.com/denverdino/aliyungo/dns"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type AlidnsProvider struct {
	client         *api.Client
	rootDomainName string
}

func init() {
	providers.RegisterProvider("alidns", &AlidnsProvider{})
}

func (a *AlidnsProvider) Init(rootDomainName string) error {
	var accessKey, secretKey string
	if accessKey = os.Getenv("ALICLOUD_ACCESS_KEY_ID"); len(accessKey) == 0 {
		return fmt.Errorf("ALICLOUD_ACCESS_KEY_ID is not set")
	}

	if secretKey = os.Getenv("ALICLOUD_ACCESS_KEY_SECRET"); len(secretKey) == 0 {
		return fmt.Errorf("ALICLOUD_ACCESS_KEY_SECRET is not set")
	}

	a.client = api.NewClient(accessKey, secretKey)
	a.rootDomainName = utils.UnFqdn(rootDomainName)

	var err error

	_, err = a.client.DescribeDomainInfo(&api.DescribeDomainInfoArgs{
		DomainName: a.rootDomainName,
	})
	if err != nil {
		return fmt.Errorf("Failed to describe root domain name for '%s': %v", a.rootDomainName, err)
	}

	logrus.Infof("Configured %s with zone '%s'", a.GetName(), a.rootDomainName)
	return nil
}

func (*AlidnsProvider) GetName() string {
	return "AliDNS"
}

func (a *AlidnsProvider) HealthCheck() error {
	_, err := a.client.DescribeDomainInfo(&api.DescribeDomainInfoArgs{
		DomainName: a.rootDomainName,
	})
	return err
}

func (a *AlidnsProvider) AddRecord(record utils.DnsRecord) error {
	for _, rec := range record.Records {
    r := a.prepareRecord(record, rec)
    _, err := a.client.AddDomainRecord(r)
		if err != nil {
			return fmt.Errorf("Alibaba Cloud API call has failed: %v", err)
		}
	}

	return nil
}

func (a *AlidnsProvider) UpdateRecord(record utils.DnsRecord) error {
	if err := a.RemoveRecord(record); err != nil {
		return err
	}

	return a.AddRecord(record)
}

func (a *AlidnsProvider) RemoveRecord(record utils.DnsRecord) error {
	records, err := a.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		_, err := a.client.DeleteDomainRecord(&api.DeleteDomainRecordArgs{
			RecordId: rec.RecordId,
    })
		if err != nil {
			return fmt.Errorf("Alibaba Cloud API call has failed: %v", err)
		}
	}

	return nil
}

func (a *AlidnsProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord
	result, err := a.client.DescribeDomainRecords(&api.DescribeDomainRecordsArgs{
		DomainName: a.rootDomainName,
	})
	if err != nil {
		return records, fmt.Errorf("Alibaba Cloud API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range result.DomainRecords.Record {
		var fqdn string
		if rec.RR == "" {
			fqdn = a.rootDomainName + "."
		} else {
			fqdn = fmt.Sprintf("%s.%s.", rec.RR, a.rootDomainName)
    }

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][rec.Type] = int(rec.TTL)
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[rec.Type]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Value)
				recordSet[rec.Type] = recordSlice
			} else {
				recordSet[rec.Type] = []string{rec.Value}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.Type] = []string{rec.Value}
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

func (a *AlidnsProvider) parseName(record utils.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, fmt.Sprintf(".%s.", a.rootDomainName))
	return name
}

func (a *AlidnsProvider) prepareRecord(record utils.DnsRecord, rec string) *api.AddDomainRecordArgs {
	name := a.parseName(record)
	return &api.AddDomainRecordArgs{
		DomainName: a.rootDomainName,
		RR:         name,
		Type:       record.Type,
		Value:      rec,
		TTL:        int32(record.TTL),
	}
}

func (a *AlidnsProvider) findRecords(record utils.DnsRecord) ([]api.RecordType, error) {
	var records []api.RecordType
	result, err := a.client.DescribeDomainRecords(&api.DescribeDomainRecordsArgs{
		DomainName: a.rootDomainName,
	})
	if err != nil {
		return records, fmt.Errorf("Alibaba Cloud API call has failed: %v", err)
	}

  name := a.parseName(record)
	for _, rec := range result.DomainRecords.Record {
		if rec.RR == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}
