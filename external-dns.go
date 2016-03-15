package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	"strings"
)

func UpdateProviderDnsRecords(metadataRecs map[string]dns.DnsRecord) ([]dns.DnsRecord, error) {
	var updated []dns.DnsRecord
	providerRecs, err := getProviderDnsRecords()
	if err != nil {
		return nil, fmt.Errorf("Provider error reading dns entries: %v", err)
	}
	logrus.Debugf("DNS records from provider: %v", providerRecs)

	removeExtraRecords(metadataRecs, providerRecs)

	updated = append(updated, addMissingRecords(metadataRecs, providerRecs)...)

	updated = append(updated, updateExistingRecords(metadataRecs, providerRecs)...)

	return updated, nil
}

func addMissingRecords(metadataRecs map[string]dns.DnsRecord, providerRecs map[string]dns.DnsRecord) []dns.DnsRecord {
	var toAdd []dns.DnsRecord
	for key := range metadataRecs {
		if _, ok := providerRecs[key]; !ok {
			toAdd = append(toAdd, metadataRecs[key])
		}
	}
	if len(toAdd) == 0 {
		logrus.Debug("No DNS records to add")
	} else {
		logrus.Infof("DNS records to add: %v", toAdd)
	}
	return updateRecords(toAdd, &Add)
}

func updateRecords(toChange []dns.DnsRecord, op *Op) []dns.DnsRecord {
	var changed []dns.DnsRecord
	for _, value := range toChange {
		switch *op {
		case Add:
			logrus.Infof("Adding dns record: %v", value)
			if err := provider.AddRecord(value); err != nil {
				logrus.Errorf("Failed to add DNS record to provider %v: %v", value, err)
			} else {
				changed = append(changed, value)
			}
		case Remove:
			logrus.Infof("Removing dns record: %v", value)
			if err := provider.RemoveRecord(value); err != nil {
				logrus.Errorf("Failed to remove DNS record from provider %v: %v", value, err)
			}
		case Update:
			logrus.Infof("Updating dns record: %v", value)
			if err := provider.UpdateRecord(value); err != nil {
				logrus.Errorf("Failed to update DNS record to provider %v: %v", value, err)
			} else {
				changed = append(changed, value)
			}
		}
	}
	return changed
}

func updateExistingRecords(metadataRecs map[string]dns.DnsRecord, providerRecs map[string]dns.DnsRecord) []dns.DnsRecord {
	var toUpdate []dns.DnsRecord
	for key := range metadataRecs {
		if _, ok := providerRecs[key]; ok {
			metadataR := make(map[string]struct{}, len(metadataRecs[key].Records))
			for _, s := range metadataRecs[key].Records {
				metadataR[s] = struct{}{}
			}

			providerR := make(map[string]struct{}, len(providerRecs[key].Records))
			for _, s := range providerRecs[key].Records {
				providerR[s] = struct{}{}
			}
			var update bool
			if len(metadataR) != len(providerR) {
				update = true
			} else {
				for key := range metadataR {
					if _, ok := providerR[key]; !ok {
						update = true
					}
				}
				for key := range providerR {
					if _, ok := metadataR[key]; !ok {
						update = true
					}
				}
			}
			if update {
				toUpdate = append(toUpdate, metadataRecs[key])
			}
		}
	}

	if len(toUpdate) == 0 {
		logrus.Debug("No DNS records to update")
	} else {
		logrus.Infof("DNS records to update: %v", toUpdate)
	}

	return updateRecords(toUpdate, &Update)
}

func removeExtraRecords(metadataRecs map[string]dns.DnsRecord, providerRecs map[string]dns.DnsRecord) []dns.DnsRecord {
	var toRemove []dns.DnsRecord
	for key := range providerRecs {
		if _, ok := metadataRecs[key]; !ok {
			toRemove = append(toRemove, providerRecs[key])
		}
	}

	if len(toRemove) == 0 {
		logrus.Debug("No DNS records to remove")
	} else {
		logrus.Infof("DNS records to remove: %v", toRemove)
	}
	return updateRecords(toRemove, &Remove)
}

func getProviderDnsRecords() (map[string]dns.DnsRecord, error) {
	allRecords, err := provider.GetRecords()
	if err != nil {
		return nil, err
	}
	ourRecords := make(map[string]dns.DnsRecord, len(allRecords))
	joins := []string{m.EnvironmentName, dns.RootDomainName}
	suffix := "." + strings.ToLower(strings.Join(joins, "."))
	for _, value := range allRecords {
		if value.Type == "A" && strings.HasSuffix(value.Fqdn, suffix) && value.TTL == dns.TTL {
			ourRecords[value.Fqdn] = value
		}
	}
	return ourRecords, nil
}
