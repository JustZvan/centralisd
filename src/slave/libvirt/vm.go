package libvirt

import (
	"bytes"
	"centralisd/src/core/protocol"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	libvirtapi "libvirt.org/go/libvirt"
)

type domainXML struct {
	Devices domainDevicesXML `xml:"devices"`
}

type domainDevicesXML struct {
	Disks []domainDiskXML `xml:"disk"`
}

type domainDiskXML struct {
	Device string              `xml:"device,attr"`
	Type   string              `xml:"type,attr"`
	Source domainDiskSourceXML `xml:"source"`
	Target domainDiskTargetXML `xml:"target"`
}

type domainDiskSourceXML struct {
	File string `xml:"file,attr"`
}

type domainDiskTargetXML struct {
	Dev string `xml:"dev,attr"`
	Bus string `xml:"bus,attr"`
}

type storageVolumeXML struct {
	Target storageVolumeTargetXML `xml:"target"`
}

type storageVolumeTargetXML struct {
	Format storageVolumeFormatXML `xml:"format"`
}

type storageVolumeFormatXML struct {
	Type string `xml:"type,attr"`
}

func LookupDomainByName(conn *libvirtapi.Connect, name string) (*libvirtapi.Domain, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil libvirt connection")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	return conn.LookupDomainByName(name)
}

func DescribeDomain(d *libvirtapi.Domain) (protocol.VMDomain, []string, error) {
	if d == nil {
		return protocol.VMDomain{}, nil, fmt.Errorf("nil domain")
	}
	name, err := d.GetName()
	if err != nil {
		return protocol.VMDomain{}, nil, err
	}
	uuid, _ := d.GetUUIDString()
	id, _ := d.GetID()
	active, _ := d.IsActive()
	persistent, _ := d.IsPersistent()
	autostart, _ := d.GetAutostart()
	info, _ := d.GetInfo()
	xmlDesc, _ := d.GetXMLDesc(0)
	isoPath := ""
	diskPaths := []string{}
	if xmlDesc != "" {
		parsed := domainXML{}
		if err := xml.Unmarshal([]byte(xmlDesc), &parsed); err == nil {
			for _, disk := range parsed.Devices.Disks {
				if disk.Device == "cdrom" && disk.Source.File != "" {
					isoPath = disk.Source.File
				}
				if disk.Device == "disk" && disk.Type == "file" && disk.Source.File != "" {
					diskPaths = append(diskPaths, disk.Source.File)
				}
			}
		}
	}
	out := protocol.VMDomain{
		ID:         uint32(id),
		UUID:       uuid,
		Name:       name,
		Active:     active,
		Persistent: persistent,
		Autostart:  autostart,
		ISOPath:    isoPath,
	}
	if info != nil {
		out.State = DomainStateString(info.State)
		out.MaxMemMB = info.MaxMem / 1024
		out.MemoryMB = info.Memory / 1024
		out.VCPUs = info.NrVirtCpu
	}
	return out, diskPaths, nil
}

func DomainStateString(state libvirtapi.DomainState) string {
	switch state {
	case libvirtapi.DOMAIN_RUNNING:
		return "running"
	case libvirtapi.DOMAIN_BLOCKED:
		return "blocked"
	case libvirtapi.DOMAIN_PAUSED:
		return "paused"
	case libvirtapi.DOMAIN_SHUTDOWN:
		return "shutdown"
	case libvirtapi.DOMAIN_CRASHED:
		return "crashed"
	case libvirtapi.DOMAIN_PMSUSPENDED:
		return "suspended"
	case libvirtapi.DOMAIN_SHUTOFF:
		return "shutoff"
	default:
		return "unknown"
	}
}

