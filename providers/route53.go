package providers

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/route53"
	"math"
)

const (
	Name = "route53"
)

var (
	client     *route53.Route53
	hostedZone *route53.HostedZone
	region     aws.Region
)

func init() {
	route53Handler := &Route53Handler{}
	if err := RegisterProvider("route53", route53Handler); err != nil {
		log.Fatal("Could not register route53 provider")
	}
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal("AWS failed to authenticate due to %v", err)
	}
	//FIXME - set the region based on environment variable
	region = aws.USWest2
	client = route53.New(auth, region)

	zoneResp, err := client.ListHostedZones("", math.MaxInt64)
	if err != nil {
		log.Fatalf("Failed to list hosted zones due to %v", err)
	}

	for _, zone := range zoneResp.HostedZones {
		if zone.Name == RootDomainName {
			hostedZone = &zone
			break
		}
	}

	//TODO - create a hosted zone if doesn't exist
	if hostedZone == nil {
		log.Fatalf("Failed to find hosted zone for root domain %s", RootDomainName)
	}
}

type Route53Handler struct {
}

func (*Route53Handler) GetName() string {
	return Name
}

func (r *Route53Handler) AddRecord(record DnsRecord) error {
	return r.updateRecord(record, "UPSERT")
}

func (r *Route53Handler) UpdateRecord(record DnsRecord) error {
	return r.updateRecord(record, "UPSERT")
}

func (r *Route53Handler) RemoveRecord(record DnsRecord) error {
	return r.updateRecord(record, "DELETE")
}

func (*Route53Handler) updateRecord(record DnsRecord, action string) error {
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

	rec_resp, err := client.ListResourceRecordSets(hostedZone.ID, &opts)
	if err != nil {
		return records, fmt.Errorf("Route53 API call has failed due to %v", err)
	}

	for _, rec := range rec_resp.Records {
		record := DnsRecord{DomainName: rec.Name, Records: rec.Records, Type: rec.Type, TTL: rec.TTL}
		records = append(records, record)
	}

	return records, nil
}
