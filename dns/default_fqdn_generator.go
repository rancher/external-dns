package dns

import (
	logrus "github.com/Sirupsen/logrus"
	"strings"
)

type DefaultFQDNGenerator struct {
}

func init() {
	defaultFQDNGenerator := &DefaultFQDNGenerator{}

	if err := RegisterFQDNGenerator(defaultFQDNGenerator.GetName(), defaultFQDNGenerator); err != nil {
		logrus.Fatalf("Could not register %s", defaultFQDNGenerator.GetName())
	}
}

func (*DefaultFQDNGenerator) GetName() string {
	return "DefaultFQDNGenerator"
}

func (*DefaultFQDNGenerator) GenerateFQDN(serviceName string, stackName string, environmentName string, rootDomainName string) string {
	domainNameEntries := []string{serviceName, stackName, environmentName, rootDomainName}
	return strings.ToLower(strings.Join(domainNameEntries, "."))
}
