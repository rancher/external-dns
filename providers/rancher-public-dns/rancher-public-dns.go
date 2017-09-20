package publicdns

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/metadata"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
	rancherPublicDNS "github.com/rancher/public-dns-client/dnsclient"
	"gopkg.in/yaml.v2"
)

const (
	CredentialsFile string = "/opt/rancher/public_dns_creds.yml"
)

type PublicDnsProvider struct {
	dnsClient  *rancherPublicDNS.RancherClient
	rootDomain string
	authToken  string
}

func init() {
	providers.RegisterProvider("rancher-public-dns", &PublicDnsProvider{})
}

//
// Methods implementing the provider.Provider interface
//

func (p *PublicDnsProvider) Init() error {
	var publicDnsURL, authToken, domain string
	var err error
	if publicDnsURL = os.Getenv("RANCHER_PUBLIC_DNS_URL"); len(publicDnsURL) == 0 {
		return fmt.Errorf("RANCHER_PUBLIC_DNS_URL is not set")
	}

	if authToken, domain, err = loadCredentialsFromDisk(); err != nil {
		return fmt.Errorf("Could not read credentials from disk: %v", err)
	}

	var installUUID string
	if installUUID, err = getInstallUUID(); err != nil {
		return err
	}

	if authToken == "" || domain == "" {
		logrus.Info("Initializing provider with service token")
		// get service token from metadata
		var serviceToken string
		if serviceToken, err = getServiceToken(); err != nil {
			return err
		}

		// get auth token and domain from rancher public dns service
		logrus.Debugf("Querying credentials using service token: %s", publicDnsURL, serviceToken)
		p.dnsClient = newPublicDNSClient(publicDnsURL, serviceToken, installUUID)
		if err = p.getAuthTokenAndRootDomain(); err != nil {
			return fmt.Errorf("Failed to query auth token and root domain: %v", err)
		}

		if err = saveCredentialsToDisk(p.authToken, p.rootDomain); err != nil {
			return fmt.Errorf("Could not save credentials to disk: %v", err)
		}

		// re-initialize client with the auth token
		p.dnsClient = newPublicDNSClient(publicDnsURL, p.authToken, installUUID)

	} else {
		logrus.Info("Initializing provider with credentials from disk")
		logrus.Debugf("authToken '%s' rootDomain '%s'", authToken, domain)
		p.authToken = authToken
		p.rootDomain = domain
		p.dnsClient = newPublicDNSClient(publicDnsURL, p.authToken, installUUID)
		// test auth token
		if err = p.getAuthTokenAndRootDomain(); err != nil {
			return fmt.Errorf("Failed to query auth token and root domain: %v", err)
		}
	}

	logrus.Infof("Configured %s with root domain '%s' and server '%s'",
		p.GetName(), p.rootDomain, publicDnsURL)

	return nil
}

func (p *PublicDnsProvider) GetName() string {
	return "Rancher Public DNS"
}

func (p *PublicDnsProvider) GetRootDomain() string {
	return p.rootDomain
}

func (p *PublicDnsProvider) HealthCheck() error {
	_, err := p.dnsClient.ApiVersion.List(nil)
	return err
}

func (p *PublicDnsProvider) AddRecord(record utils.DnsRecord) error {
	publicRec := p.preparePublicDNSRecord(record)
	_, err := p.dnsClient.DnsRecord.Create(publicRec)
	if err != nil {
		logrus.Errorf("AddRecord API call failed: %v", err)
	}

	return err
}

func (p *PublicDnsProvider) UpdateRecord(record utils.DnsRecord) error {
	updatedRecord := p.preparePublicDNSRecord(record)
	existingRecord, err := p.dnsClient.DnsRecord.ById(updatedRecord.Id)
	if err != nil {
		logrus.Errorf("UpdateRecord API call failed, getById failed: %v", err)
		return err
	}

	_, err = p.dnsClient.DnsRecord.Update(existingRecord, &updatedRecord)
	if err != nil {
		logrus.Errorf("UpdateRecord API call failed: %v", err)
		return err
	}

	return nil
}

