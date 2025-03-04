package types

import "os"

const (
	TestingEnv     string = "testing"
	DevelopmentEnv string = "development"
	ProductionEnv  string = "production"

	EnvironmentStringKey string = "MAESTRO_ENV"
	EnvironmentDefault   string = DevelopmentEnv
)

func GetEnvironmentStrFromEnv() string {
	envStr, specified := os.LookupEnv(EnvironmentStringKey)
	if !specified || envStr == "" {
		envStr = EnvironmentDefault
	}
	return envStr
}
