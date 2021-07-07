package logrus_influxdb

import (
    "context"
    "fmt"
    influxdb "github.com/influxdata/influxdb-client-go/v2"
    "github.com/influxdata/influxdb-client-go/v2/api"
)

// Returns an influxdb client
func  newInfluxDBClient(config *Config) influxdb.Client {
    protocol := "http"
    if config.UseHTTPS {
        protocol = "https"
    }
    addr := fmt.Sprintf("%s://%s:%d", protocol, config.Host, config.Port)

    return influxdb.NewClientWithOptions(addr, config.Token,
        influxdb.DefaultOptions().SetBatchSize(20))
}


// queryDB convenience function to query the database
func (hook *InfluxDBHook) queryDB(cmd string) (*api.QueryTableResult, error) {
    api := hook.client.QueryAPI(hook.org)
    return api.Query(context.Background(), cmd)

}

// Return back an error if the database does not exist in InfluxDB
func (hook *InfluxDBHook) databaseExists() (err error) {
    return nil
}

// Try to detect if the database exists and if not, automatically create one.
func (hook *InfluxDBHook) autocreateDatabase() (err error) {
    err = hook.databaseExists()
    if err == nil {
        return nil
    }
    _, err = hook.queryDB(fmt.Sprintf("CREATE DATABASE %s", hook.database))
    if err != nil {
        return err
    }
    return nil
}
