package handlers

import (
	"centralisd/src/core/protocol"
	"centralisd/src/slave/libvirt"
	"encoding/json"
	"fmt"
	"strings"

	libvirtapi "libvirt.org/go/libvirt"
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
		item, _, err := libvirt.DescribeDomain(&d)
		_ = d.Free()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return json.Marshal(items)
}

func handleLibvirtStoragePoolsList(cmd protocol.NodeCommand) (json.RawMessage, error) {
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	pools, err := libvirt.ListStoragePools(qemu)
	if err != nil {
		return nil, err
	}
	return json.Marshal(pools)
}

func handleLibvirtNetworksList(cmd protocol.NodeCommand) (json.RawMessage, error) {
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	networks, err := libvirt.ListNetworks(qemu)
	if err != nil {
		return nil, err
	}
	return json.Marshal(networks)
}

func handleLibvirtVMCreate(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.LibvirtVMCreateParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid vm create params")
	}
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return nil, fmt.Errorf("vm name is required")
	}
	if params.DiskName == "" {
		params.DiskName = params.Name
	}
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	vol, diskPath, err := libvirt.CreateDiskVolume(qemu, params.DiskPool, params.DiskName, params.DiskSizeGB)
	if err != nil {
		return nil, err
	}
	if vol != nil {
		defer vol.Free()
	}
	domainXML := libvirt.BuildDomainXML(params, diskPath)
	domain, err := qemu.DomainDefineXML(domainXML)
	if err != nil {
		return nil, err
	}
	defer domain.Free()
	if params.Autostart {
		if err := domain.SetAutostart(true); err != nil {
			return nil, err
		}
	}
	if params.Start {
		if err := domain.Create(); err != nil {
			return nil, err
		}
	}
	uuid, _ := domain.GetUUIDString()
	volumeName := ""
	if vol != nil {
		volumeName, _ = vol.GetName()
	}
	return json.Marshal(protocol.LibvirtVMCreateResult{
		Name:       params.Name,
		UUID:       uuid,
		DiskVolume: volumeName,
		DiskPath:   diskPath,
	})
}

func handleLibvirtVMStart(cmd protocol.NodeCommand) (json.RawMessage, error) {
	return handleDomainAction(cmd, func(d *libvirtapi.Domain) error { return d.Create() })
}

func handleLibvirtVMShutdown(cmd protocol.NodeCommand) (json.RawMessage, error) {
	return handleDomainAction(cmd, func(d *libvirtapi.Domain) error { return d.Shutdown() })
}

func handleLibvirtVMReboot(cmd protocol.NodeCommand) (json.RawMessage, error) {
	return handleDomainAction(cmd, func(d *libvirtapi.Domain) error { return d.Reboot(0) })
}

func handleLibvirtVMDestroy(cmd protocol.NodeCommand) (json.RawMessage, error) {
	return handleDomainAction(cmd, func(d *libvirtapi.Domain) error { return d.Destroy() })
}

func handleLibvirtVMSetAutostart(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.LibvirtVMSetAutostartParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid vm autostart params")
	}
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	domain, err := libvirt.LookupDomainByName(qemu, params.Name)
	if err != nil {
		return nil, err
	}
	defer domain.Free()
	if err := domain.SetAutostart(params.Enabled); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]bool{"enabled": params.Enabled})
}

func handleLibvirtVMAttachISO(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.LibvirtVMAttachISOParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid vm iso params")
	}
	params.Name = strings.TrimSpace(params.Name)
	params.ISOPath = strings.TrimSpace(params.ISOPath)
	if params.Name == "" || params.ISOPath == "" {
		return nil, fmt.Errorf("vm name and iso path are required")
	}
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	domain, err := libvirt.LookupDomainByName(qemu, params.Name)
	if err != nil {
		return nil, err
	}
	defer domain.Free()
	active, _ := domain.IsActive()
	if err := domain.UpdateDeviceFlags(libvirt.BuildCDROMDeviceXML(params.ISOPath), libvirt.DeviceModifyFlags(active)); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"iso_path": params.ISOPath})
}

func handleLibvirtVMDetachISO(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.LibvirtVMActionParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid vm detach iso params")
	}
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	domain, err := libvirt.LookupDomainByName(qemu, params.Name)
	if err != nil {
		return nil, err
	}
	defer domain.Free()
	active, _ := domain.IsActive()
	if err := domain.UpdateDeviceFlags(libvirt.BuildCDROMDeviceXML(""), libvirt.DeviceModifyFlags(active)); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}

func handleLibvirtVMDelete(cmd protocol.NodeCommand) (json.RawMessage, error) {
	params := protocol.LibvirtVMDeleteParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid vm delete params")
	}
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	domain, err := libvirt.LookupDomainByName(qemu, params.Name)
	if err != nil {
		return nil, err
	}
	defer domain.Free()
	_, diskPaths, err := libvirt.DescribeDomain(domain)
	if err != nil {
		return nil, err
	}
	active, _ := domain.IsActive()
	if active {
		if !params.ForceStop {
			return nil, fmt.Errorf("domain is running; force stop first")
		}
		if err := domain.Destroy(); err != nil {
			return nil, err
		}
	}
	flags := libvirtapi.DOMAIN_UNDEFINE_MANAGED_SAVE |
		libvirtapi.DOMAIN_UNDEFINE_SNAPSHOTS_METADATA |
		libvirtapi.DOMAIN_UNDEFINE_CHECKPOINTS_METADATA |
		libvirtapi.DOMAIN_UNDEFINE_NVRAM
	if err := domain.UndefineFlags(flags); err != nil {
		return nil, err
	}
	if params.RemoveVolumes {
		if err := libvirt.DeleteDomainVolumes(qemu, diskPaths); err != nil {
			return nil, err
		}
	}
	return json.Marshal(map[string]bool{"removed_volumes": params.RemoveVolumes})
}

func handleDomainAction(cmd protocol.NodeCommand, action func(*libvirtapi.Domain) error) (json.RawMessage, error) {
	params := protocol.LibvirtVMActionParams{}
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid vm action params")
	}
	qemu, err := libvirt.GetQEMU()
	if err != nil {
		return nil, err
	}
	domain, err := libvirt.LookupDomainByName(qemu, params.Name)
	if err != nil {
		return nil, err
	}
	defer domain.Free()
	if err := action(domain); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}
