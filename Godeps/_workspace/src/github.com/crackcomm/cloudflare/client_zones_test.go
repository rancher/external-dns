package cloudflare

import (
	"log"

	"golang.org/x/net/context"
)

// ExampleZonesList - Lists all zones.
func ExampleZonesList(ctx context.Context, client *Client) {
	zones, err := client.Zones.List(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, zone := range zones {
		log.Printf("%s", zone.Name)
	}
}

// ExampleZoneDetails - Gets zone details by ID.
func ExampleZoneDetails(ctx context.Context, client *Client) {
	zones, err := client.Zones.List(ctx)
	if err != nil {
		log.Fatal(err)
	} else if len(zones) == 0 {
		log.Fatal("No zones were found")
	}

	zone, err := client.Zones.Details(ctx, zones[0].ID)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Got %s = %#v", zones[0].ID, zone)
}

// ExampleZoneDelete - Deletes zone by ID.
func ExampleZoneDelete(ctx context.Context, client *Client) {
	zones, err := client.Zones.List(ctx)
	if err != nil {
		log.Fatal(err)
	} else if len(zones) == 0 {
		log.Fatal("No zones were found")
	}

	err = client.Zones.Delete(ctx, zones[0].ID)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Deleted %s", zones[0].ID)
}
