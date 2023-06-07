package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
)

const defaultNetworkPath = "/var/run/minidocker/network/network/"

type Network struct {
	Name string
}

func CreateNetwork(subnet, driver, name string) error {
	_, cidr, _ := net.ParseCIDR(subnet)

	ip, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return fmt.Errorf("allocate ip failed: %v", err)
	}

	cidr.IP = ip

	/*
		nw, err := drivers[driver].Create(cidr.String(), name)
		if err != nil {
			return fmt.Errorf("create network failed: %v", err)
		}

		fmt.Println(nw)
	*/

	return nil
}

func (nw *Network) load(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(content, nw); err != nil {
		return err
	}

	return nil
}

func Init() error {
	var bridge = &Bridge{}
	drivers = map[string]driver{
		"bridge": bridge,
	}

	var networks = map[string]*Network{}

	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
		} else {
			return err
		}
	}

	filepath.Walk(defaultNetworkPath, func(nwPath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		_, nwName := path.Split(nwPath)
		nw := &Network{
			Name: nwName,
		}

		if err := nw.load(nwPath); err != nil {
			return err
		}

		networks[nwName] = nw
		return nil
	})

	return nil
}
