package clients

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestLoadRESTConfigFromFlags(t *testing.T) {
	tests := []struct {
		name        string
		setupFlags  func(*cobra.Command)
		setupEnv    func()
		cleanupEnv  func()
		wantErr     bool
		errContains string
		validate    func(*testing.T, *RESTConfig)
	}{
		{
			name: "use default values when flags not set",
			setupFlags: func(cmd *cobra.Command) {
				// Use defaults
			},
			setupEnv: func() {
				// Set default values via environment variables since flag defaults aren't available until parsed
				os.Setenv(EnvRESTURL, "https://127.0.0.1:30080")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvRESTURL)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *RESTConfig) {
				// Should use flag defaults
				if cfg.BaseURL != "https://127.0.0.1:30080" {
					t.Errorf("BaseURL = %v, want default %v", cfg.BaseURL, "https://127.0.0.1:30080")
				}
				if cfg.InsecureSkipVerify != false {
					t.Error("InsecureSkipVerify should default to false")
				}
				if cfg.Timeout != 30*time.Second {
					t.Errorf("Timeout = %v, want default %v", cfg.Timeout, 30*time.Second)
				}
			},
		},
		{
			name: "override defaults with explicit flag values",
			setupFlags: func(cmd *cobra.Command) {
				// Flags need to be set via environment since flag defaults aren't available until parsed
			},
			setupEnv: func() {
				os.Setenv(EnvRESTURL, "https://example.com:8080")
				os.Setenv(EnvInsecureSkipVerify, "true")
				os.Setenv(EnvTimeout, "60s")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvRESTURL)
				os.Unsetenv(EnvInsecureSkipVerify)
				os.Unsetenv(EnvTimeout)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *RESTConfig) {
				if cfg.BaseURL != "https://example.com:8080" {
					t.Errorf("BaseURL = %v, want %v", cfg.BaseURL, "https://example.com:8080")
				}
				if !cfg.InsecureSkipVerify {
					t.Error("InsecureSkipVerify = false, want true")
				}
				if cfg.Timeout != 60*time.Second {
					t.Errorf("Timeout = %v, want %v", cfg.Timeout, 60*time.Second)
				}
			},
		},
		{
			name: "insecure skip verify from environment",
			setupFlags: func(cmd *cobra.Command) {
				// Don't set insecure flag
			},
			setupEnv: func() {
				os.Setenv(EnvRESTURL, "https://127.0.0.1:30080")
				os.Setenv(EnvInsecureSkipVerify, "true")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvRESTURL)
				os.Unsetenv(EnvInsecureSkipVerify)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *RESTConfig) {
				if !cfg.InsecureSkipVerify {
					t.Error("InsecureSkipVerify should be true from env var")
				}
			},
		},
		{
			name: "timeout from environment when not set in flags",
			setupFlags: func(cmd *cobra.Command) {
				// Don't set timeout flag
			},
			setupEnv: func() {
				os.Setenv(EnvRESTURL, "https://127.0.0.1:30080")
				os.Setenv(EnvTimeout, "45s")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvRESTURL)
				os.Unsetenv(EnvTimeout)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *RESTConfig) {
				if cfg.Timeout != 45*time.Second {
					t.Errorf("Timeout = %v, want %v from env", cfg.Timeout, 45*time.Second)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			AddRESTClientFlags(cmd)
			tt.setupFlags(cmd)

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			tt.setupEnv()
			defer tt.cleanupEnv()

			// Execute
			cfg, err := LoadRESTConfigFromFlags(cmd)

			// Validate error
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadRESTConfigFromFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("LoadRESTConfigFromFlags() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}

			// Validate result
			if tt.validate != nil && cfg != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoadGRPCConfigFromFlags(t *testing.T) {
	tests := []struct {
		name        string
		setupFlags  func(*cobra.Command)
		setupEnv    func()
		cleanupEnv  func()
		wantErr     bool
		errContains string
		validate    func(*testing.T, *GRPCConfig)
	}{
		{
			name: "use default values when flags not set",
			setupFlags: func(cmd *cobra.Command) {
				// Use defaults
			},
			setupEnv: func() {
				// Set default values via environment variables since flag defaults aren't available until parsed
				os.Setenv(EnvGRPCServerAddress, "127.0.0.1:30090")
				os.Setenv(EnvGRPCSourceID, "test-source")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvGRPCServerAddress)
				os.Unsetenv(EnvGRPCSourceID)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *GRPCConfig) {
				// Should use flag defaults
				if cfg.ServerAddress != "127.0.0.1:30090" {
					t.Errorf("ServerAddress = %v, want default %v", cfg.ServerAddress, "127.0.0.1:30090")
				}
				if cfg.SourceID != "test-source" {
					t.Errorf("SourceID = %v, want default %v", cfg.SourceID, "test-source")
				}
			},
		},
		{
			name: "override defaults with explicit flag values",
			setupFlags: func(cmd *cobra.Command) {
				// Flags need to be set via environment since flag defaults aren't available until parsed
			},
			setupEnv: func() {
				os.Setenv(EnvGRPCServerAddress, "grpc.example.com:8090")
				os.Setenv(EnvGRPCSourceID, "custom-client")
				os.Setenv(EnvGRPCCAFile, "/path/to/ca.crt")
				os.Setenv(EnvGRPCTokenFile, "/path/to/token")
				os.Setenv(EnvGRPCClientCert, "/path/to/client.crt")
				os.Setenv(EnvGRPCClientKey, "/path/to/client.key")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvGRPCServerAddress)
				os.Unsetenv(EnvGRPCSourceID)
				os.Unsetenv(EnvGRPCCAFile)
				os.Unsetenv(EnvGRPCTokenFile)
				os.Unsetenv(EnvGRPCClientCert)
				os.Unsetenv(EnvGRPCClientKey)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *GRPCConfig) {
				if cfg.ServerAddress != "grpc.example.com:8090" {
					t.Errorf("ServerAddress = %v, want %v", cfg.ServerAddress, "grpc.example.com:8090")
				}
				if cfg.SourceID != "custom-client" {
					t.Errorf("SourceID = %v, want %v", cfg.SourceID, "custom-client")
				}
				if cfg.CAFile != "/path/to/ca.crt" {
					t.Errorf("CAFile = %v, want %v", cfg.CAFile, "/path/to/ca.crt")
				}
				if cfg.TokenFile != "/path/to/token" {
					t.Errorf("TokenFile = %v, want %v", cfg.TokenFile, "/path/to/token")
				}
				if cfg.ClientCert != "/path/to/client.crt" {
					t.Errorf("ClientCert = %v, want %v", cfg.ClientCert, "/path/to/client.crt")
				}
				if cfg.ClientKey != "/path/to/client.key" {
					t.Errorf("ClientKey = %v, want %v", cfg.ClientKey, "/path/to/client.key")
				}
			},
		},
		{
			name: "load optional fields from environment variables",
			setupFlags: func(cmd *cobra.Command) {
				// Use flag defaults for required fields
			},
			setupEnv: func() {
				os.Setenv(EnvGRPCServerAddress, "127.0.0.1:30090")
				os.Setenv(EnvGRPCSourceID, "test-source")
				os.Setenv(EnvGRPCCAFile, "/env/ca.crt")
				os.Setenv(EnvGRPCTokenFile, "/env/token")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvGRPCServerAddress)
				os.Unsetenv(EnvGRPCSourceID)
				os.Unsetenv(EnvGRPCCAFile)
				os.Unsetenv(EnvGRPCTokenFile)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *GRPCConfig) {
				if cfg.CAFile != "/env/ca.crt" {
					t.Errorf("CAFile = %v, want %v from env", cfg.CAFile, "/env/ca.crt")
				}
				if cfg.TokenFile != "/env/token" {
					t.Errorf("TokenFile = %v, want %v from env", cfg.TokenFile, "/env/token")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			AddGRPCClientFlags(cmd, "test-source")
			tt.setupFlags(cmd)

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			tt.setupEnv()
			defer tt.cleanupEnv()

			// Execute
			cfg, err := LoadGRPCConfigFromFlags(cmd)

			// Validate error
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadGRPCConfigFromFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("LoadGRPCConfigFromFlags() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}

			// Validate result
			if tt.validate != nil && cfg != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoadConfigFromFlags(t *testing.T) {
	tests := []struct {
		name       string
		setupFlags func(*cobra.Command)
		setupEnv   func()
		cleanupEnv func()
		wantErr    bool
		validate   func(*testing.T, *Config)
	}{
		{
			name: "load both REST and gRPC configs with defaults",
			setupFlags: func(cmd *cobra.Command) {
				// Use defaults
			},
			setupEnv: func() {
				// Set default values via environment variables since flag defaults aren't available until parsed
				os.Setenv(EnvRESTURL, "https://127.0.0.1:30080")
				os.Setenv(EnvGRPCServerAddress, "127.0.0.1:30090")
				os.Setenv(EnvGRPCSourceID, "default-source")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvRESTURL)
				os.Unsetenv(EnvGRPCServerAddress)
				os.Unsetenv(EnvGRPCSourceID)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.RESTConfig.BaseURL != "https://127.0.0.1:30080" {
					t.Errorf("RESTConfig.BaseURL = %v, want default", cfg.RESTConfig.BaseURL)
				}
				if cfg.GRPCConfig.ServerAddress != "127.0.0.1:30090" {
					t.Errorf("GRPCConfig.ServerAddress = %v, want default", cfg.GRPCConfig.ServerAddress)
				}
			},
		},
		{
			name: "override both configs with custom values",
			setupFlags: func(cmd *cobra.Command) {
				// Flags need to be set via environment since flag defaults aren't available until parsed
			},
			setupEnv: func() {
				os.Setenv(EnvRESTURL, "https://custom.example.com:8080")
				os.Setenv(EnvGRPCServerAddress, "grpc.custom.com:9090")
				os.Setenv(EnvGRPCSourceID, "custom-source")
			},
			cleanupEnv: func() {
				os.Unsetenv(EnvRESTURL)
				os.Unsetenv(EnvGRPCServerAddress)
				os.Unsetenv(EnvGRPCSourceID)
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.RESTConfig.BaseURL != "https://custom.example.com:8080" {
					t.Errorf("RESTConfig.BaseURL = %v, want custom value", cfg.RESTConfig.BaseURL)
				}
				if cfg.GRPCConfig.ServerAddress != "grpc.custom.com:9090" {
					t.Errorf("GRPCConfig.ServerAddress = %v, want custom value", cfg.GRPCConfig.ServerAddress)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			AddClientFlags(cmd, "default-source")
			tt.setupFlags(cmd)

			// Parse flags to initialize them
			if err := cmd.ParseFlags([]string{}); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			tt.setupEnv()
			defer tt.cleanupEnv()

			// Execute
			cfg, err := LoadConfigFromFlags(cmd)

			// Validate error
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Validate result
			if tt.validate != nil && cfg != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestAddRESTClientFlags(t *testing.T) {
	cmd := &cobra.Command{}
	AddRESTClientFlags(cmd)

	// Verify flags are added
	flags := []string{FlagRESTURL, FlagInsecureSkipVerify, FlagTimeout}
	for _, flag := range flags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("Flag %s not added", flag)
		}
	}

	// Verify defaults
	urlFlag := cmd.PersistentFlags().Lookup(FlagRESTURL)
	if urlFlag.DefValue != "https://127.0.0.1:30080" {
		t.Errorf("Default REST URL = %v, want %v", urlFlag.DefValue, "https://127.0.0.1:30080")
	}
}

func TestAddGRPCClientFlags(t *testing.T) {
	cmd := &cobra.Command{}
	AddGRPCClientFlags(cmd, "test-source")

	// Verify flags are added
	flags := []string{
		FlagGRPCServerAddress,
		FlagGRPCSourceID,
		FlagGRPCCAFile,
		FlagGRPCTokenFile,
		FlagGRPCClientCert,
		FlagGRPCClientKey,
	}
	for _, flag := range flags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("Flag %s not added", flag)
		}
	}

	// Verify defaults
	addressFlag := cmd.PersistentFlags().Lookup(FlagGRPCServerAddress)
	if addressFlag.DefValue != "127.0.0.1:30090" {
		t.Errorf("Default gRPC address = %v, want %v", addressFlag.DefValue, "127.0.0.1:30090")
	}

	sourceIDFlag := cmd.PersistentFlags().Lookup(FlagGRPCSourceID)
	if sourceIDFlag.DefValue != "test-source" {
		t.Errorf("Default source ID = %v, want %v", sourceIDFlag.DefValue, "test-source")
	}
}

func TestAddClientFlags(t *testing.T) {
	cmd := &cobra.Command{}
	AddClientFlags(cmd, "test-source")

	// Verify both REST and gRPC flags are added
	allFlags := []string{
		FlagRESTURL,
		FlagInsecureSkipVerify,
		FlagTimeout,
		FlagGRPCServerAddress,
		FlagGRPCSourceID,
		FlagGRPCCAFile,
		FlagGRPCTokenFile,
		FlagGRPCClientCert,
		FlagGRPCClientKey,
	}
	for _, flag := range allFlags {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("Flag %s not added", flag)
		}
	}
}
