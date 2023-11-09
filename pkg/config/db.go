package config

import (
	"fmt"

	"github.com/spf13/pflag"
)

type DatabaseConfig struct {
	Dialect            string `json:"dialect"`
	SSLMode            string `json:"sslmode"`
	Debug              bool   `json:"debug"`
	MaxOpenConnections int    `json:"max_connections"`

	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`

	HostFile     string `json:"host_file"`
	PortFile     string `json:"port_file"`
	NameFile     string `json:"name_file"`
	UsernameFile string `json:"username_file"`
	PasswordFile string `json:"password_file"`
	RootCertFile string `json:"certificate_file"`
}

func NewDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Dialect:            "postgres",
		SSLMode:            "disable",
		Debug:              false,
		MaxOpenConnections: 50,

		HostFile:     "secrets/db.host",
		PortFile:     "secrets/db.port",
		NameFile:     "secrets/db.name",
		UsernameFile: "secrets/db.user",
		PasswordFile: "secrets/db.password",
		RootCertFile: "secrets/db.rootcert",
	}
}

func (c *DatabaseConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.HostFile, "db-host-file", c.HostFile, "Database host string file")
	fs.StringVar(&c.PortFile, "db-port-file", c.PortFile, "Database port file")
	fs.StringVar(&c.UsernameFile, "db-user-file", c.UsernameFile, "Database username file")
	fs.StringVar(&c.PasswordFile, "db-password-file", c.PasswordFile, "Database password file")
	fs.StringVar(&c.NameFile, "db-name-file", c.NameFile, "Database name file")
	fs.StringVar(&c.RootCertFile, "db-rootcert", c.RootCertFile, "Database root certificate file")
	fs.StringVar(&c.SSLMode, "db-sslmode", c.SSLMode, "Database ssl mode (disable | require | verify-ca | verify-full)")
	fs.BoolVar(&c.Debug, "enable-db-debug", c.Debug, " framework's debug mode")
	fs.IntVar(&c.MaxOpenConnections, "db-max-open-connections", c.MaxOpenConnections, "Maximum open DB connections for this instance")
}

func (c *DatabaseConfig) ReadFiles() error {
	err := readFileValueString(c.HostFile, &c.Host)
	if err != nil {
		return err
	}

	err = readFileValueInt(c.PortFile, &c.Port)
	if err != nil {
		return err
	}

	err = readFileValueString(c.UsernameFile, &c.Username)
	if err != nil {
		return err
	}

	err = readFileValueString(c.PasswordFile, &c.Password)
	if err != nil {
		return err
	}

	err = readFileValueString(c.NameFile, &c.Name)
	return err
}

func (c *DatabaseConfig) ConnectionString(withSSL bool) string {
	return c.ConnectionStringWithName(c.Name, withSSL)
}

func (c *DatabaseConfig) ConnectionStringWithName(name string, withSSL bool) string {
	var cmd string
	if withSSL {
		cmd = fmt.Sprintf(
			"host=%s port=%d user=%s password='%s' dbname=%s sslmode=%s sslrootcert=%s",
			c.Host, c.Port, c.Username, c.Password, name, c.SSLMode, c.RootCertFile,
		)
	} else {
		cmd = fmt.Sprintf(
			"host=%s port=%d user=%s password='%s' dbname=%s sslmode=disable",
			c.Host, c.Port, c.Username, c.Password, name,
		)
	}

	return cmd
}

func (c *DatabaseConfig) LogSafeConnectionString(withSSL bool) string {
	return c.LogSafeConnectionStringWithName(c.Name, withSSL)
}

func (c *DatabaseConfig) LogSafeConnectionStringWithName(name string, withSSL bool) string {
	if withSSL {
		return fmt.Sprintf(
			"host=%s port=%d user=%s password='<REDACTED>' dbname=%s sslmode=%s sslrootcert='<REDACTED>'",
			c.Host, c.Port, c.Username, name, c.SSLMode,
		)
	} else {
		return fmt.Sprintf(
			"host=%s port=%d user=%s password='<REDACTED>' dbname=%s",
			c.Host, c.Port, c.Username, name,
		)
	}
}
