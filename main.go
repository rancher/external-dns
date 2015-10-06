package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/metadata"
	"github.com/rancher/external-dns/providers"
	"strings"
	"time"
)

const (
	//FIXME - change metadata url to rancher-metadata
	metadataUrl = "http://localhost:90"
	poll        = 1000
)

var (
	providerName = flag.String("provider", "", "External provider name")
	stack        metadata.Stack
	provider     providers.Provider
	m            metadata.MetadataHandler
)

func init() {
	provider = providers.GetProvider(*providerName)
	flag.Parse()
	m = metadata.NewHandler(metadataUrl)
}

func main() {
	log.Infof("Starting Rancher External DNS powered by %s", provider.GetName())
	version := ""
	selfStack, err := m.GetSelfStack()
	if err != nil {
		log.Errorf("Error reading stack info: %v", err)
	}
	stack = selfStack

	for {
		newVersion, err := m.GetVersion()
		if err != nil {
			log.Errorf("Error reading version: %v", err)
		} else if version == newVersion {
			log.Debug("No changes in version: %s", newVersion)
		} else {
			log.Debug("Version has been changed. Old version: %s. New version: %s.", version, newVersion)
			ChangeDnsRecords(m)
			version = newVersion
		}
		time.Sleep(time.Duration(poll) * time.Millisecond)
	}
}

func ChangeDnsRecords(m metadata.MetadataHandler) error {
	metadataRecs, err := GetMetadataDnsRecords(m)
	if err != nil {
		log.Errorf("Error reading external dns entries: %v", err)
	}
	log.Infof("DNS records from metadata: %v", metadataRecs)

	providerRecs, err := GetProviderDnsRecords()
	if err != nil {
		log.Errorf("Provider error reading dns entries: %v", err)
	}

	log.Infof("DNS records from provider: %v", providerRecs)
	addMissingRecords(metadataRecs, providerRecs)
	removeExtraRecords(metadataRecs, providerRecs)
	updateExistingRecords(metadataRecs, providerRecs)

	return nil
}

func addMissingRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) error {
	var toAdd []providers.DnsRecord
	for key, _ := range metadataRecs {
		if _, ok := providerRecs[key]; !ok {
			toAdd = append(toAdd, metadataRecs[key])
		}
	}
	for _, value := range toAdd {
		log.Infof("Adding dns record: %v", value)
		err := provider.AddRecord(value)
		if err != nil {
			log.Errorf("Failed to add DNS record due to %v", err)
			return err
		}
	}
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
	for _, value := range toUpdate {
		log.Infof("Updating dns record: %v", value)
		err := provider.AddRecord(value)
		if err != nil {
			log.Errorf("Failed to update DNS record due to %v", err)
			return err
		}
	}
	return nil
}

func removeExtraRecords(metadataRecs map[string]providers.DnsRecord, providerRecs map[string]providers.DnsRecord) error {
	var toRemove []providers.DnsRecord
	for key, _ := range providerRecs {
		if _, ok := metadataRecs[key]; !ok {
			toRemove = append(toRemove, providerRecs[key])
		}
	}

	for _, value := range toRemove {
		log.Infof("Removing dns record: %v", value)
		err := provider.RemoveRecord(value)
		if err != nil {
			log.Errorf("Failed to remove DNS record due to %v", err)
			return err
		}
	}
	return nil
}

func GetProviderDnsRecords() (map[string]providers.DnsRecord, error) {
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

func GetMetadataDnsRecords(m metadata.MetadataHandler) (map[string]providers.DnsRecord, error) {

	containers, err := m.GetContainers()
	if err != nil {
		return nil, err
	}

	dnsEntries := make(map[string]providers.DnsRecord)
	for _, container := range containers {
		if container.StackName == stack.Name {
			hostUUID := container.HostUUID
			if len(hostUUID) == 0 {
				log.Debugf("Container's %v host_uuid is empty", container.Name)
				continue
			}
			host, err := m.GetHost(hostUUID)
			if err != nil {
				log.Infof("%v", err)
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
