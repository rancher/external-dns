package OVH

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	api "github.com/ovh/go-ovh/ovh"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type Record struct {
	Target    string `json:"target"`
	TTL       int64  `json:"ttl"`
	Zone      string `json:"zone"`
	FieldType string `json:"fieldType"`
	ID        int64  `json:"id"`
	SubDomain string `json:"subDomain"`
}

type OVHProvider struct {
	client *api.Client
	root   string
}

func init() {
	providers.RegisterProvider("ovh", &OVHProvider{})
}

func (d *OVHProvider) Init(rootDomainName string) error {
	var endpoint, applicationKey, applicationSecret, consumerKey string
	if endpoint = os.Getenv("OVH_ENDPOINT"); len(endpoint) == 0 {
		return fmt.Errorf("OVH_ENDPOINT is not set")
	}

	if applicationKey = os.Getenv("OVH_APPLICATION_KEY"); len(applicationKey) == 0 {
		return fmt.Errorf("OVH_APPLICATION_KEY is not set")
	}

	if applicationSecret = os.Getenv("OVH_APPLICATION_SECRET"); len(applicationSecret) == 0 {
		return fmt.Errorf("OVH_APPLICATION_SECRET is not set")
	}

	if consumerKey = os.Getenv("OVH_CONSUMER_KEY"); len(consumerKey) == 0 {
		return fmt.Errorf("OVH_CONSUMER_KEY is not set")
	}

	d.root = utils.UnFqdn(rootDomainName)
	client, err := api.NewClient(endpoint, applicationKey, applicationSecret, consumerKey)
	if err != nil {
		return fmt.Errorf("Failed to create OVH client: %v", err)
	}

	d.client = client

	var zones []string
	err = d.client.Get("/domain/zone", &zones)

	if err != nil {
		return fmt.Errorf("Failed to list hosted zones: %v", err)
	}

	found := false
	for _, zone := range zones {
		if zone == d.root {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("Zone for '%s' not found", d.root)
	}

	logrus.Infof("Configured %s with zone '%s'", d.GetName(), d.root)
	return nil
}

func (*OVHProvider) GetName() string {
	return "OVH"
}

func (d *OVHProvider) HealthCheck() error {
	var me interface{}
	err := d.client.Get("/me", &me)
	return err
}

func (d *OVHProvider) parseName(record utils.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, fmt.Sprintf(".%s.", d.root))
	return name
}

func (d *OVHProvider) AddRecord(record utils.DnsRecord) (err error) {
	var url string
	var body interface{}
	var resType interface{}
	for _, rec := range record.Records {
		if url, body, err = d.prepareRecord(rec, record.Type, record.Fqdn, record.TTL); err != nil {
			return err
		}
		if err = d.client.Post(url, body, resType); err != nil {
			return fmt.Errorf("OVH API call `POST %s` with body `%s` has failed: %v", url, body, err)
		}
	}
	d.refreshZone()
	return nil
}

func (d *OVHProvider) FindRecords(record utils.DnsRecord) ([]*Record, error) {
	var records []*Record

	urlRecIDs := strings.Join([]string{"/domain/zone/", d.root, "/record"}, "")

	var recIDs []int64
	err := d.client.Get(urlRecIDs, &recIDs)

	if err != nil {
		return records, fmt.Errorf("OVH API call `GET %s` has failed: %v", urlRecIDs, err)
	}

	name := d.parseName(record)
	for _, recID := range recIDs {
		urlRecord := strings.Join([]string{"/domain/zone/", d.root, "/record/", strconv.FormatInt(recID, 10)}, "")
		var rec *Record
		if err = d.client.Get(urlRecord, &rec); err != nil {
			return records, fmt.Errorf("OVH API call `GET %s` has failed: %v", urlRecord, err)
		}

		if rec.SubDomain == name && rec.FieldType == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

func (d *OVHProvider) UpdateRecord(record utils.DnsRecord) error {
	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *OVHProvider) RemoveRecord(record utils.DnsRecord) error {
	records, err := d.FindRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		urlRecord := strings.Join([]string{"/domain/zone/", d.root, "/record/", strconv.FormatInt(rec.ID, 10)}, "")
		var resType interface{}
		if err := d.client.Delete(urlRecord, &resType); err != nil {
			return fmt.Errorf("OVH API call `DELETE %s` has failed: %v", urlRecord, err)
		}
	}

	d.refreshZone()

	return nil
}

func (d *OVHProvider) GetRecords() ([]utils.DnsRecord, error) {
	var dnsRecords []utils.DnsRecord

	urlRecIDs := strings.Join([]string{"/domain/zone/", d.root, "/record"}, "")

	var recIDs []int64
	err := d.client.Get(urlRecIDs, &recIDs)

	if err != nil {
		return dnsRecords, fmt.Errorf("OVH API call `GET %s` has failed: %v", urlRecIDs, err)
	}

	var records []*Record
	for _, recID := range recIDs {
		urlRecord := strings.Join([]string{"/domain/zone/", d.root, "/record/", strconv.FormatInt(recID, 10)}, "")
		var record *Record
		if err = d.client.Get(urlRecord, &record); err != nil {
			return dnsRecords, fmt.Errorf("OVH API call `GET %s` has failed: %v", urlRecord, err)
		}
		records = append(records, record)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range records {
		var fqdn string

		if rec.SubDomain == "" {
			fqdn = fmt.Sprintf("%s.", rec.Zone)
		} else {
			fqdn = fmt.Sprintf("%s.%s.", rec.SubDomain, rec.Zone)
		}

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][rec.FieldType] = int(rec.TTL)
		recordSet, exists := recordMap[fqdn]

		if exists {
			recordSlice, sliceExists := recordSet[rec.FieldType]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Target)
				recordSet[rec.FieldType] = recordSlice
			} else {
				recordSet[rec.FieldType] = []string{rec.Target}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.FieldType] = []string{rec.Target}
		}
	}

	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			ttl := recordTTLs[fqdn][recordType]
			record := utils.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			dnsRecords = append(dnsRecords, record)
		}
	}

	return dnsRecords, nil
}

func (d *OVHProvider) prepareRecord(rec string, tp string, fqdn string, ttl int) (string, interface{}, error) {
	var url string
	url = strings.Join([]string{"/domain/zone/", d.root, "/record"}, "")
	body := make(map[string]interface{})
	name := strings.TrimSuffix(fqdn, fmt.Sprintf(".%s.", d.root))
	body["fieldType"] = tp
	body["subDomain"] = name
	body["ttl"] = ttl
	body["target"] = rec
	return url, body, nil
}

func (d *OVHProvider) refreshZone() error {
	url := strings.Join([]string{"/domain/zone/", d.root, "/refresh"}, "")
	var resType interface{}
	if err := d.client.Post(url, nil, &resType); err != nil {
		return fmt.Errorf("OVH API call `POST %s` has failed: %v", url, err)
	}
	return nil
}
