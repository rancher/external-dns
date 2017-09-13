package infoblox

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	api "github.com/fanatic/go-infoblox"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/external-dns/utils"
)

const (
	versionURL   = "/wapi/v1.4/"
	authURL      = "/wapi/v1.4/zone_auth"
	recordURL    = "/wapi/v1.4/record"
	recordTxtURL = "/wapi/v1.4/record:txt"
	recordAURL   = "/wapi/v1.4/record:a"
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

type InfobloxProvider struct {
	client   *api.Client
	zoneName string
}

func init() {
	providers.RegisterProvider("infoblox", &InfobloxProvider{})
}

func (d *InfobloxProvider) Init(rootDomainName string) error {
	var url, userName, password string
	var sslVerify, useCookies bool
	var err error
	if url = os.Getenv("INFOBLOX_URL"); len(url) == 0 {
		return fmt.Errorf("INFOBLOX_URL is not set")
	}

	if userName = os.Getenv("INFOBLOX_USER_NAME"); len(userName) == 0 {
		return fmt.Errorf("INFOBLOX_USER_NAME is not set")
	}

	if password = os.Getenv("INFOBLOX_PASSWORD"); len(password) == 0 {
		return fmt.Errorf("INFOBLOX_PASSWORD is not set")
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
	res, err := d.client.SendRequest("GET", authURL, "", nil)
	if err != nil {
		return fmt.Errorf("Infoblox API call has failed, func validateZoneName, %v", err)
	}

	var zoneAuths []Record
	if err = res.Parse(&zoneAuths); err != nil {
		return fmt.Errorf("Infoblox API parse res failed, func validateZoneName, %v", err)
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
	_, err := d.client.RecordHost().All(nil)
	return err
}

func (d *InfobloxProvider) AddRecord(record utils.DnsRecord) (err error) {
	var url, body string
	for _, rec := range record.Records {
		if url, body, err = d.prepareRecord(rec, record.Type, record.Fqdn, record.TTL); err != nil {
			return err
		}
		if _, err = d.client.SendRequest("POST", url, body, head); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				return nil
			}
			return fmt.Errorf("Infoblox API call has failed: %v", err)
		}
	}
	return nil
}

func (d *InfobloxProvider) findRecords(record utils.DnsRecord) ([]*Record, error) {
	var records []*Record
	url := recordURL + ":" + strings.ToLower(record.Type) + "?name=" + utils.UnFqdn(record.Fqdn)
	res, err := d.client.SendRequest("GET", url, "", head)
	if err != nil {
		return records, fmt.Errorf("Infoblox API call has failed: %v", err)
	}

	if err = res.Parse(&records); err != nil {
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
		if _, err := d.client.SendRequest("DELETE", versionURL+rec.Ref, "", head); err != nil {
			return fmt.Errorf("Infoblox API call has failed: %v", err)
		}
	}
	return nil
}

func (d *InfobloxProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord
	res, err := d.client.SendRequest("GET", recordAURL+"?"+recordAQuery, "", head)
	if err != nil {
		return records, fmt.Errorf("Infoblox API call has failed: %v", err)
	}
	var recordAs []*Record
	if err = res.Parse(&recordAs); err != nil {
		return records, fmt.Errorf("Infoblox API call decode has failed: %v", err)
	}

	res2, err := d.client.SendRequest("GET", recordTxtURL+"?"+recordTxtQuery, "", head)
	if err != nil {
		return records, fmt.Errorf("Infoblox API call has failed: %v", err)
	}
	var recordTxts []*Record
	if err = res2.Parse(&recordTxts); err != nil {
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
