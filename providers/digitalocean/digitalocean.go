package digitalocean

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"

	"github.com/juju/ratelimit"
	"github.com/mathuin/external-dns/providers"
	"github.com/mathuin/external-dns/utils"
)

var DigitalOceanMaxRetries int = 4

type DigitalOceanProvider struct {
	client         *godo.Client
	rootDomainName string
	TTL            int
	limiter        *ratelimit.Bucket
	records        []godo.DomainRecord
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

func (p *DigitalOceanProvider) Init(rootDomainName string) error {
	var pat string
	if pat = os.Getenv("DO_PAT"); len(pat) == 0 {
		return fmt.Errorf("DO_PAT is not set")
	}

	tokenSource := &TokenSource{
		AccessToken: pat,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	p.client = godo.NewClient(oauthClient)

	// DO's API is rate limited at 5000/hour.
	doqps := (float64)(5000.0 / 3600.0)
	p.limiter = ratelimit.NewBucketWithRate(doqps, 1)

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
	p.TTL = domains.TTL
	p.records, err = p.getAllRecords()
	if err != nil {
		return err
	}

	logrus.Infof("Configured %s for email %s and domain %s", p.GetName(), acct.Email, domains.Name)
	logrus.Debugf("Existing records:")
	for _, r := range p.records {
		logrus.Debugf(" %s %s %s", r.Name, r.Type, r.Data)
	}

	return nil
}

func (p *DigitalOceanProvider) GetName() string {
	return "Digital Ocean"
}

func (p *DigitalOceanProvider) HealthCheck() error {
	p.limiter.Wait(1)
	_, _, err := p.client.Domains.Get(p.rootDomainName)
	return err
}

func (p *DigitalOceanProvider) AddRecord(record utils.DnsRecord) error {
	logrus.Debugf("AddRecord")
	for _, r := range record.Records {
		createRequest := &godo.DomainRecordEditRequest{
			Type: record.Type,
			Name: record.Fqdn,
			Data: r,
		}
		logrus.Debugf(" request: %v", createRequest)
		p.limiter.Wait(1)
		rec, _, err := p.client.Domains.CreateRecord(p.rootDomainName, createRequest)
		if err != nil {
			return fmt.Errorf("%s API call has failed: %v", p.GetName(), err)
		}
		logrus.Debugf(" rec: %v", rec)
		rec.Name = p.qualifyName(rec.Name)
		p.records = append(p.records, *rec)
	}
	return nil
}

func (p *DigitalOceanProvider) UpdateRecord(record utils.DnsRecord) error {
	logrus.Debugf("UpdateRecord")
	if err := p.RemoveRecord(record); err != nil {
		return err
	}
	return p.AddRecord(record)
}

func (p *DigitalOceanProvider) RemoveRecord(record utils.DnsRecord) error {
	logrus.Debugf("RemoveRecord")
	records := p.getRecords(record)
	if len(records) == 0 {
		return fmt.Errorf("No such record exists")
	}
	for _, rec := range p.getRecords(record) {
		p.limiter.Wait(1)
		_, err := p.client.Domains.DeleteRecord(p.rootDomainName, rec.ID)
		if err != nil {
			return fmt.Errorf("%s API call has failed: %v", p.GetName(), err)
		}

		rnum := p.recordIndex(rec)
		p.records[rnum] = p.records[len(p.records)-1]
		p.records[len(p.records)-1] = godo.DomainRecord{}
		p.records = p.records[:len(p.records)-1]
	}
	return nil
}

func (p *DigitalOceanProvider) GetRecords() ([]utils.DnsRecord, error) {
	dnsRecords := []utils.DnsRecord{}

	recordMap := map[string]map[string][]string{}

	logrus.Debugf("GetRecords")
	for _, r := range p.records {
		logrus.Debugf(" %s %s %s", r.Name, r.Type, r.Data)
		fqdn := utils.Fqdn(r.Name)
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[r.Type]
			if sliceExists {
				recordSlice = append(recordSlice, r.Data)
				recordSet[r.Type] = recordSlice
			} else {
				recordSet[r.Type] = []string{r.Data}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][r.Type] = []string{r.Data}
		}
	}

	logrus.Debugf("recordSet")
	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			// Digital Ocean does not have per-record TTLs.
			dnsRecord := utils.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: p.TTL}
			logrus.Debugf(" %v", dnsRecord)
			dnsRecords = append(dnsRecords, dnsRecord)
		}
	}

	return dnsRecords, nil
}

// For now, this enforces a single fqdn-type match.
func (p *DigitalOceanProvider) getRecords(record utils.DnsRecord) []godo.DomainRecord {
	var gdrs []godo.DomainRecord

	for _, prec := range p.records {
		if prec.Name == record.Fqdn && prec.Type == record.Type {
			for _, urec := range record.Records {
				if urec == "" || urec == prec.Data || true {
					gdrs = append(gdrs, prec)
				}
			}
		}
	}
	logrus.Debugf("%d records", len(gdrs))
	for _, rec := range gdrs {
		logrus.Debugf(" record: %v", rec)
	}
	return gdrs
}

func (p *DigitalOceanProvider) recordIndex(rec godo.DomainRecord) int {
	for i, r := range p.records {
		if r == rec {
			return i
		}
	}
	return -1
}

func (p *DigitalOceanProvider) getAllRecords() ([]godo.DomainRecord, error) {
	logrus.Debugf("getAllRecords")
	records := []godo.DomainRecord{}
	opt := &godo.ListOptions{}
	for {
		p.limiter.Wait(1)
		drecords, resp, err := p.client.Domains.Records(p.rootDomainName, opt)
		if err != nil {
			return nil, fmt.Errorf("%s API call has failed: %v", p.GetName(), err)
		}
		for _, r := range drecords {
			if r.Name == "@" {
				logrus.Debugf("caught @")
				r.Name = p.rootDomainName
			} else {
				r.Name = p.qualifyName(r.Name)
			}
			r.Name = utils.Fqdn(r.Name)
			records = append(records, r)
		}
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, fmt.Errorf("%s API call has failed: %v", p.GetName(), err)
		}
		opt.Page = page + 1
	}
	logrus.Debugf("%d records", len(records))
	for _, rec := range records {
		logrus.Debugf(" record: %v", rec)
	}
	return records, nil
}

func (p *DigitalOceanProvider) qualifyName(name string) string {
	logrus.Debugf("qualifyName: %s", name)
	n := len(name)
	if name[n-1] != '.' {
		names := []string{name, p.rootDomainName}
		name = strings.Join(names, ".")
		logrus.Debugf("new name: %s", name)
	}
	return name
}
