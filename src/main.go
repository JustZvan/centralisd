package main

import (
	"centralisd/src/core/config"
	"centralisd/src/master"
	"centralisd/src/orchestrator/db"
	"centralisd/src/orchestrator/registry"
	"centralisd/src/orchestrator/tcp"
	"centralisd/src/orchestrator/web"
	"centralisd/src/slave"
	"log"
	"os"
	"time"
)

func sliceToSet(in []string) map[string]struct{} {
	if len(in) == 0 {
		return nil
	}
	out := map[string]struct{}{}
	for _, v := range in {
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

func whitelistClusters(wl map[string][]config.OrchestratorMasterWhitelistEntry) []string {
	out := make([]string, 0, len(wl))
	for clusterID := range wl {
		if clusterID == "" {
			continue
		}
		out = append(out, clusterID)
	}
	return out
}

func whitelistClusterMasters(wl map[string][]config.OrchestratorMasterWhitelistEntry) map[string]map[string]struct{} {
	if len(wl) == 0 {
		return nil
	}
	out := map[string]map[string]struct{}{}
	for clusterID, entries := range wl {
		if clusterID == "" {
			continue
		}
		set := map[string]struct{}{}
		for _, e := range entries {
			if e.ID == "" {
				continue
			}
			set[e.ID] = struct{}{}
		}
		if len(set) == 0 {
			continue
		}
		out[clusterID] = set
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		log.Printf("config_path is empty, using /etc/centralisd.yaml")

		configPath = "/etc/centralisd.yaml"
	}

	configuration, err := config.LoadConfig(configPath)

	if err != nil {
		log.Printf("failed to read config: %v; bailing", err)

		os.Exit(1)
	}

	switch configuration.NodeType {
	case config.Orchestrator:
		ttlSeconds := configuration.Orchestrator.StateTTLSeconds
		if ttlSeconds <= 0 {
			ttlSeconds = 60
		}
		dbConn, err := db.Open(configuration.Orchestrator.DBPath)
		if err != nil {
			log.Printf("orchestrator: db open: %v", err)
			os.Exit(1)
		}
		allowed := registry.NewAllowedClusters(dbConn)
		if err := allowed.AutoMigrate(); err != nil {
			log.Printf("orchestrator: db migrate: %v", err)
			os.Exit(1)
		}
		// Allowed clusters are derived from the whitelist map keys.
		_ = allowed.EnsureSeed(whitelistClusters(configuration.Orchestrator.MasterWhitelist))

		store := registry.NewStore(time.Duration(ttlSeconds)*time.Second, allowed)

		log.Printf("orchestrator: starting registry + web")
		go func() {
			wl := whitelistClusterMasters(configuration.Orchestrator.MasterWhitelist)
			if err := tcp.HostOrchestratorTCPServer(configuration.Orchestrator.TCPListen, store, wl); err != nil {
				log.Printf("orchestrator: tcp server: %v", err)
				os.Exit(1)
			}
		}()
		go web.ServeWeb(store, configuration.Orchestrator.WebListen)
	case config.Master:
		log.Printf("master: listening")
		nodesAllow := sliceToSet(configuration.Master.AllowedNodes)
		go master.HostMasterServer(49149, nodesAllow)
		go master.ConnectToOrchestrator(configuration)
	case config.Slave:
		log.Printf("slave: connecting to master addr=%s", configuration.Slave.Master)
		go slave.Connect(configuration.Slave.Master, configuration)
	}

	for {
		time.Sleep(time.Second * 10)
	}
}
