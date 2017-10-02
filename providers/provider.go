package providers

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/utils"
)

type Provider interface {
	Init(rootDomainName string) error
	GetName() string
	HealthCheck() error
	AddRecord(record utils.DnsRecord) error
	RemoveRecord(record utils.DnsRecord) error
	UpdateRecord(record utils.DnsRecord) error
	GetRecords() ([]utils.DnsRecord, error)
}

var (
	registeredProviders = make(map[string]Provider)
)

func GetProvider(name, rootDomainName string) (Provider, error) {
	if provider, ok := registeredProviders[name]; ok {
		if err := provider.Init(rootDomainName); err != nil {
			return nil, err
		}
		return provider, nil
	}
	return nil, fmt.Errorf("no such provider '%s'", name)
}

func RegisterProvider(name string, provider Provider) {
	if _, exists := registeredProviders[name]; exists {
		logrus.Fatalf("Provider '%s' tried to register twice", name)
	}
	registeredProviders[name] = provider
}
