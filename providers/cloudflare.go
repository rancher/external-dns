package providers

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/crackcomm/cloudflare"
	"github.com/rancher/external-dns/dns"
	"golang.org/x/net/context"
)

type CloudflareHandler struct {
	client *cloudflare.Client
	zone   *cloudflare.Zone
	ctx    context.Context
	root   string
}

func init() {
	cloudflareHandler := &CloudflareHandler{}

	email := os.Getenv("CLOUDFLARE_EMAIL")
	if len(email) == 0 {
		logrus.Infof("CLOUDFLARE_EMAIL is not set, skipping init of %s provider", cloudflareHandler.GetName())
		return
	}

	apiKey := os.Getenv("CLOUDFLARE_KEY")
	if len(apiKey) == 0 {
		logrus.Infof("CLOUDFLARE_KEY is not set, skipping init of %s provider", cloudflareHandler.GetName())
		return
	}

	if err := RegisterProvider("cloudflare", cloudflareHandler); err != nil {
		logrus.Fatal("Could not register cloudflare provider")
	}

	dns.SetRootDomain(getDefaultRootDomain())

	cloudflareHandler.client = cloudflare.New(&cloudflare.Options{
		Email: email,
		Key:   apiKey,
	})

	cloudflareHandler.ctx = context.Background()
	cloudflareHandler.root = unFqdn(dns.RootDomainName)

	if err := cloudflareHandler.setZone(); err != nil {
		logrus.Fatalf("Failed to set zone for root domain %s: %v", cloudflareHandler.root, err)
	}

	logrus.Infof("Configured %s with zone \"%s\" ", cloudflareHandler.GetName(), cloudflareHandler.root)
}

func (*CloudflareHandler) GetName() string {
	return "CloudFlare"
}

func (*CloudflareHandler) TestConnection() error {
	return nil
}

func (*CloudflareHandler) GetRootDomain() string {
	return getDefaultRootDomain()
}

func (c *CloudflareHandler) AddRecord(record dns.DnsRecord) error {
	for _, rec := range record.Records {
		r := c.prepareRecord(record)
		r.Content = rec
		err := c.client.Records.Create(c.ctx, r)
		if err != nil {
			return fmt.Errorf("CloudFlare API call has failed: %v", err)
		}
	}

	return nil
}

func (c *CloudflareHandler) UpdateRecord(record dns.DnsRecord) error {
	if err := c.RemoveRecord(record); err != nil {
		return err
	}

	return c.AddRecord(record)
}

func (c *CloudflareHandler) RemoveRecord(record dns.DnsRecord) error {
	records, err := c.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		err := c.client.Records.Delete(c.ctx, c.zone.ID, rec.ID)
		if err != nil {
			return fmt.Errorf("CloudFlare API call has failed: %v", err)
		}
	}

	return nil
}

func (c *CloudflareHandler) GetRecords(listOpts ...string) ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord
	result, err := c.client.Records.List(c.ctx, c.zone.ID)
	if err != nil {
		return records, fmt.Errorf("CloudFlare API call has failed: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range result {
		fqdn := fqdn(rec.Name)
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
			record := dns.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}
	return records, nil
}

func (c *CloudflareHandler) setZone() error {
	zones, err := c.client.Zones.List(c.ctx)
	if err != nil {
		return fmt.Errorf("CloudFlare API call has failed: %v", err)
	}

	for _, zone := range zones {
		if zone.Name == c.root {
			c.zone = zone
			break
		}
	}
	if c.zone == nil {
		return fmt.Errorf("Zone %s does not exist", c.root)
	}

	return nil
}

func (c *CloudflareHandler) prepareRecord(record dns.DnsRecord) *cloudflare.Record {
	name := unFqdn(record.Fqdn)
	return &cloudflare.Record{
		Type:   record.Type,
		Name:   name,
		TTL:    sanitizeTTL(record.TTL),
		ZoneID: c.zone.ID,
	}
}

func (c *CloudflareHandler) findRecords(record dns.DnsRecord) ([]*cloudflare.Record, error) {
	var records []*cloudflare.Record
	result, err := c.client.Records.List(c.ctx, c.zone.ID)
	if err != nil {
		return records, fmt.Errorf("CloudFlare API call has failed: %v", err)
	}

	name := unFqdn(record.Fqdn)
	for _, rec := range result {
		if rec.Name == name && rec.Type == record.Type {
			records = append(records, rec)
		}
	}

	return records, nil
}

// TTL must be between 120 and 86400 seconds
func sanitizeTTL(ttl int) int {
	if ttl < 120 {
		ttl = 120
	} else if ttl > 86400 {
		ttl = 86400
	}
	return ttl
}

func fqdn(name string) string {
	n := len(name)
	if n == 0 || name[n-1] == '.' {
		return name
	}
	return name + "."
}

func unFqdn(name string) string {
	n := len(name)
	if n != 0 && name[n-1] == '.' {
		return name[:n-1]
	}
	return name
}
