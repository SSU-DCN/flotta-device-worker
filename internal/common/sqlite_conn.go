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
	CREATE TABLE IF NOT EXISTS EndNodeDevice (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		manufacturer TEXT NULL,
		model TEXT NULL,
		sw_version TEXT NULL,
		identifiers TEXT NOT NULL,
		protocol TEXT NULL,
		connection TEXT NULL,
		battery TEXT NULL,
		availability TEXT NULL,
		topic TEXT NOT NULL,
		device_type TEXT NULL,
		readings TEXT NULL,
		state TEXT NULL,
		last_seen TEXT NOT NULL
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Errorf("Error creating table: %s \n", err.Error())
		return
	}

	// Create a table if it doesn't exist
	createTableSQL = `
		CREATE TABLE IF NOT EXISTS EndNodeDeviceEvents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			identifiers TEXT NOT NULL,
			battery TEXT NULL,
			availability TEXT NULL,
			readings TEXT NULL,
			state TEXT NULL,
			last_seen TEXT NOT NULL
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

	log.Info("Connected to SQLite database successfully")
	return db, nil
}
