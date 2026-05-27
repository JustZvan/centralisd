package docker

import (
	"centralisd/src/core/protocol"
	"context"

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

func GetContainers() ([]protocol.DockerContainer, error) {
	ctx := context.Background()

	cli, err := GetClient()

	if err != nil {
		return nil, err
	}

	containers, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})

	if err != nil {
		return nil, err
	}

	items := make([]protocol.DockerContainer, 0, len(containers.Items))
	for _, c := range containers.Items {
		items = append(items, protocol.DockerContainer{
			ID:      c.ID,
			Image:   c.Image,
			Names:   c.Names,
			State:   string(c.State),
			Status:  c.Status,
			Created: c.Created,
		})
	}

	return items, nil
}

func StopContainer(id string) error {
	cli, err := GetClient()
	ctx := context.Background()

	if err != nil {
		return err
	}

	res, err := cli.ContainerStop(ctx, id, client.ContainerStopOptions{})

	if err != nil {
		return nil
	}

	return nil
}

func GetImages() error {
	cli, err := GetClient()
	ctx := context.Background()

	if err != nil {
		return err
	}

	images, err := cli.ImageList(ctx, client.ImageListOptions{})

	if err != nil {
		return err
	}

	return nil
}

func PullImage(img string) error {
	cli, err := GetClient()
	ctx := context.Background()

	if err != nil {
		return err
	}

	_, err = cli.ImagePull(ctx, img, client.ImagePullOptions{})

	if err != nil {
		return err
	}

	return nil
}
