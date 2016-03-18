package providers

import (
	"errors"
	logrus "github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	goRancherClient "github.com/rancher/go-rancher/client"
	rancherPublicDNS "github.com/rancher/public-dns-client/client"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
)

var (
	rancher_public_dns_url string
)

type PublicDNSHandler struct {
	dnsClient  *rancherPublicDNS.RancherDNSClient
	rootDomain string
	authToken  string
}

func init() {
	rancherPublicDNSHandler := &PublicDNSHandler{}

	rancher_public_dns_url = os.Getenv("RANCHER_PUBLIC_DNS_URL")
	if len(rancher_public_dns_url) == 0 {
		logrus.Infof("RANCHER_PUBLIC_DNS_URL is not set in environment, skipping init of %s provider", rancherPublicDNSHandler.GetName())
		return
	}

	var err error

	if err = RegisterProvider("RancherPublicDNS", rancherPublicDNSHandler); err != nil {
		logrus.Fatalf("Could not register RancherPublicDNS provider, err: %v", err)
	}
}

func newPublicDNSClient(url string, authToken string) *rancherPublicDNS.RancherDNSClient {
	dnsClient, err := rancherPublicDNS.NewRancherDNSClient(&goRancherClient.ClientOpts{
		Url:             url,
		CustomAuthToken: "Bearer " + authToken,
	})

	if err != nil {
		logrus.Fatal("Failed to create RancherPublicDNS client", err)
	}

	return dnsClient
}

func loadDnsCredentialsFromDisk() (string, string, error) {
	//read yaml file to load the auth Token if exists
	if _, err := os.Stat("./public_dns_file.yml"); err == nil {

		tokenBytes, err := ioutil.ReadFile("./public_dns_file.yml")
		if err != nil {
			return "", "", err
		}
		tokenMap := make(map[string]string)
		yaml.Unmarshal(tokenBytes, &tokenMap)
		token := tokenMap["AUTH_TOKEN"]
		domain := tokenMap["ROOT_DOMAIN"]

		return token, domain, nil
	} else {
		return "", "", nil
	}
}

func saveDnsCredentialsToDisk(authToken string, rootDomain string) error {

	tokenMap := make(map[string]string)

	tokenMap["AUTH_TOKEN"] = authToken
	tokenMap["ROOT_DOMAIN"] = rootDomain

	tokenBytes, err := yaml.Marshal(&tokenMap)

	if err != nil {
		return err
	}

	output, err := os.Create(path.Join("./", "public_dns_file.yml"))

	if err != nil {
		return err
	}

	defer output.Close()

	_, err = output.Write(tokenBytes)
	if err != nil {
		return err
	}

	return nil
}

func (p *PublicDNSHandler) readAuthTokenAndRootDomain() error {
	domainInfo := &rancherPublicDNS.RootDomainInfo{}
	domainInfo.Token = p.authToken

	rootDomainInfo, err := p.dnsClient.RootDomainInfo.Create(domainInfo)
	if err != nil {
		logrus.Fatal("Failed to getRootDomain ", err)
		return err
	}
	p.rootDomain = rootDomainInfo.RootDomain
	p.authToken = rootDomainInfo.Token

	return nil
}

