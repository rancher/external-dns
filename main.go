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
	metadataUrl = "http://localhost:90"
	poll        = 1000
)

var (
	providerName = flag.String("provider", "", "External provider name")
)

func main() {
	log.Info("Starting Rancher External DNS")
	flag.Parse()
	metadataHandler := metadata.NewHandler(metadataUrl)
	version := ""

	for {
		newVersion, err := metadataHandler.GetVersion()
		if err != nil {
			log.Errorf("Error reading version: %v", err)
		} else if version == newVersion {
			log.Debug("No changes in version: %s", newVersion)
		} else {
			log.Debug("Version has been changed. Old version: %s. New version: %s.", version, newVersion)
			dnsEntries, err := GetExternalDnsRecords(metadataHandler)
			if err != nil {
				log.Errorf("Error reading external dns entries: %v", err)
			}
			log.Info(dnsEntries)
			if *providerName != "" {
				log.Infof("Provider name %s", *providerName)
			}
			provider := providers.GetProvider(*providerName)
			records, err := provider.GetRecords()
			if err != nil {
				log.Errorf("Provider error reading external dns entries: %v", err)
			}

			log.Info(records)

			version = newVersion
		}
		time.Sleep(time.Duration(poll) * time.Millisecond)
	}
}

func GetExternalDnsRecords(m metadata.MetadataHandler) (map[string]providers.ExternalDnsEntry, error) {
	dnsEntries := make(map[string]providers.ExternalDnsEntry)
	stack, err := m.GetStack()
	if err != nil {
		return dnsEntries, err
	}
	containers, err := m.GetContainers()
	if err != nil {
		return dnsEntries, err
	}

	for _, container := range containers {
		if container.StackName == stack.Name {
			ip := container.PrimaryIp
			domainNameEntries := []string{container.ServiceName, container.StackName, stack.EnvironmentName}
			domainName := strings.ToLower(strings.Join(domainNameEntries, "."))
			var dnsEntry providers.ExternalDnsEntry
			var records []string
			if _, ok := dnsEntries[domainName]; ok {
				records = []string{ip}
			} else {
				records = dnsEntries[domainName].ARecords
				records = append(records, ip)
			}
			dnsEntry = providers.ExternalDnsEntry{domainName, records}
			dnsEntries[domainName] = dnsEntry
		}
	}
	return dnsEntries, nil
}
