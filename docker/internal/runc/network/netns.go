package network

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func setInterfaceIP(name, ip string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("get interface failed: %v", err)
	}

	ipNet, err := netlink.ParseIPNet(ip)
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

func setInterfaceUP(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find link failed: %v", err)
	}

	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("set link up failed: %v", err)
	}

	return nil
}

func enterContainerNetns(link *netlink.Link, endpoint *Endpoint, pid string) func() {
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", pid), os.O_RDONLY, 0)
	if err != nil {
		logrus.Errorf("error get container net namespace, %v", err)
	}

	nsFD := f.Fd()

	runtime.LockOSThread()
	if err := netlink.LinkSetNsFd(*link, int(nsFD)); err != nil {
		logrus.Errorf("error set link setns, %v", err)
	}

	origns, err := netns.Get()
	if err != nil {
		logrus.Errorf("error get current netns, %v", err)
	}

	if err := netns.Set(netns.NsHandle(nsFD)); err != nil {
		logrus.Errorf("error set netns, %v", err)
	}

	return func() {
		netns.Set(origns)
		origns.Close()
		runtime.UnlockOSThread()
		f.Close()
	}
}

func setEndpointIpAddressAndRoute(network *Network, endpoint *Endpoint, pid string) error {
	peerLink, err := netlink.LinkByName(endpoint.Veth.PeerName)
	if err != nil {
		return err
	}

	defer enterContainerNetns(&peerLink, endpoint, pid)()

	interfaceIP := network.IpRange
	interfaceIP.IP = endpoint.IP

	if err := setInterfaceIP(endpoint.Veth.PeerName, interfaceIP.String()); err != nil {
		return err
	}

	if err := setInterfaceUP(endpoint.Veth.PeerName); err != nil {
		return err
	}

	if err := setInterfaceUP("lo"); err != nil {
		return err
	}

	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")
	defaultRoute := netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        network.IpRange.IP,
		Dst:       cidr,
	}

	if err := netlink.RouteAdd(&defaultRoute); err != nil {
		return err
	}

	return nil
}
