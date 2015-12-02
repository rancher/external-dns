package cloudflare

import (
	"log"

	"golang.org/x/net/context"
)

// ExampleRecordsList - Lists all zone DNS records.
func ExampleRecordsList(ctx context.Context, client *Client) {
	zones, err := client.Zones.List(ctx)
	if err != nil {
		log.Fatal(err)
	} else if len(zones) == 0 {
		log.Fatal("No zones were found")
	}

	records, err := client.Records.List(ctx, zones[0].ID)
	if err != nil {
		log.Fatal(err)
	}

	for _, record := range records {
		log.Printf("%#v", record)
	}
}
