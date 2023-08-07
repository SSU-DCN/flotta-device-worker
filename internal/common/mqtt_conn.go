package common

import (
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

const (
	mqttBroker   = "tcp://localhost:1883"
	mqttUsername = "flotta"
	mqttPassword = "flotta"
)

var isConnected bool
var client mqtt.Client

func MQTT_Connect() (mqtt.Client, error) {
	if isConnected {
		return client, nil
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBroker)
	opts.SetClientID("mqtt_to_sqlite_client")
	opts.SetUsername(mqttUsername)
	opts.SetPassword(mqttPassword)
	opts.KeepAlive = int64(60 * time.Second)
	opts.AutoReconnect = true

	client = mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to connect to MQTT broker: %s", token.Error())
		return nil, token.Error()
	}

	isConnected = true
	return client, nil
}