func (p *PublicDNSHandler) TestConnection() error {
	var err error

	var token, domain string
	if token, domain, err = loadDnsCredentialsFromDisk(); err != nil {
		logrus.Errorf("Could not read AUTH_TOKEN and ROOT_DOMAIN from disk, err: %v", err)
		return err
	}
	var initialized bool

	if token != "" && domain != "" {
		logrus.Infof("Initializing RancherPublicDNS provider - Found DNS token and rootdomain, server: %s", rancher_public_dns_url)
		p.authToken = token
		p.rootDomain = domain
		p.dnsClient = newPublicDNSClient(rancher_public_dns_url, p.authToken)
		//call getRootDomain to test
		if err = p.readAuthTokenAndRootDomain(); err != nil {
			logrus.Errorf("Could not read Auth Token and Root Domain from RancherPublicDNS, err: %v", err)
			return err
		}
		initialized = true
	} else if token != "" && domain == "" {
		//get new domain
		logrus.Infof("Initializing RancherPublicDNS provider - Getting new rootDomain from server: %s", rancher_public_dns_url)
		p.dnsClient = newPublicDNSClient(rancher_public_dns_url, token)
		if err = p.readAuthTokenAndRootDomain(); err != nil {
			logrus.Errorf("Could not read Auth Token and Root Domain from RancherPublicDNS, err: %v", err)
			return err
		}

		if err = saveDnsCredentialsToDisk(p.authToken, p.rootDomain); err != nil {
			logrus.Errorf("Could not save Auth Token and Root Domain to disk, err: %v", err)
			return err
		}

		//re-initialize client with the token
		p.dnsClient = newPublicDNSClient(rancher_public_dns_url, p.authToken)
		initialized = true
	} else {
		logrus.Debug("No Dns token found on disk, cannot connect to RancherPublicDNS server")
		initialized = false
	}

	if initialized {
		logrus.Infof("Configured %s with rootDomain \"%s\" ", p.GetName(), p.rootDomain)
		return nil
	} else {
		return errors.New("Cannot test connection to public dns server")
	}

}

func (p *PublicDNSHandler) GetRootDomain() string {
	return p.rootDomain
}

func (p *PublicDNSHandler) GetName() string {
	return "RancherPublicDNS"
}

func (p *PublicDNSHandler) AddRecord(record dns.DnsRecord) error {

	publicRec := p.preparePublicDNSRecord(record)
	_, err := p.dnsClient.DnsRecord.Create(publicRec)
	if err != nil {
		logrus.Errorf("RancherPublicDNS AddRecord API failed: %v", err)
	}
	return err
}

func (p *PublicDNSHandler) UpdateRecord(record dns.DnsRecord) error {

	updatedRecord := p.preparePublicDNSRecord(record)
	existingRecord, err := p.dnsClient.DnsRecord.ById(updatedRecord.Id)
	if err != nil {
		logrus.Errorf("RancherPublicDNS UpdateRecord API failed, getById failed: %v", err)
		return err
	}
	_, err = p.dnsClient.DnsRecord.Update(existingRecord, &updatedRecord)
	if err != nil {
		logrus.Errorf("RancherPublicDNS UpdateRecord API failed: %v", err)
		return err
	}
	return nil
}

func (p *PublicDNSHandler) RemoveRecord(record dns.DnsRecord) error {
	tobeRemovedRecord := p.preparePublicDNSRecord(record)
	existingRecord, err := p.dnsClient.DnsRecord.ById(tobeRemovedRecord.Id)
	if err != nil {
		logrus.Errorf("RancherPublicDNS RemoveRecord API failed, getById failed: %v", err)
		return err
	}
	err = p.dnsClient.DnsRecord.Delete(existingRecord)
	if err != nil {
		logrus.Errorf("RancherPublicDNS RemoveRecord API failed: %v", err)
		return err
	}
	return nil
}

func (p *PublicDNSHandler) GetRecords(listOpts ...string) ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord
	publicRecords, err := p.dnsClient.DnsRecord.List(nil)
	if err != nil {
		logrus.Errorf("RancherPublicDNS GetRecords API call has failed: %v", err)
		return records, err
	}

	for _, rec := range publicRecords.Data {
		dnsRec := dns.DnsRecord{
			Fqdn:    rec.Fqdn,
			Records: rec.Records,
			Type:    rec.Recordtype,
			TTL:     int(rec.Ttl),
		}
		records = append(records, dnsRec)
	}
	return records, nil
}

func (p *PublicDNSHandler) preparePublicDNSRecord(record dns.DnsRecord) *rancherPublicDNS.DnsRecord {
	return &rancherPublicDNS.DnsRecord{
		Resource: goRancherClient.Resource{
			Id:   "id-" + record.Fqdn,
			Type: rancherPublicDNS.DNS_RECORD_TYPE,
		},
		Fqdn:       record.Fqdn,
		Records:    record.Records,
		Recordtype: record.Type,
		Ttl:        int64(record.TTL),
	}
}
