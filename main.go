package main

import (
	"flag"
	"os"
	"reflect"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/config"
	"github.com/rancher/external-dns/metadata"
	"github.com/rancher/external-dns/providers"
	_ "github.com/rancher/external-dns/providers/cloudflare"
	_ "github.com/rancher/external-dns/providers/dnsimple"
	_ "github.com/rancher/external-dns/providers/gandi"
	_ "github.com/rancher/external-dns/providers/pointhq"
	_ "github.com/rancher/external-dns/providers/powerdns"
	_ "github.com/rancher/external-dns/providers/rfc2136"
	_ "github.com/rancher/external-dns/providers/route53"
	"github.com/rancher/external-dns/utils"
)

const (
	pollInterval = 1000
	// if metadata wasn't updated in 1 min, force update would be executed
	forceUpdateInterval = 1
)

type Op struct {
	Name string
}

var (
	Add    = Op{Name: "Add"}
	Remove = Op{Name: "Remove"}
	Update = Op{Name: "Update"}
)

var (
	providerName = flag.String("provider", "route53", "External provider name")
	debug        = flag.Bool("debug", false, "Debug")
	logFile      = flag.String("log", "", "Log file")

	provider providers.Provider
	m        *metadata.MetadataClient
	c        *CattleClient

	metadataRecsCached = make(map[string]utils.DnsRecord)
)

func setEnv() {
	flag.Parse()
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if *logFile != "" {
		if output, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
			logrus.Fatalf("Failed to log to file %s: %v", *logFile, err)
		} else {
			logrus.SetOutput(output)
			formatter := &logrus.TextFormatter{
				FullTimestamp: true,
			}
			logrus.SetFormatter(formatter)
		}
	}

	// get config from environment variables
	config.SetFromEnvironment()

	var err error
	// configure metadata client
	m, err = metadata.NewMetadataClient()
	if err != nil {
		logrus.Fatalf("Failed to configure rancher-metadata client: %v", err)
	}

	//configure cattle client
	c, err = NewCattleClient(config.CattleURL, config.CattleAccessKey, config.CattleSecretKey)
	if err != nil {
		logrus.Fatalf("Failed to configure cattle client: %v", err)
	}

	// get provider
	provider, err = providers.GetProvider(*providerName, config.RootDomainName)
	if err != nil {
		logrus.Fatalf("Failed to get provider '%s': %v", *providerName, err)
	}
}

func main() {
	logrus.Infof("Starting Rancher External DNS service")
	setEnv()

	go startHealthcheck()

	version := "init"
	lastUpdated := time.Now()

	ticker := time.NewTicker(time.Duration(pollInterval) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		update, updateForced := false, false
		newVersion, err := m.GetVersion()
		if err != nil {
			logrus.Errorf("Failed to get metadata version: %v", err)
		} else if version != newVersion {
			logrus.Debugf("Metadata version changed. Old: %s New: %s.", version, newVersion)
			version = newVersion
			update = true
		} else {
			if time.Since(lastUpdated).Minutes() >= forceUpdateInterval {
				logrus.Debugf("Executing force update as metadata version hasn't changed in: %d minutes",
					forceUpdateInterval)
				updateForced = true
			}
		}

		if update || updateForced {
			// get records from metadata
			metadataRecs, err := m.GetMetadataDnsRecords()
			if err != nil {
				logrus.Errorf("Failed to get DNS records from metadata: %v", err)
			}

			logrus.Debugf("DNS records from metadata: %v", metadataRecs)

			// A flapping service might cause the metadata version to change
			// in short intervals. Caching the previous metadata DNS records
			// allows us to check if the actual records have changed before
			// querying the provider records.
			if updateForced || !reflect.DeepEqual(metadataRecs, metadataRecsCached) {
				// update the provider
				updated, err := UpdateProviderDnsRecords(m.EnvironmentUUID, metadataRecs)
				if err != nil {
					logrus.Errorf("Failed to update provider with new DNS records: %v", err)
				}

				// update the service FQDN in Cattle
				for _, toUpdate := range updated {
					serviceDnsRecord := utils.ConvertToServiceDnsRecord(toUpdate)
					c.UpdateServiceDomainName(serviceDnsRecord)
				}

				metadataRecsCached = metadataRecs
				lastUpdated = time.Now()
			} else {
				logrus.Debugf("DNS records from metadata did not change")
			}
		}
	}
}
