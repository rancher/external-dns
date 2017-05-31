package utils

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	stateRecordFqdnTemplate = "external-dns-%s.%s"
)

// MetadataDnsRecord is a wrapper around a DnsRecord
// that holds information about the service and stack
// the record belongs to
type MetadataDnsRecord struct {
	ServiceName string
	StackName   string
	DnsRecord   DnsRecord
}

// DnsRecord represents a provider DNS record
type DnsRecord struct {
	Fqdn    string
	Records []string
	Type    string
	TTL     int
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
