package utils

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/valyala/fasttemplate"
)

const (
	stateRecordFqdnTemplate = "external-dns-%s.%s"
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

func StateFqdn(environmentUUID, rootDomainName string) string {
	fqdn := fmt.Sprintf(stateRecordFqdnTemplate, environmentUUID, rootDomainName)
	return strings.ToLower(fqdn)
}

func StateRecord(fqdn string, ttl int, entries map[string]struct{}) DnsRecord {
	records := make([]string, len(entries))
	idx := 0
	for entry, _ := range entries {
		records[idx] = entry
		idx++
	}
	sort.Strings(records)
	return DnsRecord{fqdn, records, "TXT", ttl}
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
