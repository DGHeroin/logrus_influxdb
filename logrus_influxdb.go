package logrus_influxdb

import (
    "context"
    "fmt"
    "github.com/influxdata/influxdb-client-go/v2/api/write"
    "os"
    "sync"
    "time"

    influxdb "github.com/influxdata/influxdb-client-go/v2"
    "github.com/sirupsen/logrus"
)

var (
    defaultAddress       = "localhost:8086"
    defaultBatchInterval = 5 * time.Second
    defaultMeasurement   = "logrus"
    defaultBatchCount    = 200
)

// InfluxDBHook delivers logs to an InfluxDB cluster.
type InfluxDBHook struct {
    sync.Mutex                       // TODO: we should clean up all of these locks
    client                           influxdb.Client
    precision, database, measurement string
    org, bucket                      string
    tagList                          []string
    lastBatchUpdate time.Time
    batchInterval   time.Duration
    batchCount      int
    syslog          bool
    facility        string
    facilityCode    int
    appName         string
    version         string
    minLevel        string
}

// NewInfluxDB returns a new InfluxDBHook.
func newInfluxDB(config *Config) (hook *InfluxDBHook, err error) {
    if config == nil {
        config = &Config{}
    }

    config.defaults()

    var client = newInfluxDBClient(config)

    // Make sure that we can connect to InfluxDB
    isReady, err := client.Ready(context.Background()) // if this takes more than 5 seconds then influxdb is probably down
    if err != nil || !isReady {
        return nil, fmt.Errorf("NewInfluxDB: Error connecting to InfluxDB, %v", err)
    }

    hook = &InfluxDBHook{
        client:        client,
        database:      config.Database,
        measurement:   config.Measurement,
        tagList:       config.Tags,
        batchInterval: config.BatchInterval,
        batchCount:    config.BatchCount,
        precision:     config.Precision,
        syslog:        config.Syslog,
        facility:      config.Facility,
        facilityCode:  config.FacilityCode,
        appName:       config.AppName,
        version:       config.Version,
        minLevel:      config.MinLevel,
        org:           config.Org,
        bucket:        config.Bucket,
    }

    err = hook.autocreateDatabase()
    if err != nil {
        return nil, err
    }

    return hook, nil
}

func parseSeverity(level string) (string, int) {
    switch level {
    case "info":
        return "info", 6
    case "error":
        return "err", 3
    case "debug":
        return "debug", 7
    case "panic":
        return "panic", 0
    case "fatal":
        return "crit", 2
    case "warning":
        return "warning", 4
    }

    return "none", -1
}

func stringInSlice(a string, list []string) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}

func (hook *InfluxDBHook) hasMinLevel(level string) bool {
    if len(hook.minLevel) > 0 {
        if hook.minLevel == "debug" {
            return true
        }

        if hook.minLevel == "info" {
            return stringInSlice(level, []string{"info", "warning", "error", "fatal", "panic"})
        }

        if hook.minLevel == "warning" {
            return stringInSlice(level, []string{"warning", "error", "fatal", "panic"})
        }

        if hook.minLevel == "error" {
            return stringInSlice(level, []string{"error", "fatal", "panic"})
        }

        if hook.minLevel == "fatal" {
            return stringInSlice(level, []string{"fatal", "panic"})
        }

        if hook.minLevel == "panic" {
            return level == "panic"
        }

        return false
    }

    return true
}

// Fire adds a new InfluxDB point based off of Logrus entry
func (hook *InfluxDBHook) Fire(entry *logrus.Entry) (err error) {

    if hook.hasMinLevel(entry.Level.String()) {
        measurement := hook.measurement
        if result, ok := getTag(entry.Data, "measurement"); ok {
            measurement = result
        }

        tags := make(map[string]string)
        data := make(map[string]interface{})

        if hook.syslog {
            hostname, err := os.Hostname()

            if err != nil {
                return err
            }

            severity, severityCode := parseSeverity(entry.Level.String())

            tags["appname"] = hook.appName
            tags["facility"] = hook.facility
            tags["host"] = hostname
            tags["hostname"] = hostname
            tags["severity"] = severity

            data["facility_code"] = hook.facilityCode
            data["message"] = entry.Message
            data["procid"] = os.Getpid()
            data["severity_code"] = severityCode
            data["timestamp"] = entry.Time.UnixNano()
            data["version"] = hook.version
        } else {
            // If passing a "message" field then it will be overridden by the entry Message
            entry.Data["message"] = entry.Message

            // Set the level of the entry
            tags["level"] = entry.Level.String()
            // getAndDel and getAndDelRequest are taken from https://github.com/evalphobia/logrus_sentry
            if logger, ok := getTag(entry.Data, "logger"); ok {
                tags["logger"] = logger
            }

            for k, v := range entry.Data {
                data[k] = v
            }

            for _, tag := range hook.tagList {
                if tagValue, ok := getTag(entry.Data, tag); ok {
                    tags[tag] = tagValue
                    delete(data, tag)
                }
            }
        }

        pt := write.NewPoint(measurement, tags, data, entry.Time)

        return hook.addPoint(pt)
    }

    return nil
}

func (hook *InfluxDBHook) addPoint(pt *write.Point) (err error) {
    hook.Lock()
    defer hook.Unlock()

    client := hook.client
    writeAPI := client.WriteAPI(hook.org, hook.bucket)

    writeAPI.WritePoint(pt)

    writeAPI.Flush()
    hook.lastBatchUpdate = time.Now().UTC()

    // Return the write error (if any).
    return err
}

/* BEGIN BACKWARDS COMPATIBILITY */

// NewInfluxDBHook /* DO NOT USE */ creates a hook to be added to an instance of logger and initializes the InfluxDB client
func NewInfluxDBHook(config *Config, batching ...bool) (hook *InfluxDBHook, err error) {
    if len(batching) == 1 && batching[0] {
        config.BatchCount = 10
    }
    return newInfluxDB(config)
}
