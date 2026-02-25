package clients

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

const (
	// REST API flag names
	FlagRESTURL            = "rest-url"
	FlagInsecureSkipVerify = "insecure-skip-verify"
	FlagTimeout            = "timeout"

	// gRPC flag names
	FlagGRPCServerAddress = "grpc-server-address"
	FlagGRPCCAFile        = "grpc-ca-file"
	FlagGRPCTokenFile     = "grpc-token-file"
	FlagGRPCClientCert    = "grpc-client-cert-file"
	FlagGRPCClientKey     = "grpc-client-key-file"
	FlagGRPCSourceID      = "grpc-source-id"

	// REST API environment variable names
	EnvRESTURL            = "MAESTRO_REST_URL"
	EnvInsecureSkipVerify = "MAESTRO_REST_INSECURE_SKIP_VERIFY"
	EnvTimeout            = "MAESTRO_REST_TIMEOUT"

	// gRPC environment variable names
	EnvGRPCServerAddress = "MAESTRO_GRPC_SERVER_ADDRESS"
	EnvGRPCCAFile        = "MAESTRO_GRPC_CA_FILE"
	EnvGRPCTokenFile     = "MAESTRO_GRPC_TOKEN_FILE"
	EnvGRPCClientCert    = "MAESTRO_GRPC_CLIENT_CERT_FILE"
	EnvGRPCClientKey     = "MAESTRO_GRPC_CLIENT_KEY_FILE"
	EnvGRPCSourceID      = "MAESTRO_GRPC_SOURCE_ID"
)

// Config holds client configuration
type Config struct {
	RESTConfig RESTConfig
	GRPCConfig GRPCConfig
}

// RESTConfig holds REST API client configuration
type RESTConfig struct {
	BaseURL            string
	InsecureSkipVerify bool
	Timeout            time.Duration
}

// GRPCConfig holds gRPC client configuration
type GRPCConfig struct {
	ServerAddress string
	CAFile        string
	TokenFile     string
	ClientCert    string
	ClientKey     string
	SourceID      string
}

// AddRESTClientFlags adds REST API client flags to a command
func AddRESTClientFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String(FlagRESTURL, "https://127.0.0.1:30080", "Maestro REST API base URL (env: MAESTRO_REST_URL)")
	cmd.PersistentFlags().Bool(FlagInsecureSkipVerify, false, "Skip TLS certificate verification for REST API (env: MAESTRO_REST_INSECURE_SKIP_VERIFY)")
	cmd.PersistentFlags().Duration(FlagTimeout, 30*time.Second, "HTTP client timeout for REST API (env: MAESTRO_REST_TIMEOUT)")
}

// AddGRPCClientFlags adds gRPC client flags to a command
func AddGRPCClientFlags(cmd *cobra.Command, defaultSourceID string) {
	cmd.PersistentFlags().String(FlagGRPCServerAddress, "127.0.0.1:30090", "gRPC server address (env: MAESTRO_GRPC_SERVER_ADDRESS)")
	cmd.PersistentFlags().String(FlagGRPCSourceID, defaultSourceID, "Source ID for gRPC client (env: MAESTRO_GRPC_SOURCE_ID)")
	cmd.PersistentFlags().String(FlagGRPCCAFile, "", "Path to CA certificate file for gRPC TLS (env: MAESTRO_GRPC_CA_FILE)")
	cmd.PersistentFlags().String(FlagGRPCTokenFile, "", "Path to token file for gRPC authentication (env: MAESTRO_GRPC_TOKEN_FILE)")
	cmd.PersistentFlags().String(FlagGRPCClientCert, "", "Path to client certificate file for mutual TLS (env: MAESTRO_GRPC_CLIENT_CERT_FILE)")
	cmd.PersistentFlags().String(FlagGRPCClientKey, "", "Path to client private key file for mutual TLS (env: MAESTRO_GRPC_CLIENT_KEY_FILE)")
}

// AddClientFlags adds both REST and gRPC client flags to a command
func AddClientFlags(cmd *cobra.Command, defaultSourceID string) {
	AddRESTClientFlags(cmd)
	AddGRPCClientFlags(cmd, defaultSourceID)
}

