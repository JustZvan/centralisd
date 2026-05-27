package handlers

import (
	"centralisd/src/core/protocol"
	"centralisd/src/slave/docker"
	"context"
	"encoding/json"

	"github.com/moby/moby/client"
)

func handleDockerContainersList(cmd protocol.NodeCommand) (json.RawMessage, error) {
	cli, err := docker.GetClient()
	ctx := context.Background()

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

	items_m, err := json.Marshal(items)

	return items_m, nil
}

func handleDockerImagesList(cmd protocol.NodeCommand) (json.RawMessage, error) {
	cli, err := docker.GetClient()
	ctx := context.Background()

	if err != nil {
		return nil, err
	}

	images, err := cli.ImageList(ctx, client.ImageListOptions{})

	if err != nil {
		return nil, err
	}

	items := make([]protocol.DockerImage, 0, len(images.Items))
	for _, image := range images.Items {
		items = append(items, protocol.DockerImage{
			ID:       image.ID,
			RepoTags: image.RepoTags,
			Size:     image.Size,
			Created:  image.Created,
			Labels:   image.Labels,
		})
	}

	items_m, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	return items_m, nil
}
