package config

import (
	"time"

	"github.com/spf13/pflag"
)

type HTTPServerConfig struct {
	Hostname      string        `json:"hostname"`
	BindPort      string        `json:"bind_port"`
	ReadTimeout   time.Duration `json:"read_timeout"`
	WriteTimeout  time.Duration `json:"write_timeout"`
	HTTPSCertFile string        `json:"https_cert_file"`
	HTTPSKeyFile  string        `json:"https_key_file"`
	EnableHTTPS   bool          `json:"enable_https"`
}

func NewHTTPServerConfig() *HTTPServerConfig {
	return &HTTPServerConfig{
		Hostname:      "localhost",
		BindPort:      "8000",
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  30 * time.Second,
		EnableHTTPS:   false,
		HTTPSCertFile: "",
		HTTPSKeyFile:  "",
	}
}

func (s *HTTPServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindPort, "http-server-bindport", s.BindPort, "HTTP server bind port")
	fs.StringVar(&s.Hostname, "server-hostname", s.Hostname, "Server's public hostname")
	fs.DurationVar(&s.ReadTimeout, "http-read-timeout", s.ReadTimeout, "HTTP server read timeout")
	fs.DurationVar(&s.WriteTimeout, "http-write-timeout", s.WriteTimeout, "HTTP server write timeout")
	fs.StringVar(&s.HTTPSCertFile, "https-cert-file", s.HTTPSCertFile, "The path to the tls.crt file.")
	fs.StringVar(&s.HTTPSKeyFile, "https-key-file", s.HTTPSKeyFile, "The path to the tls.key file.")
	fs.BoolVar(&s.EnableHTTPS, "enable-https", s.EnableHTTPS, "Enable HTTPS rather than HTTP")
}

func (s *HTTPServerConfig) ReadFiles() error {
	return nil
}
