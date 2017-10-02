package providers

import (
	"testing"
)

/*
 * --- Test Data ---
 */


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
