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

type Config struct {
	NodeType NodeType        `yaml:"nodetype"`
	Slave    SlaveYamlObject `yaml:"slave"`
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
