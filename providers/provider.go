package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"os"
	"strings"
	"time"
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
	var name string
	for {
		time.Sleep(1000 * time.Millisecond)
		name = os.Getenv("ROOT_DOMAIN")
		if len(name) == 0 {
			logrus.Error("ROOT_DOMAIN is not set")
			continue
		}
		break
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
