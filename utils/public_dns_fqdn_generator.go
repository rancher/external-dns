package utils

import (
	"fmt"
	logrus "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/valyala/fasttemplate"
	"io"
	"strings"
)

type PublicDNSFQDNGenerator struct {
}

func init() {
	publicDNSFQDNGenerator := &PublicDNSFQDNGenerator{}

	if err := RegisterFQDNGenerator(publicDNSFQDNGenerator.GetName(), publicDNSFQDNGenerator); err != nil {
		logrus.Fatalf("Could not register %s", publicDNSFQDNGenerator.GetName())
	}
}

func (*PublicDNSFQDNGenerator) GetName() string {
	return "PublicDNSFQDNGenerator"
}

func (*PublicDNSFQDNGenerator) GetDefaultTemplate() string {
	return "%{{service_name}}-%{{stack_uuid}}"
}

func (pf *PublicDNSFQDNGenerator) GenerateFQDN(template string, container metadata.Container, environmentName string, rootDomainName string) string {
	serviceName := container.ServiceName
	if container.ServiceName == "" && container.StackName != "" {
		serviceName = container.Name
	}
	return pf.fqdnFromTemplate(template, serviceName, container.StackUUID, container.StackName, environmentName, rootDomainName)
}

func (*PublicDNSFQDNGenerator) fqdnFromTemplate(template, serviceName, stackUUID, stackName, environmentName, rootDomainName string) string {
	t, err := fasttemplate.NewTemplate(template, "%{{", "}}")
	if err != nil {
		logrus.Fatalf("error while parsing fqdn template: %s", err)
	}

	fqdn := t.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		switch tag {
		case "service_name":
			return w.Write([]byte(sanitizeLabel(serviceName)))
		case "stack_uuid":
			if stackUUID != "" {
				return w.Write([]byte(sanitizeLabel(stackRandom(stackUUID))))
			} else {
				return w.Write([]byte(sanitizeLabel("nSUUID")))
			}
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

func stackRandom(stackUUID string) string {
	return stackUUID[0:6]
}
