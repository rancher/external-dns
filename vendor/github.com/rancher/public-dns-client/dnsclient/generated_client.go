package dnsclient

type RancherClient struct {
	RancherBaseClient

	ApiVersion     ApiVersionOperations
	RootDomainInfo RootDomainInfoOperations
	DnsRecord      DnsRecordOperations
}

func constructClient(rancherBaseClient *RancherBaseClientImpl) *RancherClient {
	client := &RancherClient{
		RancherBaseClient: rancherBaseClient,
	}

	client.ApiVersion = newApiVersionClient(client)
	client.RootDomainInfo = newRootDomainInfoClient(client)
	client.DnsRecord = newDnsRecordClient(client)

	return client
}

func NewRancherClient(opts *ClientOpts) (*RancherClient, error) {
	rancherBaseClient := &RancherBaseClientImpl{
		Types: map[string]Schema{},
	}
	client := constructClient(rancherBaseClient)

	err := setupRancherBaseClient(rancherBaseClient, opts)
	if err != nil {
		return nil, err
	}

	return client, nil
}
