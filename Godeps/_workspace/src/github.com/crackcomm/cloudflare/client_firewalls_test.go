package cloudflare

import (
	"log"

	"golang.org/x/net/context"
)

// ExampleFirewalls_List - Lists all firewall rules for a zone.
func ExampleFirewalls_List(ctx context.Context, client *Client) {
	zones, err := client.Zones.List(ctx)
	if err != nil {
		log.Fatal(err)
	} else if len(zones) == 0 {
		log.Fatal("No zones were found")
	}

	rules, err := client.Firewalls.List(ctx, zones[0].ID)
	if err != nil {
		log.Fatal(err)
	}

	for _, rule := range rules {
		log.Printf("%#v", rule)
	}
}
