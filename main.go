package main

import (
	"flag"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mathuin/external-dns/config"
	"github.com/mathuin/external-dns/metadata"
	"github.com/mathuin/external-dns/providers"
	_ "github.com/mathuin/external-dns/providers/cloudflare"
	_ "github.com/mathuin/external-dns/providers/digitalocean"
	_ "github.com/mathuin/external-dns/providers/dnsimple"
	_ "github.com/mathuin/external-dns/providers/gandi"
	_ "github.com/mathuin/external-dns/providers/pointhq"
	_ "github.com/mathuin/external-dns/providers/powerdns"
	_ "github.com/mathuin/external-dns/providers/rfc2136"
	_ "github.com/mathuin/external-dns/providers/route53"
	"github.com/mathuin/external-dns/utils"
)

const (
	poll = 1000
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
	debug   = flag.Bool("debug", false, "Debug")
	logFile = flag.String("log", "", "Log file")

	provider providers.Provider
	m        *metadata.MetadataClient
	c        *CattleClient
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
	provider, err = providers.GetProvider(config.ProviderName, config.RootDomainName)
	if err != nil {
		logrus.Fatalf("Failed to get provider '%s': %v", config.ProviderName, err)
	}
}

func main() {
	logrus.Infof("Starting Rancher External DNS service")
	setEnv()

	go startHealthcheck()

	version := "init"
	lastUpdated := time.Now()
	for {
		newVersion, err := m.GetVersion()
		update := false

		if err != nil {
			logrus.Errorf("Error reading version: %v", err)
		} else if version != newVersion {
			logrus.Debugf("Version has been changed. Old version: %s. New version: %s.", version, newVersion)
			version = newVersion
			update = true
		} else {
			logrus.Debugf("No changes in version: %s", newVersion)
			if time.Since(lastUpdated).Minutes() >= forceUpdateInterval {
				logrus.Debugf("Executing force update as version hasn't been changed in: %v minutes", forceUpdateInterval)
				update = true
			}
		}

		if update {
			// get records from metadata
			metadataRecs, err := m.GetMetadataDnsRecords()
			if err != nil {
				logrus.Errorf("Error reading external dns entries: %v", err)
			}
			logrus.Debugf("DNS records from metadata: %v", metadataRecs)

			//update provider
			updated, err := UpdateProviderDnsRecords(metadataRecs)
			if err != nil {
				logrus.Errorf("Failed to update provider with new DNS records: %v", err)
			}

			for _, toUpdate := range updated {
				serviceDnsRecord := utils.ConvertToServiceDnsRecord(toUpdate)
				c.UpdateServiceDomainName(serviceDnsRecord)
			}

			lastUpdated = time.Now()
		}

		time.Sleep(time.Duration(poll) * time.Millisecond)
	}
}
