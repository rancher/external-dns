package test

import (
	"github.com/rancher/external-dns/utils"
	"fmt"
)

var MockUUID = "a0a0aa00-aa0a-0a0a-aa00-000000aaa0a0"
var MockStateFQDN string = fmt.Sprintf("external-dns-%s.", MockUUID)

func NewMockDnsRecord(fqdn string, ttl int, recType string, value string) utils.DnsRecord {
	if len(recType) <= 0 {
		recType = "TXT"
	}

	return utils.DnsRecord{
		Fqdn: fqdn,
		Records: []string{
			value,
			fqdn,
		},
		Type: recType,
		TTL:  ttl,
	}
}

func NewMockMetaDataRecord(serviceName string, stackName string, fqdn string) utils.MetadataDnsRecord {
	return utils.MetadataDnsRecord{
		ServiceName: serviceName,
		StackName:   stackName,
		DnsRecord:   NewMockDnsRecord(fqdn, 300, "TXT", "Testing123"),
	}
}