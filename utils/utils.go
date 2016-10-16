package utils

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/valyala/fasttemplate"
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

func FqdnFromTemplate(template, serviceName, stackName, environmentName, rootDomainName string) string {
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

func GetStateFqdn(environmentUUID, rootDomainName string) string {
	labels := []string{"external-dns-" + environmentUUID, rootDomainName}
	return strings.ToLower(strings.Join(labels, "."))
}

// sanitizeLabel replaces characters that are not allowed in DNS labels with dashes.
// According to RFC 1123 the only characters allowed in DNS labels are A-Z, a-z, 0-9
// and dashes ("-"). The latter must not appear at the start or end of a label.
func sanitizeLabel(label string) string {
	re := regexp.MustCompile("[^a-zA-Z0-9-]")
	dashes := regexp.MustCompile("[-]+")
	label = re.ReplaceAllString(label, "-")
	label = dashes.ReplaceAllString(label, "-")
	return strings.Trim(label, "-")
}
