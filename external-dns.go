package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/providers"
	"strings"
	"time"
)

func UpdateProviderDnsRecords(metadataRecs map[string]providers.DnsRecord) error {
	providerRecs, err := getProviderDnsRecords()
	if err != nil {
		return fmt.Errorf("Provider error reading dns entries: %v", err)
	}
	logrus.Debugf("DNS records from provider: %v", providerRecs)

	addMissingRecords(metadataRecs, providerRecs)

	removeExtraRecords(metadataRecs, providerRecs)

	updateExistingRecords(metadataRecs, providerRecs)

	return nil
}

func addMissingRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) {
	var toAdd []providers.DnsRecord
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
	updateRecords(toAdd, &Add)
}

func updateRecords(toChange []providers.DnsRecord, op *Op) {
	count := 0
	for _, value := range toChange {
		updateCattle := false
		switch *op {
		case Add:
			logrus.Infof("Adding dns record: %v", value)
			if err := provider.AddRecord(value); err != nil {
				logrus.Errorf("Failed to add DNS record to provider %v: %v", value, err)
			} else {
				updateCattle = true
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
				updateCattle = true
			}
		}
		if updateCattle {
			serviceDnsRecord := convertToServiceDnsRecord(value)
			c.UpdateServiceDomainName(serviceDnsRecord)
		}

		// to workaround rate limit on Amazon (5 requests per second)
		count = count + 1
		if count == 5 {
			time.Sleep(1000 * time.Millisecond)
			count = 0
		}
	}
}

func updateExistingRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) {
	var toUpdate []providers.DnsRecord
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

	updateRecords(toUpdate, &Update)
}

func removeExtraRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) {
	var toRemove []providers.DnsRecord
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
	updateRecords(toRemove, &Remove)
}

func getProviderDnsRecords() (map[string]providers.DnsRecord, error) {
	allRecords, err := provider.GetRecords()
	if err != nil {
		return nil, err
	}
	ourRecords := make(map[string]providers.DnsRecord, len(allRecords))
	joins := []string{EnvironmentName, providers.RootDomainName}
	suffix := strings.ToLower(strings.Join(joins, "."))
	for _, value := range allRecords {
		if strings.HasSuffix(value.DomainName, suffix) {
			ourRecords[value.DomainName] = value
		}
	}
	return ourRecords, nil
}

func addToDnsEntries(dnsEntry providers.DnsRecord, dnsEntries map[string]providers.DnsRecord) {
	var records []string
	if _, ok := dnsEntries[dnsEntry.DomainName]; !ok {
		records = dnsEntry.Records
	} else {
		records = dnsEntries[dnsEntry.DomainName].Records
		records = append(records, dnsEntry.Records...)
	}
	dnsEntry = providers.DnsRecord{dnsEntry.DomainName, records, "A", 300}
	dnsEntries[dnsEntry.DomainName] = dnsEntry
}
