package rfc2136

import (
	"fmt"
	"net"
	"os"
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
}

func init() {
	providers.RegisterProvider("rfc2136", &RFC2136Provider{})
}

func (r *RFC2136Provider) Init(rootDomainName string) error {
	var host, port, keyName, secret string
	if host = os.Getenv("RFC2136_HOST"); len(host) == 0 {
		return fmt.Errorf("RFC2136_HOST is not set")
	}

	if port = os.Getenv("RFC2136_PORT"); len(port) == 0 {
		return fmt.Errorf("RFC2136_PORT is not set")
	}

	if keyName = os.Getenv("RFC2136_TSIG_KEYNAME"); len(keyName) == 0 {
		return fmt.Errorf("RFC2136_TSIG_KEYNAME is not set")
	}

	if secret = os.Getenv("RFC2136_TSIG_SECRET"); len(secret) == 0 {
		return fmt.Errorf("RFC2136_TSIG_SECRET is not set")
	}

	r.nameserver = net.JoinHostPort(host, port)
	r.zoneName = dns.Fqdn(rootDomainName)
	r.tsigKeyName = dns.Fqdn(keyName)
	r.tsigSecret = secret

	logrus.Infof("Configured %s with zone '%s' and nameserver '%s'",
		r.GetName(), r.zoneName, r.nameserver)

	return nil
}

func (*RFC2136Provider) GetName() string {
	return "RFC2136"
}

func (r *RFC2136Provider) HealthCheck() error {
	_, err := r.GetRecords()
	return err
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
		var rrType, rrValue string
		switch rr.Header().Rrtype {
		case dns.TypeCNAME:
			rrValue = rr.(*dns.CNAME).Target
			rrType = "CNAME"
		case dns.TypeA:
			rrValue = rr.(*dns.A).A.String()
			rrType = "A"
		case dns.TypeAAAA:
			rrValue = rr.(*dns.AAAA).AAAA.String()
			rrType = "AAAA"
		default:
			continue // Unhandled record type
		}

		for idx, existingRecord := range records {
			if existingRecord.Fqdn == rrFqdn && existingRecord.Type == rrType {
				records[idx].Records = append(records[idx].Records, rrValue)
				continue OuterLoop
			}
		}

		record := utils.DnsRecord{
			Fqdn:    rrFqdn,
			Type:    rrType,
			TTL:     rrTTL,
			Records: []string{rrValue},
		}

		records = append(records, record)
	}

	return records, nil
}

func (r *RFC2136Provider) sendMessage(msg *dns.Msg) error {
	c := new(dns.Client)
	c.TsigSecret = map[string]string{r.tsigKeyName: r.tsigSecret}
	c.SingleInflight = true
	msg.SetTsig(r.tsigKeyName, dns.HmacMD5, 300, time.Now().Unix())
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
	t.TsigSecret = map[string]string{r.tsigKeyName: r.tsigSecret}

	m := new(dns.Msg)
	m.SetAxfr(r.zoneName)
	m.SetTsig(r.tsigKeyName, dns.HmacMD5, 300, time.Now().Unix())

	env, err := t.In(m, r.nameserver)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch records via AXFR: %v", err)
	}

	records := make([]dns.RR, 0)
	for e := range env {
		if e.Error != nil {
			logrus.Errorf("AXFR envelope error: %v", e.Error)
			continue
		}
		records = append(records, e.RR...)
	}

	return records, nil
}
