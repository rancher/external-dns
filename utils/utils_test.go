package utils

import (
	"testing"
	"reflect"
)

type MockDnsEntry struct {
	data string
}

/*
 * --- Test Data ---
 */

var fqdnTestData = []struct {
	input string
	expected string

}{
	{
		"example.com",
		"example.com.",
	},
	{
		"foo.example.com",
		"foo.example.com.",
	},
	{
		"bar.example.com.",
		"bar.example.com.",
	},
	{
		"",
		"",
	},
}

var unFqdnTestData = []struct {
	input string
	expected string

}{
	{
		"example.com.",
		"example.com",
	},
	{
		"foo.example.com.",
		"foo.example.com",
	},
	{
		"bar.example.com.",
		"bar.example.com",
	},
	{
		"",
		"",
	},
}

var fqdnTemplateData = []struct {
	template string
	serviceName string
	stackName string
	environmentName string
	rootDomainName string
	expected string
}{
	{
		template: "",
		serviceName: "service1",
		stackName: "mystack",
		environmentName: "default",
		rootDomainName: "example.com",
		expected: ".example.com",
	},
	{
		template: "%{{stack_name}}.%{{service_name}}",
		serviceName: "service1",
		stackName: "mystack",
		environmentName: "default",
		rootDomainName: "example.com",
		expected: "mystack.service1.example.com",
	},
	{
		template: "%{{environment_name}}.%{{stack_name}}.%{{service_name}}",
		serviceName: "service1",
		stackName: "mystack",
		environmentName: "default",
		rootDomainName: "example.com",
		expected: "default.mystack.service1.example.com",
	},
}

var stateFqdnData = []struct {
	envUUID string
	rootDomainName string
	expected string
}{
	{
		envUUID: "A0A0AA00-AA0A-0A0A-AA00-000000AAA0A0",
		rootDomainName: "example.com",
		expected: "external-dns-a0a0aa00-aa0a-0a0a-aa00-000000aaa0a0.example.com",
	},
	{
		envUUID: "",
		rootDomainName: "example.com",
		expected: "external-dns-.example.com",
	},
}

var stateRecordData = []struct {
	fqdn string
	ttl int
	entries map[string]struct{}
	expected DnsRecord
}{
	{
		"example.com",
		300,
		map[string]struct{}{
			"example.com": {},
			"bar.example.com": {},
		},
		DnsRecord{"example.com",
		[]string{
				"bar.example.com",
				"example.com",
			},
			"TXT",
			300},
	},
	{
		"example.com",
		300,
		map[string]struct{}{
			"example.com": {},
			"bar.example.com": {},
			"foo.bar.example.com": {},
		},
		DnsRecord{"example.com",
			[]string{
				"bar.example.com",
				"example.com",
				"foo.bar.example.com",
			},
			"TXT",
			300},
	},
	{
		"example.com",
		300,
		map[string]struct{}{
			"a": {},
			"b": {},
			"c": {},
		},
		DnsRecord{"example.com",
			[]string{
				"a",
				"b",
				"c",
			},
			"TXT",
			300},
	},
}


/*
 * --- Tests ---
 */

func TestFqdn(t *testing.T) {
	for _, asset := range fqdnTestData {
		if result:= Fqdn(asset.input); result != asset.expected {
			t.Errorf("\nExpected: \n[%s], \ngot: \n[%s]", asset.expected, result)
		}
	}
}

func TestUnFqdn(t *testing.T) {
	for _, asset := range unFqdnTestData {
		if result:= UnFqdn(asset.input); result != asset.expected {
			t.Errorf("\nExpected: \n[%s], \ngot: \n[%s]", asset.expected, result)
		}
	}
}

func TestFqdnFromTemplate(t *testing.T) {
	for _, asset := range fqdnTemplateData {
		if result:= FqdnFromTemplate(
			asset.template,
			asset.serviceName,
			asset.stackName,
			asset.environmentName,
			asset.rootDomainName); result != asset.expected {
			t.Errorf("\nExpected: \n[%s], \ngot: \n[%s]", asset.expected, result)
		}
	}
}

func TestStateFqdn(t *testing.T) {
	for _, asset := range stateFqdnData {
		if result:= StateFqdn(asset.envUUID, asset.rootDomainName); result != asset.expected {
			t.Errorf("\nExpected: \n[%s], \ngot: \n[%s]", asset.expected, result)
		}
	}
}

func TestStateRecord(t *testing.T) {
	for _, asset := range stateRecordData {
		// this is a test, performance in testing the end result doesn't matter here
		if result := StateRecord(asset.fqdn, asset.ttl, asset.entries); !reflect.DeepEqual(result, asset.expected) {
			t.Errorf("\nFqdn Expected: \n[%v], \ngot: \n[%v]", asset.expected, result)
		}
	}
}
