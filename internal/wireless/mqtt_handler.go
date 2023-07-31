package wireless

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/project-flotta/flotta-device-worker/internal/common"
	"github.com/project-flotta/flotta-operator/models"
	log "github.com/sirupsen/logrus"
)

func MQTT_Setup() {
	// log.Infoln("MQTT HERE")

	client, err := common.MQTT_Connect()
	if err != nil {
		return
	}

	topic := "device/#"
	if token := client.Subscribe(topic, 0, OnMessageReceived); token.Wait() && token.Error() != nil {
		fmt.Println("Failed to subscribe to MQTT topic:", token.Error())
		log.Errorf("Failed to subscribe to MQTT topic: %s\n", token.Error())
		return
	}

}

func OnMessageReceived(client mqtt.Client, msg mqtt.Message) {
	log.Infof("Received Topic: %s\n", msg.Topic())
	// log.Infof("Received message: %s\n", msg.Payload())

	if strings.Contains(msg.Topic(), "availability") {
		log.Info("This is the availability topic, will do later!")
	} else {

		// Parse the received message (assuming it's in JSON format)
		// Create a variable to unmarshal JSON data into a generic structure
		var dataMap map[string]interface{}

		// Unmarshal the JSON data into the 'dataMap' variable
		err := json.Unmarshal([]byte(msg.Payload()), &dataMap)
		if err != nil {
			log.Errorf("Error parsing JSON: %s", err.Error())
			return
		}
		timestamp := time.Now().Format(time.RFC3339)

		// Access and print the parsed device data
		if deviceData, ok := dataMap["device"].(map[string]interface{}); ok {
			device := models.WirelessDevice{
				Name:         getStringValue(deviceData, "name"),
				Manufacturer: getStringValue(deviceData, "manufacturer"),
				Model:        getStringValue(deviceData, "model"),
				SwVersion:    getStringValue(deviceData, "sw_version"),
				Identifiers:  getStringValue(deviceData, "identifiers"),
				Protocol:     strings.ToLower(getStringValue(deviceData, "protocol")),
				Connection:   strings.ToLower(getStringValue(deviceData, "connection")),
				LastSeen:     timestamp,
			}

			// Access and print the parsed readings data for sensor
			if _, ok := dataMap["readings"].(map[string]interface{}); ok {
				device.DeviceType = "Sensor"
				// Convert the readings data to a JSON string
				readingsJSON, err := json.Marshal(dataMap["readings"])
				if err != nil {
					log.Errorf("Error converting Readings to JSON: %s", err.Error())
					return
				}

				// Save the JSON string to a variable or file, or use it as needed
				readingsAsString := string(readingsJSON)

				// Print the JSON string representation of the readings
				device.Readings = readingsAsString
			}

			if _, ok := dataMap["state"].(map[string]interface{}); ok {
				device.DeviceType = "Switch"

				// Get the "state" value from the map
				stateData, ok := dataMap["state"].(map[string]interface{})
				if !ok {
					log.Error("Invalid state value or type")
					return
				}

				// Get the "state" field from the "stateData" map
				stateValue, ok := stateData["state"].(string)
				if !ok {
					log.Error("Invalid state field value or type")
					return
				}
				device.State = stateValue
			}

			db, err := common.SQLiteConnect(common.DBFile)
			if err != nil {
				log.Errorf("Error openning sqlite database file: %s\n", err.Error())
			}
			defer db.Close()
			if !isEndNodeDeviceRecordExists(db, msg.Topic(), device) {
				err = insertDataToSQLite(device, msg.Topic(), db)
				if err != nil {
					log.Errorf("Error inserting end node device record to local sqlite database: %s\n", err.Error())
				} else {
					log.Info("End node device information inserted")
				}
			} else {

				log.Info("End node device already exists, updating it")
				err = updateEndNodeDevice(db, msg.Topic(), device)
				if err != nil {
					log.Errorf("Error updating end node device record in local sqlite database: %s\n", err.Error())
				} else {
					log.Info("End node device information updated")
				}
			}

		} else {
			log.Info("Device data not found in the JSON.")
		}
	}

}

func PublishMQTT(client mqtt.Client, topic string, payLoad string) error {
	token := client.Publish(topic, 0, false, payLoad)
	if token.Error() != nil {
		return token.Error()
	}
	return nil
}
