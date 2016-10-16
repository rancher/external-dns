package route53

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awsRoute53 "github.com/aws/aws-sdk-go/service/route53"
	"github.com/juju/ratelimit"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

var route53MaxRetries int = 4

type Route53Provider struct {
	client       *awsRoute53.Route53
	hostedZoneId string
	limiter      *ratelimit.Bucket
}

func init() {
	providers.RegisterProvider("route53", &Route53Provider{})
}

func (r *Route53Provider) Init(rootDomainName string) error {
	var region, accessKey, secretKey string
	if region = os.Getenv("AWS_REGION"); len(region) == 0 {
		return fmt.Errorf("AWS_REGION is not set")
	}

	if accessKey = os.Getenv("AWS_ACCESS_KEY"); len(accessKey) == 0 {
		return fmt.Errorf("AWS_ACCESS_KEY is not set")
	}

	if secretKey = os.Getenv("AWS_SECRET_KEY"); len(secretKey) == 0 {
		return fmt.Errorf("AWS_SECRET_KEY is not set")
	}

	// Comply with the API's 5 req/s rate limit. If there are other
	// clients using the same account the AWS SDK will throttle the
	// requests automatically if the global rate limit is exhausted.
	r.limiter = ratelimit.NewBucketWithRate(5.0, 1)

	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	config := aws.NewConfig().WithMaxRetries(route53MaxRetries).
		WithCredentials(creds).
		WithRegion(region)

	r.client = awsRoute53.New(session.New(config))

	if err := r.setHostedZone(rootDomainName); err != nil {
		return err
	}

	logrus.Infof("Configured %s with hosted zone '%s' in region '%s' ",
		r.GetName(), rootDomainName, region)

	return nil
}

func (r *Route53Provider) setHostedZone(rootDomainName string) error {
	if envVal := os.Getenv("ROUTE53_ZONE_ID"); envVal != "" {
		r.hostedZoneId = strings.TrimSpace(envVal)
		if err := r.validateHostedZoneId(rootDomainName); err != nil {
			return err
		}
		return nil
	}

	r.limiter.Wait(1)
	params := &awsRoute53.ListHostedZonesByNameInput{
		DNSName:  aws.String(utils.UnFqdn(rootDomainName)),
		MaxItems: aws.String("1"),
	}
	resp, err := r.client.ListHostedZonesByName(params)
	if err != nil {
		return fmt.Errorf("Could not list hosted zones: %v", err)
	}

	if len(resp.HostedZones) == 0 || *resp.HostedZones[0].Name != rootDomainName {
		return fmt.Errorf("Hosted zone for '%s' not found", rootDomainName)
	}

	zoneId := *resp.HostedZones[0].Id
	if strings.HasPrefix(zoneId, "/hostedzone/") {
		zoneId = strings.TrimPrefix(zoneId, "/hostedzone/")
	}

	r.hostedZoneId = zoneId
	return nil
}

func (r *Route53Provider) validateHostedZoneId(rootDomainName string) error {
	r.limiter.Wait(1)
	params := &awsRoute53.GetHostedZoneInput{
		Id: aws.String(r.hostedZoneId),
	}
	resp, err := r.client.GetHostedZone(params)
	if err != nil {
		return fmt.Errorf("Could not look up hosted zone ID: %v",
			r.hostedZoneId, err)
	}

	if *resp.HostedZone.Name != rootDomainName {
		return fmt.Errorf("Hosted zone ID '%s' does not match name '%s'",
			r.hostedZoneId, rootDomainName)
	}

	return nil
}

func (*Route53Provider) GetName() string {
	return "Route 53"
}

func (r *Route53Provider) HealthCheck() error {
	var params *awsRoute53.GetHostedZoneCountInput
	_, err := r.client.GetHostedZoneCount(params)
	return err
}

func (r *Route53Provider) AddRecord(record utils.DnsRecord) error {
	return r.changeRecord(record, "UPSERT")
}

func (r *Route53Provider) UpdateRecord(record utils.DnsRecord) error {
	return r.changeRecord(record, "UPSERT")
}

func (r *Route53Provider) RemoveRecord(record utils.DnsRecord) error {
	return r.changeRecord(record, "DELETE")
}

func (r *Route53Provider) changeRecord(record utils.DnsRecord, action string) error {
	r.limiter.Wait(1)
	records := make([]*awsRoute53.ResourceRecord, len(record.Records))
	for idx, value := range record.Records {
		if record.Type == "TXT" {
			value = `"` + value + `"`
		}
		records[idx] = &awsRoute53.ResourceRecord{
			Value: aws.String(value),
		}
	}

	params := &awsRoute53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneId),
		ChangeBatch: &awsRoute53.ChangeBatch{
			Comment: aws.String("Managed by Rancher"),
			Changes: []*awsRoute53.Change{
				{
					Action: aws.String(action),
					ResourceRecordSet: &awsRoute53.ResourceRecordSet{
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

func (r *Route53Provider) GetRecords() ([]utils.DnsRecord, error) {
	r.limiter.Wait(1)
	dnsRecords := []utils.DnsRecord{}
	rrSets := []*awsRoute53.ResourceRecordSet{}
	params := &awsRoute53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(r.hostedZoneId),
		MaxItems:     aws.String("100"),
	}

	err := r.client.ListResourceRecordSetsPages(params,
		func(page *awsRoute53.ListResourceRecordSetsOutput, lastPage bool) bool {
			rrSets = append(rrSets, page.ResourceRecordSets...)
			return !lastPage
		})
	if err != nil {
		return dnsRecords, fmt.Errorf("Route 53 API call has failed: %v", err)
	}

	for _, rrSet := range rrSets {
		// skip proprietary Route 53 alias resource record sets
		if rrSet.AliasTarget != nil {
			logrus.Debug("Skipped Route53 alias RRset")
			continue
		}
		records := []string{}
		for _, rr := range rrSet.ResourceRecords {
			value := *rr.Value
			if *rrSet.Type == "TXT" {
				value = strings.Trim(value, `"`)
			}
			records = append(records, value)
		}

		dnsRecord := utils.DnsRecord{
			Fqdn:    *rrSet.Name,
			Records: records,
			Type:    *rrSet.Type,
			TTL:     int(*rrSet.TTL),
		}
		dnsRecords = append(dnsRecords, dnsRecord)
	}

	return dnsRecords, nil
}
