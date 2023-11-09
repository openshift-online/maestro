package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

type ApplicationConfig struct {
	Server      *ServerConfig      `json:"server"`
	Metrics     *MetricsConfig     `json:"metrics"`
	HealthCheck *HealthCheckConfig `json:"health_check"`
	Database    *DatabaseConfig    `json:"database"`
	OCM         *OCMConfig         `json:"ocm"`
	Sentry      *SentryConfig      `json:"sentry"`
}

func NewApplicationConfig() *ApplicationConfig {
	return &ApplicationConfig{
		Server:      NewServerConfig(),
		Metrics:     NewMetricsConfig(),
		HealthCheck: NewHealthCheckConfig(),
		Database:    NewDatabaseConfig(),
		OCM:         NewOCMConfig(),
		Sentry:      NewSentryConfig(),
	}
}

func (c *ApplicationConfig) AddFlags(flagset *pflag.FlagSet) {
	flagset.AddGoFlagSet(flag.CommandLine)
	c.Server.AddFlags(flagset)
	c.Metrics.AddFlags(flagset)
	c.HealthCheck.AddFlags(flagset)
	c.Database.AddFlags(flagset)
	c.OCM.AddFlags(flagset)
	c.Sentry.AddFlags(flagset)
}

func (c *ApplicationConfig) ReadFiles() []string {
	readFiles := []struct {
		f    func() error
		name string
	}{
		{c.Server.ReadFiles, "Server"},
		{c.Database.ReadFiles, "Database"},
		{c.OCM.ReadFiles, "OCM"},
		{c.Metrics.ReadFiles, "Metrics"},
		{c.HealthCheck.ReadFiles, "HealthCheck"},
		{c.Sentry.ReadFiles, "Sentry"},
	}
	messages := []string{}
	for _, rf := range readFiles {
		if err := rf.f(); err != nil {
			msg := fmt.Sprintf("%s %s", rf.name, err.Error())
			messages = append(messages, msg)
		}
	}
	return messages
}

// Read the contents of file into integer value
func readFileValueInt(file string, val *int) error {
	fileContents, err := ReadFile(file)
	if err != nil {
		return err
	}

	*val, err = strconv.Atoi(fileContents)
	return err
}

// Read the contents of file into string value
func readFileValueString(file string, val *string) error {
	fileContents, err := ReadFile(file)
	if err != nil {
		return err
	}

	*val = strings.TrimSuffix(fileContents, "\n")
	return err
}

// Read the contents of file into boolean value
func readFileValueBool(file string, val *bool) error {
	fileContents, err := ReadFile(file)
	if err != nil {
		return err
	}

	*val, err = strconv.ParseBool(fileContents)
	return err
}

func ReadFile(file string) (string, error) {
	// If the value is in quotes, unquote it
	unquotedFile, err := strconv.Unquote(file)
	if err != nil {
		// values without quotes will raise an error, ignore it.
		unquotedFile = file
	}

	// If no file is provided, leave val unchanged.
	if unquotedFile == "" {
		return "", nil
	}

	// Ensure the absolute file path is used
	absFilePath := unquotedFile
	if !filepath.IsAbs(unquotedFile) {
		absFilePath = filepath.Join(GetProjectRootDir(), unquotedFile)
	}

	// Read the file
	buf, err := os.ReadFile(absFilePath)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// Return project root path based on the relative path of this file
func GetProjectRootDir() string {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(filepath.Join(b, "..", ".."))
	return basepath
}
