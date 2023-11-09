package config

import (
	"github.com/spf13/pflag"
)

type OCMConfig struct {
	BaseURL          string `json:"base_url"`
	ClientID         string `json:"client-id"`
	ClientIDFile     string `json:"client-id_file"`
	ClientSecret     string `json:"client-secret"`
	ClientSecretFile string `json:"client-secret_file"`
	SelfToken        string `json:"self_token"`
	SelfTokenFile    string `json:"self_token_file"`
	TokenURL         string `json:"token_url"`
	Debug            bool   `json:"debug"`
	EnableMock       bool   `json:"enable_mock"`
}

func NewOCMConfig() *OCMConfig {
	return &OCMConfig{
		BaseURL:          "https://api.integration.openshift.com",
		TokenURL:         "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token",
		ClientIDFile:     "secrets/ocm-service.clientId",
		ClientSecretFile: "secrets/ocm-service.clientSecret",
		SelfTokenFile:    "",
		Debug:            false,
		EnableMock:       true,
	}
}

func (c *OCMConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ClientIDFile, "ocm-client-id-file", c.ClientIDFile, "File containing OCM API privileged account client-id")
	fs.StringVar(&c.ClientSecretFile, "ocm-client-secret-file", c.ClientSecretFile, "File containing OCM API privileged account client-secret")
	fs.StringVar(&c.SelfTokenFile, "self-token-file", c.SelfTokenFile, "File containing OCM API privileged offline SSO token")
	fs.StringVar(&c.BaseURL, "ocm-base-url", c.BaseURL, "The base URL of the OCM API, integration by default")
	fs.StringVar(&c.TokenURL, "ocm-token-url", c.TokenURL, "The base URL that OCM uses to request tokens, stage by default")
	fs.BoolVar(&c.Debug, "ocm-debug", c.Debug, "Debug flag for OCM API")
	fs.BoolVar(&c.EnableMock, "enable-ocm-mock", c.EnableMock, "Enable mock ocm clients")
}

func (c *OCMConfig) ReadFiles() error {
	if c.EnableMock {
		return nil
	}
	err := readFileValueString(c.ClientIDFile, &c.ClientID)
	if err != nil {
		return err
	}
	err = readFileValueString(c.ClientSecretFile, &c.ClientSecret)
	if err != nil {
		return err
	}
	err = readFileValueString(c.SelfTokenFile, &c.SelfToken)
	return err
}
