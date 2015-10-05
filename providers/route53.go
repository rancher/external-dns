package providers

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/route53"
	"math"
)

var (
	client     *route53.Route53
	hostedZone *route53.HostedZone
	region     aws.Region
)

const (
	//FIXME - set the root domain name based on the env vars
	//QUESTION: rootDomainName || hostedZoneId?
	rootDomainName = "rancher-test.com."
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
		if zone.Name == rootDomainName {
			hostedZone = &zone
			break
		}
	}

	//FIXME - create a hosted zone if doesn't exist
	if hostedZone == nil {
		log.Fatalf("Failed to find hosted zone for root domain %s", rootDomainName)
	}
}

type Route53Handler struct {
}

func (*Route53Handler) AddRecord(record ExternalDnsEntry) error {
	recordSet := route53.ResourceRecordSet{Name: record.DomainName, Type: "A", Records: record.ARecords}
	update := route53.Change{"UPSERT", recordSet}
	changes := []route53.Change{update}
	req := route53.ChangeResourceRecordSetsRequest{Comment: "Updated by Rancher", Changes: changes}
	client.ChangeResourceRecordSets(hostedZone.ID, &req)
	return nil
}

func (*Route53Handler) RemoveRecord(record ExternalDnsEntry) error {
	return nil
}

func (*Route53Handler) GetRecords() ([]ExternalDnsEntry, error) {
	var records []ExternalDnsEntry
	opts := route53.ListOpts{}

	rec_resp, err := client.ListResourceRecordSets(hostedZone.ID, &opts)
	if err != nil {
		return records, fmt.Errorf("Route53 API call has failed due to %v", err)
	}

	for _, rec := range rec_resp.Records {
		record := ExternalDnsEntry{DomainName: rec.Name, ARecords: rec.Records}
		records = append(records, record)
	}

	return records, nil
}
