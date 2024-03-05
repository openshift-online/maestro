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
	EnableJWT     bool          `json:"enable_jwt"`
	EnableAuthz   bool          `json:"enable_authz"`
	JwkCertFile   string        `json:"jwk_cert_file"`
	JwkCertURL    string        `json:"jwk_cert_url"`
	ACLFile       string        `json:"acl_file"`
}

func NewHTTPServerConfig() *HTTPServerConfig {
	return &HTTPServerConfig{
		Hostname:      "localhost",
		BindPort:      "8000",
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  30 * time.Second,
		EnableHTTPS:   false,
		EnableJWT:     true,
		EnableAuthz:   true,
		JwkCertFile:   "",
		JwkCertURL:    "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs",
		ACLFile:       "",
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
	fs.BoolVar(&s.EnableJWT, "enable-jwt", s.EnableJWT, "Enable JWT authentication validation")
	fs.BoolVar(&s.EnableAuthz, "enable-authz", s.EnableAuthz, "Enable Authorization on endpoints, should only be disabled for debug")
	fs.StringVar(&s.JwkCertFile, "jwk-cert-file", s.JwkCertFile, "JWK Certificate file")
	fs.StringVar(&s.JwkCertURL, "jwk-cert-url", s.JwkCertURL, "JWK Certificate URL")
	fs.StringVar(&s.ACLFile, "acl-file", s.ACLFile, "Access control list file")
}

func (s *HTTPServerConfig) ReadFiles() error {
	return nil
}
