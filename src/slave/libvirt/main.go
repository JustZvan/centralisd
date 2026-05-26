package libvirt

import (
	"libvirt.org/go/libvirt"
)

var g_conn *libvirt.Connect = nil

func GetQEMU() (*libvirt.Connect, error) {
	if g_conn != nil {
		return g_conn, nil
	}

	conn, err := libvirt.NewConnect("qemu:///system")

	if err != nil {
		return nil, err
	}

	g_conn = conn

	return conn, nil
}

func GetDomains(conn *libvirt.Connect) ([]libvirt.Domain, error) {
	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)

	if err != nil {
		return nil, err
	}

	return domains, nil
}
