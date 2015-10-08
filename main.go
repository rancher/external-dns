package main

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/metadata"
	"github.com/rancher/external-dns/providers"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	//FIXME - change metadata url to rancher-metadata
	metadataUrl = "http://localhost:90"
	poll        = 1000
)

type Op struct {
	Name string
}

var (
	Add    = Op{Name: "Add"}
	Remove = Op{Name: "Remove"}
	Update = Op{Name: "Update"}
)

var (
	providerName = flag.String("provider", "", "External provider name")
	debug        = flag.Bool("debug", false, "Debug")
	logFile      = flag.String("log", "", "Log file")
	stack        metadata.Stack
	provider     providers.Provider
	m            metadata.MetadataHandler
)

func setEnv() {
	flag.Parse()
	provider = providers.GetProvider(*providerName)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}
	if *logFile != "" {
		if output, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
			log.Fatalf("Failed to log to file %s: %v", *logFile, err)
		} else {
			log.SetOutput(output)
			formatter := &log.TextFormatter{
				FullTimestamp: true,
			}
			log.SetFormatter(formatter)
		}
	}
	m = metadata.NewHandler(metadataUrl)
	selfStack, err := m.GetSelfStack()
	if err != nil {
		log.Errorf("Error reading stack info: %v", err)
	}
	stack = selfStack
}

func main() {
	log.Infof("Starting Rancher External DNS service")
	setEnv()
	log.Infof("Powered by %s", provider.GetName())

	version := "init"

	for {
		newVersion, err := m.GetVersion()
		if err != nil {
			log.Errorf("Error reading version: %v", err)
		} else if version == newVersion {
			log.Debug("No changes in version: %s", newVersion)
		} else {
			log.Debug("Version has been changed. Old version: %s. New version: %s.", version, newVersion)
			err := updateDnsRecords(m)
			if err != nil {
				log.Errorf("Failed to update DNS records: %v", err)
			}
			version = newVersion
		}
		time.Sleep(time.Duration(poll) * time.Millisecond)
	}
}

func updateDnsRecords(m metadata.MetadataHandler) error {
	metadataRecs, err := getMetadataDnsRecords(m)
	if err != nil {
		log.Errorf("Error reading external dns entries: %v", err)
	}
	log.Debugf("DNS records from metadata: %v", metadataRecs)

	providerRecs, err := getProviderDnsRecords()
	if err != nil {
		log.Errorf("Provider error reading dns entries: %v", err)
	}

	log.Debugf("DNS records from provider: %v", providerRecs)
	err = addMissingRecords(metadataRecs, providerRecs)
	if err != nil {
		log.Errorf("Failed to add missing records: %v", err)
	}
	err = removeExtraRecords(metadataRecs, providerRecs)
	if err != nil {
		log.Errorf("Failed to remove extra records: %v", err)
	}
	err = updateExistingRecords(metadataRecs, providerRecs)
	if err != nil {
		log.Errorf("Failed to update existing records records: %v", err)
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
		log.Debug("No DNS records to add")
		return nil
	} else {
		log.Infof("DNS records to add: %v", toAdd)
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
				log.Infof("Adding dns record: %v", value)
				err := provider.AddRecord(value)
				if err != nil {
					return fmt.Errorf("Failed to add DNS record %v: %v", value, err)
				}
			case Remove:
				log.Infof("Removing dns record: %v", value)
				err := provider.RemoveRecord(value)
				if err != nil {
					return fmt.Errorf("Failed to remove DNS record %v: %v", value, err)
				}
			case Update:
				log.Infof("Updating dns record: %v", value)
				err := provider.UpdateRecord(value)
				if err != nil {
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
		log.Debug("No DNS records to update")
		return nil
	} else {
		log.Infof("DNS records to update: %v", toUpdate)
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
		log.Debug("No DNS records to remove")
		return nil
	} else {
		log.Infof("DNS records to remove: %v", toRemove)
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
