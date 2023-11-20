package config

import (
	"github.com/spf13/pflag"

	mqttoptions "open-cluster-management.io/api/cloudevents/generic/options/mqtt"
)

type MessageBrokerConfig struct {
	MessageBrokerType  string                   `json:"message_broker_type"`
	MQTTOptions        *mqttoptions.MQTTOptions `json:"mqtt"`
	MQTTBrokerHostFile string                   `json:"mqtt_broker_host_file"`
	MQTTUserNameFile   string                   `json:"mqtt_username_file"`
	MQTTPasswordFile   string                   `json:"mqtt_password_file"`
}

func NewMessageBrokerConfig() *MessageBrokerConfig {
	mqttOptions := mqttoptions.NewMQTTOptions()
	mqttOptions.CAFile = "secrets/mqtt.rootcert"
	mqttOptions.ClientCertFile = "secrets/mqtt.clientcert"
	mqttOptions.ClientKeyFile = "secrets/mqtt.clientkey"
	return &MessageBrokerConfig{
		MessageBrokerType:  "mqtt",
		MQTTOptions:        mqttOptions,
		MQTTBrokerHostFile: "secrets/mqtt.host",
		MQTTUserNameFile:   "secrets/mqtt.user",
		MQTTPasswordFile:   "secrets/mqtt.password",
	}
}

func (c *MessageBrokerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.MessageBrokerType, "message-broker-type", c.MessageBrokerType, "Message broker type (default: mqtt)")
	c.MQTTOptions.AddFlags(fs)
	fs.StringVar(&c.MQTTBrokerHostFile, "mqtt-broker-host-file", c.MQTTBrokerHostFile, "MQTT broker address file")
	fs.StringVar(&c.MQTTUserNameFile, "mqtt-username-file", c.MQTTUserNameFile, "MQTT username file")
	fs.StringVar(&c.MQTTPasswordFile, "mqtt-password-file", c.MQTTPasswordFile, "MQTT password file")
}

func (c *MessageBrokerConfig) ReadFiles() error {
	err := readFileValueString(c.MQTTBrokerHostFile, &c.MQTTOptions.BrokerHost)
	if err != nil {
		return err
	}

	err = readFileValueString(c.MQTTUserNameFile, &c.MQTTOptions.Username)
	if err != nil {
		return err
	}

	err = readFileValueString(c.MQTTPasswordFile, &c.MQTTOptions.Password)
	if err != nil {
		return err
	}

	err = readFileValueString(c.MQTTOptions.CAFile, &c.MQTTOptions.CAFile)
	if err != nil {
		return err
	}

	err = readFileValueString(c.MQTTOptions.ClientCertFile, &c.MQTTOptions.ClientCertFile)
	if err != nil {
		return err
	}

	err = readFileValueString(c.MQTTOptions.ClientKeyFile, &c.MQTTOptions.ClientKeyFile)
	if err != nil {
		return err
	}

	return nil
}
