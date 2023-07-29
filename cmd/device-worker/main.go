package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/project-flotta/flotta-device-worker/internal/ansible"
	"github.com/project-flotta/flotta-device-worker/internal/common"
	configuration2 "github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer"
	hardware2 "github.com/project-flotta/flotta-device-worker/internal/hardware"
	heartbeat2 "github.com/project-flotta/flotta-device-worker/internal/heartbeat"
	"github.com/project-flotta/flotta-device-worker/internal/logs"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/project-flotta/flotta-device-worker/internal/mount"
	os2 "github.com/project-flotta/flotta-device-worker/internal/os"
	registration2 "github.com/project-flotta/flotta-device-worker/internal/registration"
	"github.com/project-flotta/flotta-device-worker/internal/server"
	workload2 "github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-operator/models"

	"net"
	"os"
	"path"
	"time"

	_ "github.com/mattn/go-sqlite3"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var yggdDispatchSocketAddr string
var gracefulRebootChannel chan struct{}

const (
	defaultDataDir = "/var/local/yggdrasil"
)

func initSystemdDirectory() error {
	systemddir := filepath.Join(os.Getenv("HOME"), ".config/systemd/user/")
	// init the flotta user systemd units directory
	err := os.MkdirAll(systemddir, 0750)
	if err != nil {
		return err
	}

	// chown the dir
	theuser, err := user.Lookup(os.Getenv("USER"))
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(theuser.Uid)
	gid, _ := strconv.Atoi(theuser.Gid)

	return filepath.Walk(filepath.Join(os.Getenv("HOME"), ".config"), func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = syscall.Chown(name, uid, gid)
		}
		return err
	})
}

func main() {
	logLevel, ok := os.LookupEnv("YGG_LOG_LEVEL")
	if !ok {
		logLevel = "ERROR"
	}
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		level = log.ErrorLevel
	}

	log.SetLevel(level)

	log.SetFormatter(&log.TextFormatter{
		ForceColors:      false,
		DisableColors:    true,
		DisableTimestamp: true,
	})

	log.SetOutput(os.Stdout)

	// Get initialization values from the environment.
	yggdDispatchSocketAddr, ok = os.LookupEnv("YGG_SOCKET_ADDR")
	if !ok {
		log.Fatal("missing YGG_SOCKET_ADDR environment variable")
	}

	baseDataDir, ok := os.LookupEnv("YGG_CONFIG_DIR")
	if !ok {
		log.Warnf("missing BASE_DATA_DIR environment variable. Using default: %s", defaultDataDir)
		baseDataDir = defaultDataDir
	}

	err = initSystemdDirectory()
	if err != nil {
		log.Warnf("Failed to create systemd directory for the user.")
	}

	// For RPM installation we stick with using flotta user:
	if flotta, err := user.Lookup("flotta"); err == nil && os.Getenv("FLOTTA_XDG_RUNTIME_DIR") == "" {
		if err = os.Setenv("FLOTTA_XDG_RUNTIME_DIR", fmt.Sprintf("/run/user/%s", flotta.Uid)); err != nil {
			log.Warnf("Failed to set XDG_RUNTIME_DIR env var for flotta user. Podman/systemd may misbehave.")
		}
	}

	// Dial the dispatcher on its well-known address.
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(yggdDispatchSocketAddr, opts)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a dispatcher client
	dispatcherClient := pb.NewDispatcherClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Register as a handler of the "device" type.
	r, err := dispatcherClient.Register(ctx, &pb.RegistrationRequest{Handler: "device", Pid: int64(os.Getpid())})
	if err != nil {
		log.Fatal(err)
	}
	if !r.GetRegistered() {
		log.Fatal("yggdrasil registration failed")
	}

	// Listen on the provided socket address.
	l, err := net.Listen("unix", r.GetAddress())
	if err != nil {
		log.Fatalf("cannot start listening on %s err: %v", r.GetAddress(), err)
	}

	//presetup SQLite database for end nodes
	common.SetupSqliteDB()

	// Register as a Worker service with gRPC and start accepting connections.
	dataDir := path.Join(baseDataDir, "device")
	log.Infof("Data directory: %s", dataDir)
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		log.Fatal(fmt.Errorf("cannot create directory: %w", err))
	}
	deviceId, ok := os.LookupEnv("YGG_CLIENT_ID")
	if !ok {
		log.Warn("DEVICE_ID environment variable has not been set")
		deviceId = "unknown"
	}
	configManager := configuration2.NewConfigurationManager(dataDir)

	// --- Client metrics configuration ---
	metricsStore, err := metrics.NewTSDB(dataDir)
	if err != nil {
		log.Fatalf("cannot initialize TSDB, err: %v", err)
	}
	configManager.RegisterObserver(metricsStore)
	// Metrics Daemon
	metricsDaemon := metrics.NewMetricsDaemon(metricsStore)

	workloadMetricWatcher := metrics.NewWorkloadMetrics(metricsDaemon)
	configManager.RegisterObserver(workloadMetricWatcher)

	systemMetricsWatcher, err := metrics.NewSystemMetrics(metricsDaemon)
	if err != nil {
		log.Fatalf("cannot initialize system metrics. DeviceID: %s; err: %v", deviceId, err)
	}
	configManager.RegisterObserver(systemMetricsWatcher)

	dataTransferWatcher := metrics.NewDataTransferMetrics(metricsDaemon)
	configManager.RegisterObserver(dataTransferWatcher)

	wl, err := workload2.NewWorkloadManager(dataDir, deviceId)
	if err != nil {
		log.Fatalf("cannot start Workload Manager. DeviceID: %s; err: %v", deviceId, err)
	}

	logsWrapper := logs.NewWorkloadsLogsTarget(wl)
	configManager.RegisterObserver(logsWrapper)
	wl.RegisterObserver(logsWrapper)
	configManager.RegisterObserver(wl)
	wl.RegisterObserver(workloadMetricWatcher)

	remoteWrite := metrics.NewRemoteWrite(dataDir, deviceId, metricsStore)
	configManager.RegisterObserver(remoteWrite)

	hw := hardware2.HardwareInfo{}
	hw.Init(nil)

	gracefulRebootChannel = make(chan struct{})
	deviceOs := os2.NewOS(gracefulRebootChannel, os2.NewOsExecCommands())
	configManager.RegisterObserver(deviceOs)

	mountManager, err := mount.New()
	if err != nil {
		log.Fatalf("cannot create Mount Manager: %s", err)
	}
	configManager.RegisterObserver(mountManager)

	dataMonitor := datatransfer.NewMonitor(wl, configManager)
	wl.RegisterObserver(dataMonitor)
	configManager.RegisterObserver(dataMonitor)
	dataMonitor.Start()

	if err != nil {
		log.Fatalf("cannot start metrics store. DeviceID: %s; err: %v", deviceId, err)
	}

	reg, err := registration2.NewRegistration(deviceId, &hw, dispatcherClient, configManager, wl)
	if err != nil {
		log.Fatalf("cannot start registration process:  DeviceID: %s; err: %v", deviceId, err)
	}

	dataDirPlaybook := path.Join(baseDataDir, "devicePlaybooks")
	if err := os.MkdirAll(dataDirPlaybook, 0750); err != nil {
		log.Fatalf("cannot create directory: %v", err)
	}
	ansibleManager, err := ansible.NewAnsibleManager(configManager, dispatcherClient, dataDirPlaybook, deviceId, reg)
	if err != nil {
		log.Errorf("cannot start ansible manager, err: %v", err)
	} else {
		configManager.RegisterObserver(ansibleManager)

		err = ansibleManager.ExecutePendingPlaybooks()
		if err != nil {
			log.Errorf("cannot run previous ansible playbooks, err: %v", err)
		}
	}

	go func() {
		mqttSqlite()
	}()

	hbs := heartbeat2.NewHeartbeatService(dispatcherClient, configManager, wl, &hw, ansibleManager, dataMonitor, deviceOs, reg)
	configManager.RegisterObserver(hbs)

	reg.DeregisterLater(
		wl,
		configManager,
		hbs,
		ansibleManager,
		dataMonitor,
		systemMetricsWatcher,
		metricsStore,
	)
	s := grpc.NewServer()
	pb.RegisterWorkerServer(s, server.NewDeviceServer(configManager, reg, ansibleManager))
	if !configManager.IsInitialConfig() {
		hbs.Start()
		ansibleManager.Start()
	} else {
		reg.RegisterDevice()
	}

	setupSignalHandler(metricsStore, ansibleManager)

	go listenStartGracefulRebootChannel(wl, dataMonitor, systemMetricsWatcher, metricsStore, hbs, ansibleManager,
		gracefulRebootChannel, deviceOs)

	if err := s.Serve(l); err != nil {
		log.Fatalf("cannot start worker server, err: %v", err)
	}

}

