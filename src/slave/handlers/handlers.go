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
	"docker.containers.list":     handleDockerContainersList,
	"docker.images.list":         handleDockerImagesList,
	"docker.image.pull":          handleDockerImagePull,
	"docker.image.remove":        handleDockerImageRemove,
	"docker.container.create":    handleDockerContainerCreate,
	"docker.container.start":     handleDockerContainerStart,
	"docker.container.stop":      handleDockerContainerStop,
	"docker.container.restart":   handleDockerContainerRestart,
	"docker.container.remove":    handleDockerContainerRemove,
	"libvirt.domains.list":       handleLibvirtDomainsList,
	"libvirt.storage.pools.list": handleLibvirtStoragePoolsList,
	"libvirt.networks.list":      handleLibvirtNetworksList,
	"libvirt.domain.create":      handleLibvirtVMCreate,
	"libvirt.domain.start":       handleLibvirtVMStart,
	"libvirt.domain.shutdown":    handleLibvirtVMShutdown,
	"libvirt.domain.reboot":      handleLibvirtVMReboot,
	"libvirt.domain.destroy":     handleLibvirtVMDestroy,
	"libvirt.domain.delete":      handleLibvirtVMDelete,
	"libvirt.domain.autostart":   handleLibvirtVMSetAutostart,
	"libvirt.domain.iso.attach":  handleLibvirtVMAttachISO,
	"libvirt.domain.iso.detach":  handleLibvirtVMDetachISO,
	"noop":                       handleNoop,
}
