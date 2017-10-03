package providers

import (
	"github.com/rancher/external-dns/utils"
	"testing"
	"github.com/rancher/external-dns/test"
	"reflect"
)

var providerTestData = []struct {
	name     string
	provider Provider
	testData []utils.DnsRecord
}{
	{
		"Provider-1",
		NewMockProvider(),
		[]utils.DnsRecord{
			test.NewMockDnsRecord(test.MockStateFQDN, 300, "TXT", "Testing123-TXT"),
			test.NewMockDnsRecord(test.MockStateFQDN, 300, "A", "Testing123-A"),
		},

	},
}

/*
 * --- Tests ---
 */


func testProvider(t *testing.T, provider Provider) {
	var probe *Probe = provider.(MockProvider).Probe

	if provider.Init("example.com"); !probe.HasInit {
		t.Errorf("Expected MockProvider to be initialized. Probe found init to be false.")
	}

	if provider.HealthCheck(); !probe.HasHealthCheck {
		t.Errorf("Expected HealthCheck to run and recorded on probe. Probe found hasHealthCheck to be false.")
	}

	if provider.AddRecord(utils.DnsRecord{}); !probe.HasAddRecord {
		t.Errorf("Expected AddRecord to run and recorded on probe. Probe found hasAddRecord to be false.")
	}

	if provider.RemoveRecord(utils.DnsRecord{}); !probe.HasRemoveRecord {
		t.Errorf("Expected RemoveRecord to run and recorded on probe. Probe found hasRemoveRecord to be false.")
	}

	if provider.UpdateRecord(utils.DnsRecord{}); !probe.HasUpdateRecord {
		t.Errorf("Expected UpdateRecord to run and recorded on probe. Probe found hasUpdateRecord to be false.")
	}

	if provider.GetRecords(); !probe.HasUpdateRecord {
		t.Errorf("Expected GetRecords to run and recorded on probe. Probe found hasGetRecords to be false.")
	}

	if provider.GetName(); !probe.HasGetName {
		t.Errorf("Expected GetName to run and recorded on probe. Probe found hasGetName to be false.")
	}
}

// Bit of sanity to check the MockProvider. We make use of the probe later.
func TestMockProvider(t *testing.T) {
	for _, asset := range providerTestData {
		provider := asset.provider

		testProvider(t, provider)
	}
}

func Test_MockProvider_Records(t *testing.T) {
	dnsRecord := test.NewMockDnsRecord("example.com", 300, "TXT", "Testing123")
	dnsRecords := []utils.DnsRecord{dnsRecord}

	testProvider := NewMockProvider()
	testProvider.SetRecords(dnsRecords)
	actual, _ := testProvider.GetRecords()

	if !reflect.DeepEqual(actual, dnsRecords) {
		t.Errorf("MockProvider.{Set|Get}Records failed.")
	}
}