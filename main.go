package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/dns"
	"github.com/rancher/external-dns/metadata"
	"github.com/rancher/external-dns/providers"
	"os"
	"time"
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
	providerName = flag.String("provider", "", "External provider name")
	debug        = flag.Bool("debug", false, "Debug")
	logFile      = flag.String("log", "", "Log file")

	provider providers.Provider
	m        *metadata.MetadataClient
	c        *CattleClient
)

func setEnv() {
	flag.Parse()
	provider = providers.GetProvider(*providerName)
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

	// configure metadata client
	mClient, err := metadata.NewMetadataClient()
	if err != nil {
		logrus.Fatalf("Failed to configure rancher-metadata client: %v", err)
	}
	m = mClient

	cattleUrl := os.Getenv("CATTLE_URL")
	if len(cattleUrl) == 0 {
		logrus.Fatalf("CATTLE_URL is not set")
	}

	cattleApiKey := os.Getenv("CATTLE_ACCESS_KEY")
	if len(cattleApiKey) == 0 {
		logrus.Fatalf("CATTLE_ACCESS_KEY is not set")
	}

	cattleSecretKey := os.Getenv("CATTLE_SECRET_KEY")
	if len(cattleSecretKey) == 0 {
		logrus.Fatalf("CATTLE_SECRET_KEY is not set")
	}

	//configure cattle client
	c, err = NewCattleClient(cattleUrl, cattleApiKey, cattleSecretKey)
	if err != nil {
		logrus.Fatalf("Failed to configure cattle client: %v", err)
	}
}

func main() {
	logrus.Infof("Starting Rancher External DNS service")
	setEnv()
	logrus.Infof("Powered by %s", provider.GetName())

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
				serviceDnsRecord := dns.ConvertToServiceDnsRecord(toUpdate)
				c.UpdateServiceDomainName(serviceDnsRecord)
			}

			lastUpdated = time.Now()
		}

		time.Sleep(time.Duration(poll) * time.Millisecond)
	}
}
