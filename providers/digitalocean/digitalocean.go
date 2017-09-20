package digitalocean

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	api "github.com/digitalocean/godo"
	"golang.org/x/oauth2"

	"github.com/juju/ratelimit"
	"github.com/rancher/external-dns/config"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type DigitalOceanProvider struct {
	client         *api.Client
	rootDomainName string
	limiter        *ratelimit.Bucket
}

func init() {
	providers.RegisterProvider("digitalocean", &DigitalOceanProvider{})
}

type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

func (p *DigitalOceanProvider) Init() error {
	rootDomainName := utils.GetDefaultRootDomain()
	var pat string
	if pat = os.Getenv("DO_PAT"); len(pat) == 0 {
		return fmt.Errorf("DO_PAT is not set")
	}

	tokenSource := &TokenSource{
		AccessToken: pat,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	p.client = api.NewClient(oauthClient)

	// DO's API is rate limited at 5000/hour.
	doqps := (float64)(5000.0 / 3600.0)
	p.limiter = ratelimit.NewBucketWithRate(doqps, 100)

	p.rootDomainName = utils.UnFqdn(rootDomainName)

	// Retrieve email address associated with this PAT.
	p.limiter.Wait(1)
	acct, _, err := p.client.Account.Get()
	if err != nil {
		return err
	}

	// Now confirm that domain is accessible under this PAT.
	p.limiter.Wait(1)
	domains, _, err := p.client.Domains.Get(p.rootDomainName)
	if err != nil {
		return err
	}

	// DO's TTLs are domain-wide.
	config.TTL = domains.TTL
	logrus.Infof("Configured %s with email %s and domain %s", p.GetName(), acct.Email, domains.Name)
	return nil
}

func (p *DigitalOceanProvider) GetName() string {
	return "DigitalOcean"
}

func (*DigitalOceanProvider) GetRootDomain() string {
	return utils.GetDefaultRootDomain()
}

func (p *DigitalOceanProvider) HealthCheck() error {
	p.limiter.Wait(1)
	_, _, err := p.client.Domains.Get(p.rootDomainName)
	return err
}

func (p *DigitalOceanProvider) AddRecord(record utils.DnsRecord) error {
	for _, r := range record.Records {
		createRequest := &api.DomainRecordEditRequest{
			Type: record.Type,
			Name: record.Fqdn,
			Data: r,
		}

		logrus.Debugf("Creating record: %v", createRequest)
		p.limiter.Wait(1)
		_, _, err := p.client.Domains.CreateRecord(p.rootDomainName, createRequest)
		if err != nil {
			return fmt.Errorf("API call has failed: %v", err)
		}
	}

	return nil
}

func (p *DigitalOceanProvider) UpdateRecord(record utils.DnsRecord) error {
	if err := p.RemoveRecord(record); err != nil {
		return err
	}

	return p.AddRecord(record)
}

func (p *DigitalOceanProvider) RemoveRecord(record utils.DnsRecord) error {
	// We need to fetch paginated results to get all records
	doRecords, err := p.fetchDoRecords()
	if err != nil {
		return fmt.Errorf("RemoveRecord: %v", err)
	}

	for _, rec := range doRecords {
		// DO records don't have fully-qualified names like ours
		fqdn := p.nameToFqdn(rec.Name)
		if fqdn == record.Fqdn && rec.Type == record.Type {
			p.limiter.Wait(1)
			logrus.Debugf("Deleting record: %v", rec)
			_, err := p.client.Domains.DeleteRecord(p.rootDomainName, rec.ID)
			if err != nil {
				return fmt.Errorf("API call has failed: %v", err)
			}
		}
	}

	return nil
}

func (p *DigitalOceanProvider) GetRecords() ([]utils.DnsRecord, error) {
	dnsRecords := []utils.DnsRecord{}
	recordMap := map[string]map[string][]string{}
	doRecords, err := p.fetchDoRecords()
	if err != nil {
		return nil, fmt.Errorf("GetRecords: %v", err)
	}

	for _, rec := range doRecords {
		fqdn := p.nameToFqdn(rec.Name)
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[rec.Type]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Data)
				recordSet[rec.Type] = recordSlice
			} else {
				recordSet[rec.Type] = []string{rec.Data}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.Type] = []string{rec.Data}
		}
	}

	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			// DigitalOcean does not have per-record TTLs.
			dnsRecord := utils.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: config.TTL}
			dnsRecords = append(dnsRecords, dnsRecord)
		}
	}

	return dnsRecords, nil
}

// fetchDoRecords retrieves all records for the root domain from Digital Ocean.
func (p *DigitalOceanProvider) fetchDoRecords() ([]api.DomainRecord, error) {
	doRecords := []api.DomainRecord{}
	opt := &api.ListOptions{
		// Use the maximum of 200 records per page
		PerPage: 200,
	}
	for {
		p.limiter.Wait(1)
		records, resp, err := p.client.Domains.Records(p.rootDomainName, opt)
		if err != nil {
			return nil, fmt.Errorf("API call has failed: %v", err)
		}

		if len(records) > 0 {
			doRecords = append(doRecords, records...)
		}

		if resp.Links == nil || resp.Links.IsLastPage() || len(records) == 0 {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, fmt.Errorf("Failed to get current page: %v", err)
		}

		opt.Page = page + 1
	}

	logrus.Debugf("Fetched %d DO records", len(doRecords))
	return doRecords, nil
}

func (p *DigitalOceanProvider) nameToFqdn(name string) string {
	var fqdn string
	if name == "@" {
		fqdn = p.rootDomainName
	} else {
		names := []string{name, p.rootDomainName}
		fqdn = strings.Join(names, ".")
	}

	return utils.Fqdn(fqdn)
}
