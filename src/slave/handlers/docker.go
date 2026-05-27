package handlers

import (
	"centralisd/src/core/protocol"
	"centralisd/src/slave/docker"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
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

func handleDockerImagePull(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.DockerImagePullParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, errors.New("invalid image pull params")
	}
	image := strings.TrimSpace(params.Image)
	if image == "" {
		return nil, errors.New("image is required")
	}
	cli, err := docker.GetClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	resp, err := cli.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Close()
	if err := resp.Wait(ctx); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}

func handleDockerImageRemove(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.DockerImageRemoveParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, errors.New("invalid image remove params")
	}
	image := strings.TrimSpace(params.Image)
	if image == "" {
		return nil, errors.New("image is required")
	}
	cli, err := docker.GetClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	res, err := cli.ImageRemove(ctx, image, client.ImageRemoveOptions{Force: params.Force, PruneChildren: params.PruneChildren})
	if err != nil {
		return nil, err
	}
	result := protocol.DockerImageRemoveResult{}
	for _, item := range res.Items {
		if item.Deleted != "" {
			result.Deleted = append(result.Deleted, item.Deleted)
		}
		if item.Untagged != "" {
			result.Untagged = append(result.Untagged, item.Untagged)
		}
	}
	return json.Marshal(result)
}

func handleDockerContainerCreate(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.DockerContainerCreateParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, errors.New("invalid container create params")
	}
	image := strings.TrimSpace(params.Image)
	if image == "" {
		return nil, errors.New("image is required")
	}
	cli, err := docker.GetClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	config := &container.Config{Image: image}
	if strings.TrimSpace(params.Command) != "" {
		config.Cmd = strings.Fields(params.Command)
	}
	if strings.TrimSpace(params.Entrypoint) != "" {
		config.Entrypoint = strings.Fields(params.Entrypoint)
	}
	if strings.TrimSpace(params.WorkingDir) != "" {
		config.WorkingDir = strings.TrimSpace(params.WorkingDir)
	}
	if len(params.Env) > 0 {
		config.Env = params.Env
	}

	if len(params.Ports) > 0 {
		config.ExposedPorts = network.PortSet{}
		for _, raw := range params.Ports {
			p, err := network.ParsePort(strings.TrimSpace(raw))
			if err != nil {
				return nil, errors.New("invalid port: " + raw)
			}
			config.ExposedPorts[p] = struct{}{}
		}
	}

	hostConfig := &container.HostConfig{
		PublishAllPorts: params.PublishAll,
		AutoRemove:      params.AutoRemove,
	}
	if params.RestartPolicy != "" {
		policy := container.RestartPolicy{Name: container.RestartPolicyMode(strings.TrimSpace(params.RestartPolicy))}
		if err := container.ValidateRestartPolicy(policy); err != nil {
			return nil, err
		}
		hostConfig.RestartPolicy = policy
	}

	res, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     config,
		HostConfig: hostConfig,
		Name:       strings.TrimSpace(params.Name),
	})
	if err != nil {
		return nil, err
	}
	if params.Start {
		if _, err := cli.ContainerStart(ctx, res.ID, client.ContainerStartOptions{}); err != nil {
			return nil, err
		}
	}
	return json.Marshal(protocol.DockerContainerCreateResult{ID: res.ID, Warnings: res.Warnings})
}

func handleDockerContainerStart(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.DockerContainerStartParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, errors.New("invalid container start params")
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, errors.New("id is required")
	}
	cli, err := docker.GetClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if _, err := cli.ContainerStart(ctx, params.ID, client.ContainerStartOptions{}); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}

func handleDockerContainerStop(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.DockerContainerStopParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, errors.New("invalid container stop params")
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, errors.New("id is required")
	}
	cli, err := docker.GetClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	options := client.ContainerStopOptions{}
	if params.TimeoutSeconds != nil {
		options.Timeout = params.TimeoutSeconds
	}
	if _, err := cli.ContainerStop(ctx, params.ID, options); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}

func handleDockerContainerRestart(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.DockerContainerRestartParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, errors.New("invalid container restart params")
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, errors.New("id is required")
	}
	cli, err := docker.GetClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	options := client.ContainerRestartOptions{}
	if params.TimeoutSeconds != nil {
		options.Timeout = params.TimeoutSeconds
	}
	if _, err := cli.ContainerRestart(ctx, params.ID, options); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}

func handleDockerContainerRemove(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.DockerContainerRemoveParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, errors.New("invalid container remove params")
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, errors.New("id is required")
	}
	cli, err := docker.GetClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	_, err = cli.ContainerRemove(ctx, params.ID, client.ContainerRemoveOptions{Force: params.Force, RemoveVolumes: params.RemoveVolumes})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}
