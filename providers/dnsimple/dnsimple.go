package dnsimple

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/dnsimple/dnsimple-go/dnsimple"
	"github.com/juju/ratelimit"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type DNSimpleProvider struct {
	client    *dnsimple.Client
	accountID string
	root      string
	limiter   *ratelimit.Bucket
}

func init() {
	providers.RegisterProvider("dnsimple", &DNSimpleProvider{})
}

func (d *DNSimpleProvider) Init(rootDomainName string) error {
	var oauthToken string

	if len(os.Getenv("DNSIMPLE_EMAIL")) > 0 {
		return fmt.Errorf("DNSimple API v2 requires an account identifier and the new OAuth token. Please upgrade your configuration.")
	}

	if oauthToken = os.Getenv("DNSIMPLE_TOKEN"); len(oauthToken) == 0 {
		return fmt.Errorf("DNSIMPLE_TOKEN is not set")
	}

	d.root = utils.UnFqdn(rootDomainName)
	d.client = dnsimple.NewClient(dnsimple.NewOauthTokenCredentials(oauthToken))
	d.limiter = ratelimit.NewBucketWithRate(1.5, 5)

	whoamiResponse, err := d.client.Identity.Whoami()
	if err != nil {
		return fmt.Errorf("DNSimple Authentication failed: %v", err)
	}
	if whoamiResponse.Data.Account == nil {
		return fmt.Errorf("DNSimple User tokens are not supported, use an Account token")
	}
	d.accountID = strconv.Itoa(whoamiResponse.Data.Account.ID)

	_, err = d.client.Zones.GetZone(d.accountID, d.root)
	if err != nil {
		return fmt.Errorf("Failed to get zone for '%s': %v", d.root)
	}

	logrus.Infof("Configured %s with zone '%s'", d.GetName(), d.root)
	return nil
}

func (*DNSimpleProvider) GetName() string {
	return "DNSimple"
}

func (d *DNSimpleProvider) HealthCheck() error {
	d.limiter.Wait(1)
	_, err := d.client.Identity.Whoami()
	return err
}

func (d *DNSimpleProvider) parseName(record utils.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, fmt.Sprintf(".%s.", d.root))
	return name
}

func (d *DNSimpleProvider) AddRecord(record utils.DnsRecord) error {
	name := d.parseName(record)
	for _, rec := range record.Records {
		recordInput := dnsimple.ZoneRecord{
			Name:    name,
			TTL:     record.TTL,
			Type:    record.Type,
			Content: rec,
		}
		d.limiter.Wait(1)
		_, err := d.client.Zones.CreateRecord(d.accountID, d.root, recordInput)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleProvider) findRecords(record utils.DnsRecord) ([]dnsimple.ZoneRecord, error) {
	var zoneRecords []dnsimple.ZoneRecord

	d.limiter.Wait(1)
	recordsResponse, err := d.client.Zones.ListRecords(d.accountID, d.root, nil)
	if err != nil {
		return zoneRecords, fmt.Errorf("DNSimple API call has failed: %v", err)
	}

	name := d.parseName(record)
	for _, zoneRecord := range recordsResponse.Data {
		if zoneRecord.Name == name && zoneRecord.Type == record.Type {
			zoneRecords = append(zoneRecords, zoneRecord)
		}
	}

	return zoneRecords, nil
}

func (d *DNSimpleProvider) UpdateRecord(record utils.DnsRecord) error {
	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *DNSimpleProvider) RemoveRecord(record utils.DnsRecord) error {
	zoneRecords, err := d.findRecords(record)
	if err != nil {
		return err
	}

	for _, zoneRecord := range zoneRecords {
		d.limiter.Wait(1)
		_, err := d.client.Zones.DeleteRecord(d.accountID, d.root, zoneRecord.ID)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord

	d.limiter.Wait(1)
	recordsResponse, err := d.client.Zones.ListRecords(d.accountID, d.root, nil)
	if err != nil {
		return records, fmt.Errorf("DNSimple API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, zoneRecord := range recordsResponse.Data {
		var fqdn string
		if zoneRecord.Name == "" {
			fqdn = d.root + "."
		} else {
			fqdn = fmt.Sprintf("%s.%s.", zoneRecord.Name, d.root)
		}

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][zoneRecord.Type] = zoneRecord.TTL
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[zoneRecord.Type]
			if sliceExists {
				recordSlice = append(recordSlice, zoneRecord.Content)
				recordSet[zoneRecord.Type] = recordSlice
			} else {
				recordSet[zoneRecord.Type] = []string{zoneRecord.Content}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][zoneRecord.Type] = []string{zoneRecord.Content}
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
