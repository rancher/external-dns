package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/go-rancher/client"
	"strings"
)

type ExternalDnsRecord struct {
	DomainName  string `json:"domainName"`
	StackName   string `json:"stackName"`
	ServiceName string `json:"serviceName"`
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

func getRecords(metadataRecs map[string]providers.DnsRecord) ([]ExternalDnsRecord, error) {
	var records []ExternalDnsRecord
	for domainName := range metadataRecs {
		splitted := strings.Split(domainName, ".")
		record := ExternalDnsRecord{domainName, splitted[1], splitted[0]}
		logrus.Infof("DNS record from metadata: %v, %v, %v", record.DomainName, record.ServiceName, record.StackName)
		records = append(records, record)
	}
	return records, nil
}

func (c *CattleClient) UpdateExternalDns(metadataRecs map[string]providers.DnsRecord, EnvironmentName string) error {
	irecords := []interface{}{}
	records, err := getRecords(metadataRecs)
	for _, record := range records {
		logrus.Infof("%v", record.DomainName)
		irecords = append(irecords, record)
	}
	event := &client.ExternalDnsEvent{
		EventType:  "dns.update",
		ExternalId: EnvironmentName,
		DnsRecords: irecords,
	}
	_, err = c.rancherClient.ExternalDnsEvent.Create(event)
	return err
}
