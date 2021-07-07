package main

import (
	"github.com/DGHeroin/logrus_influxdb"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	hook, err := logrus_influxdb.NewInfluxDB(nil)
	if err == nil {
		log.Hooks.Add(hook)
	}
}