func listenStartGracefulRebootChannel(wl *workload2.WorkloadManager, dataMonitor *datatransfer.Monitor,
	systemMetricsWatcher *metrics.SystemMetrics, metricsStore *metrics.TSDB, hbs *heartbeat2.Heartbeat,
	ansibleManager *ansible.Manager, gracefulRebootChannel chan struct{}, deviceOs *os2.OS) {
	// listen to the channel for getting StartGracefulReboot signal
	for {
		<-gracefulRebootChannel
		log.Info("A graceful reboot request was received")
		if err := wl.StopWorkloads(); err != nil {
			log.Fatalf("cannot graceful reboot the workloads: %v", err)
		}

		if err := dataMonitor.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot dataMonitor: %v", err)
		}

		if err := systemMetricsWatcher.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot systemMetricsWatcher: %v", err)
		}

		if err := metricsStore.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot metricsStore: %v", err)
		}

		if err := hbs.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot the heartbeat service: %v", err)
		}
		if ansibleManager != nil {
			ansibleManager.WaitPlaybookCompletion()
		}

		deviceOs.GracefulRebootCompletionChannel <- struct{}{}
	}
}

func setupSignalHandler(metricsStore *metrics.TSDB, ansibleManager *ansible.Manager) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-c
		log.Infof("Got %s signal. Aborting...\n", sig)
		closeComponents(metricsStore, ansibleManager)
		os.Exit(0)
	}()
}

func closeComponents(metricsStore *metrics.TSDB, ansibleManager *ansible.Manager) {
	if err := metricsStore.Close(); err != nil {
		log.Error(err)
	}
	if ansibleManager != nil {
		ansibleManager.WaitPlaybookCompletion()
	}
}

//EXTENDED NEW CODE
const (
	mqttBroker   = "tcp://localhost:1883"
	mqttUsername = "flotta"
	mqttPassword = "flotta"
)

func mqttSqlite() {
	log.Infoln("MQTT HERE")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBroker)
	opts.SetClientID("mqtt_to_sqlite_client")
	opts.SetUsername(mqttUsername)
	opts.SetPassword(mqttPassword)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Errorf("Failed to connect to MQTT broker: %s", token.Error())
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
	log.Infof("Received message: %s\n", msg.Payload())

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
				Protocol:     getStringValue(deviceData, "protocol"),
				Connection:   getStringValue(deviceData, "connection"),
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
