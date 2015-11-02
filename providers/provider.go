package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"os"
	"strings"
)

var (
	RootDomainName string
)

type Provider interface {
	AddRecord(record DnsRecord) error
	RemoveRecord(record DnsRecord) error
	UpdateRecord(record DnsRecord) error
	GetRecords() ([]DnsRecord, error)
	GetName() string
}

type DnsRecord struct {
	DomainName string
	Records    []string
	Type       string
	TTL        int
}

var (
	providers map[string]Provider
)

func init() {
	name := os.Getenv("ROOT_DOMAIN")
	if len(name) == 0 {
		logrus.Fatalf("ROOT_DOMAIN is not set")
	}

	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	RootDomainName = name
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
