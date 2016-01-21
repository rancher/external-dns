package providers

import (
	"fmt"
	logrus "github.com/Sirupsen/logrus"
	"github.com/juju/ratelimit"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/route53"
	"github.com/rancher/external-dns/dns"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
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

	InitializeRoute53()

}

func InitializeRoute53() {
	route53Handler := &Route53Handler{}

	dns.SetRootDomain(getDefaultRootDomain())

	if err := setRegion(); err != nil {
		logrus.Fatalf("Failed to set region: %v", err)
	}

	if err := setHostedZone(); err != nil {
		logrus.Fatalf("Failed to set hosted zone for root domain %s: %v", dns.RootDomainName, err)
	}

	// Throttle Route53 API calls to 3 req/s
	// AWS limit is 5rec/s per account, so leaving the room for other clients
	limiter = ratelimit.NewBucketWithRate(3.0, 1)

	//check network health
	err := route53Handler.TestConnection()

	if err != nil {
		logrus.Fatalf("Failed to connect to route53 service: %v", err)
	}

	UnRegisterProvider("route53")

	if err := RegisterProvider("route53", route53Handler); err != nil && err.Error() != "provider already registered" {
		logrus.Fatalf("Could not register route53 provider %v", err)
	}

	logrus.Infof("Configured %s with hosted zone \"%s\" in region \"%s\" ", route53Handler.GetName(), dns.RootDomainName, region.Name)
}

func (*Route53Handler) TestConnection() error {
	var err error
	maxTime := 20 * time.Second

	for i := 1 * time.Second; i < maxTime; i *= time.Duration(2) {
		if _, err = client.ListHostedZones("", 3); err != nil {
			time.Sleep(i)
		} else {
			return nil
		}
	}
	return err
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

func (*Route53Handler) GetRootDomain() string {
	return getDefaultRootDomain()
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

func (*Route53Handler) GetRecords(listOpts ...string) ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord

	opts := route53.ListOpts{}

	if len(listOpts) > 0 {
		for i, option := range listOpts {
			switch i {
			case 0:
				if listOpts[0] != "" {
					opts.Name = option
				}

			case 1:
				if listOpts[1] != "" {
					opts.Type = option
				}

			case 2:
				maxItems, err := strconv.Atoi(option)
				if err != nil {
					logrus.Debugf("Error parsing the maxItems filter %v, not applying this filter", err)
				} else {
					opts.MaxItems = maxItems
				}

			}
		}
	}

	logrus.Debugf("Route53 GetRecords filtered by name: %s, by type: %s, by maxItems: %d", opts.Name, opts.Type, opts.MaxItems)

	tempOpts := route53.ListOpts{}
	tempOpts.Name = opts.Name
	tempOpts.Type = opts.Type
	tempOpts.MaxItems = opts.MaxItems

	for {
		limiter.Wait(1)
		resp, err := client.ListResourceRecordSets(hostedZone.ID, &tempOpts)

		if err != nil {
			return records, fmt.Errorf("Route53 API call has failed: %v", err)
		}
		i := 0
		for _, rec := range resp.Records {
			if opts.Name != "" && strings.HasSuffix(rec.Name, opts.Name) {
				record := dns.DnsRecord{Fqdn: rec.Name, Records: rec.Records, Type: rec.Type, TTL: rec.TTL}
				records = append(records, record)
				i++
			}
		}

		logrus.Debugf("Recordset size %v", i)

		if resp.IsTruncated {
			if opts.Name != "" && !strings.HasSuffix(resp.NextRecordName, opts.Name) {
				break
			} else if opts.Type != "" && !strings.EqualFold(resp.NextRecordType, opts.Type) {
				break
			}
			tempOpts.Name = resp.NextRecordName
		} else {
			break
		}
	}

	logrus.Debugf("Records found are these %v", records)

	return records, nil
}
