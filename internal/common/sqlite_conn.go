package common

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

const DBFile = "flotta.db"

func SetupSqliteDB() {

	log.Info("Setup end nodes sqlite local database")
	// Check if the database file already exists
	if _, err := os.Stat(DBFile); err == nil {
		log.Error("Database file already exists, skipping creation...\n")
	} else if os.IsNotExist(err) {
		// Create the SQLite database file if it doesn't exist
		file, err := os.Create(DBFile)
		if err != nil {
			log.Errorf("Error creating database file: %s \n", err.Error())
			return
		}
		file.Close()
		log.Info("Database file created successfully.")
	} else {
		log.Errorf("Error checking database file: %s \n", err.Error())
		return
	}

	// Open a connection to the SQLite database

	db, err := SQLiteConnect(DBFile)
	if err != nil {
		log.Errorf("Error openning sqlite database file: %s\n", err.Error())
	}
	defer db.Close()

	// Create a table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS wireless_device (
		wireless_device_id INTEGER PRIMARY KEY AUTOINCREMENT,
		wireless_device_name TEXT NOT NULL,
		wireless_device_manufacturer TEXT NULL,
		wireless_device_model TEXT NULL,
		wireless_device_sw_version TEXT NULL,
		wireless_device_identifier TEXT NOT NULL,
		wireless_device_protocol TEXT NULL,
		wireless_device_connection TEXT NULL,
		wireless_device_battery TEXT NULL,
		wireless_device_availability TEXT NULL,
		wireless_device_description TEXT NULL,
		wireless_device_last_seen TEXT NOT NULL
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Errorf("Error creating table: %s \n", err.Error())
		return
	}

	// Create a table if it doesn't exist
	createTableSQL = `
		CREATE TABLE IF NOT EXISTS device_property (
			device_property_id INTEGER PRIMARY KEY AUTOINCREMENT,
			wireless_device_identifier TEXT NOT NULL,
			property_identifier TEXT NOT NULL,
			
			property_service_uuid TEXT NULL,
			property_name TEXT NOT NULL,
			property_access_mode TEXT NOT NULL,
			property_reading TEXT NULL,
			property_state TEXT NULL,
			property_unit TEXT NULL,
			property_description TEXT NULL,
			property_last_seen TEXT NULL
		);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Errorf("Error creating table: %s \n", err.Error())
		return
	}

	// Create a table if it doesn't exist
	createTableSQL = `
		CREATE TABLE IF NOT EXISTS known_device (
			known_device_id INTEGER PRIMARY KEY AUTOINCREMENT,
			wireless_device_identifier TEXT NOT NULL,
			wireless_device_name TEXT NULL
		);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Errorf("Error creating table: %s \n", err.Error())
		return
	}

	log.Info("Table created successfully or already exists!")
}

func SQLiteConnect(dbFile string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	// Set connection properties (optional)
	db.SetMaxOpenConns(10) // Set the maximum number of open connections
	db.SetMaxIdleConns(5)  // Set the maximum number of idle connections

	// Perform a simple query to ensure the connection is valid
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping the database: %w", err)
	}

	// log.Info("Connected to SQLite database successfully")
	return db, nil
}
