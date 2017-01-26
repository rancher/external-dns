package pointdns

import (
    "fmt"
)

type Record struct {
    Id          int     `json:"id,omitempty"`
    Name        string  `json:"name,omitempty"`
    Data        string  `json:"data,omitempty"`
    Ttl         int     `json:"ttl,omitempty"`
    Aux         string  `json:"aux,omitempty"`
    ZoneId      int     `json:"zone_id,omitempty"`
    RecordType  string `json:"record_type,omitempty"`

    pointClient *PointClient
}

type RootRecord struct {
    Record Record `json:"zone_record"`
}

func recordPath (zone interface{}, record *Record) string {
    path := fmt.Sprintf("zones/%s/records", zoneIdentifier(zone))
    if record != nil && record.Id > 0 {
        path += fmt.Sprintf("/%d", record.Id)
    }
    return path
}

func (client *PointClient) Records(zone interface{}) ([]Record, error) {
    rawRecords := []RootRecord{}
    err := client.Get(recordPath(zone, nil), &rawRecords)
    if err != nil {
        return []Record{}, err
    }
    records := []Record{}

    for _, record := range rawRecords {
        record.Record.pointClient = client
        records = append(records, record.Record)
    }
    return records, nil
}

func (client *PointClient) Record(zone interface{}, record *Record) (Record, error) {
    rawRecord := RootRecord{}
    err := client.Get(recordPath(zone, record), &rawRecord)
    if err != nil {
        return Record{}, err
    }
    rawRecord.Record.pointClient = client
    return rawRecord.Record, nil
}

func (rootRecord RootRecord) Id() int {
    return rootRecord.Record.Id
}

func (zone *Zone) Records() ([]Record, error) {
    records, err := zone.pointClient.Records(*zone)
    return records, err
}

func (record *Record) Delete() (bool, error) {
    zone := Zone{Id: record.ZoneId}
    err := record.pointClient.Delete(recordPath(zone, record), nil)
    if err != nil {
        return false, err
    }
    return true, nil
}

func (client *PointClient) CreateRecord(record *Record) (bool, error) {
    record.pointClient = client
    return record.Save()
}

func (record *Record) Save() (bool, error) {
    path := recordPath(Zone{Id: record.ZoneId}, record)
    rootRecord := RootRecord{Record: *record}
    returnedRecord := RootRecord{}
    err := record.pointClient.Save(path, rootRecord, &returnedRecord)
    if err != nil {
        return false, err
    }
    returnedRecord.Record.pointClient = record.pointClient
    *record = returnedRecord.Record
    return true, nil
}

