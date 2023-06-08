package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
)

type Bridge struct{}

func (b *Bridge) String() string {
	return "bridge"
}

func createBridgeInterface(name string) error {
	if _, err := net.InterfaceByName(name); err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	nla := netlink.NewLinkAttrs()
	nla.Name = name

	bridge := &netlink.Bridge{LinkAttrs: nla}
	if err := netlink.LinkAdd(bridge); err != nil {
		return fmt.Errorf("Create bridge failed: %v", err)
	}

	return nil
}

func setBridgeInterfaceIP(name, rawIP string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("get interface failed: %v", err)
	}

	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return fmt.Errorf("parse ip net failed: %v", err)
	}

	addr := &netlink.Addr{
		IPNet: ipNet,
		Label: "",
		Flags: 0,
		Scope: 0,
	}

	return netlink.AddrAdd(iface, addr)
}

func setBridgeInterfaceUp(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find link failed: %v", err)
	}

	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("set link up failed: %v", err)
	}

	return nil
}

func setupIpTables(bridgeName string, subnet *net.IPNet) error {
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	if output, err := cmd.Output(); err != nil {
		return fmt.Errorf("iptables output: %v", output)
	}

	return nil
}

func (b *Bridge) initBridge(network *Network) error {
	bridgeName := network.Name
	if err := createBridgeInterface(bridgeName); err != nil {
		return err
	}

	gatewayIP := network.IpRange
	gatewayIP.IP = network.IpRange.IP
	if err := setBridgeInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return err
	}

	if err := setBridgeInterfaceUp(bridgeName); err != nil {
		return err
	}

	if err := setupIpTables(bridgeName, network.IpRange); err != nil {
		return err
	}

	return nil
}

func (b *Bridge) Create(subnet, name string) (*Network, error) {
	ip, ipRange, _ := net.ParseCIDR(subnet)
	ipRange.IP = ip

	network := &Network{
		Name:    name,
		IpRange: ipRange,
		Driver:  b.String(),
	}

	if err := b.initBridge(network); err != nil {
		return nil, err
	}

	return network, nil
}
