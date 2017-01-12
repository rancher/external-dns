package main

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/config"
	"github.com/rancher/external-dns/utils"
)

func UpdateProviderDnsRecords(metadataRecs map[string]utils.DnsRecord) ([]utils.DnsRecord, error) {
	var updated []utils.DnsRecord
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

func addMissingRecords(metadataRecs map[string]utils.DnsRecord, providerRecs map[string]utils.DnsRecord) []utils.DnsRecord {
	var toAdd []utils.DnsRecord
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

func updateRecords(toChange []utils.DnsRecord, op *Op) []utils.DnsRecord {
	var changed []utils.DnsRecord
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

func updateExistingRecords(metadataRecs map[string]utils.DnsRecord, providerRecs map[string]utils.DnsRecord) []utils.DnsRecord {
	var toUpdate []utils.DnsRecord
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

func removeExtraRecords(metadataRecs map[string]utils.DnsRecord, providerRecs map[string]utils.DnsRecord) []utils.DnsRecord {
	var toRemove []utils.DnsRecord
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

func getProviderDnsRecords() (map[string]utils.DnsRecord, error) {
	allRecords, err := provider.GetRecords()
	if err != nil {
		return nil, err
	}

	ourRecords := make(map[string]utils.DnsRecord, len(allRecords))
	if len(allRecords) == 0 {
		return ourRecords, nil
	}

	stateFqdn := utils.StateFqdn(m.EnvironmentUUID, config.RootDomainName)
	ourFqdns := make(map[string]struct{})

	// Get the FQDNs that were created by us from the state RRSet
	logrus.Debugf("Checking for state RRSet %s", stateFqdn)
	for _, rec := range allRecords {
		if rec.Fqdn == stateFqdn && rec.Type == "TXT" {
			logrus.Debugf("FQDNs from state RRSet: %v", rec.Records)
			for _, value := range rec.Records {
				ourFqdns[value] = struct{}{}
			}
			ourRecords[stateFqdn] = rec
			break
		}
	}

	for _, rec := range allRecords {
		_, ok := ourFqdns[rec.Fqdn]
		if ok && rec.Type == "A" {
			ourRecords[rec.Fqdn] = rec
		}
	}
	return ourRecords, nil
}

// upgrade path from previous versions of external-dns.
// checks for any pre-existing A records with names matching the legacy
// suffix and TTLs matching the value of config.TTL. If any are found,
// a state RRSet is created in the zone using the FQDNs of the records
// as values.
func EnsureUpgradeToStateRRSet() error {
	allRecords, err := provider.GetRecords()
	if err != nil {
		return err
	}

	stateFqdn := utils.StateFqdn(m.EnvironmentUUID, config.RootDomainName)
	logrus.Debugf("Checking for state RRSet %s", stateFqdn)
	for _, rec := range allRecords {
		if rec.Fqdn == stateFqdn && rec.Type == "TXT" {
			logrus.Debugf("Found state RRSet with %d records", len(rec.Records))
			return nil
		}
	}

	logrus.Debug("State RRSet not found")
	ourFqdns := make(map[string]struct{})
	// records created by previous versions will match this suffix
	joins := []string{m.EnvironmentName, config.RootDomainName}
	suffix := "." + strings.ToLower(strings.Join(joins, "."))
	logrus.Infof("DNS suffix: %s", suffix)
	for _, value := range allRecords {
		if value.Type == "A" && strings.HasSuffix(value.Fqdn, suffix) && value.TTL == config.TTL {
			ourRecords[value.Fqdn] = struct{}{}
		}
	}

	if len(ourFqdns) > 0 {
		logrus.Infof("Creating RRSet '%s TXT' for %d pre-existing records", stateFqdn, len(ourFqdns))
		stateRec := utils.StateRecord(stateFqdn, config.TTL, ourFqdns)
		if err := provider.AddRecord(stateRec); err != nil {
			return fmt.Errorf("Failed to add RRSet to provider %v: %v", stateRec, err)
		}
	}

	return nil
}
