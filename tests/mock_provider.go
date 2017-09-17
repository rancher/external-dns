package tests

import "github.com/rancher/external-dns/utils"

type MockProviderBase struct {
	hasInit         bool
	hasHealthCheck  bool
	hasAddRecord    bool
	hasRemoveRecord bool
	hasUpdateRecord bool
	hasGetRecords bool
}

func (m *MockProviderBase) Init(rootDomainName string) error {
	m.hasInit = true
	return nil
}

func (m *MockProviderBase) HealthCheck() error {
	m.hasHealthCheck = true
	return nil
}

func (m *MockProviderBase) AddRecord(record utils.DnsRecord) error {
	m.hasAddRecord = true
	return nil
}

func (m *MockProviderBase) RemoveRecord(record utils.DnsRecord) error {
	m.hasRemoveRecord = true
	return nil
}

func (m *MockProviderBase) UpdateRecord(record utils.DnsRecord) error {
	m.hasUpdateRecord = true
	return nil
}

func (m *MockProviderBase) GetRecords() ([]utils.DnsRecord, error) {
	m.hasGetRecords = true

	return []utils.DnsRecord{}, nil
}