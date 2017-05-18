package dnsclient

const (
	ROOT_DOMAIN_INFO_TYPE = "rootDomainInfo"
)

type RootDomainInfo struct {
	Resource

	Actions map[string]interface{} `json:"actions,omitempty" yaml:"actions,omitempty"`

	Links map[string]interface{} `json:"links,omitempty" yaml:"links,omitempty"`

	RootDomain string `json:"rootDomain,omitempty" yaml:"root_domain,omitempty"`

	Token string `json:"token,omitempty" yaml:"token,omitempty"`

	Type string `json:"type,omitempty" yaml:"type,omitempty"`
}

type RootDomainInfoCollection struct {
	Collection
	Data   []RootDomainInfo `json:"data,omitempty"`
	client *RootDomainInfoClient
}

type RootDomainInfoClient struct {
	rancherClient *RancherClient
}

type RootDomainInfoOperations interface {
	List(opts *ListOpts) (*RootDomainInfoCollection, error)
	Create(opts *RootDomainInfo) (*RootDomainInfo, error)
	Update(existing *RootDomainInfo, updates interface{}) (*RootDomainInfo, error)
	ById(id string) (*RootDomainInfo, error)
	Delete(container *RootDomainInfo) error
}

func newRootDomainInfoClient(rancherClient *RancherClient) *RootDomainInfoClient {
	return &RootDomainInfoClient{
		rancherClient: rancherClient,
	}
}

func (c *RootDomainInfoClient) Create(container *RootDomainInfo) (*RootDomainInfo, error) {
	resp := &RootDomainInfo{}
	err := c.rancherClient.doCreate(ROOT_DOMAIN_INFO_TYPE, container, resp)
	return resp, err
}

func (c *RootDomainInfoClient) Update(existing *RootDomainInfo, updates interface{}) (*RootDomainInfo, error) {
	resp := &RootDomainInfo{}
	err := c.rancherClient.doUpdate(ROOT_DOMAIN_INFO_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RootDomainInfoClient) List(opts *ListOpts) (*RootDomainInfoCollection, error) {
	resp := &RootDomainInfoCollection{}
	err := c.rancherClient.doList(ROOT_DOMAIN_INFO_TYPE, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *RootDomainInfoCollection) Next() (*RootDomainInfoCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RootDomainInfoCollection{}
		err := cc.client.rancherClient.doNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RootDomainInfoClient) ById(id string) (*RootDomainInfo, error) {
	resp := &RootDomainInfo{}
	err := c.rancherClient.doById(ROOT_DOMAIN_INFO_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *RootDomainInfoClient) Delete(container *RootDomainInfo) error {
	return c.rancherClient.doResourceDelete(ROOT_DOMAIN_INFO_TYPE, &container.Resource)
}
