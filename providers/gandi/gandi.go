package gandi

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	gandiClient "github.com/prasmussen/gandi-api/client"
	gandiDomain "github.com/prasmussen/gandi-api/domain"
	gandiZone "github.com/prasmussen/gandi-api/domain/zone"
	gandiRecord "github.com/prasmussen/gandi-api/domain/zone/record"
	gandiZoneVersion "github.com/prasmussen/gandi-api/domain/zone/version"
	gandiOperation "github.com/prasmussen/gandi-api/operation"
	"github.com/mathuin/external-dns/providers"
	"github.com/mathuin/external-dns/utils"
)

type GandiProvider struct {
	record      *gandiRecord.Record
	zoneHandler *gandiZone.Zone
	zone        *gandiZone.ZoneInfoBase
	zoneVersion *gandiZoneVersion.Version
	operation   *gandiOperation.Operation
	root        string
	zoneDomain  string
	zoneSuffix  string
	sub         string
}

func init() {
	providers.RegisterProvider("gandi", &GandiProvider{})
}

func (g *GandiProvider) Init(rootDomainName string) error {
	var apiKey string
	if apiKey = os.Getenv("GANDI_APIKEY"); len(apiKey) == 0 {
		return fmt.Errorf("GANDI_APIKEY is not set")
	}

	systemType := gandiClient.Production
	testing := os.Getenv("GANDI_TESTING")
	if len(testing) != 0 {
		logrus.Infof("GANDI_TESTING is set, using testing platform")
		systemType = gandiClient.Testing
	}

	client := gandiClient.New(apiKey, systemType)
	g.record = gandiRecord.New(client)
	g.zoneVersion = gandiZoneVersion.New(client)
	g.operation = gandiOperation.New(client)

	root := utils.UnFqdn(rootDomainName)
	split_root := strings.Split(root, ".")
	split_zoneDomain := split_root[len(split_root)-2 : len(split_root)]
	zoneDomain := strings.Join(split_zoneDomain, ".")

	domain := gandiDomain.New(client)
	domainInfo, err := domain.Info(zoneDomain)
	if err != nil {
		return fmt.Errorf("Failed to get zone ID for domain %s: %v", zoneDomain, err)
	}
	zoneId := domainInfo.ZoneId

	zone := gandiZone.New(client)
	zones, err := zone.List()
	if err != nil {
		return fmt.Errorf("Failed to list hosted zones: %v", err)
	}

	found := false
	for _, z := range zones {
		if z.Id == zoneId {
			g.root = root
			g.zone = z
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("Zone for '%s' not found", root)
	}

	g.zoneDomain = zoneDomain
	g.zoneSuffix = fmt.Sprintf(".%s", zoneDomain)
	g.sub = strings.TrimSuffix(root, zoneDomain)
	g.zoneHandler = zone

	logrus.Infof("Configured %s for domain '%s' using zone '%s'", g.GetName(), root, g.zone.Name)
	return nil
}

func (*GandiProvider) GetName() string {
	return "Gandi"
}

func (g *GandiProvider) HealthCheck() error {
	_, err := g.operation.Count()
	return err
}

func (g *GandiProvider) AddRecord(record utils.DnsRecord) error {
	newVersion, err := g.newZoneVersion()
	if err != nil {
		return fmt.Errorf("Failed to add new record: %v", err)
	}

	if err := g.versionAddRecord(newVersion, record); err != nil {
		return fmt.Errorf("Failed to add new record: %v", err)
	}

	if err := g.setZoneVersion(newVersion); err != nil {
		return fmt.Errorf("Failed to add new record: %v", err)
	}

	return nil
}

func (g *GandiProvider) UpdateRecord(record utils.DnsRecord) error {
	newVersion, err := g.newZoneVersion()
	if err != nil {
		return fmt.Errorf("Failed to update record: %v", err)
	}

	if err := g.versionRemoveRecord(newVersion, record); err != nil {
		return fmt.Errorf("Failed to update record: %v", err)
	}

	if err := g.versionAddRecord(newVersion, record); err != nil {
		return fmt.Errorf("Failed to update record: %v", err)
	}

	if err := g.setZoneVersion(newVersion); err != nil {
		return fmt.Errorf("Failed to update record: %v", err)
	}

	return err
}

func (g *GandiProvider) RemoveRecord(record utils.DnsRecord) error {
	newVersion, err := g.newZoneVersion()
	if err != nil {
		return fmt.Errorf("Failed to remove record: %v", err)
	}

	if err := g.versionRemoveRecord(newVersion, record); err != nil {
		return fmt.Errorf("Failed to remove record: %v", err)
	}

	if err := g.setZoneVersion(newVersion); err != nil {
		return fmt.Errorf("Failed to add new record: %v", err)
	}

	return nil
}

func (g *GandiProvider) findRecords(record utils.DnsRecord, version int64) ([]gandiRecord.RecordInfo, error) {
	var records []gandiRecord.RecordInfo
	resp, err := g.record.List(g.zone.Id, version)
	if err != nil {
		return records, fmt.Errorf("Failed to find record in zone: %v", err)
	}

	name := g.parseName(record)
	for _, rec := range resp {
		recName := fmt.Sprintf("%s.%s.", rec.Name, g.zoneDomain)
		if recName == name && rec.Type == record.Type {
			records = append(records, *rec)
		}
	}

	return records, nil
}

func (g *GandiProvider) GetRecords() ([]utils.DnsRecord, error) {
	var records []utils.DnsRecord

	recordResp, err := g.record.List(g.zone.Id, g.zone.Version)
	if err != nil {
		return records, fmt.Errorf("Failed to get records in zone: %v", err)
	}

	recordMap := map[string]map[string][]string{}
	recordTTLs := map[string]map[string]int{}

	for _, rec := range recordResp {
		var fqdn string
		if rec.Name == "" {
			fqdn = fmt.Sprintf("%s.", g.zoneDomain)
		} else {
			fqdn = fmt.Sprintf("%s.%s.", rec.Name, g.zoneDomain)
		}

		recordTTLs[fqdn] = map[string]int{}
		recordTTLs[fqdn][rec.Type] = int(rec.Ttl)
		recordSet, exists := recordMap[fqdn]
		if exists {
			recordSlice, sliceExists := recordSet[rec.Type]
			if sliceExists {
				recordSlice = append(recordSlice, rec.Value)
				recordSet[rec.Type] = recordSlice
			} else {
				recordSet[rec.Type] = []string{rec.Value}
			}
		} else {
			recordMap[fqdn] = map[string][]string{}
			recordMap[fqdn][rec.Type] = []string{rec.Value}
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

func (g *GandiProvider) parseName(record utils.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, g.zoneSuffix)
	return name
}

func (g *GandiProvider) versionAddRecord(version int64, record utils.DnsRecord) error {
	name := g.parseName(record)
	for _, rec := range record.Records {
		suffix := fmt.Sprintf(".%s.", g.zoneDomain)
		sub := strings.TrimSuffix(name, suffix)
		logrus.Infof("Adding record %s", sub)
		args := gandiRecord.RecordAdd{
			Zone:    g.zone.Id,
			Version: version,
			Name:    sub,
			Type:    record.Type,
			Value:   rec,
			Ttl:     int64(record.TTL),
		}

		_, err := g.record.Add(args)
		if err != nil {
			return fmt.Errorf("Failed to add record: %v", err)
		}
	}

	return nil
}

func (g *GandiProvider) versionRemoveRecord(version int64, record utils.DnsRecord) error {
	records, err := g.findRecords(record, version)
	if err != nil {
		return err
	}

	for _, rec := range records {
		logrus.Infof("Removing record %s with ID %v", rec.Name, rec.Id)
		_, err := g.record.Delete(g.zone.Id, version, rec.Id)
		if err != nil {
			return fmt.Errorf("Failed to remove record: %v", err)
		}
	}

	return nil
}

func (g *GandiProvider) newZoneVersion() (int64, error) {
	// Get latest zone version
	zoneInfo, err := g.zoneHandler.Info(g.zone.Id)
	if err != nil {
		return 0, fmt.Errorf("Failed to refresh zone information: %v", g.zone.Name, err)
	}

	newVersion, err := g.zoneVersion.New(g.zone.Id, zoneInfo.Version)
	if err != nil {
		return 0, fmt.Errorf("Failed to create new version of zone %s: %v", g.zone.Name, err)
	}

	return newVersion, nil
}

func (g *GandiProvider) setZoneVersion(version int64) error {
	_, err := g.zoneVersion.Set(g.zone.Id, version)
	if err != nil {
		return fmt.Errorf("Failed to set version of zone %s to %v: %v", g.zone.Name, version, err)
	}

	return nil
}