func (p *PublicDnsProvider) RemoveRecord(record utils.DnsRecord) error {
	tobeRemovedRecord := p.preparePublicDNSRecord(record)
	existingRecord, err := p.dnsClient.DnsRecord.ById(tobeRemovedRecord.Id)
	if err != nil {
		logrus.Errorf("RemoveRecord API call failed, getById failed: %v", err)
		return err
	}

	err = p.dnsClient.DnsRecord.Delete(existingRecord)
	if err != nil {
		logrus.Errorf("RemoveRecord API call failed: %v", err)
		return err
	}

	return nil
}

func (p *PublicDnsProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord
	publicRecords, err := p.dnsClient.DnsRecord.List(nil)
	if err != nil {
		logrus.Errorf("GetRecords API call failed: %v", err)
		return records, err
	}

	for _, rec := range publicRecords.Data {
		dnsRec := utils.DnsRecord{
			Fqdn:    rec.Fqdn,
			Records: rec.Records,
			Type:    rec.Recordtype,
			TTL:     int(rec.Ttl),
		}
		records = append(records, dnsRec)
	}

	return records, nil
}

//
// Private methods
//

func (p *PublicDnsProvider) preparePublicDNSRecord(record utils.DnsRecord) *rancherPublicDNS.DnsRecord {
	return &rancherPublicDNS.DnsRecord{
		Resource: rancherPublicDNS.Resource{
			Id:   "id-" + record.Fqdn,
			Type: rancherPublicDNS.DNS_RECORD_TYPE,
		},
		Fqdn:       record.Fqdn,
		Records:    record.Records,
		Recordtype: record.Type,
		Ttl:        int64(record.TTL),
	}
}

func (p *PublicDnsProvider) getAuthTokenAndRootDomain() error {
	domainInfo := &rancherPublicDNS.RootDomainInfo{}
	domainInfo.Token = p.authToken

	rootDomainInfo, err := p.dnsClient.RootDomainInfo.Create(domainInfo)
	if err != nil {
		return err
	}

	p.rootDomain = rootDomainInfo.RootDomain
	p.authToken = rootDomainInfo.Token

	return nil
}

//
// Private functions
//

func newPublicDNSClient(url string, authToken string, installUUID string) *rancherPublicDNS.RancherClient {
	dnsClient, err := rancherPublicDNS.NewRancherClient(&rancherPublicDNS.ClientOpts{
		Url:             url,
		CustomAuthToken: "Bearer " + authToken,
		InstallUUID:     installUUID,
	})

	if err != nil {
		logrus.Fatal("Failed to create public dns client", err)
	}

	return dnsClient
}

func getServiceToken() (string, error) {
	var token string
	m, err := metadata.NewMetadataClient()
	if err != nil {
		return "", fmt.Errorf("Failed to configure metadata client: %v", err)
	}

	token, err = m.GetServiceToken()
	if err != nil {
		return "", fmt.Errorf("Failed to get service token from metadata: %v", err)
	}

	return token, nil
}

func getInstallUUID() (string, error) {
	var installUUID string
	m, err := metadata.NewMetadataClient()
	if err != nil {
		return "", fmt.Errorf("Failed to configure metadata client: %v", err)
	}

	installUUID, err = m.GetInstallUUID()
	if err != nil {
		return "", fmt.Errorf("Failed to get installUUID from metadata: %v", err)
	}

	return installUUID, nil
}

func loadCredentialsFromDisk() (string, string, error) {
	if _, err := os.Stat(CredentialsFile); os.IsNotExist(err) {
		return "", "", nil
	}

	tokenBytes, err := ioutil.ReadFile(CredentialsFile)
	if err != nil {
		return "", "", err
	}

	tokenMap := make(map[string]string)
	yaml.Unmarshal(tokenBytes, &tokenMap)
	return tokenMap["AUTH_TOKEN"], tokenMap["ROOT_DOMAIN"], nil
}

func saveCredentialsToDisk(authToken string, rootDomain string) error {
	tokenMap := make(map[string]string)
	tokenMap["AUTH_TOKEN"] = authToken
	tokenMap["ROOT_DOMAIN"] = rootDomain

	tokenBytes, err := yaml.Marshal(&tokenMap)
	if err != nil {
		return err
	}

	dir := filepath.Dir(CredentialsFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}

	f, err := os.Create(CredentialsFile)
	if err != nil {
		return err
	}

	defer f.Close()
	_, err = f.Write(tokenBytes)
	if err != nil {
		return err
	}

	f.Sync()
	return nil
}
