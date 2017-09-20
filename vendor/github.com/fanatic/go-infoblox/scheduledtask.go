package infoblox

// https://192.168.2.200/wapidoc/objects/scheduledtask.html
func (c *Client) ScheduledTask() *Resource {
	return &Resource{
		conn:       c,
		wapiObject: "scheduledtask",
	}
}

type ScheduledTaskObject struct {
	Object
}

func (c *Client) ScheduledTaskObject(ref string) *ScheduledTaskObject {
	return &ScheduledTaskObject{
		Object{
			Ref: ref,
			r:   c.ScheduledTask(),
		},
	}
}

func (c *Client) FindScheduledTask(name string) ([]ScheduledTaskObject, error) {
	field := "changed_objects.name"
	o := Options{ReturnFields: []string{"changed_objects"}, ReturnBasicFields: true}
	conditions := []Condition{Condition{Field: &field, Value: name}}
	resp, err := c.ScheduledTask().find(conditions, &o)
	if err != nil {
		return nil, err
	}

	var out []ScheduledTaskObject
	err = resp.Parse(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
