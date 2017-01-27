package cloudflare

import (
	"bytes"
	"encoding/json"

	"golang.org/x/net/context"
)

// Records - Cloudflare Records API Client.
type Records struct {
	// Options - Client options.
	*Options
}

// Create - Creates a zone DNS record.
// Required parameters of a record are - `type`, `name` and `content`.
// Optional parameters of a record are - `ttl`.
func (records *Records) Create(ctx context.Context, record *Record) (err error) {
	buffer := new(bytes.Buffer)
	err = json.NewEncoder(buffer).Encode(record)
	if err != nil {
		return
	}
	response, err := httpDo(ctx, records.Options, "POST", apiURL("/zones/%s/dns_records", record.ZoneID), buffer)
	if err != nil {
		return
	}
	defer response.Body.Close()
	result, err := readResponse(response.Body)
	if err != nil {
		return
	}
	res := new(Record)
	err = json.Unmarshal(result.Result, &res)
	if err != nil {
		return
	}
	record.ID = res.ID
	record.TTL = res.TTL
	record.Locked = res.Locked
	record.Proxied = res.Proxied
	record.Proxiable = res.Proxiable
	record.CreatedOn = res.CreatedOn
	record.ModifiedOn = res.ModifiedOn
	return
}

// List - Lists all zone DNS records.
func (records *Records) List(ctx context.Context, zoneID string) ([]*Record, error) {
	return records.listPages(ctx, zoneID, 1)
}

// Details - Requests zone DNS record details by zone ID and record ID.
func (records *Records) Details(ctx context.Context, zoneID, recordID string) (record *Record, err error) {
	response, err := httpDo(ctx, records.Options, "GET", apiURL("/zones/%s/dns_records/%s", zoneID, recordID), nil)
	if err != nil {
		return
	}
	defer response.Body.Close()
	result, err := readResponse(response.Body)
	if err != nil {
		return
	}
	record = new(Record)
	err = json.Unmarshal(result.Result, &record)
	return
}

// Patch - Patches a zone DNS record.
func (records *Records) Patch(ctx context.Context, record *Record) (err error) {
	buffer := new(bytes.Buffer)
	err = json.NewEncoder(buffer).Encode(record)
	if err != nil {
		return
	}
	response, err := httpDo(ctx, records.Options, "PUT", apiURL("/zones/%s/dns_records/%s", record.ZoneID, record.ID), buffer)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, err = readResponse(response.Body)
	return
}

// Delete - Deletes zone DNS record by zone ID and record ID.
func (records *Records) Delete(ctx context.Context, zoneID, recordID string) (err error) {
	response, err := httpDo(ctx, records.Options, "DELETE", apiURL("/zones/%s/dns_records/%s", zoneID, recordID), nil)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, err = readResponse(response.Body)
	return
}

// listPages - Gets all pages starting from `page`.
func (records *Records) listPages(ctx context.Context, zoneID string, page int) (list []*Record, err error) {
	response, err := httpDo(ctx, records.Options, "GET", apiURL("/zones/%s/dns_records?page=%d&per_page=50", zoneID, page), nil)
	if err != nil {
		return
	}
	defer response.Body.Close()
	result, err := readResponse(response.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(result.Result, &list)
	if err != nil {
		return
	}
	if result.ResultInfo == nil || page >= result.ResultInfo.TotalPages {
		return
	}
	next, err := records.listPages(ctx, zoneID, page+1)
	if err != nil {
		return
	}
	return append(list, next...), nil
}
