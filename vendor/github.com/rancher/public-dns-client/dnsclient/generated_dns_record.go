package dnsclient

const (
	DNS_RECORD_TYPE = "dnsRecord"
)

type DnsRecord struct {
	Resource

	Actions map[string]interface{} `json:"actions,omitempty" yaml:"actions,omitempty"`

	Fqdn string `json:"fqdn,omitempty" yaml:"fqdn,omitempty"`

	Links map[string]interface{} `json:"links,omitempty" yaml:"links,omitempty"`

	Records []string `json:"records,omitempty" yaml:"records,omitempty"`

	Recordtype string `json:"recordtype,omitempty" yaml:"recordtype,omitempty"`

	Ttl int64 `json:"ttl,omitempty" yaml:"ttl,omitempty"`

	Type string `json:"type,omitempty" yaml:"type,omitempty"`
}

type DnsRecordCollection struct {
	Collection
	Data   []DnsRecord `json:"data,omitempty"`
	client *DnsRecordClient
}

type DnsRecordClient struct {
	rancherClient *RancherClient
}

type DnsRecordOperations interface {
	List(opts *ListOpts) (*DnsRecordCollection, error)
	Create(opts *DnsRecord) (*DnsRecord, error)
	Update(existing *DnsRecord, updates interface{}) (*DnsRecord, error)
	ById(id string) (*DnsRecord, error)
	Delete(container *DnsRecord) error
}

func newDnsRecordClient(rancherClient *RancherClient) *DnsRecordClient {
	return &DnsRecordClient{
		rancherClient: rancherClient,
	}
}

func (c *DnsRecordClient) Create(container *DnsRecord) (*DnsRecord, error) {
	resp := &DnsRecord{}
	err := c.rancherClient.doCreate(DNS_RECORD_TYPE, container, resp)
	return resp, err
}

func (c *DnsRecordClient) Update(existing *DnsRecord, updates interface{}) (*DnsRecord, error) {
	resp := &DnsRecord{}
	existing.Resource.Links = make(map[string]string)
	for key, value := range existing.Links {

		if str, ok := value.(string); ok {
			existing.Resource.Links[key] = str
		}
	}
	err := c.rancherClient.doUpdate(DNS_RECORD_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DnsRecordClient) List(opts *ListOpts) (*DnsRecordCollection, error) {
	resp := &DnsRecordCollection{}
	err := c.rancherClient.doList(DNS_RECORD_TYPE, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *DnsRecordCollection) Next() (*DnsRecordCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DnsRecordCollection{}
		err := cc.client.rancherClient.doNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DnsRecordClient) ById(id string) (*DnsRecord, error) {
	resp := &DnsRecord{}
	err := c.rancherClient.doById(DNS_RECORD_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *DnsRecordClient) Delete(container *DnsRecord) error {
	container.Resource.Links = make(map[string]string)
	for key, value := range container.Links {

		if str, ok := value.(string); ok {
			container.Resource.Links[key] = str
		}
	}
	return c.rancherClient.doResourceDelete(DNS_RECORD_TYPE, &container.Resource)
}
