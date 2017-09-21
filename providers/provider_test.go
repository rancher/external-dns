package providers

import (
	"github.com/rancher/external-dns/utils"
	"testing"
)

/*
 * --- Test Data ---
 */

var providerTestData = []struct {
	name     string
	provider Provider
}{
	{
		"Provider-1",
		NewMockProvider(),
	},
}

/*
 * --- Tests ---
 */

func testProvider(t *testing.T, provider Provider) {
	var probe *Probe = provider.(MockProvider).Probe

	if provider.Init("example.com"); !probe.hasInit {
		t.Errorf("Expected MockProvider to be initialized. Probe found init to be false.")
	}

	if provider.HealthCheck(); !probe.hasHealthCheck {
		t.Errorf("Expected HealthCheck to run and recorded on probe. Probe found hasHealthCheck to be false.")
	}

	if provider.AddRecord(utils.DnsRecord{}); !probe.hasAddRecord {
		t.Errorf("Expected AddRecord to run and recorded on probe. Probe found hasAddRecord to be false.")
	}

	if provider.RemoveRecord(utils.DnsRecord{}); !probe.hasRemoveRecord {
		t.Errorf("Expected RemoveRecord to run and recorded on probe. Probe found hasRemoveRecord to be false.")
	}

	if provider.UpdateRecord(utils.DnsRecord{}); !probe.hasUpdateRecord {
		t.Errorf("Expected UpdateRecord to run and recorded on probe. Probe found hasUpdateRecord to be false.")
	}

	if provider.GetRecords(); !probe.hasUpdateRecord {
		t.Errorf("Expected GetRecords to run and recorded on probe. Probe found hasGetRecords to be false.")
	}
}

// Bit of sanity to check the MockProvider. We make use of the probe later.
func TestMockProvider(t *testing.T) {
	for _, asset := range providerTestData {
		provider := asset.provider

		testProvider(t, provider)
	}
}

func TestRegisterProvider(t *testing.T) {
	var provider Provider = NewMockProvider()

	previousCount := len(registeredProviders)

	if RegisterProvider("mock-provider", provider); len(registeredProviders) == previousCount {
		t.Error("RegisterProvider failed register 'mock-provider'.")
	}

	if prov, err := GetProvider("mock-provider", "example.com"); err != nil {
		t.Error("GetProvider failed to get 'mock-provider'.")
	} else {
		testProvider(t, prov)
	}
}
