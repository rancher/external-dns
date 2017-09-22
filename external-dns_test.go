package main

import (
	"fmt"
	"github.com/rancher/external-dns/utils"
	"reflect"
	"testing"
)

func NewMockDnsRecord(fqdn string, ttl int) utils.DnsRecord {
	return utils.DnsRecord{
		Fqdn: fqdn,
		Records: []string{
			fmt.Sprintf("bar.%s", fqdn),
			fqdn,
		},
		Type: "TXT",
		TTL:  ttl,
	}
}

func NewMockMetaDataRecord(serviceName string, stackName string, fqdn string) utils.MetadataDnsRecord {
	return utils.MetadataDnsRecord{
		ServiceName: serviceName,
		StackName:   stackName,
		DnsRecord:   NewMockDnsRecord(fqdn, 300),
	}
}

var metadata_example = NewMockMetaDataRecord("service-1", "stack-1", "example.com")
var metadata_foo_example = NewMockMetaDataRecord("service-1", "stack-1", "foo.example.com")
var dnsrecord_example = NewMockDnsRecord("example.com", 300)

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

// func UpdateProviderDnsRecords(metadataRecs map[string]utils.MetadataDnsRecord) ([]utils.MetadataDnsRecord, error) {

func Test_addMissingRecords(t *testing.T) {
	for _, asset := range addMissingRecords_testData {
		results := addMissingRecords(asset.inputMeta, asset.inputProvider)
		if !reflect.DeepEqual(results, asset.expected) {
			t.Errorf("\nExpected: \n[%v], \ngot: \n[%v]", asset.expected, results)
		}
	}
}

// updateRecords(toChange []utils.MetadataDnsRecord, op *Op) []utils.MetadataDnsRecord

// updateExistingRecords(metadataRecs map[string]utils.MetadataDnsRecord, providerRecs map[string]utils.DnsRecord) []utils.MetadataDnsRecord

// removeExtraRecords(metadataRecs map[string]utils.MetadataDnsRecord, providerRecs map[string]utils.DnsRecord) []utils.MetadataDnsRecord

// getProviderDnsRecords() (map[string]utils.DnsRecord, map[string]utils.DnsRecord, error)

// func EnsureUpgradeToStateRRSet() error
