package metadata

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/config"
	"github.com/rancher/external-dns/utils"
	"github.com/rancher/go-rancher-metadata/metadata"
	"time"
)

const (
	metadataUrl = "http://rancher-metadata/2015-12-19"
)

type MetadataClient struct {
	MetadataClient  *metadata.Client
	EnvironmentName string
	EnvironmentUUID string
}

func getEnvironment(m *metadata.Client) (string, string, error) {
	timeout := 30 * time.Second
	var err error
	var stack metadata.Stack
	for i := 1 * time.Second; i < timeout; i *= time.Duration(2) {
		stack, err = m.GetSelfStack()
		if err != nil {
			logrus.Errorf("Error reading stack info: %v...will retry", err)
			time.Sleep(i)
		} else {
			return stack.EnvironmentName, stack.EnvironmentUUID, nil
		}
	}
	return "", "", fmt.Errorf("Error reading stack info: %v", err)
}

func NewMetadataClient() (*MetadataClient, error) {
	m, err := metadata.NewClientAndWait(metadataUrl)
	if err != nil {
		logrus.Fatalf("Failed to configure rancher-metadata: %v", err)
	}

	envName, envUUID, err := getEnvironment(m)
	if err != nil {
		logrus.Fatalf("Error reading stack info: %v", err)
	}

	return &MetadataClient{
		MetadataClient:  m,
		EnvironmentName: envName,
		EnvironmentUUID: envUUID,
	}, nil
}

func (m *MetadataClient) GetVersion() (string, error) {
	return m.MetadataClient.GetVersion()
}

func (m *MetadataClient) GetMetadataDnsRecords() (map[string]utils.DnsRecord, error) {
	dnsEntries := make(map[string]utils.DnsRecord)
	err := m.getContainersDnsRecords(dnsEntries, "", "")
	if err != nil {
		return dnsEntries, err
	}
	return dnsEntries, nil
}

func (m *MetadataClient) getContainersDnsRecords(dnsEntries map[string]utils.DnsRecord, serviceName string, stackName string) error {
	containers, err := m.MetadataClient.GetContainers()
	if err != nil {
		return err
	}

	ourFqdns := make(map[string]struct{})
	for _, container := range containers {
		if len(container.ServiceName) == 0 || len(container.Ports) == 0 || !containerStateOK(container) {
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
		host, err := m.MetadataClient.GetHost(hostUUID)
		if err != nil {
			logrus.Infof("%v", err)
			continue
		}

		ip, ok := host.Labels["io.rancher.host.external_dns_ip"]

		if !ok || ip == "" {
			ip = host.AgentIP
		}

		fqdn := utils.FqdnFromTemplate(config.NameTemplate, container.ServiceName, container.StackName,
			m.EnvironmentName, config.RootDomainName)
		records := []string{ip}
		dnsEntry := utils.DnsRecord{fqdn, records, "A", config.TTL}

		addToDnsEntries(dnsEntry, dnsEntries)
		ourFqdns[fqdn] = struct{}{}
	}

	if len(ourFqdns) > 0 {
		records := make([]string, len(ourFqdns))
		idx := 0
		for fqdn, _ := range ourFqdns {
			records[idx] = fqdn
			idx++
		}
		fqdn := utils.GetStateFqdn(m.EnvironmentUUID, config.RootDomainName)
		dnsEntry := utils.DnsRecord{fqdn, records, "TXT", config.TTL}
		addToDnsEntries(dnsEntry, dnsEntries)
	}

	return nil
}

func addToDnsEntries(dnsEntry utils.DnsRecord, dnsEntries map[string]utils.DnsRecord) {
	var records []string
	if _, ok := dnsEntries[dnsEntry.Fqdn]; !ok {
		records = dnsEntry.Records
	} else {
		records = dnsEntries[dnsEntry.Fqdn].Records
		records = append(records, dnsEntry.Records...)
	}
	dnsEntry = utils.DnsRecord{dnsEntry.Fqdn, records, dnsEntry.Type, dnsEntry.TTL}
	dnsEntries[dnsEntry.Fqdn] = dnsEntry
}

func containerStateOK(container metadata.Container) bool {
	switch container.State {
	case "running":
	default:
		return false
	}

	switch container.HealthState {
	case "healthy":
	case "updating-healthy":
	case "":
	default:
		return false
	}

	return true
}
