package docker

import (
	"github.com/moby/moby/client"
)

var g_client *client.Client = nil

func GetClient() (*client.Client, error) {
	if g_client != nil {
		return g_client, nil
	}

	client, err := client.New()

	if err != nil {
		return nil, err
	}

	g_client = client

	return client, nil
}
