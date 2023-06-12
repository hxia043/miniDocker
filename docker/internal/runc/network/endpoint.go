package network

import (
	"net"

	"github.com/vishvananda/netlink"
)

type Endpoint struct {
	ID          string
	IP          net.IP
	Veth        netlink.Veth
	PortMapping string
}
