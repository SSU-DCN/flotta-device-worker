package wireless

import (
	"database/sql"

	"github.com/project-flotta/flotta-device-worker/internal/common"
	"github.com/project-flotta/flotta-operator/models"
)

func GetConnectedWirelessDevices(db *sql.DB) ([]*models.WirelessDevice, error) {
	rows, err := db.Query("SELECT name, manufacturer, model, sw_version, identifiers, protocol, connection, battery, availability, device_type, last_seen, state, readings FROM EndNodeDevice ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.WirelessDevice
	for rows.Next() {
		// fmt.Println("ITEMS")

		// Create a new instance of models.WirelessDevice
		item := &models.WirelessDevice{}

		var readingsPtr *string // Declare a pointer to string
		var statePtr *string    // Declare a pointer to string

		err := rows.Scan(&item.Name, &item.Manufacturer, &item.Model, &item.SwVersion, &item.Identifiers, &item.Protocol, &item.Connection, &item.Battery, &item.Availability, &item.DeviceType, &item.LastSeen, &statePtr, &readingsPtr)
		if err != nil {
			return nil, err
		}
		// Now, assign the value from readingsPtr to the item.Readings field
		if readingsPtr != nil {
			item.Readings = *readingsPtr // Dereference the pointer and assign the string value
		} else {
			item.Readings = "" // Set to an empty string if the value is NULL (readingsPtr is nil)
		}
		if statePtr != nil {
			item.State = *statePtr // Dereference the pointer and assign the string value
		} else {
			item.State = "" // Set to an empty string if the value is NULL (readingsPtr is nil)
		}
		items = append(items, item)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return items, nil
}

func ActionForWiFiMqttDevice(db *sql.DB, wirelessDeviceConfiguration models.WirelessDevice) error {

	client, err := common.MQTT_Connect()
	if err != nil {
		return err
	}

	topic, err := GetEndNodeDeviceTopic(db, wirelessDeviceConfiguration.Identifiers)
	if err != nil {
		return err
	}

	err = PublishMQTT(client, topic, wirelessDeviceConfiguration.State)
	if err != nil {
		return err
	}
	return nil
}
