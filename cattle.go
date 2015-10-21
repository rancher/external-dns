package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/go-rancher/client"
	"strings"
)

type ExternalDnsRecord struct {
	DomainName  string
	StackName   string
	ServiceName string
}

type CattleInterface interface {
	SyncStoragePool(string, []string) error
}

type CattleClient struct {
	rancherClient *client.RancherClient
}

func NewCattleClient(cattleUrl string, cattleAccessKey string, cattleSecretKey string) (*CattleClient, error) {
	apiClient, err := client.NewRancherClient(&client.ClientOpts{
		Url:       cattleUrl,
		AccessKey: cattleAccessKey,
		SecretKey: cattleSecretKey,
	})

	if err != nil {
		return nil, err
	}

	return &CattleClient{
		rancherClient: apiClient,
	}, nil
}

func updateCattleServices(metadataRecs map[string]providers.DnsRecord) error {
	for domainName := range metadataRecs {
		splitted := strings.Split(domainName, ".")
		logrus.Infof("DNS record from metadata: %v, %v, %v", domainName, splitted[0], splitted[1])
	}
	return nil
}
