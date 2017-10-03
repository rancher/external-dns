package providers

import "github.com/rancher/external-dns/utils"

type Probe struct {
	HasInit         bool
	HasHealthCheck  bool
	HasAddRecord    bool
	HasRemoveRecord bool
	HasUpdateRecord bool
	HasGetRecords   bool
	HasSetRecords   bool
	HasGetName      bool
}

type MockProvider struct {
	Provider
	Probe      *Probe
	dnsRecords []utils.DnsRecord
}

func (m MockProvider) Init(rootDomainName string) error {
	m.Probe.HasInit = true
	return nil
}

func (m MockProvider) GetName() string {
	m.Probe.HasGetName = true
	return "MockProvider"
}

func (m MockProvider) HealthCheck() error {
	m.Probe.HasHealthCheck = true
	return nil
}

func (m MockProvider) AddRecord(record utils.DnsRecord) error {
	m.Probe.HasAddRecord = true
	return nil
}

func (m MockProvider) RemoveRecord(record utils.DnsRecord) error {
	m.Probe.HasRemoveRecord = true
	return nil
}

func (m MockProvider) UpdateRecord(record utils.DnsRecord) error {
	m.Probe.HasUpdateRecord = true
	return nil
}

func (m MockProvider) GetRecords() ([]utils.DnsRecord, error) {
	m.Probe.HasGetRecords = true
	return m.dnsRecords, nil
}

func (m *MockProvider) SetRecords(dnsRecords []utils.DnsRecord) {
	m.Probe.HasSetRecords = true
	m.dnsRecords = dnsRecords
}

func NewMockProvider() MockProvider {
	return MockProvider{
		Probe: &Probe{},
	}
}
