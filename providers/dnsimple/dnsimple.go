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
	api "github.com/weppos/go-dnsimple/dnsimple"
)

type DNSimpleProvider struct {
	client    *api.Client
	client2   *dnsimple.Client
	accountID string
	root      string
	limiter   *ratelimit.Bucket
}

func init() {
	providers.RegisterProvider("dnsimple", &DNSimpleProvider{})
}

func (d *DNSimpleProvider) Init(rootDomainName string) error {
	var email, apiToken, oauthToken string

	if email = os.Getenv("DNSIMPLE_EMAIL"); len(email) == 0 {
		return fmt.Errorf("DNSIMPLE_EMAIL is not set")
	}

	if apiToken = os.Getenv("DNSIMPLE_TOKEN"); len(apiToken) == 0 {
		return fmt.Errorf("DNSIMPLE_TOKEN is not set")
	}

	if oauthToken = os.Getenv("DNSIMPLE_TOKEN"); len(oauthToken) == 0 {
		return fmt.Errorf("DNSIMPLE_TOKEN is not set")
	}

	d.root = utils.UnFqdn(rootDomainName)
	d.client = api.NewClient(apiToken, email)
	d.client2 = dnsimple.NewClient(dnsimple.NewOauthTokenCredentials(oauthToken))
	d.limiter = ratelimit.NewBucketWithRate(1.5, 5)

	whoamiResponse, err := d.client2.Identity.Whoami()
	if err != nil {
		return fmt.Errorf("DNSimple Authentication failed: %v", err)
	}
	if whoamiResponse.Data.Account == nil {
		return fmt.Errorf("DNSimple User tokens are not supported, use an Account token")
	}
	d.accountID = strconv.Itoa(whoamiResponse.Data.Account.ID)

	_, err = d.client2.Zones.GetZone(d.accountID, d.root)
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
	_, err := d.client2.Identity.Whoami()
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
		_, err := d.client2.Zones.CreateRecord(d.accountID, d.root, recordInput)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleProvider) findRecords(record utils.DnsRecord) ([]api.Record, error) {
	var records []api.Record

	d.limiter.Wait(1)
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

func (d *DNSimpleProvider) UpdateRecord(record utils.DnsRecord) error {
	err := d.RemoveRecord(record)
	if err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *DNSimpleProvider) RemoveRecord(record utils.DnsRecord) error {
	records, err := d.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		d.limiter.Wait(1)
		_, err := d.client2.Zones.DeleteRecord(d.accountID, d.root, rec.Id)
		if err != nil {
			return fmt.Errorf("DNSimple API call has failed: %v", err)
		}
	}

	return nil
}

func (d *DNSimpleProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord

	d.limiter.Wait(1)
	recordResp, _, err := d.client.Domains.ListRecords(d.root, "", "")
	if err != nil {
		return records, fmt.Errorf("DNSimple API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range recordResp {
		var fqdn string
		if rec.Name == "" {
			fqdn = d.root + "."
		} else {
			fqdn = fmt.Sprintf("%s.%s.", rec.Name, d.root)
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
			record := utils.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}
	return records, nil
}
