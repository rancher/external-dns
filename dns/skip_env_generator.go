package dns

import (
	logrus "github.com/Sirupsen/logrus"
	"strings"
)

type SkipEnvGenerator struct {
}

func init() {
	skipEnvGenerator := &SkipEnvGenerator{}

	if err := RegisterFQDNGenerator(skipEnvGenerator.GetName(), skipEnvGenerator); err != nil {
		logrus.Fatalf("Could not register %s", skipEnvGenerator.GetName())
	}
}

func (*SkipEnvGenerator) GetName() string {
	return "SkipEnvGenerator"
}

func (*SkipEnvGenerator) GenerateFQDN(serviceName string, stackName string, environmentName string, rootDomainName string) string {
	domainNameEntries := []string{serviceName, stackName, rootDomainName}
	return strings.ToLower(strings.Join(domainNameEntries, "."))
}
