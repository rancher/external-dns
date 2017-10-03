package providers

import "github.com/rancher/external-dns/utils"

type Probe struct {
	CountInit         int
	CountHealthCheck  int
	CountAddRecord    int
	CountRemoveRecord int
	CountUpdateRecord int
	CountGetRecords   int
	CountSetRecords   int
	CountGetName      int
}

type MockProvider struct {
	Provider
	Probe      *Probe
	dnsRecords []utils.DnsRecord
}

func (m MockProvider) Init(rootDomainName string) error {
	m.Probe.CountInit++
	return nil
}

func (m MockProvider) GetName() string {
	m.Probe.CountGetName++
	return "MockProvider"
}

func (m MockProvider) HealthCheck() error {
	m.Probe.CountHealthCheck++
	return nil
}

func (m MockProvider) AddRecord(record utils.DnsRecord) error {
	m.Probe.CountAddRecord++
	return nil
}

func (m MockProvider) RemoveRecord(record utils.DnsRecord) error {
	m.Probe.CountRemoveRecord++
	return nil
}

func (m MockProvider) UpdateRecord(record utils.DnsRecord) error {
	m.Probe.CountUpdateRecord++
	return nil
}

func (m MockProvider) GetRecords() ([]utils.DnsRecord, error) {
	m.Probe.CountGetRecords++
	return m.dnsRecords, nil
}

func (m *MockProvider) SetRecords(dnsRecords []utils.DnsRecord) {
	m.Probe.CountSetRecords++
	m.dnsRecords = dnsRecords
}

func NewMockProvider() MockProvider {
	return MockProvider{
		Probe: &Probe{},
	}
}
