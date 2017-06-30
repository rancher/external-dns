package rfc2136

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

type RFC2136Provider struct {
	nameserver  string
	zoneName    string
	tsigKeyName string
	tsigSecret  string
	insecure    bool
}

func init() {
	providers.RegisterProvider("rfc2136", &RFC2136Provider{})
}

func (r *RFC2136Provider) Init(rootDomainName string) error {
	var host, port, keyName, secret string
	var insecure bool
	var err error

	if host = os.Getenv("RFC2136_HOST"); len(host) == 0 {
		return fmt.Errorf("RFC2136_HOST is not set")
	}

	if port = os.Getenv("RFC2136_PORT"); len(port) == 0 {
		return fmt.Errorf("RFC2136_PORT is not set")
	}

	if insecureStr := os.Getenv("RFC2136_INSECURE"); len(insecureStr) == 0 {
		insecure = false
	} else {
		if insecure, err = strconv.ParseBool(insecureStr); err != nil {
			return fmt.Errorf("RFC2136_INSECURE must be a boolean value")
		}
	}

	if !insecure {
		if keyName = os.Getenv("RFC2136_TSIG_KEYNAME"); len(keyName) == 0 {
			return fmt.Errorf("RFC2136_TSIG_KEYNAME is not set")
		}

		if secret = os.Getenv("RFC2136_TSIG_SECRET"); len(secret) == 0 {
			return fmt.Errorf("RFC2136_TSIG_SECRET is not set")
		}
	}

	r.nameserver = net.JoinHostPort(host, port)
	r.zoneName = dns.Fqdn(rootDomainName)
	if !insecure {
		r.tsigKeyName = dns.Fqdn(keyName)
		r.tsigSecret = secret
	}

	r.insecure = insecure

	logrus.Infof("Configured %s with zone '%s' and nameserver '%s'",
		r.GetName(), r.zoneName, r.nameserver)

	return nil
}

func (*RFC2136Provider) GetName() string {
	return "RFC2136"
}

func (r *RFC2136Provider) HealthCheck() error {
	m := new(dns.Msg)
	m.SetQuestion(r.zoneName, dns.TypeSOA)
	err := r.sendMessage(m)
	if err != nil {
		return fmt.Errorf("Failed to query zone SOA record: %v", err)
	}

	return nil
}

func (r *RFC2136Provider) AddRecord(record utils.DnsRecord) error {
	logrus.Debugf("Adding RRset '%s %s'", record.Fqdn, record.Type)
	m := new(dns.Msg)
	m.SetUpdate(r.zoneName)
	rrs := make([]dns.RR, 0)
	for _, rec := range record.Records {
		logrus.Debugf("Adding RR: '%s %d %s %s'", record.Fqdn, record.TTL, record.Type, rec)
		rr, err := dns.NewRR(fmt.Sprintf("%s %d %s %s", record.Fqdn, record.TTL, record.Type, rec))
		if err != nil {
			return fmt.Errorf("Failed to build RR: %v", err)
		}
		rrs = append(rrs, rr)
	}

	m.Insert(rrs)
	err := r.sendMessage(m)
	if err != nil {
		return fmt.Errorf("RFC2136 query failed: %v", err)
	}

	return nil
}

func (r *RFC2136Provider) RemoveRecord(record utils.DnsRecord) error {
	logrus.Debugf("Removing RRset '%s %s'", record.Fqdn, record.Type)
	m := new(dns.Msg)
	m.SetUpdate(r.zoneName)
	rr, err := dns.NewRR(fmt.Sprintf("%s 0 %s 0.0.0.0", record.Fqdn, record.Type))
	if err != nil {
		return fmt.Errorf("Could not construct RR: %v", err)
	}

	rrs := make([]dns.RR, 1)
	rrs[0] = rr
	m.RemoveRRset(rrs)
	err = r.sendMessage(m)
	if err != nil {
		return fmt.Errorf("RFC2136 query failed: %v", err)
	}

	return nil
}

func (r *RFC2136Provider) UpdateRecord(record utils.DnsRecord) error {
	err := r.RemoveRecord(record)
	if err != nil {
		return err
	}

	return r.AddRecord(record)
}

func (r *RFC2136Provider) GetRecords() ([]utils.DnsRecord, error) {
	records := make([]utils.DnsRecord, 0)
	list, err := r.list()
	if err != nil {
		return records, err
	}

OuterLoop:
	for _, rr := range list {
		if rr.Header().Class != dns.ClassINET {
			continue
		}

		rrFqdn := rr.Header().Name
		rrTTL := int(rr.Header().Ttl)
		var rrType string
		var rrValues []string
		switch rr.Header().Rrtype {
		case dns.TypeCNAME:
			rrValues = []string{rr.(*dns.CNAME).Target}
			rrType = "CNAME"
		case dns.TypeA:
			rrValues = []string{rr.(*dns.A).A.String()}
			rrType = "A"
		case dns.TypeAAAA:
			rrValues = []string{rr.(*dns.AAAA).AAAA.String()}
			rrType = "AAAA"
		case dns.TypeTXT:
			rrValues = rr.(*dns.TXT).Txt
			rrType = "TXT"
		default:
			continue // Unhandled record type
		}

		for idx, existingRecord := range records {
			if existingRecord.Fqdn == rrFqdn && existingRecord.Type == rrType {
				records[idx].Records = append(records[idx].Records, rrValues...)
				continue OuterLoop
			}
		}

		record := utils.DnsRecord{
			Fqdn:    rrFqdn,
			Type:    rrType,
			TTL:     rrTTL,
			Records: rrValues,
		}

		records = append(records, record)
	}

	return records, nil
}

func (r *RFC2136Provider) sendMessage(msg *dns.Msg) error {
	c := new(dns.Client)
	c.SingleInflight = true

	if !r.insecure {
		c.TsigSecret = map[string]string{r.tsigKeyName: r.tsigSecret}
		msg.SetTsig(r.tsigKeyName, dns.HmacMD5, 300, time.Now().Unix())
	}

	resp, _, err := c.Exchange(msg, r.nameserver)
	if err != nil {
		return err
	}

	if resp != nil && resp.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("Bad return code: %s", dns.RcodeToString[resp.Rcode])
	}

	return nil
}

func (r *RFC2136Provider) list() ([]dns.RR, error) {
	logrus.Debugf("Fetching records for '%s'", r.zoneName)
	t := new(dns.Transfer)
	if !r.insecure {
		t.TsigSecret = map[string]string{r.tsigKeyName: r.tsigSecret}
	}

	m := new(dns.Msg)
	m.SetAxfr(r.zoneName)
	if !r.insecure {
		m.SetTsig(r.tsigKeyName, dns.HmacMD5, 300, time.Now().Unix())
	}

	env, err := t.In(m, r.nameserver)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch records via AXFR: %v", err)
	}

	records := make([]dns.RR, 0)
	for e := range env {
		if e.Error != nil {
			if e.Error == dns.ErrSoa {
				logrus.Error("AXFR error: unexpected response received from the server")
			} else {
				logrus.Errorf("AXFR error: %v", e.Error)
			}
			continue
		}
		records = append(records, e.RR...)
	}

	return records, nil
}
