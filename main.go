package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/providers"
	"github.com/rancher/go-rancher-metadata/metadata"
	"os"
	"time"
)

const (
	//FIXME - change metadata url to rancher-metadata
	metadataUrl = "http://localhost:90"
	poll        = 1000
	// if metadata wasn't updated in 10 min, force update would be executed
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
	providerName    = flag.String("provider", "", "External provider name")
	debug           = flag.Bool("debug", false, "Debug")
	logFile         = flag.String("log", "", "Log file")
	EnvironmentName string
	provider        providers.Provider
	m               metadata.Handler
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
	m = metadata.NewHandler(metadataUrl)
	selfStack, err := m.GetSelfStack()
	if err != nil {
		logrus.Fatalf("Error reading stack info: %v", err)
	}
	EnvironmentName = selfStack.EnvironmentName
}

func main() {
	logrus.Infof("Starting Rancher External DNS service")
	setEnv()
	logrus.Infof("Powered by %s", provider.GetName())

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
			if err := UpdateDnsRecords(m); err != nil {
				logrus.Errorf("Failed to update DNS records: %v", err)
			}
			lastUpdated = time.Now()
		}

		time.Sleep(time.Duration(poll) * time.Millisecond)
	}
}
