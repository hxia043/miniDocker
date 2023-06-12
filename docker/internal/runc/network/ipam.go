package network

import (
	"bufio"
	upath "docker/internal/utils/path"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
)

const ipamDefaultAllocatePath = "/var/run/minidocker/network/ipam/subnet.json"

type IPAM struct {
	SubnetAllocatePath string
	Subnets            map[string]string
}

var ipAllocator = &IPAM{
	SubnetAllocatePath: ipamDefaultAllocatePath,
}

func (ipam *IPAM) load() error {
	exist, err := upath.PathExist(ipam.SubnetAllocatePath)
	if err != nil {
		return fmt.Errorf("find subnet allocate path failed: %v", err)
	}
	if !exist {
		return nil
	}

	content, _ := os.ReadFile(ipam.SubnetAllocatePath)
	json.Unmarshal(content, &ipam.Subnets)

	return nil
}

func (ipam *IPAM) dump() error {
	dir, _ := path.Split(ipam.SubnetAllocatePath)
	if exist, err := upath.PathExist(dir); err != nil {
		return err
	} else {
		if !exist {
			if err := os.MkdirAll(dir, 0644); err != nil {
				return err
			}
		}
	}

	file, _ := os.OpenFile(ipam.SubnetAllocatePath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer file.Close()

	subnets, _ := json.MarshalIndent(ipam.Subnets, "", "    ")

	w := bufio.NewWriter(file)
	if _, err := w.WriteString(string(subnets)); err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return err
	}

	return nil
}

func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	ipam.Subnets = map[string]string{}

	if err = ipam.load(); err != nil {
		return
	}

	_, subnet, _ = net.ParseCIDR(subnet.String())

	one, size := subnet.Mask.Size()
	if _, exist := ipam.Subnets[subnet.String()]; !exist {
		ipam.Subnets[subnet.String()] = strings.Repeat("0", 1<<uint8(size-one))
	}

	for bit := range ipam.Subnets[subnet.String()] {
		if ipam.Subnets[subnet.String()][bit] == '0' {
			ipmap := []byte(ipam.Subnets[subnet.String()])
			ipmap[bit] = '1'
			ipam.Subnets[subnet.String()] = string(ipmap)

			ip = subnet.IP
			for t := uint(4); t > 0; t -= 1 {
				[]byte(ip)[4-t] += uint8(bit >> ((t - 1) * 8))
			}

			ip[3] += 1
			break
		}
	}

	err = ipam.dump()
	return
}