func ListStoragePools(conn *libvirtapi.Connect) ([]protocol.LibvirtStoragePool, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil libvirt connection")
	}
	pools, err := conn.ListAllStoragePools(libvirtapi.CONNECT_LIST_STORAGE_POOLS_ACTIVE | libvirtapi.CONNECT_LIST_STORAGE_POOLS_INACTIVE)
	if err != nil {
		return nil, err
	}
	out := make([]protocol.LibvirtStoragePool, 0, len(pools))
	for _, pool := range pools {
		item, err := describeStoragePool(&pool)
		_ = pool.Free()
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func ListNetworks(conn *libvirtapi.Connect) ([]protocol.LibvirtNetwork, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil libvirt connection")
	}
	nets, err := conn.ListAllNetworks(libvirtapi.CONNECT_LIST_NETWORKS_ACTIVE | libvirtapi.CONNECT_LIST_NETWORKS_INACTIVE)
	if err != nil {
		return nil, err
	}
	out := make([]protocol.LibvirtNetwork, 0, len(nets))
	for _, net := range nets {
		name, _ := net.GetName()
		uuid, _ := net.GetUUIDString()
		active, _ := net.IsActive()
		persistent, _ := net.IsPersistent()
		autostart, _ := net.GetAutostart()
		out = append(out, protocol.LibvirtNetwork{
			Name:       name,
			UUID:       uuid,
			Active:     active,
			Persistent: persistent,
			Autostart:  autostart,
		})
		_ = net.Free()
	}
	return out, nil
}

func CreateDiskVolume(conn *libvirtapi.Connect, poolName, diskName string, diskSizeGB uint64) (*libvirtapi.StorageVol, string, error) {
	if conn == nil {
		return nil, "", fmt.Errorf("nil libvirt connection")
	}
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		poolName = "default"
	}
	diskName = strings.TrimSpace(diskName)
	if diskName == "" {
		return nil, "", fmt.Errorf("disk name is required")
	}
	if diskSizeGB == 0 {
		diskSizeGB = 20
	}
	if filepath.Ext(diskName) == "" {
		diskName += ".qcow2"
	}
	pool, err := conn.LookupStoragePoolByName(poolName)
	if err != nil {
		return nil, "", err
	}
	defer pool.Free()
	xmlConfig := fmt.Sprintf("<volume><name>%s</name><capacity unit='GiB'>%d</capacity><target><format type='qcow2'/></target></volume>", xmlEscape(diskName), diskSizeGB)
	vol, err := pool.StorageVolCreateXML(xmlConfig, 0)
	if err != nil {
		return nil, "", err
	}
	path, err := vol.GetPath()
	if err != nil {
		_ = vol.Free()
		return nil, "", err
	}
	return vol, path, nil
}

func BuildDomainXML(params protocol.LibvirtVMCreateParams, diskPath string) string {
	network := strings.TrimSpace(params.Network)
	if network == "" {
		network = "default"
	}
	if params.MemoryMB == 0 {
		params.MemoryMB = 2048
	}
	if params.VCPUs == 0 {
		params.VCPUs = 2
	}
	var b strings.Builder
	b.WriteString("<domain type='kvm'>")
	b.WriteString("<name>" + xmlEscape(strings.TrimSpace(params.Name)) + "</name>")
	b.WriteString(fmt.Sprintf("<memory unit='MiB'>%d</memory>", params.MemoryMB))
	b.WriteString(fmt.Sprintf("<currentMemory unit='MiB'>%d</currentMemory>", params.MemoryMB))
	b.WriteString(fmt.Sprintf("<vcpu placement='static'>%d</vcpu>", params.VCPUs))
	b.WriteString("<os><type arch='x86_64' machine='pc'>hvm</type><boot dev='hd'/>")
	if strings.TrimSpace(params.ISOPath) != "" {
		b.WriteString("<boot dev='cdrom'/>")
	}
	b.WriteString("</os>")
	b.WriteString("<features><acpi/><apic/><pae/></features>")
	b.WriteString("<clock offset='utc'/>")
	b.WriteString("<on_poweroff>destroy</on_poweroff><on_reboot>restart</on_reboot><on_crash>restart</on_crash>")
	b.WriteString("<devices>")
	b.WriteString("<disk type='file' device='disk'><driver name='qemu' type='qcow2'/><source file='" + xmlEscape(diskPath) + "'/><target dev='vda' bus='virtio'/></disk>")
	if strings.TrimSpace(params.ISOPath) != "" {
		b.WriteString("<disk type='file' device='cdrom'><driver name='qemu' type='raw'/><source file='" + xmlEscape(strings.TrimSpace(params.ISOPath)) + "'/><target dev='hdc' bus='ide'/><readonly/></disk>")
	}
	b.WriteString("<interface type='network'><source network='" + xmlEscape(network) + "'/><model type='virtio'/></interface>")
	b.WriteString("<graphics type='vnc' autoport='yes' listen='0.0.0.0'/>")
	b.WriteString("<serial type='pty'/><console type='pty'/><input type='tablet' bus='usb'/><video><model type='virtio'/></video>")
	b.WriteString("</devices></domain>")
	return b.String()
}

