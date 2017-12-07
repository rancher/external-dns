package infoblox

// https://192.168.2.200/wapidoc/objects/ipv4address.html
func (c *Client) Ipv4address() *Resource {
	return &Resource{
		conn:       c,
		wapiObject: "ipv4address",
	}
}

type Ipv4addressObject struct {
	Object
	DHCPClientIdentifier string   `json:"dhcp_client_identifier,omitempty"`
	IPAddress            string   `json:"ip_address,omitempty"`
	IsConflict           bool     `json:"is_conflict,omitempty"`
	LeaseState           string   `json:"lease_state,omitempty"`
	MACAddress           string   `json:"mac_address,omitempty"`
	Names                []string `json:"names,omitempty"`
	Network              string   `json:"network,omitempty"`
	NetworkView          string   `json:"network_view,omitempty"`
	Objects              []string `json:"objects,omitempty"`
	Status               string   `json:"status,omitempty"`
	Types                []string `json:"types,omitempty"`
	Usage                []string `json:"usage,omitempty"`
	Username             string   `json:"username,omitempty"`
}

func (c *Client) Ipv4addressObject(ref string) *Ipv4addressObject {
	ip := Ipv4addressObject{}
	ip.Object = Object{
		Ref: ref,
		r:   c.Ipv4address(),
	}
	return &ip
}

func (c *Client) FindIP(ip string) ([]Ipv4addressObject, error) {
	field := "ip_address"
	conditions := []Condition{Condition{Field: &field, Value: ip}}
	resp, err := c.Ipv4address().find(conditions, nil)
	if err != nil {
		return nil, err
	}

	var out []Ipv4addressObject
	err = resp.Parse(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) FindUnusedIPInRange(start string, end string) ([]Ipv4addressObject, error) {
	ip_field := "ip_address"
	status_field := "status"
	conditions := []Condition{
		Condition{Field: &ip_field, Value: start, Modifiers: ">"},
		Condition{Field: &ip_field, Value: end, Modifiers: "<"},
		Condition{Field: &status_field, Value: "UNUSED"},
	}
	resp, err := c.Ipv4address().find(conditions, nil)
	if err != nil {
		return nil, err
	}

	var out []Ipv4addressObject
	err = resp.Parse(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
