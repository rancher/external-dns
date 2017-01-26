package cloudflare

// Options - Cloudflare API Client Options.
type Options struct {
	Email, Key string
}

// Client - Cloudflare API Client.
type Client struct {
	// Zones - Zones API Client.
	*Zones
	// Records - Records API Client.
	*Records
	// Firewalls - Firewalls API Client.
	*Firewalls
	// Options - API Client options.
	*Options
}

// New - Creates a new Cloudflare client.
func New(opts *Options) *Client {
	return &Client{
		Zones:     &Zones{Options: opts},
		Records:   &Records{Options: opts},
		Firewalls: &Firewalls{Options: opts},
		Options:   opts,
	}
}
