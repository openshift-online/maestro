package config

import (
	"math"
	"time"

	"github.com/spf13/pflag"
)

type GRPCServerConfig struct {
	EnableGRPCServer        bool          `json:"enable_grpc_server"`
	DisableTLS              bool          `json:"disable_grpc_tls"`
	TLSCertFile             string        `json:"grpc_tls_cert_file"`
	TLSKeyFile              string        `json:"grpc_tls_key_file"`
	BrokerTLSCertFile       string        `json:"grpc_broker_tls_cert_file"`
	BrokerTLSKeyFile        string        `json:"grpc_broker_tls_key_file"`
	GRPCAuthNType           string        `json:"grpc_authn_type"`
	GRPCAuthorizerConfig    string        `json:"grpc_authorizer_config"`
	ClientCAFile            string        `json:"grpc_client_ca_file"`
	BrokerClientCAFile      string        `json:"grpc_broker_client_ca_file"`
	ServerBindPort          string        `json:"server_bind_port"`
	BrokerBindPort          string        `json:"broker_bind_port"`
	MaxConcurrentStreams    uint32        `json:"max_concurrent_steams"`
	MaxReceiveMessageSize   int           `json:"max_receive_message_size"`
	MaxSendMessageSize      int           `json:"max_send_message_size"`
	ConnectionTimeout       time.Duration `json:"connection_timeout"`
	WriteBufferSize         int           `json:"write_buffer_size"`
	ReadBufferSize          int           `json:"read_buffer_size"`
	MaxConnectionAge        time.Duration `json:"max_connection_age"`
	ClientMinPingInterval   time.Duration `json:"client_min_ping_interval"`
	ServerPingInterval      time.Duration `json:"server_ping_interval"`
	ServerPingTimeout       time.Duration `json:"server_ping_timeout"`
	PermitPingWithoutStream bool          `json:"permit_ping_without_stream"`
}

func NewGRPCServerConfig() *GRPCServerConfig {
	return &GRPCServerConfig{}
}

func (s *GRPCServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&s.EnableGRPCServer, "enable-grpc-server", true, "Enable gRPC server")
	fs.StringVar(&s.ServerBindPort, "grpc-server-bindport", "8090", "gPRC server bind port")
	fs.StringVar(&s.BrokerBindPort, "grpc-broker-bindport", "8091", "gPRC broker bind port")
	fs.Uint32Var(&s.MaxConcurrentStreams, "grpc-max-concurrent-streams", math.MaxUint32, "gPRC max concurrent streams")
	fs.IntVar(&s.MaxReceiveMessageSize, "grpc-max-receive-message-size", 1024*1024*4, "gPRC max receive message size")
	fs.IntVar(&s.MaxSendMessageSize, "grpc-max-send-message-size", math.MaxInt32, "gPRC max send message size")
	fs.DurationVar(&s.ConnectionTimeout, "grpc-connection-timeout", 120*time.Second, "gPRC connection timeout")
	fs.DurationVar(&s.MaxConnectionAge, "grpc-max-connection-age", time.Duration(math.MaxInt64), "A duration for the maximum amount of time connection may exist before closing")
	fs.DurationVar(&s.ClientMinPingInterval, "grpc-client-min-ping-interval", 5*time.Second, "Server will terminate the connection if the client pings more than once within this duration")
	fs.DurationVar(&s.ServerPingInterval, "grpc-server-ping-interval", 30*time.Second, "Duration after which the server pings the client if no activity is detected")
	fs.DurationVar(&s.ServerPingTimeout, "grpc-server-ping-timeout", 10*time.Second, "Duration the client waits for a response after sending a keepalive ping")
	fs.BoolVar(&s.PermitPingWithoutStream, "permit-ping-without-stream", false, "Allow keepalive pings even when there are no active streams")
	fs.IntVar(&s.WriteBufferSize, "grpc-write-buffer-size", 32*1024, "gPRC write buffer size")
	fs.IntVar(&s.ReadBufferSize, "grpc-read-buffer-size", 32*1024, "gPRC read buffer size")
	fs.BoolVar(&s.DisableTLS, "disable-grpc-tls", false, "Disable TLS for gRPC server, default is false")
	fs.StringVar(&s.TLSCertFile, "grpc-tls-cert-file", "", "The path to the tls.crt file")
	fs.StringVar(&s.TLSKeyFile, "grpc-tls-key-file", "", "The path to the tls.key file")
	fs.StringVar(&s.BrokerTLSCertFile, "grpc-broker-tls-cert-file", "", "The path to the broker tls.crt file")
	fs.StringVar(&s.BrokerTLSKeyFile, "grpc-broker-tls-key-file", "", "The path to the broker tls.key file")
	fs.StringVar(&s.GRPCAuthNType, "grpc-authn-type", "mock", "Specify the gRPC authentication type (e.g., mock, mtls or token)")
	fs.StringVar(&s.GRPCAuthorizerConfig, "grpc-authorizer-config", "", "Path to the gRPC authorizer configuration file")
	fs.StringVar(&s.ClientCAFile, "grpc-client-ca-file", "", "The path to the client ca file, must specify if using mtls authentication type")
	fs.StringVar(&s.BrokerClientCAFile, "grpc-broker-client-ca-file", "", "The path to the broker client ca file")
}
