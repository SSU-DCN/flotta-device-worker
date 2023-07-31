package common

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

const (
	mqttBroker   = "tcp://localhost:1883"
	mqttUsername = "flotta"
	mqttPassword = "flotta"
)

func MQTT_Connect() (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBroker)
	opts.SetClientID("mqtt_to_sqlite_client")
	opts.SetUsername(mqttUsername)
	opts.SetPassword(mqttPassword)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to connect to MQTT broker: %s", token.Error())
		return nil, token.Error()
	}
	return client, nil
}
