package infoblox

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"io/ioutil"

	"github.com/Sirupsen/logrus"
	api "github.com/fanatic/go-infoblox"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

const (
	versionURL   = "/wapi/v1.5/"
	authURL      = versionURL + "zone_auth"
	recordURL    = versionURL + "record"
	recordTxtURL = versionURL + "record:txt"
	recordAURL   = versionURL + "record:a"
	maxResults   = "1000"
	firstPage    = "_return_as_object=1&_max_results=" + maxResults + "&_paging=1"
)

var (
	recordAQuery   = "_return_fields=ttl,name,zone,disable,ipv4addr"
	recordTxtQuery = "_return_fields=ttl,name,zone,disable,text"
	head           = map[string]string{
		"Content-Type": "application/json",
	}
)

type Record struct {
	Ref      string `json:"_ref"`
	Fqdn     string `json:"fqdn"`
	Name     string `json:"name"`
	Zone     string `json:"zone"`
	TTL      int    `json:"ttl"`
	Disable  bool   `json:"disable"`
	IPv4addr string `json:"ipv4addr"`
	Text     string `json:"text"`
	Type     string
	Rec      string
}

type ResultPagination struct {
	Page_id	string 	  `json:"next_page_id"`
	Result 	[]*Record `json:"result"`
}

type InfobloxProvider struct {
	client   *api.Client
	zoneName string
}

func init() {
	providers.RegisterProvider("infoblox", &InfobloxProvider{})
}

func (d *InfobloxProvider) Init(rootDomainName string) error {
	var url, userName, password, secretFile string
	var sslVerify, useCookies bool
	var err error
	if url = os.Getenv("INFOBLOX_URL"); len(url) == 0 {
		return fmt.Errorf("INFOBLOX_URL is not set")
	}

	if userName = os.Getenv("INFOBLOX_USER_NAME"); len(userName) == 0 {
		return fmt.Errorf("INFOBLOX_USER_NAME is not set")
	}

	if password = os.Getenv("INFOBLOX_PASSWORD"); len(password) == 0 {
		if secretFile = os.Getenv("INFOBLOX_SECRET"); len(secretFile) == 0 {
			return fmt.Errorf("INFOBLOX_PASSWORD nor INFOBLOX_SECRET are not set")
		}

		// If password nil, using secrets
		p, err := ioutil.ReadFile(secretFile)
		if err != nil {
			return fmt.Errorf("Error reading INFOBLOX_SECRET %s: %v", secretFile, err)
		}
		if len(p) == 0 {
			return fmt.Errorf("Got empty password from INFOBLOX_SECRET")
		}

		logrus.Debugf("Infoblox using INFOBLOX_SECRET %s", secretFile)
		password = string(p)
	}

	if sslVerifyStr := os.Getenv("SSL_VERIFY"); len(sslVerifyStr) == 0 {
		sslVerify = false
	} else {
		if sslVerify, err = strconv.ParseBool(sslVerifyStr); err != nil {
			return fmt.Errorf("SSL_VERIFY must be a boolean value")
		}
	}

	if useCookiesStr := os.Getenv("USE_COOKIES"); len(useCookiesStr) == 0 {
		useCookies = false
	} else {
		if useCookies, err = strconv.ParseBool(useCookiesStr); err != nil {
			return fmt.Errorf("USE_COOKIES must be a boolean value")
		}
	}

	d.client = api.NewClient(url, userName, password, sslVerify, useCookies)
	d.zoneName = utils.UnFqdn(rootDomainName)

	if err = d.validateZoneName(d.zoneName); err != nil {
		return err
	}
	return nil
}

func (d *InfobloxProvider) validateZoneName(zoneName string) error {
	zoneAuths, err := d.SendRequest("GET", authURL, "", nil)
	if err != nil {
		return fmt.Errorf("Infoblox API failed, func validateZoneName, %v", err)
	}

	for _, v := range zoneAuths {
		if v.Fqdn == zoneName {
			return nil
		}
	}
	return fmt.Errorf("Could not find ZoneName %s in infoblox", zoneName)
}

func (*InfobloxProvider) GetName() string {
	return "Infoblox"
}

func (d *InfobloxProvider) HealthCheck() error {
	max := 1
	opts := &api.Options{
		MaxResults: &max,
	}
	_, err := d.client.RecordHost().All(opts)
	return err
}

func (d *InfobloxProvider) AddRecord(record utils.DnsRecord) (err error) {
	var url, body string
	for _, rec := range record.Records {
		if url, body, err = d.prepareRecord(rec, record.Type, record.Fqdn, record.TTL); err != nil {
			return err
		}
		if _, err = d.SendRequest("POST", url, body, head); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				return nil
			}
			return fmt.Errorf("Infoblox API call has failed: %v", err)
		}
	}
	return nil
}

