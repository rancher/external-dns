package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/juju/ratelimit"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/route53"
	"github.com/rancher/external-dns/dns"
	"math"
	"os"
)

const (
	name = "Route53"
)

var (
	client     *route53.Route53
	hostedZone *route53.HostedZone
	region     aws.Region
	limiter    *ratelimit.Bucket
)

func init() {
	if len(os.Getenv("AWS_REGION")) == 0 {
		logrus.Info("AWS_REGION is not set, skipping init of Route53 provider")
		return
	}

	if len(os.Getenv("AWS_ACCESS_KEY")) == 0 {
		logrus.Info("AWS_ACCESS_KEY is not set, skipping init of Route53 provider")
		return
	}

	if len(os.Getenv("AWS_SECRET_KEY")) == 0 {
		logrus.Info("AWS_SECRET_KEY is not set, skipping init of Route53 provider")
		return
	}

	route53Handler := &Route53Handler{}
	if err := RegisterProvider("route53", route53Handler); err != nil {
		logrus.Fatal("Could not register route53 provider")
	}

	if err := setRegion(); err != nil {
		logrus.Fatalf("Failed to set region: %v", err)
	}

	if err := setHostedZone(); err != nil {
		logrus.Fatalf("Failed to set hosted zone for root domain %s: %v", dns.RootDomainName, err)
	}

	// Throttle Route53 API calls to 3 req/s
	// AWS limit is 5rec/s per account, so leaving the room for other clients
	limiter = ratelimit.NewBucketWithRate(3.0, 1)

	logrus.Infof("Configured %s with hosted zone \"%s\" in region \"%s\" ", route53Handler.GetName(), dns.RootDomainName, region.Name)
}

func setRegion() error {

	regionName := os.Getenv("AWS_REGION")
	r, ok := aws.Regions[regionName]
	if !ok {
		return fmt.Errorf("Could not find region by name %s", regionName)
	}
	region = r
	auth, err := aws.EnvAuth()
	if err != nil {
		logrus.Fatal("AWS failed to authenticate: %v", err)
	}
	client = route53.New(auth, region)

	return nil
}

func setHostedZone() error {
	zoneResp, err := client.ListHostedZones("", math.MaxInt64)
	if err != nil {
		logrus.Fatalf("Failed to list hosted zones: %v", err)
	}
	for _, zone := range zoneResp.HostedZones {
		if zone.Name == dns.RootDomainName {
			hostedZone = &zone
			break
		}
	}
	if hostedZone == nil {
		logrus.Fatalf("Hosted zone %s is missing", dns.RootDomainName)
	}
	return nil
}

type Route53Handler struct {
}

func (*Route53Handler) GetName() string {
	return name
}

func (r *Route53Handler) AddRecord(record dns.DnsRecord) error {
	return r.changeRecord(record, "UPSERT")
}

func (r *Route53Handler) UpdateRecord(record dns.DnsRecord) error {
	return r.changeRecord(record, "UPSERT")
}

func (r *Route53Handler) RemoveRecord(record dns.DnsRecord) error {
	return r.changeRecord(record, "DELETE")
}

func (*Route53Handler) changeRecord(record dns.DnsRecord, action string) error {
	recordSet := route53.ResourceRecordSet{Name: record.Fqdn, Type: record.Type, Records: record.Records, TTL: record.TTL}
	update := route53.Change{action, recordSet}
	changes := []route53.Change{update}
	req := route53.ChangeResourceRecordSetsRequest{Comment: "Updated by Rancher", Changes: changes}
	limiter.Wait(1)
	_, err := client.ChangeResourceRecordSets(hostedZone.ID, &req)
	return err
}

func (*Route53Handler) GetRecords() ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord
	opts := route53.ListOpts{}
	limiter.Wait(1)
	resp, err := client.ListResourceRecordSets(hostedZone.ID, &opts)
	if err != nil {
		return records, fmt.Errorf("Route53 API call has failed: %v", err)
	}

	for _, rec := range resp.Records {
		record := dns.DnsRecord{Fqdn: rec.Name, Records: rec.Records, Type: rec.Type, TTL: rec.TTL}
		records = append(records, record)
	}

	return records, nil
}
