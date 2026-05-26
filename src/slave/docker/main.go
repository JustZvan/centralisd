package docker

import (
	"context"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"slices"
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

func GetContainers() (*client.ContainerListResult, error) {
	ctx := context.Background()

	cli, err := GetClient()

	if err != nil {
		return nil, err
	}

	containers, err := cli.ContainerList(ctx, client.ContainerListOptions{})

	if err != nil {
		return nil, err
	}

	return &containers, nil
}
