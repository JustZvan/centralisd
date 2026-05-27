package handlers

import (
	"centralisd/src/core/protocol"
	"centralisd/src/slave/libvirt"
	"encoding/json"
)

func handleLibvirtDomainsList(cmd protocol.NodeCommand) (json.RawMessage, error) {
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}

	domains, err := libvirt.GetDomains(qemu)
	if err != nil {
		return nil, err
	}

	items := make([]protocol.VMDomain, 0, len(domains))
	for _, d := range domains {
		name, _ := d.GetName()
		uuid, _ := d.GetUUIDString()
		id, _ := d.GetID()
		active, _ := d.IsActive()
		items = append(items, protocol.VMDomain{ID: uint32(id), UUID: uuid, Name: name, Active: active})
		_ = d.Free()
	}

	return json.Marshal(items)
}
