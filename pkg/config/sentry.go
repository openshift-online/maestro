package config

import (
	"time"

	"github.com/spf13/pflag"
)

type SentryConfig struct {
	Enabled bool          `json:"enabled"`
	Key     string        `json:"key"`
	URL     string        `json:"url"`
	Project string        `json:"project"`
	Debug   bool          `json:"debug"`
	Timeout time.Duration `json:"timeout"`

	KeyFile string `json:"key_file"`
}

func NewSentryConfig() *SentryConfig {
	return &SentryConfig{
		Enabled: false,
		Key:     "",
		URL:     "glitchtip.devshift.net",
		Project: "53", // 16 is the ocm-service-dev project for local dev/testing
		Debug:   false,
		KeyFile: "secrets/sentry.key",
	}
}

func (c *SentryConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.Enabled, "enable-sentry", c.Enabled, "Enable sentry error monitoring")
	fs.StringVar(&c.KeyFile, "sentry-key-file", c.KeyFile, "File containing Sentry key")
	fs.StringVar(&c.URL, "sentry-url", c.URL, "Base URL of Sentry isntance")
	fs.StringVar(&c.Project, "sentry-project", c.Project, "Sentry project to report to")
	fs.BoolVar(&c.Debug, "enable-sentry-debug", c.Debug, "Enable sentry error monitoring")
	fs.DurationVar(&c.Timeout, "sentry-timeout", 5*time.Second, "Timeout for all requests made to Sentry")
}

func (c *SentryConfig) ReadFiles() error {
	if !c.Enabled {
		return nil
	}
	return readFileValueString(c.KeyFile, &c.Key)
}
