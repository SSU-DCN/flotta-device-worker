package wireless

import (
	"encoding/json"
	"fmt"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/project-flotta/flotta-device-worker/internal/common"
	"github.com/project-flotta/flotta-operator/models"
	log "github.com/sirupsen/logrus"
)

func MQTT_Setup() {
	log.Infoln("MQTT HERE")

	client, err := common.MQTT_Connect()
	if err != nil {
		log.Errorf("Failed to connect to MQTT: %s", err.Error())

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

		// Access and print the parsed device data
		if deviceData, ok := dataMap["device"].(map[string]interface{}); ok {
			wirelessDevice := models.WirelessDevice{
				WirelessDeviceName:         getStringValue(deviceData, "wireless_device_name"),
				WirelessDeviceManufacturer: getStringValue(deviceData, "wireless_device_manufacturer"),
				WirelessDeviceModel:        getStringValue(deviceData, "wireless_device_model"),
				WirelessDeviceSwVersion:    getStringValue(deviceData, "wireless_device_sw_version"),
				WirelessDeviceIdentifier:   getStringValue(deviceData, "wireless_device_identifier"),
				WirelessDeviceProtocol:     strings.ToLower(getStringValue(deviceData, "wireless_device_protocol")),
				WirelessDeviceConnection:   strings.ToLower(getStringValue(deviceData, "wireless_device_connection")),
				WirelessDeviceAvailability: getStringValue(deviceData, "wireless_device_availability"),
				WirelessDeviceBattery:      getStringValue(deviceData, "wireless_device_battery"),
				WirelessDeviceDescription:  getStringValue(deviceData, "wireless_device_description"),
				WirelessDeviceLastSeen:     getStringValue(deviceData, "wireless_device_last_seen"),
			}

			var wirelessDeviceProperties []*models.DeviceProperty

			if _, ok := dataMap["properties"].(map[string]interface{}); ok {
				// Convert the properties data to a JSON string
				// Marshal the properties data
				propertiesRawData, err := json.Marshal(dataMap["properties"])
				if err != nil {
					log.Errorf("Error converting Device Properties to JSON: %s", err.Error())
					return
				}

				// Unmarshal the properties raw data into a map
				var propertiesData []map[string]interface{}
				err = json.Unmarshal(propertiesRawData, &propertiesData)
				if err != nil {
					log.Errorf("Error parsing JSON: %s", err.Error())
					return
				}

				for _, propertyData := range propertiesData {
					wirelessDeviceProperty := models.DeviceProperty{
						PropertyAccessMode:       getStringValue(propertyData, "property_access_mode"),
						PropertyDescription:      getStringValue(propertyData, "property_description"),
						PropertyIdentifier:       getStringValue(propertyData, "property_identifier"),
						WirelessDeviceIdentifier: getStringValue(propertyData, "wireless_device_identifier"),
						PropertyLastSeen:         getStringValue(propertyData, "property_last_seen"),
						PropertyName:             getStringValue(propertyData, "property_name"),
						PropertyReading:          getStringValue(propertyData, "property_reading"),
						PropertyServiceUUID:      getStringValue(propertyData, "property_service_uuid"),
						PropertyState:            getStringValue(propertyData, "property_state"),
						PropertyUnit:             getStringValue(propertyData, "property_unit"),
					}

					wirelessDeviceProperties = append(wirelessDeviceProperties, &wirelessDeviceProperty)
				}

			}

			wirelessDevice.DeviceProperties = wirelessDeviceProperties

			db, err := common.SQLiteConnect(common.DBFile)
			if err != nil {
				log.Errorf("Error openning sqlite database file: %s\n", err.Error())
			}
			defer db.Close()
			if !isEndNodeDeviceRecordExists(db, wirelessDevice) {
				err = saveWirelessDeviceData(wirelessDevice, db)
				if err != nil {
					log.Errorf("Error inserting end node device record to local sqlite database: %s\n", err.Error())
				} else {
					log.Info("End node device information inserted")
				}
			} else {

				log.Info("End node device already exists, updating it")
				err = updateEndNodeDevice(db, wirelessDevice)
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

func PublishMQTT(client mqtt.Client, topic string, payLoad interface{}) error {
	token := client.Publish(topic, 0, false, payLoad)
	if token.Error() != nil {
		log.Errorf("Error publishing to topic: %s\n", token.Error())
		return token.Error()
	}
	return nil
}
