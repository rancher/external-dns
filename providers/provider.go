package providers

import (
	"fmt"
)

type Provider interface {
	AddRecord(record ExternalDnsEntry) error
	RemoveRecord(record ExternalDnsEntry) error
	GetRecords() (string, error)
}

type ExternalDnsEntry struct {
	DomainName string
	ARecords   []string
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
