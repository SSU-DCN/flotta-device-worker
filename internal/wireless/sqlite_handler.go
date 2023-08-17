package wireless

import (
	"database/sql"
	"fmt"

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

	_, err := db.Exec("UPDATE wireless_device SET wireless_device_description=? WHERE wireless_device_name = ?  AND wireless_device_identifier=?",
		device.WirelessDeviceDescription, device.WirelessDeviceName, device.WirelessDeviceIdentifier)
	if err != nil {
		log.Errorf("Error updating EndNode data: %s", err.Error())
		return err
	}

	saveDeviceProperties(device.DeviceProperties, db)

	return nil
}

func saveDeviceProperties(deviceProperties []*models.DeviceProperty, db *sql.DB) error {
	for _, deviceProperty := range deviceProperties {

		insertWirelessDevicePropertySQL := "INSERT INTO device_property (wireless_device_identifier, property_identifier, property_service_uuid, property_name, property_access_mode, property_reading, property_state, property_unit, property_description,  property_last_seen) VALUES (?,?,?,?,?,?,?,?,?,?);"
		_, err := db.Exec(insertWirelessDevicePropertySQL, deviceProperty.WirelessDeviceIdentifier, deviceProperty.PropertyIdentifier, deviceProperty.PropertyServiceUUID, deviceProperty.PropertyName, deviceProperty.PropertyAccessMode, deviceProperty.PropertyReading, deviceProperty.PropertyState,
			deviceProperty.PropertyUnit, deviceProperty.PropertyDescription, deviceProperty.PropertyLastSeen)
		if err != nil {
			log.Errorf("Error inserting device property  data: %s", err.Error())
			return err
		}
	}
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

func GetEndNodeDeviceTopic(db *sql.DB, identifier string) (string, error) {
	var result string

	query := "SELECT topic FROM wireless_device WHERE identifiers=? LIMIT 1"

	err := db.QueryRow(query, identifier).Scan(&result)

	if err != nil {
		if err == sql.ErrNoRows {
			// No matching record found.
			return "", fmt.Errorf("no matching record found for identifiers: %s", identifier)
		}
		return "", err
	}

	return result, nil
}
