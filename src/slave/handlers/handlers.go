package handlers

import (
	"bufio"
	"centralisd/src/core/protocol"
	"encoding/json"
)

type packetHandler func(writer *bufio.Writer, packet protocol.Packet) error

type commandHandler func(cmd protocol.NodeCommand) (json.RawMessage, error)

var PacketHandlers = map[string]packetHandler{
	string(protocol.PacketHeartbeat):   handleHeartbeat,
	string(protocol.PacketNodeCommand): handleNodeCommand,
}

var commandHandlers = map[string]commandHandler{
	"docker.containers.list": handleDockerContainersList,
	"docker.images.list":     handleDockerImagesList,
	"libvirt.domains.list":   handleLibvirtDomainsList,
	"noop":                   handleNoop,
}
