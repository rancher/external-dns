package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/go-rancher-metadata/metadata"
	"strings"
)

type ServiceDnsRecord struct {
	DomainName  string
	ServiceName string
	StackName   string
}

func getMetadataDnsRecords(m metadata.Handler) (map[string]providers.DnsRecord, error) {
	dnsEntries := make(map[string]providers.DnsRecord)
	err := getContainersDnsRecords(m, dnsEntries, "", "")
	if err != nil {
		return dnsEntries, err
	}
	return dnsEntries, nil
}

func getContainersDnsRecords(m metadata.Handler, dnsEntries map[string]providers.DnsRecord, serviceName string, stackName string) error {
	containers, err := m.GetContainers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		if len(container.ServiceName) == 0 {
			continue
		}

		if len(serviceName) != 0 {
			if serviceName != container.ServiceName {
				continue
			}
			if stackName != container.StackName {
				continue
			}
		}

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
		domainNameEntries := []string{container.ServiceName, container.StackName, EnvironmentName, providers.RootDomainName}
		domainName := strings.ToLower(strings.Join(domainNameEntries, "."))
		records := []string{ip}
		dnsEntry := providers.DnsRecord{domainName, records, "A", 300}

		addToDnsEntries(m, dnsEntry, dnsEntries)
	}

	return nil
}

func convertToServiceDnsRecord(dnsRecord providers.DnsRecord) ServiceDnsRecord {
	domainName := dnsRecord.DomainName
	splitted := strings.Split(domainName, ".")
	record := ServiceDnsRecord{domainName, splitted[0], splitted[1]}
	return record
}
