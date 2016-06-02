package config

import (
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/external-dns/utils"
)

var (
	RootDomainName  string
	TTL             int
	CattleURL       string
	CattleAccessKey string
	CattleSecretKey string
)

func SetFromEnvironment() {
	CattleURL = getEnv("CATTLE_URL")
	CattleAccessKey = getEnv("CATTLE_ACCESS_KEY")
	CattleSecretKey = getEnv("CATTLE_SECRET_KEY")
	RootDomainName = utils.Fqdn(getEnv("ROOT_DOMAIN"))
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
