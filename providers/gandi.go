package providers

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	gandi "github.com/prasmussen/gandi-api/client"
	gandiDomain "github.com/prasmussen/gandi-api/domain"
	gandiZone "github.com/prasmussen/gandi-api/domain/zone"
	gandiZoneVersion "github.com/prasmussen/gandi-api/domain/zone/version"
	gandiRecord "github.com/prasmussen/gandi-api/domain/zone/record"
)

type GandiHandler struct {
	record      *gandiRecord.Record
	zoneHandler *gandiZone.Zone
	zone        *gandiZone.ZoneInfoBase
	zoneVersion *gandiZoneVersion.Version
	root        string
	zoneDomain  string
	zoneSuffix  string
	sub         string
}

func init() {
	gandiHandler := &GandiHandler{}

	apiKey := os.Getenv("GANDI_APIKEY")
	if len(apiKey) == 0 {
		logrus.Infof("GANDI_APIKEY is not set, skipping init of %s provider", gandiHandler.GetName())
		return
	}

	if err := RegisterProvider("gandi", gandiHandler); err != nil {
		logrus.Fatal("Could not register Gandi provider")
	}

	systemType := gandi.Production
	testing := os.Getenv("GANDI_TESTING")
	if len(testing) != 0 {
		logrus.Infof("GANDI_TESTING is set, using testing platform")
		systemType = gandi.Testing
	}

	client := gandi.New(apiKey, systemType)
	gandiHandler.record = gandiRecord.New(client)
	gandiHandler.zoneVersion = gandiZoneVersion.New(client)

	root := strings.TrimSuffix(dns.RootDomainName, ".")
	split_root := strings.Split(root, ".")
	split_zoneDomain := split_root[len(split_root)-2:len(split_root)]
	zoneDomain := strings.Join(split_zoneDomain, ".")

	domain := gandiDomain.New(client)
	domainInfo, err := domain.Info(zoneDomain)
	if err != nil {
		logrus.Fatalf("Failed to get zone ID for domain %s: %v", zoneDomain, err)
	}
	zoneId := domainInfo.ZoneId

	zone := gandiZone.New(client)
	zones, err := zone.List()
	if err != nil {
		logrus.Fatalf("Failed to list hosted zones: %v", err)
	}

	found := false
	for _, z := range zones {
		if z.Id == zoneId {
			gandiHandler.root = root
			gandiHandler.zone = z
			found = true
			break
		}
	}

	if !found {
		logrus.Fatalf("Hosted zone %s is missing", root)
	}

	gandiHandler.zoneDomain = zoneDomain
	gandiHandler.zoneSuffix = fmt.Sprintf(".%s", zoneDomain)
	gandiHandler.sub = strings.TrimSuffix(root, zoneDomain)
	gandiHandler.zoneHandler = zone

	logrus.Infof("Configured %s for domain %s using hosted zone %q ", gandiHandler.GetName(), root, gandiHandler.zone.Name)
}

func (*GandiHandler) GetName() string {
	return "Gandi"
}

func (g *GandiHandler) AddRecord(record dns.DnsRecord) error {
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

func (g *GandiHandler) UpdateRecord(record dns.DnsRecord) error {
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

func (g *GandiHandler) RemoveRecord(record dns.DnsRecord) error {
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

func (g *GandiHandler) findRecords(record dns.DnsRecord, version int64) ([]gandiRecord.RecordInfo, error) {
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

func (g *GandiHandler) GetRecords() ([]dns.DnsRecord, error) {
	var records []dns.DnsRecord

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
			record := dns.DnsRecord{Fqdn: fqdn, Records: recordSlice, Type: recordType, TTL: ttl}
			records = append(records, record)
		}
	}

	return records, nil
}

func (g *GandiHandler) parseName(record dns.DnsRecord) string {
	name := strings.TrimSuffix(record.Fqdn, g.zoneSuffix)

	return name
}

func (g *GandiHandler) versionAddRecord(version int64, record dns.DnsRecord) error {

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

func (g *GandiHandler) versionRemoveRecord(version int64, record dns.DnsRecord) error {
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

func (g *GandiHandler) newZoneVersion() (int64, error) {
	// Get latest zone version
	zoneInfo, err := g.zoneHandler.Info(g.zone.Id)
	if err != nil {
		logrus.Fatalf("Failed to refresh zone information: %v", g.zone.Name, err)
	}

	newVersion, err := g.zoneVersion.New(g.zone.Id, zoneInfo.Version)
	if err != nil {
		logrus.Fatalf("Failed to create new version of zone %s: %v", g.zone.Name, err)
	}

	return newVersion, nil
}

func (g *GandiHandler) setZoneVersion(version int64) (error) {
	_, err := g.zoneVersion.Set(g.zone.Id, version)
	if err != nil {
		logrus.Fatalf("Failed to set version of zone %s to %v: %v", g.zone.Name, version, err)
	}

	return err
}
