package utils

import (
	"testing"
)

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
