package config

import (
	"path/filepath"

	"github.com/spf13/pflag"

	"open-cluster-management.io/api/cloudevents/generic/options/mqtt"
)

type MessageBrokerConfig struct {
	EnableMock        bool              `json:"enable_message_broker_mock"`
	SourceID          string            `json:"source_id"`
	ClientID          string            `json:"client_id"`
	MessageBrokerType string            `json:"message_broker_type"`
	MQTTOptions       *mqtt.MQTTOptions `json:"mqtt_options"`
	MQTTConfig        string            `json:"mqtt-config-file"`
}

func NewMessageBrokerConfig() *MessageBrokerConfig {
	return &MessageBrokerConfig{
		EnableMock:        false,
		SourceID:          "maestro",
		ClientID:          "maestro",
		MessageBrokerType: "mqtt",
		MQTTConfig:        filepath.Join(GetProjectRootDir(), "secrets/mqtt.config"),
	}
}

func (c *MessageBrokerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.EnableMock, "enable-message-broker-mock", c.EnableMock, "Enable message broker mock")
	fs.StringVar(&c.SourceID, "source-id", c.SourceID, "Source ID")
	fs.StringVar(&c.ClientID, "client-id", c.ClientID, "Client ID")
	fs.StringVar(&c.MessageBrokerType, "message-broker-type", c.MessageBrokerType, "Message broker type (default: mqtt)")
	fs.StringVar(&c.MQTTConfig, "mqtt-config-file", c.MQTTConfig, "The config file path of mqtt broker")
}

func (c *MessageBrokerConfig) ReadFile() error {
	var err error
	c.MQTTOptions, err = mqtt.BuildMQTTOptionsFromFlags(c.MQTTConfig)
	if err != nil {
		return err
	}
	return nil
}
