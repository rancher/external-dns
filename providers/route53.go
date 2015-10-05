package providers

import (
	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/route53"
)

func init() {
	route53Handler := &Route53Handler{}
	if err := RegisterProvider("route53", route53Handler); err != nil {
		log.Fatal("Could not register route53 provider")
	}
}

type Route53Handler struct {
}

func (*Route53Handler) AddRecord(record ExternalDnsEntry) error {
	return nil
}

func (*Route53Handler) RemoveRecord(record ExternalDnsEntry) error {
	return nil
}

func (*Route53Handler) GetRecords() (string, error) {
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	client := route53.New(auth, aws.USWest2)
	resp, err := client.ListHostedZones("", 1)

	if err != nil {
		log.Fatal(err)
	}

	// This is the test, real dns config stuff is coming later
	log.Infof("My hosted zones count %+v", len(resp.HostedZones))
	return "", nil
}
