go-pointdns
===========

This library provides easy access to point zone & record management. For information about the services offered on Point see [the website](http://pointhq.com)

## Authentication

To access your Point account, you'll need to define your username & password. The username is your email address and the password is the API token which, can be found in My Account tab.

## Example

```go
package main

import (
    "fmt"
    pointdns "github.com/copper/go-pointdns"
)

func main() {
    email := "user@example.com"
    apiToken := "6v1a532e-384g-f3345-ca34-d71a34f376545"
    client := pointdns.NewClient(email, apiToken)

    // Create a new zone
    newZone := pointdns.Zone{Name: "example.com"}

    savedZone, _ := client.CreateZone(&newZone)
    if savedZone {
        fmt.Println("Zone successfully created")
    }

    // Get list of zones
    zones, _ := client.Zones()
    for _, zone := range zones {
        fmt.Println("Zone:\n", zone.Name)
    }

    // Create a new record
    newRecord := pointdns.Record{
        Name: "www.example.com.",
        Data: "1.2.3.4",
        RecordType: "A",
        Ttl: 1800,
        ZoneId: newZone.Id,
    }

    savedRecord, _ := client.CreateRecord(&newRecord)
    if savedRecord {
        fmt.Println("Record successfully created")
    }

    // Update a record
    newRecord.Data = "4.3.2.1"
    saved, _ := newRecord.Save()
    if saved {
        fmt.Println("Record successfully updated")
    }

    // Get list of records for zone
    records, _ := newZone.Records()
    for _, record := range records {
        fmt.Println("Record:\n", record.Name)
    }

    // Delete a record
    deletedRecord, _ := newRecord.Delete()
    if deletedRecord {
        fmt.Println("Record successfully deleted")
    }

    // Delete a zone
    deletedZone, _ := newZone.Delete()
    if deletedZone {
        fmt.Println("Zone successfully deleted")
    }
}

```

Copyright (c) 2014 Copper Inc. See LICENSE for details.
