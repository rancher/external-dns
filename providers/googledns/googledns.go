package googledns

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/juju/ratelimit"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
	cnet "golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1beta2"
)

var (
	projectId string = os.Getenv("GOOGLEDNS_PROJECT_ID")
)

type GoogleDNSProvider struct {
	client       *dns.Service
	hostedZoneId string
	limiter      *ratelimit.Bucket
}

func init() {
	providers.RegisterProvider("googledns", &GoogleDNSProvider{})
}

// Init creates a Google DNS client with credentials from DefaultClient, which uses FindDefaultCredentials

// FindDefaultCredentials searches for "Application Default Credentials".
//
// It looks for credentials in the following places,
// preferring the first location found:
//
//   1. A JSON file whose path is specified by the
//      GOOGLE_APPLICATION_CREDENTIALS environment variable.
//   2. A JSON file in a location known to the gcloud command-line tool.
//      On Windows, this is %APPDATA%/gcloud/application_default_credentials.json.
//      On other systems, $HOME/.config/gcloud/application_default_credentials.json.
//   3. On Google App Engine it uses the appengine.AccessToken function.
//   4. On Google Compute Engine and Google App Engine Managed VMs, it fetches
//      credentials from the metadata server.
//      (In this final case any provided scopes are ignored.)

func (r *GoogleDNSProvider) Init(rootDomainName string) error {
	r.limiter = ratelimit.NewBucketWithRate(5.0, 1)

	ctx := cnet.Context(cnet.Background())

	c, _ := google.DefaultClient(ctx, dns.CloudPlatformScope)

	r.client, _ = dns.New(c)

	if err := r.setHostedZone(rootDomainName); err != nil {
		return fmt.Errorf("Failed to configure hosted zone: %v", err)
	}

	logrus.Infof("Configured %s with hosted zone %s",
		r.GetName(), rootDomainName)

	return nil
}

func (r *GoogleDNSProvider) setHostedZone(rootDomainName string) error {
	r.limiter.Wait(1)

	zones, err := r.client.ManagedZones.List(projectId).Do()

	if len(zones.ManagedZones) == 0 {
		return fmt.Errorf("Hosted zone for '%s' not found, got empty list", rootDomainName)
	}

	for _, zone := range zones.ManagedZones {
		logrus.Debugf("Found Zone: %v (%v)\n", zone.Name, zone.DnsName)
		if zone.DnsName == rootDomainName {
			logrus.Debugf("Found desired zone %v !\n", zone.Name)
			r.hostedZoneId = string(zone.Name)
			return nil
		}
	}

	return err

}

func (*GoogleDNSProvider) GetName() string {
	return "Google Cloud DNS"
}

func (r *GoogleDNSProvider) HealthCheck() error {
	return nil
}

func (r *GoogleDNSProvider) AddRecord(record utils.DnsRecord) error {
	return r.changeRecord(record, "ADD")
}

func (r *GoogleDNSProvider) UpdateRecord(record utils.DnsRecord) error {
	return r.changeRecord(record, "UPDATE")
}

func (r *GoogleDNSProvider) RemoveRecord(record utils.DnsRecord) error {
	return r.changeRecord(record, "DELETE")
}

func (r *GoogleDNSProvider) changeRecord(record utils.DnsRecord, action string) error {
	r.limiter.Wait(1)

	var records []*dns.ResourceRecordSet

	rec := dns.ResourceRecordSet{
		Kind:            "dns#resourceRecordSet",
		Name:            record.Fqdn,
		Rrdatas:         record.Records,
		Ttl:             int64(record.TTL),
		Type:            record.Type,
		ForceSendFields: nil,
		NullFields:      nil,
	}

	records = append(records, &rec)

	change := dns.Change{
		Additions: nil,
		Deletions: nil,
		Kind:      "dns#change",
	}

	if action == "ADD" {
		change.Additions = records
	} else if action == "DELETE" || action == "UPDATE" {

		var recs []*dns.ResourceRecordSet

		rrSets, _ := r.client.ResourceRecordSets.List(projectId, r.hostedZoneId).Do()

		for _, rrSet := range rrSets.Rrsets {

			if rrSet.Name == record.Fqdn {
				recs = append(recs, rrSet)
			}
		}

		if action == "UPDATE" {
			change.Additions = records
		}
		change.Deletions = recs
	}

	_, err := r.client.Changes.Create(projectId, r.hostedZoneId, &change).Do()

	return err
}

func (r *GoogleDNSProvider) GetRecords() ([]utils.DnsRecord, error) {
	r.limiter.Wait(1)
	dnsRecords := []utils.DnsRecord{}

	rrSets, err := r.client.ResourceRecordSets.List(projectId, r.hostedZoneId).Do()

	if err != nil {
		return dnsRecords, fmt.Errorf("Google Cloud DNS API call has failed: %v", err)
	}

	records := []string{}

	for _, rrSet := range rrSets.Rrsets {

		records = rrSet.Rrdatas

		logrus.Debugf("rrSet: %s", rrSet)
		logrus.Debugf("records: %s", records)

		dnsRecord := utils.DnsRecord{
			Fqdn:    rrSet.Name,
			Records: records,
			Type:    rrSet.Type,
			TTL:     int(rrSet.Ttl),
		}
		dnsRecords = append(dnsRecords, dnsRecord)
	}

	return dnsRecords, nil
}
