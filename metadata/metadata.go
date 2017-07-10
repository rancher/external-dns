package metadata

import (
	"fmt"
	"net"
	"strings"
	"time"
	"regexp"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/config"
	"github.com/rancher/external-dns/utils"
	"github.com/rancher/go-rancher-metadata/metadata"
)

const (
	metadataUrl = "http://rancher-metadata.rancher.internal/2015-12-19"
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

func (m *MetadataClient) GetMetadataDnsRecords() (map[string]utils.MetadataDnsRecord, error) {
	dnsEntries := make(map[string]utils.MetadataDnsRecord)
	err := m.getContainersDnsRecords(dnsEntries)
	if err != nil {
		return dnsEntries, err
	}
	return dnsEntries, nil
}

func (m *MetadataClient) getContainersDnsRecords(dnsEntries map[string]utils.MetadataDnsRecord) error {
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

			var externalIP string
			if ip, ok := host.Labels["io.rancher.host.external_dns_ip"]; ok && len(ip) > 0 {
				externalIP = ip
			} else if len(container.Ports) > 0 {
				if ip, ok := parsePortToIP(container.Ports[0]); ok {
					externalIP = ip
				}
			}

			// fallback to host agent IP
			if len(externalIP) == 0 {
				logrus.Debugf("Fallback to host.AgentIP %s for container %s", host.AgentIP, container.Name)
				externalIP = host.AgentIP
			}

			if net.ParseIP(externalIP) == nil {
				logrus.Errorf("Skipping container %s: Invalid IP address %s", container.Name, externalIP)
			}

			nameTemplate, ok := service.Labels["io.rancher.service.external_dns_name_template"]
			if !ok {
				nameTemplate = config.NameTemplate
			}

			fqdn := utils.FqdnFromTemplate(nameTemplate, container.ServiceName, container.StackName,
				m.EnvironmentName, config.RootDomainName)

			addToDnsEntries(fqdn, externalIP, container.ServiceName, container.StackName, dnsEntries, "A")
			ourFqdns[fqdn] = struct{}{}
		}
		
		//Checks specifically for load balancers to correctly route requested hostnames
		//to the proper places.
		if service.Kind == "loadBalancerService" {
			for _, portRule := range service.LBConfig.PortRules{
				for _, container := range service.Containers {
					fqdn := ""
					hostName := portRule.Hostname 

					nameTemplate, ok := service.Labels["io.rancher.service.external_dns_name_template"]
					if !ok {
						nameTemplate = config.NameTemplate
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

					var externalIP string
					if ip, ok := host.Labels["io.rancher.host.external_dns_ip"]; ok && len(ip) > 0 {
						externalIP = ip
					} else if len(container.Ports) > 0 {
						if ip, ok := parsePortToIP(container.Ports[0]); ok {
							externalIP = ip
						}
					}

					if len(externalIP) == 0 {
						logrus.Debugf("Fallback to host.AgentIP %s for container %s", host.AgentIP, container.Name)
						externalIP = host.AgentIP
					}
					//Checks regex to see if there is a wildcard at the end of the requested hostname
					//EX: host.*
					//If there is, append our root domain name to it and add a . to make it Fqdn
					if matched, err := regexp.MatchString("\\.\\*$", hostName); matched{
						hostName = strings.TrimRight(hostName, "\\*")
						hostName = strings.TrimRight(hostName, "\\.")
						fqdn := hostName + "." + config.RootDomainName
						addToDnsEntries(fqdn, externalIP, container.ServiceName, container.StackName, dnsEntries, "A")
						ourFqdns[fqdn] = struct{}{}
					}else if err != nil{
						logrus.Warnf("Regex matching error: %v", err)
					//Checks to see if there is a full domain name already matching the root domain name
					//If there is, we just want to register it to dns
					//If not, we still need to append our root domain name and probably all the other stuff
					} else {
						fqdn := utils.FqdnFromTemplate(nameTemplate, hostName, service.StackName,
							m.EnvironmentName, config.RootDomainName)
						addToDnsEntries(fqdn, externalIP, container.ServiceName, container.StackName, dnsEntries, "A")
						ourFqdns[fqdn] = struct{}{}
						}
					}
				}
			}
		}
	}

	if len(ourFqdns) > 0 {
		stateFqdn := utils.StateFqdn(m.EnvironmentUUID, config.RootDomainName)
		stateRec := utils.StateRecord(stateFqdn, config.TTL, ourFqdns)
		dnsEntries[stateFqdn] = utils.MetadataDnsRecord{
			ServiceName: "",
			StackName:   "",
			DnsRecord:   stateRec,
		}
	}

	return nil
}

func addToDnsEntries(fqdn, ip, service, stack string, dnsEntries map[string]utils.MetadataDnsRecord, entryType string) {
	var records []string
	if _, ok := dnsEntries[fqdn]; !ok {
		records = []string{ip}
	} else {
		records = dnsEntries[fqdn].DnsRecord.Records
		// skip if the records already have that IP
		for _, val := range records {
			if val == ip {
				return
			}
		}
		records = append(records, ip)
	}
	if entryType == "A"{
		dnsEntries[fqdn] = utils.MetadataDnsRecord{
			ServiceName: service,
			StackName:   stack,
			DnsRecord:   utils.DnsRecord{fqdn, records, entryType, config.TTL},
		}
	} /*else if entryType == "CNAME" {
		dnsEntries[fqdn] = utils.MetadataDnsRecord{
			ServiceName: service,
			StackName:   stack,
			DnsRecord:   utils.DnsRecord{fqdn, records, entryType, nil},
		}
	}*/
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

// expects port string as 'ip:publicPort:privatePort'
// returns usable ip address
func parsePortToIP(port string) (string, bool) {
	parts := strings.Split(port, ":")
	if len(parts) == 3 {
		ip := parts[0]
		if len(ip) > 0 && ip != "0.0.0.0" {
			return ip, true
		}
	}

	return "", false
}
