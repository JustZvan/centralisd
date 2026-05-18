package config

import (
	"os"

	"github.com/goccy/go-yaml"
)

type NodeType string

const (
	Master       NodeType = "master"
	Orchestrator NodeType = "orchestrator"
	Slave        NodeType = "slave"
)

type Config struct {
	NodeType NodeType
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
