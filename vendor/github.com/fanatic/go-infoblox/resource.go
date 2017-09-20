package infoblox

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
)

//Resource represents a WAPI object type
type Resource struct {
	conn       *Client
	wapiObject string
}

type Options struct {
	//The maximum number of objects to be returned.  If set to a negative
	//number the appliance will return an error when the number of returned
	//objects would exceed the setting. The default is -1000. If this is
	//set to a positive number, the results will be truncated when necessary.
	MaxResults *int

	ReturnFields      []string //A list of returned fields
	ReturnBasicFields bool     // Return basic fields in addition to ReturnFields
}

// Conditions are used for searching
type Condition struct {
	Field     *string // EITHER A documented field of the object (only set one)
	Attribute *string // OR the name of an extensible attribute (only set one)
	Modifiers string  // Optional search modifiers "!:~<>" (otherwise exact match)
	Value     string  // Value or regular expression to search for
}

// All returns an array of all records for this resource
func (r Resource) All(opts *Options) ([]map[string]interface{}, error) {
	return r.Find([]Condition{}, opts)
}

// Find resources with query parameters. Conditions are combined with AND
// logic.  When a field is a list of extensible attribute that can have multiple
// values, the condition is true if any value in the list matches.
func (r Resource) Find(query []Condition, opts *Options) ([]map[string]interface{}, error) {
	resp, err := r.find(query, opts)

	var out []map[string]interface{}
	err = resp.Parse(&out)
	if err != nil {
		return nil, fmt.Errorf("%+v\n", err)
	}
	return out, nil
}

func (r Resource) find(query []Condition, opts *Options) (*APIResponse, error) {
	q := r.getQuery(opts, query, url.Values{})

	resp, err := r.conn.SendRequest("GET", r.resourceURI()+"?"+q.Encode(), "", nil)
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %v\n", err)
	}

	return resp, nil
}

func (r Resource) Create(data url.Values, opts *Options, body interface{}) (string, error) {
	q := r.getQuery(opts, []Condition{}, data)
	q.Set("_return_fields", "") //Force object response

	var err error
	head := make(map[string]string)
	var bodyStr, urlStr string
	if body == nil {
		// Send URL-encoded data in the request body
		urlStr = r.resourceURI()
		bodyStr = q.Encode()
		head["Content-Type"] = "application/x-www-form-urlencoded"
	} else {
		// Put url-encoded data in the URL and send the body parameter as a JSON body.
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("Error creating request: %v\n", err)
		}
		log.Printf("POST body: %s\n", bodyJSON)
		urlStr = r.resourceURI() + "?" + q.Encode()
		bodyStr = string(bodyJSON)
		head["Content-Type"] = "application/json"
	}

	resp, err := r.conn.SendRequest("POST", urlStr, bodyStr, head)
	if err != nil {
		return "", fmt.Errorf("Error sending request: %v\n", err)
	}

	//fmt.Printf("%v", resp.ReadBody())

	// If you POST to record:host with a scheduled creation time, it sends back a string regardless of the presence of _return_fields
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

func (r Resource) getQuery(opts *Options, query []Condition, extra url.Values) url.Values {
	v := extra

	returnFieldOption := "_return_fields"
	if opts != nil && opts.ReturnBasicFields {
		returnFieldOption = "_return_fields+"
	}

	if opts != nil && opts.ReturnFields != nil {
		v.Set(returnFieldOption, strings.Join(opts.ReturnFields, ","))
	}

	if opts != nil && opts.MaxResults != nil {
		v.Set("_max_results", strconv.Itoa(*opts.MaxResults))
	}

	for _, cond := range query {
		search := ""
		if cond.Field != nil {
			search += *cond.Field
		} else if cond.Attribute != nil {
			search += "*" + *cond.Attribute
		}
		search += cond.Modifiers
		v.Set(search, cond.Value)
	}

	return v
}

func (r Resource) resourceURI() string {
	return BASE_PATH + r.wapiObject
}
