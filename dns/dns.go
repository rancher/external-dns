package dns

import (
	"github.com/Sirupsen/logrus"
	"os"
	"strconv"
	"strings"
)

var (
	RootDomainName string
	TTL            int
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

func init() {
	var name string
	name = os.Getenv("ROOT_DOMAIN")
	if len(name) == 0 {
		logrus.Fatalf("ROOT_DOMAIN is not set")
	}
	TTLEnv := os.Getenv("TTL")
	i, err := strconv.Atoi(TTLEnv)
	if err != nil {
		TTL = 300
	} else {
		TTL = i
	}

	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	RootDomainName = name
}

func ConvertToServiceDnsRecord(dnsRecord DnsRecord) ServiceDnsRecord {
	splitted := strings.Split(dnsRecord.Fqdn, ".")
	serviceRecord := ServiceDnsRecord{dnsRecord.Fqdn, splitted[0], splitted[1]}
	return serviceRecord
}

func ConvertToFqdn(serviceName string, stackName string, environmentName string) string {
	domainNameEntries := []string{serviceName, stackName, environmentName, RootDomainName}
	return strings.ToLower(strings.Join(domainNameEntries, "."))
}
