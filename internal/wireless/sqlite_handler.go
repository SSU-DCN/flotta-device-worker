package wireless

import (
	"database/sql"
	"fmt"
	"unicode"

	"github.com/project-flotta/flotta-operator/models"
	log "github.com/sirupsen/logrus"
)

func saveWirelessDeviceData(data models.WirelessDevice, db *sql.DB) error {

	insertWirelessDeviceSQL := "INSERT INTO wireless_device (wireless_device_name, wireless_device_manufacturer, wireless_device_model, wireless_device_sw_version, wireless_device_identifier, wireless_device_protocol, wireless_device_connection,wireless_device_battery, wireless_device_availability, wireless_device_description, wireless_device_last_seen) VALUES (?,?,?,?,?,?,?,?,?,?,?);"
	_, err := db.Exec(insertWirelessDeviceSQL, data.WirelessDeviceName, data.WirelessDeviceManufacturer, data.WirelessDeviceModel, data.WirelessDeviceSwVersion, data.WirelessDeviceIdentifier, data.WirelessDeviceProtocol,
		data.WirelessDeviceConnection, data.WirelessDeviceBattery, data.WirelessDeviceAvailability, data.WirelessDeviceDescription, data.WirelessDeviceLastSeen)
	if err != nil {
		log.Errorf("Error inserting device data: %s", err.Error())
		return err
	}

	saveDeviceProperties(data.DeviceProperties, db)
	return nil
}

func isEndNodeDeviceRecordExists(db *sql.DB, device models.WirelessDevice) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM wireless_device WHERE wireless_device_name = ? AND wireless_device_identifier=?", device.WirelessDeviceName, device.WirelessDeviceIdentifier).Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	return count > 0
}

func updateEndNodeDevice(db *sql.DB, device models.WirelessDevice) error {
	log.Info("Update END NODE success updating End")

	_, err := db.Exec("UPDATE wireless_device SET wireless_device_description=?, wireless_device_last_seen=?  WHERE wireless_device_name = ?  AND wireless_device_identifier=?",
		device.WirelessDeviceDescription, device.WirelessDeviceLastSeen, device.WirelessDeviceName, device.WirelessDeviceIdentifier)
	if err != nil {
		log.Errorf("Error updating EndNode data: %s", err.Error())
		return err
	}

	saveDeviceProperties(device.DeviceProperties, db)

	return nil
}

func saveDeviceProperties(deviceProperties []*models.DeviceProperty, db *sql.DB) error {

	// log.Info("saving device properties")
	// log.Info("device properties LENGTH: ", len(deviceProperties))
	//separate device property reading and its unit
	for _, deviceProperty := range deviceProperties {
		if deviceProperty.PropertyName == "Service Changed" {
			continue
		}

		if deviceProperty.PropertyAccessMode == "Read" {
			property_unit := ""
			property_reading := ""
			if deviceProperty.PropertyUnit == "" {
				// log.Infof("PROPERTY UNIT INISDE: %s, READING: %s", property_unit, property_reading)
				measurement := deviceProperty.PropertyReading
				property_reading, property_unit, _ = separateMeasurement(measurement)
			} else {
				property_unit = deviceProperty.PropertyUnit
				property_reading = deviceProperty.PropertyReading
			}

			// log.Infof("PROPERTY UNIT OUTSIDE: %s, READING: %s", property_unit, property_reading)

			insertWirelessDevicePropertySQL := "INSERT INTO device_property (wireless_device_identifier, property_identifier, property_service_uuid, property_name, property_access_mode, property_reading,property_state, property_unit, property_description,  property_last_seen) VALUES (?,?,?,?,?,?,?,?,?,?);"
			_, err := db.Exec(insertWirelessDevicePropertySQL, deviceProperty.WirelessDeviceIdentifier, deviceProperty.PropertyIdentifier, deviceProperty.PropertyServiceUUID, deviceProperty.PropertyName, deviceProperty.PropertyAccessMode, property_reading, deviceProperty.PropertyState,
				property_unit, deviceProperty.PropertyDescription, deviceProperty.PropertyLastSeen)
			if err != nil {
				log.Errorf("Error inserting device property  data: %s", err.Error())
				return err
			}
		} else {

			insertWirelessDevicePropertySQL := "INSERT INTO device_property (wireless_device_identifier, property_identifier, property_service_uuid, property_name, property_access_mode, property_reading, property_state,property_unit, property_description,  property_last_seen) VALUES (?,?,?,?,?,?,?,?,?,?);"
			_, err := db.Exec(insertWirelessDevicePropertySQL, deviceProperty.WirelessDeviceIdentifier, deviceProperty.PropertyIdentifier, deviceProperty.PropertyServiceUUID, deviceProperty.PropertyName, deviceProperty.PropertyAccessMode, deviceProperty.PropertyReading, deviceProperty.PropertyState, deviceProperty.PropertyUnit, deviceProperty.PropertyDescription, deviceProperty.PropertyLastSeen)
			if err != nil {
				log.Errorf("Error inserting device property  data: %s", err.Error())
				return err
			}
		}
	}
	defer db.Close()
	return nil
}

// getStringValue is a helper function to safely retrieve string values from a map
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}

func separateMeasurement(input string) (string, string, error) {
	var valueStr string
	var unitStr string

	for _, char := range input {
		if unicode.IsDigit(char) || char == '.' {
			valueStr += string(char)
		} else {
			unitStr += string(char)
		}
	}

	if valueStr == "" || unitStr == "" {
		log.Error("Error processing measurement '%s': %s\n", valueStr, unitStr)

		return "", "", fmt.Errorf("invalid measurement format")
	}

	return valueStr, unitStr, nil
}
