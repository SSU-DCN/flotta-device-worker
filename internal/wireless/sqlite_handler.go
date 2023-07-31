package wireless

import (
	"database/sql"
	"fmt"

	"github.com/project-flotta/flotta-operator/models"
	log "github.com/sirupsen/logrus"
)

func insertDataToSQLite(data models.WirelessDevice, topic string, db *sql.DB) error {

	// Insert a record with timestamp into the table

	if data.DeviceType == "Sensor" {
		insertSQL := "INSERT INTO EndNodeDevice (name, manufacturer, model, sw_version, identifiers, protocol, connection,battery, availability, topic, device_type, readings, last_seen) VALUES (?, ?,?, ?,?, ?, ?,?, ?, ?,?, ?, ?);"
		_, err := db.Exec(insertSQL, data.Name, data.Manufacturer, data.Model, data.SwVersion, data.Identifiers, data.Protocol,
			data.Connection, data.Battery, data.Availability, topic, data.DeviceType, data.Readings, data.LastSeen)
		if err != nil {
			log.Errorf("Error inserting data: %s", err.Error())
			return err
		}
		insertSensorData(data, db)
	} else {
		insertSQL := "INSERT INTO EndNodeDevice (name, manufacturer, model, sw_version, identifiers, protocol, connection,battery, availability, topic, device_type, state,last_seen) VALUES (?, ?, ?,?, ?, ?,?, ?, ?,?,?, ?, ?);"
		_, err := db.Exec(insertSQL, data.Name, data.Manufacturer, data.Model, data.SwVersion, data.Identifiers, data.Protocol,
			data.Connection, data.Battery, data.Availability, topic, data.DeviceType, data.State, data.LastSeen)
		if err != nil {
			log.Errorf("Error inserting data: %s", err.Error())
			return err
		}
		insertSwitchData(data, db)
	}

	return nil
}

func isEndNodeDeviceRecordExists(db *sql.DB, topic string, device models.WirelessDevice) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM EndNodeDevice WHERE name = ? AND manufacturer = ? AND model =? AND identifiers=? AND topic=?", device.Name, device.Manufacturer, device.Model, device.Identifiers, topic).Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	return count > 0
}

func updateEndNodeDevice(db *sql.DB, topic string, device models.WirelessDevice) error {
	if device.DeviceType == "Sensor" {
		_, err := db.Exec("UPDATE EndNodeDevice SET sw_version=?, protocol= ?, connection=?, last_seen = ?, battery=?, availability=?, readings=? WHERE name = ?  AND identifiers=?",
			device.SwVersion, device.Protocol, device.Connection, device.LastSeen, device.Battery, device.Availability, device.Readings, device.Name, device.Identifiers)
		if err != nil {
			log.Errorf("Error updating EndNode data: %s", err.Error())
			return err
		}
		insertSensorData(device, db)
	} else {
		_, err := db.Exec("UPDATE EndNodeDevice SET sw_version=?, protocol= ?, connection=?, last_seen = ?, battery=?, availability=?, state=?  WHERE name = ? AND identifiers=?",
			device.SwVersion, device.Protocol, device.Connection, device.LastSeen, device.Battery, device.Availability, device.State, device.Name, device.Identifiers)
		if err != nil {
			log.Errorf("Error updating EndNode data: %s", err.Error())
			return err
		}
		insertSwitchData(device, db)
	}

	return nil
}

func insertSensorData(data models.WirelessDevice, db *sql.DB) error {
	insertSQL := "INSERT INTO EndNodeDeviceEvents (identifiers, battery, availability, readings, last_seen) VALUES (?, ?, ?,?, ?);"
	_, err := db.Exec(insertSQL, data.Identifiers, data.Battery, data.Availability, data.Readings, data.LastSeen)
	if err != nil {
		log.Errorf("Error inserting data for sensor: %s", err.Error())
		return err
	}
	return nil
}

func insertSwitchData(data models.WirelessDevice, db *sql.DB) error {
	insertSQL := "INSERT INTO EndNodeDeviceEvents (identifiers, battery, availability, state, last_seen) VALUES (?, ?, ?,?, ?);"
	_, err := db.Exec(insertSQL, data.Identifiers, data.Battery, data.Availability, data.State, data.LastSeen)
	if err != nil {
		log.Errorf("Error inserting data for switch: %s", err.Error())
		return err
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

	query := "SELECT topic FROM EndNodeDevice WHERE identifiers=? LIMIT 1"

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
