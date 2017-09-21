package providers

import "github.com/rancher/external-dns/utils"

type Probe struct {
	hasInit         bool
	hasHealthCheck  bool
	hasAddRecord    bool
	hasRemoveRecord bool
	hasUpdateRecord bool
	hasGetRecords   bool
}

type MockProvider struct {
	Provider
	Probe *Probe
}

func (m MockProvider) Init(rootDomainName string) error {
	m.Probe.hasInit = true
	return nil
}

func (m MockProvider) HealthCheck() error {
	m.Probe.hasHealthCheck = true
	return nil
}

func (m MockProvider) AddRecord(record utils.DnsRecord) error {
	m.Probe.hasAddRecord = true
	return nil
}

func (m MockProvider) RemoveRecord(record utils.DnsRecord) error {
	m.Probe.hasRemoveRecord = true
	return nil
}

func (m MockProvider) UpdateRecord(record utils.DnsRecord) error {
	m.Probe.hasUpdateRecord = true
	return nil
}

func (m MockProvider) GetRecords() ([]utils.DnsRecord, error) {
	m.Probe.hasGetRecords = true

	return []utils.DnsRecord{}, nil
}

func NewMockProvider() MockProvider {
	return MockProvider{
		Probe: &Probe{},
	}
}