func BuildCDROMDeviceXML(isoPath string) string {
	var b strings.Builder
	b.WriteString("<disk type='file' device='cdrom'><driver name='qemu' type='raw'/>")
	if strings.TrimSpace(isoPath) != "" {
		b.WriteString("<source file='" + xmlEscape(strings.TrimSpace(isoPath)) + "'/>")
	}
	b.WriteString("<target dev='hdc' bus='ide'/><readonly/></disk>")
	return b.String()
}

func DeviceModifyFlags(active bool) libvirtapi.DomainDeviceModifyFlags {
	if active {
		return libvirtapi.DOMAIN_DEVICE_MODIFY_CONFIG | libvirtapi.DOMAIN_DEVICE_MODIFY_LIVE
	}
	return libvirtapi.DOMAIN_DEVICE_MODIFY_CONFIG
}

func DeleteDomainVolumes(conn *libvirtapi.Connect, diskPaths []string) error {
	if conn == nil {
		return fmt.Errorf("nil libvirt connection")
	}
	for _, path := range diskPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		vol, err := conn.LookupStorageVolByPath(path)
		if err != nil {
			continue
		}
		deleteErr := vol.Delete(libvirtapi.STORAGE_VOL_DELETE_NORMAL)
		_ = vol.Free()
		if deleteErr != nil {
			return deleteErr
		}
	}
	return nil
}

func describeStoragePool(pool *libvirtapi.StoragePool) (protocol.LibvirtStoragePool, error) {
	if pool == nil {
		return protocol.LibvirtStoragePool{}, fmt.Errorf("nil storage pool")
	}
	name, err := pool.GetName()
	if err != nil {
		return protocol.LibvirtStoragePool{}, err
	}
	uuid, _ := pool.GetUUIDString()
	active, _ := pool.IsActive()
	persistent, _ := pool.IsPersistent()
	autostart, _ := pool.GetAutostart()
	info, _ := pool.GetInfo()
	vols, err := pool.ListAllStorageVolumes(0)
	if err != nil {
		return protocol.LibvirtStoragePool{}, err
	}
	volumeItems := make([]protocol.LibvirtStorageVolume, 0, len(vols))
	for _, vol := range vols {
		item, err := describeStorageVolume(&vol)
		_ = vol.Free()
		if err != nil {
			return protocol.LibvirtStoragePool{}, err
		}
		volumeItems = append(volumeItems, item)
	}
	out := protocol.LibvirtStoragePool{
		Name:       name,
		UUID:       uuid,
		Active:     active,
		Persistent: persistent,
		Autostart:  autostart,
		Volumes:    volumeItems,
	}
	if info != nil {
		out.Capacity = info.Capacity
		out.Allocation = info.Allocation
		out.Available = info.Available
	}
	return out, nil
}

func describeStorageVolume(vol *libvirtapi.StorageVol) (protocol.LibvirtStorageVolume, error) {
	if vol == nil {
		return protocol.LibvirtStorageVolume{}, fmt.Errorf("nil storage volume")
	}
	name, err := vol.GetName()
	if err != nil {
		return protocol.LibvirtStorageVolume{}, err
	}
	key, _ := vol.GetKey()
	path, _ := vol.GetPath()
	info, _ := vol.GetInfo()
	format := ""
	xmlDesc, _ := vol.GetXMLDesc(0)
	if xmlDesc != "" {
		parsed := storageVolumeXML{}
		if err := xml.Unmarshal([]byte(xmlDesc), &parsed); err == nil {
			format = parsed.Target.Format.Type
		}
	}
	out := protocol.LibvirtStorageVolume{
		Name:   name,
		Key:    key,
		Path:   path,
		Format: format,
	}
	if info != nil {
		out.Capacity = info.Capacity
		out.Allocation = info.Allocation
	}
	return out, nil
}

func xmlEscape(raw string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(raw))
	return buf.String()
}
