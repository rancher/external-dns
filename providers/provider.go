package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	"os"
	"strings"
)

type Provider interface {
	TestConnection() error
	AddRecord(record dns.DnsRecord) error
	RemoveRecord(record dns.DnsRecord) error
	UpdateRecord(record dns.DnsRecord) error
	GetRecords(listOpts ...string) ([]dns.DnsRecord, error)
	GetName() string
	GetRootDomain() string
}

var (
	providers map[string]Provider
)

func GetProvider(name string) Provider {
	if provider, ok := providers[name]; ok {
		return provider
	}
	return providers["route53"]
}

func UnRegisterProvider(name string) Provider {
	if _, ok := providers[name]; ok {
		providers[name] = nil
	}
	return nil
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

func getDefaultRootDomain() string {
	var name string
	name = os.Getenv("ROOT_DOMAIN")
	if len(name) == 0 {
		logrus.Fatalf("ROOT_DOMAIN is not set")
	}
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	return name
}
