package main

import (
	"centralisd/src/core/config"
	"centralisd/src/core/web"
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
	}

	switch configuration.NodeType {
	case config.Orchestrator:
		println("[+] We are Orchestrator, starting HTTP server!")
		go web.ServeWeb()
	case config.Master:
		println("[+] We are Master")
	case config.Slave:
		println("[+] We are Slave, connecting to Master")
	}

	for {
		time.Sleep(time.Second * 10)
	}
}
