package providers

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/juju/ratelimit"
	"github.com/rancher/external-dns/dns"
)

var route53MaxRetries int = 3

type Route53Handler struct {
	client       *route53.Route53
	hostedZoneId string
	limiter      *ratelimit.Bucket
}

func init() {
	var region, accessKey, secretKey string
	if region = os.Getenv("AWS_REGION"); len(region) == 0 {
		logrus.Info("AWS_REGION is not set, skipping init of Route 53 provider")
		return
	}

	if accessKey = os.Getenv("AWS_ACCESS_KEY"); len(accessKey) == 0 {
		logrus.Info("AWS_ACCESS_KEY is not set, skipping init of Route 53 provider")
		return
	}

	if secretKey = os.Getenv("AWS_SECRET_KEY"); len(secretKey) == 0 {
		logrus.Info("AWS_SECRET_KEY is not set, skipping init of Route 53 provider")
		return
	}

	handler := &Route53Handler{}
	if err := RegisterProvider("route53", handler); err != nil {
		logrus.Fatal("Could not register route53 provider")
	}

	// Comply with the API's 5 req/s rate limit. If there are other
	// clients using the same account the AWS SDK will throttle our
	// requests once the API returns a "Rate exceeded" error.
	handler.limiter = ratelimit.NewBucketWithRate(5.0, 1)

	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	config := aws.NewConfig().WithMaxRetries(route53MaxRetries).
		WithCredentials(creds).
		WithRegion(region)

	handler.client = route53.New(session.New(config))

	if err := handler.setHostedZone(); err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Configured %s with hosted zone '%s' in region '%s' ",
		handler.GetName(), dns.RootDomainName, region)
}

func (r *Route53Handler) setHostedZone() error {
	if envVal := os.Getenv("ROUTE53_ZONE_ID"); envVal != "" {
		r.hostedZoneId = strings.TrimSpace(envVal)
		return nil
	}

	r.limiter.Wait(1)
	params := &route53.ListHostedZonesByNameInput{
		DNSName:  aws.String(strings.TrimSuffix(dns.RootDomainName, ".")),
		MaxItems: aws.String("1"),
	}
	resp, err := r.client.ListHostedZonesByName(params)
	if err != nil {
		return fmt.Errorf("Could not list hosted zones: %v", err)
	}

	if len(resp.HostedZones) == 0 || *resp.HostedZones[0].Name != dns.RootDomainName {
		return fmt.Errorf("Hosted zone '%s' not found", dns.RootDomainName)
	}

	zoneId := *resp.HostedZones[0].Id
	if strings.HasPrefix(zoneId, "/hostedzone/") {
		zoneId = strings.TrimPrefix(zoneId, "/hostedzone/")
	}

	r.hostedZoneId = zoneId
	return nil
}

func (*Route53Handler) GetName() string {
	return "Route 53"
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

func (r *Route53Handler) changeRecord(record dns.DnsRecord, action string) error {
	r.limiter.Wait(1)
	records := make([]*route53.ResourceRecord, len(record.Records))
	for idx, value := range record.Records {
		records[idx] = &route53.ResourceRecord{
			Value: aws.String(value),
		}
	}

	params := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneId),
		ChangeBatch: &route53.ChangeBatch{
			Comment: aws.String("Managed by Rancher"),
			Changes: []*route53.Change{
				{
					Action: aws.String(action),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            aws.String(record.Fqdn),
						Type:            aws.String(record.Type),
						TTL:             aws.Int64(int64(record.TTL)),
						ResourceRecords: records,
					},
				},
			},
		},
	}

	_, err := r.client.ChangeResourceRecordSets(params)
	return err
}

func (r *Route53Handler) GetRecords() ([]dns.DnsRecord, error) {
	r.limiter.Wait(1)
	dnsRecords := []dns.DnsRecord{}
	rrSets := []*route53.ResourceRecordSet{}
	params := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneId),
		MaxItems:     aws.String("100"),
	}

	err := r.client.ListResourceRecordSetsPages(params,
		func(page *route53.ListResourceRecordSetsOutput, lastPage bool) bool {
			rrSets = append(rrSets, page.ResourceRecordSets...)
			return !lastPage
		})
	if err != nil {
		return dnsRecords, fmt.Errorf("Route 53 API call has failed: %v", err)
	}

	for _, rrSet := range rrSets {
		// skip proprietary Route 53 alias resource record sets
		if rrSet.AliasTarget != nil {
			continue
		}
		records := []string{}
		for _, rr := range rrSet.ResourceRecords {
			records = append(records, *rr.Value)
		}

		dnsRecord := dns.DnsRecord{
			Fqdn:    *rrSet.Name,
			Records: records,
			Type:    *rrSet.Type,
			TTL:     int(*rrSet.TTL),
		}
		dnsRecords = append(dnsRecords, dnsRecord)
	}

	return dnsRecords, nil
}
