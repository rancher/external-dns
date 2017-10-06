package test

import (
	"github.com/rancher/external-dns/utils"
	"fmt"
)

var MockUUID = "a0a0aa00-aa0a-0a0a-aa00-000000aaa0a0"
var MockStateFQDN string = fmt.Sprintf("external-dns-%s.", MockUUID)

func NewMockDnsRecord(fqdn string, ttl int, recType string, value string, records []string) utils.DnsRecord {
	recordType := func() string {
		if len(recType) <= 0 {
			return "TXT"
		}

		return recType
	}

	dnsRecords := func() []string {
		if len(records) <= 0 {
			return []string{
				value,
				fqdn,
			}
		}

		return records
	}

	return utils.DnsRecord{
		Fqdn: fqdn,
		Records: dnsRecords(),
		Type: recordType(),
		TTL:  ttl,
	}
}

func NewMockMetaDataRecord(serviceName string, stackName string, fqdn string, records []string) utils.MetadataDnsRecord {
	return utils.MetadataDnsRecord{
		ServiceName: serviceName,
		StackName:   stackName,
		DnsRecord:   NewMockDnsRecord(fqdn, 300, "TXT", "Testing123", records),
	}

}