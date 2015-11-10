package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	"github.com/rancher/external-dns/metadata"
)

type Provider interface {
	AddRecord(record dns.DnsRecord) error
	RemoveRecord(record dns.DnsRecord) error
	UpdateRecord(record dns.DnsRecord) error
	GetRecords() ([]dns.DnsRecord, error)
	GetName() string
}

var (
	providers map[string]Provider
)

func init() {
	// try to resolve rancher-metadata before going further
	// the resolution indicates that the network has been set
	_, err := metadata.NewMetadataClient()
	if err != nil {
		logrus.Fatalf("Failed to configure rancher-metadata client: %v", err)
	}
}

func GetProvider(name string) Provider {
	if provider, ok := providers[name]; ok {
		return provider
	}
	return providers["route53"]
}

func RegisterProvider(name string, provider Provider) error {
	if providers == nil {
		providers = make(map[string]Provider)
	}
	if _, exists := providers[name]; exists {
		return fmt.Errorf("provider already registered")
	}
	providers[name] = provider
	return nil
}
