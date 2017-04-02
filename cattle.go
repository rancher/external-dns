package main

import (
	"github.com/rancher/external-dns/utils"
	rancher "github.com/rancher/go-rancher/v2"
)

type CattleClient struct {
	rancherClient *rancher.RancherClient
}

func NewCattleClient(cattleUrl string, accessKey string, secretKey string) (*CattleClient, error) {
	client, err := rancher.NewRancherClient(&rancher.ClientOpts{
		Url:       cattleUrl,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})

	if err != nil {
		return nil, err
	}

	return &CattleClient{
		rancherClient: client,
	}, nil
}

func (c *CattleClient) UpdateServiceDomainName(serviceDnsRecord utils.ServiceDnsRecord) error {
	event := &rancher.ExternalDnsEvent{
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
	opts := &rancher.ListOpts{}
	_, err := c.rancherClient.ExternalDnsEvent.List(opts)
	return err
}
