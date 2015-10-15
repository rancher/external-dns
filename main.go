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

	for {
		newVersion, err := m.GetVersion()
		if err != nil {
			logrus.Errorf("Error reading version: %v", err)
		} else if version == newVersion {
			logrus.Debug("No changes in version: %s", newVersion)
		} else {
			logrus.Debug("Version has been changed. Old version: %s. New version: %s.", version, newVersion)
			if err := UpdateDnsRecords(m); err != nil {
				logrus.Errorf("Failed to update DNS records: %v", err)
			}
			version = newVersion
		}
		time.Sleep(time.Duration(poll) * time.Millisecond)
	}
}