func (d *InfobloxProvider) findRecords(record utils.DnsRecord) ([]*Record, error) {
	url := recordURL + ":" + strings.ToLower(record.Type) + "?name=" + utils.UnFqdn(record.Fqdn) + "&zone=" + d.zoneName

	records, err := d.SendRequest("GET", url, "", head)
	if err != nil {
		return records, fmt.Errorf("Infoblox API call decode has failed: %v", err)
	}

	return records, nil
}

func (d *InfobloxProvider) UpdateRecord(record utils.DnsRecord) error {
	if err := d.RemoveRecord(record); err != nil {
		return err
	}

	return d.AddRecord(record)
}

func (d *InfobloxProvider) RemoveRecord(record utils.DnsRecord) error {
	records, err := d.findRecords(record)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if _, err := d.SendRequest("DELETE", versionURL+rec.Ref, "", head); err != nil {
			return fmt.Errorf("Infoblox API call has failed: %v", err)
		}
	}
	return nil
}

func (d *InfobloxProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord

	recordAs, err := d.SendRequest("GET", recordAURL + "?" + recordAQuery + "&zone=" + d.zoneName, "", head)
	if err != nil {
		return records, fmt.Errorf("Infoblox API call decode has failed: %v", err)
	}

	recordTxts, err := d.SendRequest("GET", recordTxtURL + "?" + recordTxtQuery + "&zone=" + d.zoneName, "", head)
	if err != nil {
		return records, fmt.Errorf("Infoblox API call decode has failed: %v", err)
	}

	d.setRecordType("A", recordAs)
	d.setRecordType("TXT", recordTxts)

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}
	for _, rec := range append(recordAs, recordTxts...) {
		if rec.Disable {
			continue
		}

		fqdn := fmt.Sprintf("%s.", rec.Name)
		if rec.Name == "" {
			fqdn = fmt.Sprintf("%s.", d.zoneName)
		}

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][rec.Type] = rec.TTL
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[rec.Type]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Rec)
				recordSet[rec.Type] = recordSlice
			} else {
				recordSet[rec.Type] = []string{rec.Rec}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.Type] = []string{rec.Rec}
		}
	}

	for fqdn, recordSet := range recordMap {
		for recordType, recordSlice := range recordSet {
			ttl := recordTTLs[fqdn][recordType]
			record := utils.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}
	return records, nil
}

func (d *InfobloxProvider) prepareRecord(rec string, tp string, fqdn string, ttl int) (string, string, error) {
	var url string
	body := make(map[string]interface{})
	if tp == "TXT" {
		body["text"] = rec
		url = recordTxtURL
	} else if tp == "A" {
		body["ipv4addr"] = rec
		url = recordAURL
	} else {
		logrus.Warnf("Warning unsupport record type: %s", tp)
		return "", "", nil
	}
	body["name"] = utils.UnFqdn(fqdn)
	body["ttl"] = ttl
	body["comment"] = tp
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", "", fmt.Errorf("Error prepareRecord request: %v", err)
	}
	return url, string(bodyJSON), nil
}

func (d *InfobloxProvider) setRecordType(ty string, records []*Record) []*Record {
	for _, v := range records {
		v.Type = ty
		if v.Type == "A" {
			v.Rec = v.IPv4addr
		} else if v.Type == "TXT" {
			v.Rec = v.Text
		}
	}
	return records
}

func (d *InfobloxProvider) SendRequest(method, urlStr, body string, head map[string]string) ([]*Record, error) {
	if urlStr == "" {
		return nil, fmt.Errorf("SendRequest to infoblox url is nil")
	}

	if method == "GET" {
		// Using pagination on all get requests
		// Checking if needs '?' or '&'
		var url string
		if strings.Contains(urlStr, "?") {
			url = urlStr + "&" + firstPage
		} else {
			url = urlStr + "?" + firstPage
		}
		logrus.Debugf("SendRequest to infoblox with pagination: [method]%s, [url] %s", method, url)

		res, err := d.client.SendRequest(method, url, body, head)
		if err != nil {
			return nil, err
		}

		var result ResultPagination
		if err = res.Parse(&result); err != nil {
			return nil, err
		}
		logrus.Debugf("SendRequest infoblox get %d of %s records", len(result.Result), maxResults)
		records := result.Result
		for result.Page_id != "" {
			// Adding _page_id to nextPage url
			nextPage := url + "&_page_id=" + result.Page_id
			logrus.Debugf("SendRequest infoblox getting next _page_id %s", result.Page_id)
			// emptying result.Page_id
			result.Page_id = ""

			res, err := d.client.SendRequest(method, nextPage, body, head)
			if err != nil {
				return nil, err
			}
			if err = res.Parse(&result); err != nil {
				return nil, err
			}

			logrus.Debugf("SendRequest infoblox get %d of %s records", len(result.Result), maxResults)
			for _, record := range result.Result {
				records = append(records, record)
			}
		}
		return records, nil
	}

	logrus.Debugf("SendRequest to infoblox: [method]%s, [url] %s, [body] %s, [head] %v", method, urlStr, body)
	_, err := d.client.SendRequest(method, urlStr, body, head)

	return nil, err	

}
