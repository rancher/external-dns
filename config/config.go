package config

import (
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/utils"
)

const (
	defaultNameTemplate         = "%{{service_name}}.%{{stack_name}}.%{{environment_name}}"
	defaultForcedUpdateInterval = 1
	defaultTTL                  = 120
)

var (
	RootDomainName       string
	TTL                  int
	CattleURL            string
	CattleAccessKey      string
	CattleSecretKey      string
	NameTemplate         string
	ForcedUpdateInterval int
)

func SetFromEnvironment() {
	CattleURL = getEnv("CATTLE_URL")
	CattleAccessKey = getEnv("CATTLE_ACCESS_KEY")
	CattleSecretKey = getEnv("CATTLE_SECRET_KEY")
	RootDomainName = utils.Fqdn(getEnv("ROOT_DOMAIN"))

	template := os.Getenv("NAME_TEMPLATE")
	if len(template) == 0 {
		NameTemplate = defaultNameTemplate
	} else {
		NameTemplate = template
	}

	interval := os.Getenv("FORCED_UPDATE_INTERVAL")
	if len(interval) == 0 {
		ForcedUpdateInterval = defaultForcedUpdateInterval
	} else {
		var err error
		if ForcedUpdateInterval, err = strconv.Atoi(interval); err != nil {
			logrus.Fatal("FORCED_UPDATE_INTERVAL is not an interval string")
		}
	}

	ttl := os.Getenv("TTL")
	if len(ttl) == 0 {
		TTL = defaultTTL
	} else {
		var err error
		if TTL, err = strconv.Atoi(ttl); err != nil {
			logrus.Fatal("TTL is not an interval string")
		}
	}
}

func getEnv(name string) string {
	envVar := os.Getenv(name)
	if len(envVar) == 0 {
		logrus.Fatalf("Environment variable '%s' is not set", name)
	}
	return envVar
}
