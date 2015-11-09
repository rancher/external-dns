package metadata

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	"github.com/rancher/go-rancher-metadata/metadata"
)

const (
	metadataUrl = "http://rancher-metadata/latest"
)

type MetadataClient struct {
	MetadataClient  *metadata.Client
	EnvironmentName string
}

func NewMetadataClient() (*MetadataClient, error) {
	m, err := metadata.NewClientAndWait(metadataUrl)
	if err != nil {
		logrus.Fatalf("Failed to configure rancher-metadata: %v", err)
	}
	selfStack, err := m.GetSelfStack()
	if err != nil {
		logrus.Fatalf("Error reading stack info: %v", err)
	}

	return &MetadataClient{
		MetadataClient:  m,
		EnvironmentName: selfStack.EnvironmentName,
	}, nil
}

func (m *MetadataClient) GetVersion() (string, error) {
	return m.MetadataClient.GetVersion()
}

func (m *MetadataClient) GetMetadataDnsRecords() (map[string]dns.DnsRecord, error) {
	dnsEntries := make(map[string]dns.DnsRecord)
	err := m.getContainersDnsRecords(dnsEntries, "", "")
	if err != nil {
		return dnsEntries, err
	}
	return dnsEntries, nil
}

func (m *MetadataClient) getContainersDnsRecords(dnsEntries map[string]dns.DnsRecord, serviceName string, stackName string) error {
	containers, err := m.MetadataClient.GetContainers()
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

		if len(container.Ports) == 0 {
			continue
		}

		hostUUID := container.HostUUID
		if len(hostUUID) == 0 {
			logrus.Debugf("Container's %v host_uuid is empty", container.Name)
			continue
		}
		host, err := m.MetadataClient.GetHost(hostUUID)
		if err != nil {
			logrus.Infof("%v", err)
			continue
		}
		ip := host.AgentIP
		fqdn := dns.ConvertToFqdn(container.ServiceName, container.StackName, m.EnvironmentName)
		records := []string{ip}
		dnsEntry := dns.DnsRecord{fqdn, records, "A", dns.TTL}

		addToDnsEntries(dnsEntry, dnsEntries)
	}

	return nil
}

func addToDnsEntries(dnsEntry dns.DnsRecord, dnsEntries map[string]dns.DnsRecord) {
	var records []string
	if _, ok := dnsEntries[dnsEntry.Fqdn]; !ok {
		records = dnsEntry.Records
	} else {
		records = dnsEntries[dnsEntry.Fqdn].Records
		records = append(records, dnsEntry.Records...)
	}
	dnsEntry = dns.DnsRecord{dnsEntry.Fqdn, records, "A", dns.TTL}
	dnsEntries[dnsEntry.Fqdn] = dnsEntry
}
