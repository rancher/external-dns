package dns

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
)

var (
	RootDomainName    string
	TTL               int
	FQDNGeneratorName string
)

type DnsRecord struct {
	Fqdn    string
	Records []string
	Type    string
	TTL     int
}

type ServiceDnsRecord struct {
	Fqdn        string
	ServiceName string
	StackName   string
}

func init() {
	TTLEnv := os.Getenv("TTL")
	i, err := strconv.Atoi(TTLEnv)
	if err != nil {
		TTL = 300
	} else {
		TTL = i
	}
}

func SetRootDomain(rootDomain string) {
	RootDomainName = rootDomain
}

func ConvertToServiceDnsRecord(dnsRecord DnsRecord) ServiceDnsRecord {
	splitted := strings.Split(dnsRecord.Fqdn, ".")
	serviceRecord := ServiceDnsRecord{dnsRecord.Fqdn, splitted[0], splitted[1]}
	return serviceRecord
}

func ConvertToFqdn(serviceName string, stackName string, environmentName string) string {

	fqdnGenerator := GetFQDNGenerator(FQDNGeneratorName)
	return fqdnGenerator.GenerateFQDN(serviceName, stackName, environmentName, RootDomainName)

}

func SaveServiceTokenToDisk(authToken string) error {

	//read yaml file to load the auth Token if exists
	if _, err := os.Stat("./public_dns_file.yml"); err == nil {

		tokenBytes, err := ioutil.ReadFile("./public_dns_file.yml")
		if err != nil {
			return err
		}
		tokenMap := make(map[string]string)
		yaml.Unmarshal(tokenBytes, &tokenMap)
		token := tokenMap["AUTH_TOKEN"]

		if strings.EqualFold(authToken, token) {
			return nil
		}
	}

	tokenMap := make(map[string]string)

	tokenMap["AUTH_TOKEN"] = authToken
	tokenMap["ROOT_DOMAIN"] = ""

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
