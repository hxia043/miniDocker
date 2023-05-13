package subsystem

import (
	utils "docker/internal/utils/cgroup"
	"fmt"
	"os"
	"path"
	"strconv"
)

type Cpu struct{}

func (cpu *Cpu) Set(cgroupPath string, resource *ResourceConfig) error {
	if cpuCgroupPath, err := utils.GetCgroupPath(cpu.Name(), cgroupPath, true); err == nil {
		if resource.CpuShare != "" {
			if err := os.WriteFile(path.Join(cpuCgroupPath, "cpu.shares"), []byte(resource.CpuShare), 0644); err != nil {
				return fmt.Errorf("set cgroup cpu share fail %v", err)
			}
		}
		return nil
	} else {
		return err
	}
}

func (cpu *Cpu) Remove(cgroupPath string) error {
	if cpuCgroupPath, err := utils.GetCgroupPath(cpu.Name(), cgroupPath, false); err == nil {
		return os.RemoveAll(cpuCgroupPath)
	} else {
		return err
	}
}

func (cpu *Cpu) Apply(cgroupPath string, pid int) error {
	if cpuCgroupPath, err := utils.GetCgroupPath(cpu.Name(), cgroupPath, false); err == nil {
		if err := os.WriteFile(path.Join(cpuCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("set cgroup proc failed: %v", err)
		}
		return nil
	} else {
		return fmt.Errorf("get cgroup %s error: %v", cgroupPath, err)
	}
}

func (cpu *Cpu) Name() string {
	return "cpu"
}
