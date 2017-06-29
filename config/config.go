package config

import (
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/Jorcooly/external-dns/utils"
)

const (
	defaultNameTemplate = "%{{service_name}}.%{{stack_name}}.%{{environment_name}}"
)

var (
	RootDomainName  string
	TTL             int
	CattleURL       string
	CattleAccessKey string
	CattleSecretKey string
	NameTemplate    string
)

func SetFromEnvironment() {
	CattleURL = getEnv("CATTLE_URL")
	CattleAccessKey = getEnv("CATTLE_ACCESS_KEY")
	CattleSecretKey = getEnv("CATTLE_SECRET_KEY")
	RootDomainName = utils.Fqdn(getEnv("ROOT_DOMAIN"))
	NameTemplate = os.Getenv("NAME_TEMPLATE")
	if len(NameTemplate) == 0 {
		NameTemplate = defaultNameTemplate
	}

	TTLEnv := os.Getenv("TTL")
	i, err := strconv.Atoi(TTLEnv)
	if err != nil {
		TTL = 300
	} else {
		TTL = i
	}
}

func getEnv(name string) string {
	envVar := os.Getenv(name)
	if len(envVar) == 0 {
		logrus.Fatalf("Environment variable '%s' is not set", name)
	}
	return envVar
}
