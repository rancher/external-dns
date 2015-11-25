package cloudflare

// Options - Cloudflare API Client Options.
type Options struct {
	Email, Key string
}

// Client - Cloudflare API Client.
type Client struct {
	*Zones
	*Records
	opts *Options
}

// New - Creates a new Cloudflare client.
func New(opts *Options) *Client {
	return &Client{
		Zones:   &Zones{opts: opts},
		Records: &Records{opts: opts},
		opts:    opts,
	}
}
