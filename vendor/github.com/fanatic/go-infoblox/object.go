package infoblox

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
)

//Resource represents a WAPI object
type Object struct {
	Ref string `json:"_ref"`
	r   *Resource
}

func (o Object) Get(opts *Options) (map[string]interface{}, error) {
	resp, err := o.get(opts)
	if err != nil {
		return nil, err
	}

	var out map[string]interface{}
	err = resp.Parse(&out)
	if err != nil {
		return nil, fmt.Errorf("%+v\n", err)
	}

	return out, nil
}

func (o Object) get(opts *Options) (*APIResponse, error) {
	q := o.r.getQuery(opts, []Condition{}, url.Values{})

	resp, err := o.r.conn.SendRequest("GET", o.objectURI()+"?"+q.Encode(), "", nil)
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %v\n", err)
	}
	return resp, nil
}

func (o Object) Update(data url.Values, opts *Options, body interface{}) (string, error) {
	q := o.r.getQuery(opts, []Condition{}, data)
	q.Set("_return_fields", "") //Force object response

	var err error
	head := make(map[string]string)
	var bodyStr, urlStr string
	if body == nil {
		// Send URL-encoded data in the request body
		urlStr = o.objectURI()
		bodyStr = q.Encode()
		head["Content-Type"] = "application/x-www-form-urlencoded"
	} else {
		// Put url-encoded data in the URL and send the body parameter as a JSON body.
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("Error creating request: %v\n", err)
		}
		log.Printf("PUT body: %s\n", bodyJSON)
		urlStr = o.objectURI() + "?" + q.Encode()
		bodyStr = string(bodyJSON)
		head["Content-Type"] = "application/json"
	}

	resp, err := o.r.conn.SendRequest("PUT", urlStr, bodyStr, head)
	if err != nil {
		return "", fmt.Errorf("Error sending request: %v\n", err)
	}

	//fmt.Printf("%v", resp.ReadBody())

	var responseData interface{}
	var ret string
	if err := resp.Parse(&responseData); err != nil {
		return "", fmt.Errorf("%+v\n", err)
	}
	switch s := responseData.(type) {
	case string:
		ret = s
	case map[string]interface{}:
		ret = s["_ref"].(string)
	default:
		return "", fmt.Errorf("Invalid return type %T", s)
	}

	return ret, nil
}

func (o Object) Delete(opts *Options) error {
	q := o.r.getQuery(opts, []Condition{}, url.Values{})

	resp, err := o.r.conn.SendRequest("DELETE", o.objectURI()+"?"+q.Encode(), "", nil)
	if err != nil {
		return fmt.Errorf("Error sending request: %v\n", err)
	}

	//fmt.Printf("%v", resp.ReadBody())

	var out interface{}
	err = resp.Parse(&out)
	if err != nil {
		return fmt.Errorf("%+v\n", err)
	}
	return nil
}

func (o Object) FunctionCall(functionName string, jsonBody interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(jsonBody)
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %v\n", err)
	}

	resp, err := o.r.conn.SendRequest("POST", fmt.Sprintf("%s?_function=%s", o.objectURI(), functionName), string(data), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %v\n", err)
	}

	//fmt.Printf("%v", resp.ReadBody())

	var out map[string]interface{}
	err = resp.Parse(&out)
	if err != nil {
		return nil, fmt.Errorf("%+v\n", err)
	}
	return out, nil
}

func (o Object) objectURI() string {
	return BASE_PATH + o.Ref
}
