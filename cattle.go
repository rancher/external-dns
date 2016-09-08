package main

import (
	"github.com/mathuin/external-dns/utils"
	"github.com/rancher/go-rancher/client"
)

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

func (c *CattleClient) UpdateServiceDomainName(serviceDnsRecord utils.ServiceDnsRecord) error {

	event := &client.ExternalDnsEvent{
		EventType:   "dns.update",
		ExternalId:  serviceDnsRecord.Fqdn,
		ServiceName: serviceDnsRecord.ServiceName,
		StackName:   serviceDnsRecord.StackName,
		Fqdn:        serviceDnsRecord.Fqdn,
	}
	_, err := c.rancherClient.ExternalDnsEvent.Create(event)
	return err
}

func (c *CattleClient) TestConnect() error {
	opts := &client.ListOpts{}
	_, err := c.rancherClient.ExternalDnsEvent.List(opts)
	return err
}
