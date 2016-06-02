package utils

import (
	"strings"
)

type DnsRecord struct {
	Fqdn    string
	Records []string
	Type    string
	TTL     int
}

type ServiceDnsRecord struct {
	Fqdn        string
	ServiceName string
	StackName   string
}

func ConvertToServiceDnsRecord(dnsRecord DnsRecord) ServiceDnsRecord {
	splitted := strings.Split(dnsRecord.Fqdn, ".")
	serviceRecord := ServiceDnsRecord{dnsRecord.Fqdn, splitted[0], splitted[1]}
	return serviceRecord
}

func ConvertToFqdn(serviceName, stackName, environmentName, rootDomainName string) string {
	labels := []string{serviceName, stackName, environmentName, rootDomainName}
	return strings.ToLower(strings.Join(labels, "."))
}

// Fqdn ensures that the name is a fqdn adding a trailing dot if necessary.
func Fqdn(name string) string {
	n := len(name)
	if n == 0 || name[n-1] == '.' {
		return name
	}
	return name + "."
}

// UnFqdn converts the fqdn into a name removing the trailing dot.
func UnFqdn(name string) string {
	n := len(name)
	if n != 0 && name[n-1] == '.' {
		return name[:n-1]
	}
	return name
}
