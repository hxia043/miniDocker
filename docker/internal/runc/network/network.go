package network

import (
	"docker/internal/utils/cmdtable"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"

	"github.com/gosuri/uitable"
	"github.com/vishvananda/netlink"
)

const defaultNetworkPath = "/var/run/minidocker/network/network/"

type Network struct {
	Name    string
	IpRange *net.IPNet
	Driver  string
}

func ListNetwork() error {
	var networks = map[string]*Network{}

	filepath.Walk(defaultNetworkPath, func(networkPath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		_, networkName := path.Split(networkPath)
		network := &Network{
			Name: networkName,
		}

		if err := network.load(networkPath); err != nil {
			return err
		}

		networks[networkName] = network
		return nil
	})

	table := uitable.New()
	table.AddRow("NAME", "DRIVER", "NETWORK")
	for _, network := range networks {
		table.AddRow(network.Name, network.Driver, network.IpRange)
	}

	return cmdtable.EncodeTable(os.Stdout, table)
}

func DeleteNetwork(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("get interface failed: %v", err)
	}

	if err := netlink.LinkDel(iface); err != nil {
		return fmt.Errorf("delete interface failed: %v", err)
	}

	networkPath := path.Join(defaultNetworkPath, name)
	return os.Remove(networkPath)
}

func CreateNetwork(subnet, driver, name string) error {
	_, cidr, _ := net.ParseCIDR(subnet)

	ip, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return fmt.Errorf("allocate ip failed: %v", err)
	}

	cidr.IP = ip

	network, err := drivers[driver].Create(cidr.String(), name)
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}

	return network.dump(defaultNetworkPath)
}

func (network *Network) dump(dumpPath string) error {
	networkPath := path.Join(dumpPath, network.Name)
	file, _ := os.OpenFile(networkPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer file.Close()

	content, _ := json.MarshalIndent(network, "", "    ")
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("failed to write network: %v", err)
	}

	return nil
}

func (network *Network) load(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(content, network); err != nil {
		return err
	}

	return nil
}

func Init() error {
	var bridge = &Bridge{}
	drivers = map[string]driver{
		"bridge": bridge,
	}

	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
		} else {
			return err
		}
	}

	return nil
}
