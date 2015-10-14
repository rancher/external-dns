package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/metadata"
	"github.com/rancher/external-dns/providers"
	"strings"
	"sync"
)

func UpdateDnsRecords(m metadata.MetadataHandler) error {
	metadataRecs, err := getMetadataDnsRecords(m)
	if err != nil {
		return fmt.Errorf("Error reading external dns entries: %v", err)
	}
	logrus.Debugf("DNS records from metadata: %v", metadataRecs)

	providerRecs, err := getProviderDnsRecords()
	if err != nil {
		return fmt.Errorf("Provider error reading dns entries: %v", err)
	}

	logrus.Debugf("DNS records from provider: %v", providerRecs)
	if err = addMissingRecords(metadataRecs, providerRecs); err != nil {
		return fmt.Errorf("Failed to add missing records: %v", err)
	}

	if err = removeExtraRecords(metadataRecs, providerRecs); err != nil {
		return fmt.Errorf("Failed to remove extra records: %v", err)
	}

	if err = updateExistingRecords(metadataRecs, providerRecs); err != nil {
		return fmt.Errorf("Failed to update existing records records: %v", err)
	}
	return nil
}

func addMissingRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) error {
	var toAdd []providers.DnsRecord
	for key, _ := range metadataRecs {
		if _, ok := providerRecs[key]; !ok {
			toAdd = append(toAdd, metadataRecs[key])
		}
	}
	if len(toAdd) == 0 {
		logrus.Debug("No DNS records to add")
		return nil
	} else {
		logrus.Infof("DNS records to add: %v", toAdd)
	}
	return updateRecords(toAdd, &Add)
}

func updateRecords(toChange []providers.DnsRecord, op *Op) error {
	values := make(chan providers.DnsRecord)
	var wg sync.WaitGroup
	wg.Add(len(toChange))

	for _, value := range toChange {
		go func(value providers.DnsRecord) {
			defer wg.Done()
			values <- value
		}(value)
	}

	go func() error {
		for value := range values {
			switch *op {
			case Add:
				logrus.Infof("Adding dns record: %v", value)
				if err := provider.AddRecord(value); err != nil {
					return fmt.Errorf("Failed to add DNS record %v: %v", value, err)
				}
			case Remove:
				logrus.Infof("Removing dns record: %v", value)

				if err := provider.RemoveRecord(value); err != nil {
					return fmt.Errorf("Failed to remove DNS record %v: %v", value, err)
				}
			case Update:
				logrus.Infof("Updating dns record: %v", value)
				if err := provider.UpdateRecord(value); err != nil {
					return fmt.Errorf("Failed to update DNS record %v: %v", value, err)
				}
			}
		}
		return nil
	}()
	wg.Wait()
	return nil
}

func updateExistingRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) error {
	var toUpdate []providers.DnsRecord
	for key, _ := range metadataRecs {
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
				for key, _ := range metadataR {
					if _, ok := providerR[key]; !ok {
						update = true
					}
				}
				for key, _ := range providerR {
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
		return nil
	} else {
		logrus.Infof("DNS records to update: %v", toUpdate)
	}

	return updateRecords(toUpdate, &Update)
}

func removeExtraRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) error {
	var toRemove []providers.DnsRecord
	for key, _ := range providerRecs {
		if _, ok := metadataRecs[key]; !ok {
			toRemove = append(toRemove, providerRecs[key])
		}
	}

	if len(toRemove) == 0 {
		logrus.Debug("No DNS records to remove")
		return nil
	} else {
		logrus.Infof("DNS records to remove: %v", toRemove)
	}
	return updateRecords(toRemove, &Remove)
}

func getProviderDnsRecords() (map[string]providers.DnsRecord, error) {
	allRecords, err := provider.GetRecords()
	if err != nil {
		return nil, err
	}
	ourRecords := make(map[string]providers.DnsRecord, len(allRecords))
	joins := []string{stack.EnvironmentName, providers.RootDomainName}
	suffix := strings.ToLower(strings.Join(joins, "."))
	for _, value := range allRecords {
		if strings.HasSuffix(value.DomainName, suffix) {
			ourRecords[value.DomainName] = value
		}
	}
	return ourRecords, nil
}

func getMetadataDnsRecords(m metadata.MetadataHandler) (map[string]providers.DnsRecord, error) {

	containers, err := m.GetContainers()
	if err != nil {
		return nil, err
	}

	dnsEntries := make(map[string]providers.DnsRecord)
	for _, container := range containers {
		if container.StackName == stack.Name {
			hostUUID := container.HostUUID
			if len(hostUUID) == 0 {
				logrus.Debugf("Container's %v host_uuid is empty", container.Name)
				continue
			}
			host, err := m.GetHost(hostUUID)
			if err != nil {
				logrus.Infof("%v", err)
				continue
			}
			ip := host.AgentIP
			domainNameEntries := []string{container.ServiceName, container.StackName, stack.EnvironmentName, providers.RootDomainName}
			domainName := strings.ToLower(strings.Join(domainNameEntries, "."))
			var dnsEntry providers.DnsRecord
			var records []string
			if _, ok := dnsEntries[domainName]; ok {
				records = []string{ip}
			} else {
				records = dnsEntries[domainName].Records
				records = append(records, ip)
			}
			dnsEntry = providers.DnsRecord{domainName, records, "A", 300}
			dnsEntries[domainName] = dnsEntry
		}
	}
	records := make(map[string]providers.DnsRecord, len(dnsEntries))
	for _, value := range dnsEntries {
		records[value.DomainName] = value
	}
	return records, nil
}
