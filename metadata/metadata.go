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
	MetadataClient  metadata.Client
	EnvironmentName string
	EnvironmentUUID string
}

func getEnvironment(m metadata.Client) (string, string, error) {
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
	err := m.getContainersDnsRecords(dnsEntries)
	if err != nil {
		return dnsEntries, err
	}
	return dnsEntries, nil
}

func (m *MetadataClient) getContainersDnsRecords(dnsEntries map[string]utils.DnsRecord) error {
	services, err := m.MetadataClient.GetServices()
	if err != nil {
		return err
	}

	ourFqdns := make(map[string]struct{})
	hostMeta := make(map[string]metadata.Host)
	for _, service := range services {

		// Check for Service Label: io.rancher.service.external_dns
		// Accepts 'always', 'auto' (default), or 'never'
		policy, ok := service.Labels["io.rancher.service.external_dns"]
		if !ok {
			policy = "auto"
		} else if policy == "never" {
			logrus.Debugf("Service %v is Disabled", service.Name)
			continue
		}

		if service.Kind != "service" && service.Kind != "loadBalancerService" {
			continue
		}

		for _, container := range service.Containers {

			if (len(container.Ports) == 0 && policy != "always") || !containerStateOK(container) {
				continue
			}

			hostUUID := container.HostUUID
			if len(hostUUID) == 0 {
				logrus.Debugf("Container's %v host_uuid is empty", container.Name)
				continue
			}

			var host metadata.Host
			if _, ok := hostMeta[hostUUID]; ok {
				host = hostMeta[hostUUID]
			} else {
				host, err = m.MetadataClient.GetHost(hostUUID)
				if err != nil {
					logrus.Warnf("Failed to get host metadata: %v", err)
					continue
				}
				hostMeta[hostUUID] = host
			}

			// Check for Host Label: io.rancher.host.external_dns
			// Accepts 'true' (default) or 'false'
			if label, ok := host.Labels["io.rancher.host.external_dns"]; ok {
				if label == "false" {
					logrus.Debugf("Container %v Host %s is Disabled", container.Name, host.Name)
					continue
				}
			}

			ip, ok := host.Labels["io.rancher.host.external_dns_ip"]
			if !ok || ip == "" {
				ip = host.AgentIP
			}

			fqdn := utils.FqdnFromTemplate(config.NameTemplate, container.ServiceName, container.StackName,
				m.EnvironmentName, config.RootDomainName)

			addToDnsEntries(fqdn, ip, dnsEntries)
			ourFqdns[fqdn] = struct{}{}
		}
	}

	if len(ourFqdns) > 0 {
		stateFqdn := utils.StateFqdn(m.EnvironmentUUID, config.RootDomainName)
		stateRec := utils.StateRecord(stateFqdn, config.TTL, ourFqdns)
		dnsEntries[stateFqdn] = stateRec
	}

	return nil
}

func addToDnsEntries(fqdn, ip string, dnsEntries map[string]utils.DnsRecord) {
	var records []string
	if _, ok := dnsEntries[fqdn]; !ok {
		records = []string{ip}
	} else {
		records = dnsEntries[fqdn].Records
		// skip if the records already have that IP
		for _, val := range records {
			if val == ip {
				return
			}
		}
		records = append(records, ip)
	}

	dnsEntries[fqdn] = utils.DnsRecord{fqdn, records, "A", config.TTL}
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
