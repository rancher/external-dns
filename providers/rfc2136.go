package providers

import (
	"github.com/Sirupsen/logrus"
	rdns "github.com/rancher/external-dns/dns"
	"os"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"time"
)

type RFC2136Handler struct {
	server     string
	port       string
	keyname    string
	key        string
	rootdomain string
}

func (b RFC2136Handler) sendMessage(msg *dns.Msg) error {
	c := new(dns.Client)
	c.TsigSecret = map[string]string{b.keyname: b.key}
	c.SingleInflight = true
	msg.SetTsig(b.keyname, dns.HmacMD5, 300, time.Now().Unix())
	nameserver := net.JoinHostPort(b.server, b.port)

	r, _, err := c.Exchange(msg, nameserver)
	if err != nil {
		return err
	}

	if r != nil && r.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("Bad return code: %s", dns.RcodeToString[r.Rcode])
	}

	return nil
}

func (b RFC2136Handler) remove(fqdn string, rtype string) error {
	logrus.Debugf("removing dns entry %s", fqdn)
	m := new(dns.Msg)
	m.SetUpdate(b.rootdomain)
	rr, rrerr := dns.NewRR(fmt.Sprintf("%s 0 %s 0.0.0.0", dns.Fqdn(fqdn), rtype))
	if rrerr != nil {
		return rrerr
	}
	rrs := make([]dns.RR, 1)
	rrs[0] = rr
	m.RemoveRRset(rrs)
	return b.sendMessage(m)
}

func (b RFC2136Handler) list() ([]dns.RR, error) {
	logrus.Debugf("listing dns entries for %s", b.rootdomain)
	t := new(dns.Transfer)
	t.TsigSecret = map[string]string{b.keyname: b.key}

	m := new(dns.Msg)

	nameserver := net.JoinHostPort(b.server, b.port)
	m.SetAxfr(b.rootdomain)
	m.SetTsig(b.keyname, dns.HmacMD5, 300, time.Now().Unix())

	env, err := t.In(m, nameserver)
	if err != nil {
		return nil, err;
	}
	envelope := 0
	records := make([]dns.RR, 0)

	// TODO what is an enveloppe, can i receive several of them ?
	for e := range env {
		if e.Error != nil {
			fmt.Printf(";; %s\n", e.Error.Error())
			return nil, e.Error
		}
		records = append(records, e.RR...)
		envelope++
	}
	return records, nil
}

func init() {
	server := os.Getenv("DNS_HOST")
	port := os.Getenv("DNS_PORT")
	keyname := os.Getenv("TSIG_KEYNAME")
	key := os.Getenv("TSIG_KEY")
	if len(server) == 0 || len(port) == 0 || len(keyname) == 0 || len(key) == 0 {
		logrus.Info("RFC2136 environnement not set, skipping init of RFC2136 provider")
		return
	}

	rfc2136Handler := &RFC2136Handler{server, port, dns.Fqdn(keyname), key, dns.Fqdn(rdns.RootDomainName)}
	if err := RegisterProvider("RFC2136", rfc2136Handler); err != nil {
		logrus.Fatal("Could not register RFC2136 provider")
	}
	logrus.Infof("Configured %s with zone %s and server %s", rfc2136Handler.GetName(), rdns.RootDomainName, server)

}

/* Provider implementation */
func (b *RFC2136Handler) AddRecord(record rdns.DnsRecord) error {

	logrus.Debugf("adding dns entry %s", record.Fqdn)
	m := new(dns.Msg)
	m.SetUpdate(b.rootdomain)

	rrs := make([]dns.RR, 0)
	for _, rec := range record.Records {
		logrus.Debugf("adding dns RR %s for %s", rec, record.Fqdn)

		rr, err := dns.NewRR(fmt.Sprintf("%s %d %s %s", record.Fqdn, record.TTL, record.Type, rec))
		if err != nil {
			return err
		}
		rrs = append(rrs, rr)
	}
	m.Insert(rrs)
	return b.sendMessage(m)
}

func (b *RFC2136Handler) RemoveRecord(record rdns.DnsRecord) error {
	err := b.remove(record.Fqdn, record.Type)
	if err != nil {
		return fmt.Errorf("RFC2136 API call has failed: %v", err)
	}
	return nil
}

func (b *RFC2136Handler) UpdateRecord(record rdns.DnsRecord) error {
	err := b.RemoveRecord(record)
	if err != nil {
		return err
	} else {
		return b.AddRecord(record)
	}
}

func (b *RFC2136Handler) GetRecords() ([]rdns.DnsRecord, error) {
	list, err := b.list()
	records := make([]rdns.DnsRecord, 0)
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

		record := rdns.DnsRecord{
			Fqdn: rrFqdn,
			Type: rrType,
			TTL: rrTTL,
			Records: []string{rrValue},
		}
		records = append(records, record)
	}

	return records, nil
}

func (b *RFC2136Handler) GetName() string {
	return "RFC2136"
}
