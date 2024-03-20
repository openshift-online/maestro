package config

import (
	"math"
	"time"

	"github.com/spf13/pflag"
)

type GRPCServerConfig struct {
	EnableGRPCServer      bool          `json:"enable_grpc_server"`
	TLSCertFile           string        `json:"grpc_tls_cert_file"`
	TLSKeyFile            string        `json:"grpc_tls_key_file"`
	EnableTLS             bool          `json:"enable_grpc_tls"`
	BindPort              string        `json:"bind_port"`
	MaxConcurrentStreams  uint32        `json:"max_concurrent_steams"`
	MaxReceiveMessageSize int           `json:"max_receive_message_size"`
	MaxSendMessageSize    int           `json:"max_send_message_size"`
	ConnectionTimeout     time.Duration `json:"connection_timeout"`
	WriteBufferSize       int           `json:"write_buffer_size"`
	ReadBufferSize        int           `json:"read_buffer_size"`
	MaxConnectionAge      time.Duration `json:"max_connection_age"`
}

func NewGRPCServerConfig() *GRPCServerConfig {
	return &GRPCServerConfig{}
}

func (s *GRPCServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&s.EnableGRPCServer, "enable-grpc-server", false, "Enable gRPC server")
	fs.StringVar(&s.BindPort, "grpc-server-bindport", "8090", "gPRC server bind port")
	fs.Uint32Var(&s.MaxConcurrentStreams, "grpc-max-concurrent-streams", math.MaxUint32, "gPRC max concurrent streams")
	fs.IntVar(&s.MaxReceiveMessageSize, "grpc-max-receive-message-size", 1024*1024*4, "gPRC max receive message size")
	fs.IntVar(&s.MaxSendMessageSize, "grpc-max-send-message-size", math.MaxInt32, "gPRC max send message size")
	fs.DurationVar(&s.ConnectionTimeout, "grpc-connection-timeout", 120*time.Second, "gPRC connection timeout")
	fs.DurationVar(&s.MaxConnectionAge, "grpc-max-connection-age", time.Duration(math.MaxInt64), "A duration for the maximum amount of time connection may exist before closing")
	fs.IntVar(&s.WriteBufferSize, "grpc-write-buffer-size", 32*1024, "gPRC write buffer size")
	fs.IntVar(&s.ReadBufferSize, "grpc-read-buffer-size", 32*1024, "gPRC read buffer size")
	fs.StringVar(&s.TLSCertFile, "grpc-tls-cert-file", "", "The path to the tls.crt file")
	fs.StringVar(&s.TLSKeyFile, "grpc-tls-key-file", "", "The path to the tls.key file")
	fs.BoolVar(&s.EnableTLS, "enable-grpc-tls", false, "Enable TLS for gRPC server")
}
