package powerdns

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dghubble/sling"
)

// Error struct
type Error struct {
	Message string `json:"error"`
}

// Error Returns
func (e Error) Error() string {
	return fmt.Sprintf("%v", e.Message)
}

// APIVersion struct
type APIVersion struct {
	URL     string `json:"url"`
	Version int    `json:"version"`
}

// ServerInfo struct
type ServerInfo struct {
	ConfigURL  string `json:"config_url"`
	DaemonType string `json:"daemon_type"`
	ID         string `json:"id"`
	Type       string `json:"type"`
	URL        string `json:"url"`
	Version    string `json:"version"`
	ZonesURL   string `json:"zones_url"`
}

// Zone struct
type Zone struct {
	ID             string   `json:"id"`
	URL            string   `json:"url"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	DNSsec         bool     `json:"dnssec"`
	Serial         int      `json:"serial"`
	NotifiedSerial int      `json:"notified_serial"`
	LastCheck      int      `json:"last_check"`
	RRsets         []RRset  `json:"rrsets"`
	Records        []Record `json:"records"`
}

// Record struct
type Record struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Disabled bool   `json:"disabled"`
}

// RRset struct
type RRset struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	TTL        int      `json:"ttl"`
	Records    []Record `json:"records"`
	ChangeType string   `json:"changetype"`
}

// RRsets struct
type RRsets struct {
	Sets []RRset `json:"rrsets"`
}

// PowerDNS struct
type PowerDNS struct {
	scheme     string
	hostname   string
	port       string
	path       string
	server     string
	domain     string
	apikey     string
	apiVersion int
}

// New returns a new PowerDNS
func New(baseURL string, server string, domain string, apikey string) (*PowerDNS, error) {

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid url: %v", baseURL, err)
	}
	hp := strings.Split(u.Host, ":")
	hostname := hp[0]

	var port string
	if len(hp) > 1 {
		port = hp[1]
	} else {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	if server == "" {
		server = "localhost"
	}

	powerdns := &PowerDNS{
		scheme:     u.Scheme,
		hostname:   hostname,
		port:       port,
		path:       u.Path,
		server:     server,
		domain:     domain,
		apikey:     apikey,
		apiVersion: -1,
	}

	powerdns.apiVersion, err = powerdns.detectAPIVersion()
	if err != nil {
		return nil, err
	}

	return powerdns, nil
}

// AddRecord ...
func (p *PowerDNS) AddRecord(name string, recordType string, ttl int, content []string) error {

	return p.ChangeRecord(name, recordType, ttl, content, "REPLACE")
}

// DeleteRecord ...
func (p *PowerDNS) DeleteRecord(name string, recordType string, ttl int, content []string) error {

	return p.ChangeRecord(name, recordType, ttl, content, "DELETE")
}

// ChangeRecord ...
func (p *PowerDNS) ChangeRecord(name string, recordType string, ttl int, contents []string, action string) error {

	// Add trailing dot for V1 and removes it for V0
	if p.apiVersion == 1 {
		name = addTrailingCharacter(name, '.')
	} else {
		name = strings.TrimRight(name, ".")
	}

	rrset := RRset{
		Name: name,
		Type: recordType,
		TTL:  ttl,
	}

	for _, content := range contents {
		if rrset.Type == "TXT" {
			content = "\"" + strings.Replace(content, "\"", "", -1) + "\""
		}
		rrset.Records = append(rrset.Records, Record{
			Content: content,
			Name:    name,
			TTL:     ttl,
			Type:    recordType,
		})
	}

	return p.patchRRset(rrset, action)
}

// GetRecords ...
func (p *PowerDNS) GetRecords() ([]Record, error) {

	var records []Record

	zone := new(Zone)
	rerr := new(Error)

	resp, err := p.getSling().Path(p.path+"/servers/"+p.server+"/zones/"+p.domain).Set("X-API-Key", p.apikey).Receive(zone, rerr)

	if err != nil {
		return records, fmt.Errorf("PowerDNS API call has failed: %v", err)
	}

	if resp.StatusCode >= 400 {
		rerr.Message = strings.Join([]string{resp.Status, rerr.Message}, " ")
		return records, rerr
	}

	if len(zone.Records) > 0 {
		for i, record := range zone.Records {
			if record.Type == "TXT" {
				zone.Records[i].Content = strings.Replace(record.Content, "\"", "", -1)
			}
		}
		records = zone.Records
	} else {
		for _, rrset := range zone.RRsets {
			for _, rec := range rrset.Records {
				if rrset.Type == "TXT" {
					rec.Content = strings.Replace(rec.Content, "\"", "", -1)
				}
				if p.apiVersion == 1 {
					rrset.Name = strings.TrimRight(rrset.Name, ".")
				}
				record := Record{
					Name:     rrset.Name,
					Type:     rrset.Type,
					Content:  rec.Content,
					TTL:      rrset.TTL,
					Disabled: rec.Disabled,
				}
				records = append(records, record)
			}
		}
	}

	return records, err
}

func (p *PowerDNS) patchRRset(rrset RRset, action string) error {

	rrset.ChangeType = "REPLACE"

	if action == "DELETE" {
		rrset.ChangeType = "DELETE"
	}

	sets := RRsets{}
	sets.Sets = append(sets.Sets, rrset)

	rerr := new(Error)
	zone := new(Zone)

	resp, err := p.getSling().Path(p.path+"/servers/"+p.server+"/zones/").Patch(p.domain).BodyJSON(sets).Receive(zone, rerr)

	if err == nil && resp.StatusCode >= 400 {
		rerr.Message = strings.Join([]string{resp.Status, rerr.Message}, " ")
		return rerr
	}

	if resp.StatusCode == 204 {
		return nil
	}

	return err
}

func (p *PowerDNS) detectAPIVersion() (int, error) {

	versions := new([]APIVersion)
	info := new(ServerInfo)
	rerr := new(Error)

	resp, err := p.getSling().Path("api").Receive(versions, rerr)
	if resp == nil && err != nil {
		return -1, err
	}

	if resp.StatusCode == 404 {
		resp, err = p.getSling().Path("servers/").Path(p.server).Receive(info, rerr)
		if resp == nil && err != nil {
			return -1, err
		}
	}

	if resp.StatusCode != 200 {
		rerr.Message = strings.Join([]string{resp.Status, rerr.Message}, " ")
		return -1, rerr
	}

	if err != nil {
		return -1, err
	}

	latestVersion := APIVersion{"", 0}
	for _, v := range *versions {
		if v.Version > latestVersion.Version {
			latestVersion = v
		}
	}
	p.path = p.path + latestVersion.URL

	return latestVersion.Version, err
}

func (p *PowerDNS) getSling() *sling.Sling {

	u := new(url.URL)
	u.Host = p.hostname + ":" + p.port
	u.Scheme = p.scheme
	u.Path = p.path

	// Add trailing slash if necessary
	u.Path = addTrailingCharacter(u.Path, '/')

	return sling.New().Base(u.String()).Set("X-API-Key", p.apikey)
}

func addTrailingCharacter(name string, character byte) string {

	// Add trailing dot if necessary
	if len(name) > 0 && name[len(name)-1] != character {
		name += string(character)
	}

	return name
}
