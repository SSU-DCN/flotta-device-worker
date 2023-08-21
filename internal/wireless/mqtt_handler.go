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

	topic := "plugin/#"
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
	} else if msg.Topic() == "plugin/edge/upstream/ble/unpaired" {
		log.Info("received unpaired devices")
		//check the unpaired devices and return registered devices only for pairing and getting the properties
		var discoveredWirelessDevices []*models.DbWirelessDevice
		err := json.Unmarshal([]byte(msg.Payload()), &discoveredWirelessDevices)
		if err != nil {
			log.Errorf("Error parsing data in %s topic, Error: %s", msg.Topic(), err.Error())
			return
		}

		db, err := common.SQLiteConnect(common.DBFile)
		if err != nil {
			log.Errorf("Error openning sqlite database file: %s\n", err.Error())
		}
		defer db.Close()
		err = FilterUnknownDevicesForRegistration(db, discoveredWirelessDevices)
		if err != nil {
			log.Errorf("Error filtering discovered devices in %s topic, Error: %s", msg.Topic(), err.Error())
			return
		}
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
		// if deviceData, ok := dataMap["device"].(map[string]interface{}); ok {
		wirelessDevice := models.WirelessDevice{
			WirelessDeviceName:         getStringValue(dataMap, "wireless_device_name"),
			WirelessDeviceManufacturer: getStringValue(dataMap, "wireless_device_manufacturer"),
			WirelessDeviceModel:        getStringValue(dataMap, "wireless_device_model"),
			WirelessDeviceSwVersion:    getStringValue(dataMap, "wireless_device_sw_version"),
			WirelessDeviceIdentifier:   getStringValue(dataMap, "wireless_device_identifier"),
			WirelessDeviceProtocol:     strings.ToLower(getStringValue(dataMap, "wireless_device_protocol")),
			WirelessDeviceConnection:   strings.ToLower(getStringValue(dataMap, "wireless_device_connection")),
			WirelessDeviceAvailability: getStringValue(dataMap, "wireless_device_availability"),
			WirelessDeviceBattery:      getStringValue(dataMap, "wireless_device_battery"),
			WirelessDeviceDescription:  getStringValue(dataMap, "wireless_device_description"),
			WirelessDeviceLastSeen:     getStringValue(dataMap, "wireless_device_last_seen"),
		}

		var wirelessDeviceProperties []*models.DeviceProperty

		if propertiesData, ok := dataMap["device_properties"].([]interface{}); ok {
			// Convert the properties data to a JSON string
			// Marshal the properties data
			log.Info("DEVICE PROPERTIES RECEIVED")

			// propertiesRawData := propertiesData
			// if err != nil {
			// 	log.Errorf("Error converting Device Properties to JSON: %s", err.Error())
			// 	return
			// }

			// // Unmarshal the properties raw data into a map
			// var propertiesData []map[string]interface{}
			// err = json.Unmarshal(propertiesRawData, &propertiesData)
			// if err != nil {
			// 	log.Errorf("Error parsing JSON: %s", err.Error())
			// 	return
			// }

			for _, propertyDataItem := range propertiesData {
				if propertyData, ok := propertyDataItem.(map[string]interface{}); ok {

					// log.Info(propertyData)

					wirelessDeviceProperty := models.DeviceProperty{
						PropertyAccessMode:       getStringValue(propertyData, "property_access_mode"),
						PropertyDescription:      getStringValue(propertyData, "property_description"),
						PropertyIdentifier:       getStringValue(propertyData, "property_identifier"),
						WirelessDeviceIdentifier: getStringValue(dataMap, "wireless_device_identifier"),
						PropertyLastSeen:         getStringValue(propertyData, "property_last_seen"),
						PropertyName:             getStringValue(propertyData, "property_name"),
						PropertyReading:          getStringValue(propertyData, "property_reading"),
						PropertyServiceUUID:      getStringValue(propertyData, "property_service_uuid"),
						PropertyState:            getStringValue(propertyData, "property_state"),
						PropertyUnit:             getStringValue(propertyData, "property_unit"),
					}

					wirelessDeviceProperties = append(wirelessDeviceProperties, &wirelessDeviceProperty)
				} else {
					log.Error("Invalid property data format")
				}
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

		// } else {
		// 	log.Info("Device data not found in the JSON.")
		// }
	}

}

func PublishMQTT(client mqtt.Client, topic string, payLoad interface{}) error {
	payloadJSONData, err := json.Marshal(payLoad)
	if err != nil {
		log.Errorf("Error marshaling JSON: %s\n", err)
		return err
	}
	token := client.Publish(topic, 0, false, payloadJSONData)
	if token.Error() != nil {
		log.Errorf("Error publishing to topic: %s\n", token.Error())
		return token.Error()
	}
	return nil
}
