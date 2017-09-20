package utils

import (
	"fmt"
	logrus "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/valyala/fasttemplate"
	"io"
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

func (*DefaultFQDNGenerator) GetDefaultTemplate() string {
	return "%{{service_name}}.%{{stack_name}}.%{{environment_name}}"
}

func (df *DefaultFQDNGenerator) GenerateFQDN(template string, container metadata.Container, environmentName string, rootDomainName string) string {
	serviceName := container.ServiceName
	stackName := container.StackName
	if container.ServiceName == "" && container.StackName != "" {
		serviceName = container.Name
	}

	return df.fqdnFromTemplate(template, serviceName, stackName, environmentName, rootDomainName)
}

func (*DefaultFQDNGenerator) fqdnFromTemplate(template, serviceName, stackName, environmentName, rootDomainName string) string {
	t, err := fasttemplate.NewTemplate(template, "%{{", "}}")
	if err != nil {
		logrus.Fatalf("error while parsing fqdn template: %s", err)
	}

	fqdn := t.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		switch tag {
		case "service_name":
			return w.Write([]byte(sanitizeLabel(serviceName)))
		case "stack_name":
			return w.Write([]byte(sanitizeLabel(stackName)))
		case "environment_name":
			return w.Write([]byte(sanitizeLabel(environmentName)))
		default:
			return 0, fmt.Errorf("invalid placeholder '%q' in fqdn template", tag)
		}
	})

	labels := []string{fqdn, rootDomainName}
	return strings.ToLower(strings.Join(labels, "."))
}
