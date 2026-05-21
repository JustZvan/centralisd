package main

import (
	"centralisd/src/core/config"
	"centralisd/src/master"
	"centralisd/src/orchestrator/web"
	"centralisd/src/slave"
	"os"
	"time"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		println("[-] CONFIG_PATH is empty, using /etc/centralisd.yaml")

		configPath = "/etc/centralisd.yaml"
	}

	configuration, err := config.LoadConfig(configPath)

	if err != nil {
		println("[-] Failed to read config, bailing!")
		println(err.Error())

		os.Exit(1)
	}

	switch configuration.NodeType {
	case config.Orchestrator:
		println("[+] we're orchestrator, starting HTTP server!")
		go web.ServeWeb()
	case config.Master:
		println("[+] we're master, listening!")
		go master.HostMasterServer(49149)
	case config.Slave:
		println("[+] We are slave, connecting to Master")
		go slave.Connect(configuration.Slave.Master, configuration)
	}

	for {
		time.Sleep(time.Second * 10)
	}
}
