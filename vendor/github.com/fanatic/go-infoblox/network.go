package infoblox

import "fmt"

// https://192.168.2.200/wapidoc/objects/network.html
func (c *Client) Network() *Resource {
	return &Resource{
		conn:       c,
		wapiObject: "network",
	}
}

type NetworkObject struct {
	Object
	Comment     string  `json:"comment,omitempty"`
	Network     string  `json:"network,omitempty"`
	NetworkView string  `json:"network_view,omitempty"`
	Netmask     int     `json:"netmask,omitempty"`
	ExtAttrs    ExtAttr `json:"extattrs,omitempty"`
}

type ExtAttr map[string]struct {
	Value interface{} `json:"value"`
}

func (e ExtAttr) Get(key string) (string, bool) {
	v, ok := e[key]
	if !ok {
		return "", false
	}
	o, ok := v.Value.(string)
	return o, ok
}

func (e ExtAttr) GetFloat(key string) (float64, bool) {
	v, ok := e[key]
	if !ok {
		return -1, false
	}
	o, ok := v.Value.(float64)
	return o, ok
}

func (c *Client) NetworkObject(ref string) *NetworkObject {
	obj := NetworkObject{}
	obj.Object = Object{
		Ref: ref,
		r:   c.Network(),
	}
	return &obj
}

//Invoke the same-named function on the network resource in WAPI,
//returning an array of available IP addresses.
//You may optionally specify how many IPs you want (num) and which ones to
//exclude from consideration (array of IPv4 addrdess strings).
type NextAvailableIPParams struct {
	Exclude []string `json:"exclude,omitempty"`
	Num     int      `json:"num,omitempty"`
}

func (n NetworkObject) NextAvailableIP(num int, exclude []string) (map[string]interface{}, error) {
	if num == 0 {
		num = 1
	}

	v := &NextAvailableIPParams{
		Exclude: exclude,
		Num:     num,
	}

	out, err := n.FunctionCall("next_available_ip", v)
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %v\n", err)
	}
	return out, nil
}

func (c *Client) FindNetworkByNetwork(net string) ([]NetworkObject, error) {
	field := "network"
	o := Options{ReturnFields: []string{"extattrs", "netmask"}}
	conditions := []Condition{Condition{Field: &field, Value: net}}
	resp, err := c.Network().find(conditions, &o)
	if err != nil {
		return nil, err
	}

	var out []NetworkObject
	err = resp.Parse(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) FindNetworkByExtAttrs(attrs map[string]string) ([]NetworkObject, error) {
	conditions := []Condition{}
	for k, v := range attrs {
		attr := k
		conditions = append(conditions, Condition{Attribute: &attr, Value: v})
	}
	o := Options{ReturnFields: []string{"extattrs", "netmask"}, ReturnBasicFields: true}
	resp, err := c.Network().find(conditions, &o)
	if err != nil {
		return nil, err
	}

	var out []NetworkObject
	err = resp.Parse(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
