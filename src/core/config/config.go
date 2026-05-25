package config

import (
	"os"

	"github.com/goccy/go-yaml"
)

var VERSION = "0.0.0"

type NodeType string

const (
	Master       NodeType = "master"
	Orchestrator NodeType = "orchestrator"
	Slave        NodeType = "slave"
)

type SlaveYamlObject struct {
	Master      string `yaml:"master"`
	PrivKeyPath string `yaml:"privkeypath"`
	PubKeyPath  string `yaml:"pubkeypath"`
}

type MasterYamlObject struct {
	// Orchestrator is the host:port of the orchestrator TCP registry endpoint.
	// When empty, the master won’t attempt to connect.
	Orchestrator string `yaml:"orchestrator"`
	Cluster      string `yaml:"cluster"`
	Name         string `yaml:"name"`
	// Advertise is an address other components should use to reach this master.
	// Example: 10.0.0.10:49149
	Advertise string `yaml:"advertise"`
	PrivKeyPath string `yaml:"privkeypath"`
	PubKeyPath  string `yaml:"pubkeypath"`
	// AllowedNodes is a list of slave/node IDs allowed to connect to this master.
	// The ID format is base64url(raw(sha256(pubkey))).
	AllowedNodes []string `yaml:"allowednodes"`
}

type OrchestratorYamlObject struct {
	WebListen string `yaml:"weblisten"`
	TCPListen string `yaml:"tcplisten"`
	DBPath    string `yaml:"dbpath"`

	// MasterWhitelist maps clusterID -> list of allowed masters for that cluster.
	// The ID format is base64url(raw(sha256(pubkey))).
	MasterWhitelist map[string][]OrchestratorMasterWhitelistEntry `yaml:"masterwhitelist"`
	// StateTTLSeconds controls how long masters stay visible without updates.
	StateTTLSeconds int `yaml:"statettlseconds"`
}

type OrchestratorMasterWhitelistEntry struct {
	ID string `yaml:"id"`
}

type Config struct {
	NodeType      NodeType              `yaml:"nodetype"`
	Slave         SlaveYamlObject       `yaml:"slave"`
	Master        MasterYamlObject      `yaml:"master"`
	Orchestrator  OrchestratorYamlObject `yaml:"orchestrator"`
}

func LoadConfig(path string) (Config, error) {
	dat, err := os.ReadFile(path)

	if err != nil {
		return Config{}, err
	}

	config := Config{}

	err = yaml.Unmarshal(dat, &config)

	if err != nil {
		return Config{}, err
	}

	return config, nil
}
