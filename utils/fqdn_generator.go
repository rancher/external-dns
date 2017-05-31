package utils

import (
	"fmt"
	"github.com/rancher/go-rancher-metadata/metadata"
)

type FQDNGenerator interface {
	GetName() string
	GenerateFQDN(template string, container metadata.Container, environmentName string, rootDomainName string) string
	GetDefaultTemplate() string
}

var (
	fqdnGenerators map[string]FQDNGenerator
)

func GetFQDNGenerator(name string) FQDNGenerator {
	if fqdnGenerator, ok := fqdnGenerators[name]; ok {
		return fqdnGenerator
	}
	return fqdnGenerators["DefaultFQDNGenerator"]
}

func RegisterFQDNGenerator(name string, fqdnGenerator FQDNGenerator) error {
	if fqdnGenerators == nil {
		fqdnGenerators = make(map[string]FQDNGenerator)
	}
	if _, exists := fqdnGenerators[name]; exists {
		return fmt.Errorf("fqdnGenerator already registered")
	}
	fqdnGenerators[name] = fqdnGenerator
	return nil
}