// LoadRESTConfigFromFlags loads REST client configuration from command flags with environment variable fallback
func LoadRESTConfigFromFlags(cmd *cobra.Command) (*RESTConfig, error) {
	restURL, err := cmd.Flags().GetString(FlagRESTURL)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagRESTURL, err)
	}
	if !cmd.Flags().Changed(FlagRESTURL) {
		if v := os.Getenv(EnvRESTURL); v != "" {
			restURL = v
		}
	}

	if restURL == "" {
		return nil, fmt.Errorf("REST API URL is required (use --%s flag or %s env var)", FlagRESTURL, EnvRESTURL)
	}

	insecureSkipVerify, err := cmd.Flags().GetBool(FlagInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagInsecureSkipVerify, err)
	}
	if !cmd.Flags().Changed(FlagInsecureSkipVerify) {
		if v := os.Getenv(EnvInsecureSkipVerify); v != "" {
			parsed, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("invalid %s: %w", EnvInsecureSkipVerify, err)
			}
			insecureSkipVerify = parsed
		}
	}

	timeout, err := cmd.Flags().GetDuration(FlagTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagTimeout, err)
	}
	if !cmd.Flags().Changed(FlagTimeout) {
		if envTimeout := os.Getenv(EnvTimeout); envTimeout != "" {
			parsed, err := time.ParseDuration(envTimeout)
			if err != nil {
				return nil, fmt.Errorf("invalid %s: %w", EnvTimeout, err)
			}
			timeout = parsed
		}
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("--%s must be greater than 0", FlagTimeout)
	}

	return &RESTConfig{
		BaseURL:            restURL,
		InsecureSkipVerify: insecureSkipVerify,
		Timeout:            timeout,
	}, nil
}

// LoadGRPCConfigFromFlags loads gRPC client configuration from command flags with environment variable fallback
func LoadGRPCConfigFromFlags(cmd *cobra.Command) (*GRPCConfig, error) {
	grpcServerAddress, err := cmd.Flags().GetString(FlagGRPCServerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagGRPCServerAddress, err)
	}
	if !cmd.Flags().Changed(FlagGRPCServerAddress) {
		if v := os.Getenv(EnvGRPCServerAddress); v != "" {
			grpcServerAddress = v
		}
	}

	if grpcServerAddress == "" {
		return nil, fmt.Errorf("gRPC server address is required (use --%s flag or %s env var)", FlagGRPCServerAddress, EnvGRPCServerAddress)
	}

	grpcSourceID, err := cmd.Flags().GetString(FlagGRPCSourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagGRPCSourceID, err)
	}
	if !cmd.Flags().Changed(FlagGRPCSourceID) {
		if v := os.Getenv(EnvGRPCSourceID); v != "" {
			grpcSourceID = v
		}
	}

	if grpcSourceID == "" {
		return nil, fmt.Errorf("gRPC source id is required (use --%s flag or %s env var)", FlagGRPCSourceID, EnvGRPCSourceID)
	}

	grpcCAFile, err := cmd.Flags().GetString(FlagGRPCCAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagGRPCCAFile, err)
	}
	if !cmd.Flags().Changed(FlagGRPCCAFile) {
		if v := os.Getenv(EnvGRPCCAFile); v != "" {
			grpcCAFile = v
		}
	}

	grpcTokenFile, err := cmd.Flags().GetString(FlagGRPCTokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagGRPCTokenFile, err)
	}
	if !cmd.Flags().Changed(FlagGRPCTokenFile) {
		if v := os.Getenv(EnvGRPCTokenFile); v != "" {
			grpcTokenFile = v
		}
	}

	grpcClientCert, err := cmd.Flags().GetString(FlagGRPCClientCert)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagGRPCClientCert, err)
	}
	if !cmd.Flags().Changed(FlagGRPCClientCert) {
		if v := os.Getenv(EnvGRPCClientCert); v != "" {
			grpcClientCert = v
		}
	}

	grpcClientKey, err := cmd.Flags().GetString(FlagGRPCClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read --%s: %w", FlagGRPCClientKey, err)
	}
	if !cmd.Flags().Changed(FlagGRPCClientKey) {
		if v := os.Getenv(EnvGRPCClientKey); v != "" {
			grpcClientKey = v
		}
	}

	return &GRPCConfig{
		ServerAddress: grpcServerAddress,
		CAFile:        grpcCAFile,
		TokenFile:     grpcTokenFile,
		ClientCert:    grpcClientCert,
		ClientKey:     grpcClientKey,
		SourceID:      grpcSourceID,
	}, nil
}

// LoadConfigFromFlags loads both REST and gRPC client configuration from command flags with environment variable fallback
func LoadConfigFromFlags(cmd *cobra.Command) (*Config, error) {
	// Try to load REST configuration
	restConfig, err := LoadRESTConfigFromFlags(cmd)
	if err != nil {
		return nil, err
	}

	// Try to load gRPC configuration
	grpcConfig, err := LoadGRPCConfigFromFlags(cmd)
	if err != nil {
		return nil, err
	}

	return &Config{
		RESTConfig: *restConfig,
		GRPCConfig: *grpcConfig,
	}, nil
}
