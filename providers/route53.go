package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/route53"
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
)

func init() {
	route53Handler := &Route53Handler{}
	if err := RegisterProvider("route53", route53Handler); err != nil {
		logrus.Fatal("Could not register route53 provider")
	}

	if err := setRegion(); err != nil {
		logrus.Fatalf("Failed to set region: %v", err)
	}

	if err := setHostedZone(); err != nil {
		logrus.Fatalf("Failed to set hosted zone for root domain %s: %v", RootDomainName, err)
	}

	logrus.Infof("Configured %s with hosted zone \"%s\" in region \"%s\" ", route53Handler.GetName(), RootDomainName, region.Name)
}

func setRegion() error {

	regionName := os.Getenv("AWS_REGION")
	if len(regionName) == 0 {
		return fmt.Errorf("AWS_REGION is not set")
	}

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
		if zone.Name == RootDomainName {
			hostedZone = &zone
			break
		}
	}
	if hostedZone == nil {
		logrus.Fatalf("Hosted zone %s is missing", RootDomainName)
	}
	return nil
}

type Route53Handler struct {
}

func (*Route53Handler) GetName() string {
	return name
}

func (r *Route53Handler) AddRecord(record DnsRecord) error {
	return r.changeRecord(record, "UPSERT")
}

func (r *Route53Handler) UpdateRecord(record DnsRecord) error {
	return r.changeRecord(record, "UPSERT")
}

func (r *Route53Handler) RemoveRecord(record DnsRecord) error {
	return r.changeRecord(record, "DELETE")
}

func (*Route53Handler) changeRecord(record DnsRecord, action string) error {
	recordSet := route53.ResourceRecordSet{Name: record.DomainName, Type: record.Type, Records: record.Records, TTL: record.TTL}
	update := route53.Change{action, recordSet}
	changes := []route53.Change{update}
	req := route53.ChangeResourceRecordSetsRequest{Comment: "Updated by Rancher", Changes: changes}
	_, err := client.ChangeResourceRecordSets(hostedZone.ID, &req)
	return err
}

func (*Route53Handler) GetRecords() ([]DnsRecord, error) {
	var records []DnsRecord
	opts := route53.ListOpts{}

	resp, err := client.ListResourceRecordSets(hostedZone.ID, &opts)
	if err != nil {
		return records, fmt.Errorf("Route53 API call has failed: %v", err)
	}

	for _, rec := range resp.Records {
		record := DnsRecord{DomainName: rec.Name, Records: rec.Records, Type: rec.Type, TTL: rec.TTL}
		records = append(records, record)
	}

	return records, nil
}
