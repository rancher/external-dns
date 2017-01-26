package pointdns

import (
    "fmt"
)

type Zone struct {
    Id      int     `json:"id,omitempty"`
    Name    string  `json:"name,omitempty"`
    Ttl     int     `json:"ttl,omitempty"`
    Group   string  `json:"group,omitempty"`
    UserId  int     `json:"user-id,omitempty"`

    pointClient *PointClient
}

type RootZone struct {
    Zone Zone `json:"zone"`
}

func zoneIdentifier(value interface{}) string {
	switch value := value.(type) {
	case string:
		return value
	case int:
		return fmt.Sprintf("%d", value)
	case Zone:
		return fmt.Sprintf("%d", value.Id)
	case Record:
		return fmt.Sprintf("%d", value.ZoneId)
	}
	return ""
}

func zonePath (zone interface{}) string {
    if zone != nil {
        return fmt.Sprintf("zones/%s", zoneIdentifier(zone))
    }
    return "zones"
}

func (client *PointClient) Zones() ([]Zone, error) {
    rawZones := []RootZone{}

    err := client.Get(zonePath(nil), &rawZones)
    if err != nil {
        return []Zone{}, err
    }
    zones := []Zone{}
    for _, zone := range rawZones {
        zone.Zone.pointClient = client
        zones = append(zones, zone.Zone)
    }
    return zones, nil
}

func (client *PointClient) Zone(zone interface{}) (Zone, error) {
    rawZone := RootZone{}
    err := client.Get(zonePath(zone), &rawZone)
    if err != nil {
        return Zone{}, err
    }
    return rawZone.Zone, nil
}

func (rootZone RootZone) Id() int {
    return rootZone.Zone.Id
}

func (zone *Zone) Delete() (bool, error) {
    path := zonePath(*zone)
    err := zone.pointClient.Delete(path, nil)
    if err != nil {
        return false, err
    }
    return true, nil
}

func (client *PointClient) CreateZone(zone *Zone) (bool, error) {
    zone.pointClient = client
    return zone.Save()
}

func (zone *Zone) Save() (bool, error) {
    path := zonePath(*zone)
    rootZone := RootZone{Zone: *zone}
    returnedZone := RootZone{}
    err := zone.pointClient.Save(path, rootZone, &returnedZone)
    if err != nil {
        return false, err
    }
    returnedZone.Zone.pointClient = zone.pointClient
    *zone = returnedZone.Zone
    return true, nil
}

