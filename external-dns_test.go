package main

import (
	"github.com/rancher/external-dns/metadata"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
	"reflect"
	"testing"
	"github.com/rancher/external-dns/test"
)


var metadata_example = test.NewMockMetaDataRecord("service-1", "stack-1", "example.com")
var metadata_foo_example = test.NewMockMetaDataRecord("service-1", "stack-1", "foo.example.com")
var dnsrecord_example = test.NewMockDnsRecord("example.com", 300, "TXT", "Testing123")

/*
 * --- Test Data ---
 */

var addMissingRecords_testData = []struct {
	inputMeta     map[string]utils.MetadataDnsRecord
	inputProvider map[string]utils.DnsRecord
	expected      []utils.MetadataDnsRecord
}{
	{ // inputMeta (which has DnsRecord embedded) will be compared with inputProvider
		map[string]utils.MetadataDnsRecord{
			"example.com":     metadata_example,
			"foo.example.com": metadata_foo_example,
		},
		map[string]utils.DnsRecord{"example.com": dnsrecord_example},
		[]utils.MetadataDnsRecord{
			metadata_foo_example,
		},
	},
}

// tuple_n
var getProviderDnsRecords_testData = []struct {
	input      []utils.DnsRecord
	expected_1 map[string]utils.DnsRecord
	expected_2 map[string]utils.DnsRecord
	expected_3 error
}{
	{
		[]utils.DnsRecord{},
		map[string]utils.DnsRecord{},
		map[string]utils.DnsRecord{},
		nil,
	},
	{
		[]utils.DnsRecord{
			test.NewMockDnsRecord(test.MockStateFQDN, 300, "TXT", "Testing123-TXT"),
			test.NewMockDnsRecord(test.MockStateFQDN, 300, "A", "Testing123-A"),
		},
		map[string]utils.DnsRecord{
			test.MockStateFQDN: test.NewMockDnsRecord(test.MockStateFQDN, 300, "A", "Testing123-A")},
		map[string]utils.DnsRecord{
			test.MockStateFQDN: test.NewMockDnsRecord(test.MockStateFQDN, 300, "A", "Testing123-A")},
		nil,
	},
}

/*
 * --- Tests ---
 */

// Fyi, a unit test exists for testing the provider package and the mock provider.
func registerMockProvider(records []utils.DnsRecord) *providers.MockProvider {
	mockProvider := providers.NewMockProvider()
	mockProvider.SetRecords(records)
	provider = mockProvider
	return &mockProvider
}

// func UpdateProviderDnsRecords(metadataRecs map[string]utils.MetadataDnsRecord) ([]utils.MetadataDnsRecord, error)
//	-> addMissingRecords
//	-> updateExistingRecords

func Test_addMissingRecords(t *testing.T) {
	for _, asset := range addMissingRecords_testData {
		results := addMissingRecords(asset.inputMeta, asset.inputProvider)
		if !reflect.DeepEqual(results, asset.expected) {
			t.Errorf("\nExpected: \n[%v], \ngot: \n[%v]", asset.expected, results)
		}
	}
}

// updateRecords(toChange []utils.MetadataDnsRecord, op *Op) []utils.MetadataDnsRecord
//	-> AddRecord
//  -> RemoveRecord
//  -> UpdateRecord

// updateExistingRecords(metadataRecs map[string]utils.MetadataDnsRecord, providerRecs map[string]utils.DnsRecord) []utils.MetadataDnsRecord
//	-> UpdateRecords

// removeExtraRecords(metadataRecs map[string]utils.MetadataDnsRecord, providerRecs map[string]utils.DnsRecord) []utils.MetadataDnsRecord
//	-> updateRecords

// getProviderDnsRecords() (map[string]utils.DnsRecord, map[string]utils.DnsRecord, error)
func Test_getProviderDnsRecords(t *testing.T) {
	// Mock environment
	m = &metadata.MetadataClient{
		EnvironmentName: "test",
		EnvironmentUUID: test.MockUUID,
		MetadataClient:  nil,
	}

	for idx, asset := range getProviderDnsRecords_testData {
		registerMockProvider(asset.input)

		result_1, result_2, result_3 := getProviderDnsRecords()
		if !reflect.DeepEqual(result_1, asset.expected_1) {
			t.Errorf("\nTest Data Index #%d, Expected tuple #1: \n[%v], \ngot: \n[%v]", idx, asset.expected_1, result_1)
		}
		if !reflect.DeepEqual(result_2, asset.expected_2) {
			t.Errorf("\nTest Data Index #%d Expected tuple #2: \n[%v], \ngot: \n[%v]", idx, asset.expected_2, result_2)
		}
		if !reflect.DeepEqual(result_3, asset.expected_3) {
			t.Errorf("\nTest Data Index #%d Expected tuple #3: \n[%v], \ngot: \n[%v]", idx, asset.expected_3, result_3)
		}
	}
}

// func EnsureUpgradeToStateRRSet() error
