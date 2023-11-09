package ocm

import (
	"fmt"

	sdkClient "github.com/openshift-online/ocm-sdk-go"
)

type Client struct {
	config     *Config
	logger     sdkClient.Logger
	connection *sdkClient.Connection

	Authorization OCMAuthorization
}

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	SelfToken    string
	TokenURL     string
	Debug        bool
}

func NewClient(config Config) (*Client, error) {
	// Create a logger that has the debug level enabled:
	logger, err := sdkClient.NewGoLoggerBuilder().
		Debug(config.Debug).
		Build()
	if err != nil {
		return nil, fmt.Errorf("Unable to build OCM logger: %s", err.Error())
	}

	client := &Client{
		config: &config,
		logger: logger,
	}
	err = client.newConnection()
	if err != nil {
		return nil, fmt.Errorf("Unable to build OCM connection: %s", err.Error())
	}
	client.Authorization = &authorization{client: client}
	return client, nil
}

func NewClientMock(config Config) (*Client, error) {
	client := &Client{
		config: &config,
	}
	client.Authorization = &authorizationMock{client: client}
	return client, nil
}

func (c *Client) newConnection() error {
	builder := sdkClient.NewConnectionBuilder().
		Logger(c.logger).
		URL(c.config.BaseURL).
		MetricsSubsystem("api_outbound")

	if c.config.ClientID != "" && c.config.ClientSecret != "" {
		builder = builder.Client(c.config.ClientID, c.config.ClientSecret)
	} else if c.config.SelfToken != "" {
		builder = builder.Tokens(c.config.SelfToken)
	} else {
		return fmt.Errorf("Can't build OCM client connection. No Client/Secret or Token has been provided.")
	}

	connection, err := builder.Build()

	if err != nil {
		return fmt.Errorf("Can't build OCM client connection: %s", err.Error())
	}
	c.connection = connection
	return nil
}

func (c *Client) Close() {
	if c.connection != nil {
		c.connection.Close()
	}
}

type service struct {
	client *Client
}
