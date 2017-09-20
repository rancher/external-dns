package infoblox

func (c *Client) RecordPtr() *Resource {
	return &Resource{
		conn:       c,
		wapiObject: "record:ptr",
	}
}

type RecordPtrObject struct {
	Object
	Comment  string `json:"comment,omitempty"`
	Ipv4Addr string `json:"ipv4addr,omitempty"`
	Ipv6Addr string `json:"ipv6addr,omitempty"`
	Name     string `json:"name,omitempty"`
	PtrDname string `json:"ptrdname,omitempty"`
	Ttl      int    `json:"ttl,omitempty"`
	View     string `json:"view,omitempty"`
}

func (c *Client) RecordPtrObject(ref string) *RecordPtrObject {
	ptr := RecordPtrObject{}
	ptr.Object = Object{
		Ref: ref,
		r:   c.RecordPtr(),
	}
	return &ptr
}
