package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/utils"
)

type Provider interface {
	// Init is called once on startup to initialize the provider
	Init() error
	// GetRootDomain returns the root domain for the provider
	// Standard providers should return the value from
	// utils.GetDefaultRootDomain()
	GetRootDomain() string
	// GetName returns the provider's name
	GetName() string
	// HealthCheck checks the connection to the DNS provider
	HealthCheck() error
	// AddRecord creates the specified record on the DNS provider
	AddRecord(record utils.DnsRecord) error
	// RemoveRecords deletes the specified record on the DNS provider
	RemoveRecord(record utils.DnsRecord) error
	// UpdateRecord updates the specified record on the DNS provider
	UpdateRecord(record utils.DnsRecord) error
	// GetRecords returns all records in the DNS provider zone
	GetRecords() ([]utils.DnsRecord, error)
}

var (
	providers = make(map[string]Provider)
)

func GetProvider(name string) (Provider, error) {
	if provider, ok := providers[name]; ok {
		if err := provider.Init(); err != nil {
			return nil, err
		}
		return provider, nil
	}
	return nil, fmt.Errorf("No such provider '%s'", name)
}

func RegisterProvider(name string, provider Provider) {
	if _, exists := providers[name]; exists {
		logrus.Fatalf("Provider '%s' tried to register twice", name)
	}
	providers[name] = provider
}
