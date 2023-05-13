package subsystem

import (
	utils "docker/internal/utils/cgroup"
	"fmt"
	"os"
	"path"
	"strconv"
)

type CpuSet struct{}

func (cpuset *CpuSet) Set(cgroupPath string, resource *ResourceConfig) error {
	if cpusetCgroupPath, err := utils.GetCgroupPath(cpuset.Name(), cgroupPath, true); err == nil {
		if resource.CpuSet != "" {
			if err := os.WriteFile(path.Join(cpusetCgroupPath, "cpuset.cpus"), []byte(resource.CpuSet), 0644); err != nil {
				return fmt.Errorf("set cgroup cpuset fail %v", err)
			}
		}
		return nil
	} else {
		return err
	}
}

func (cpuset *CpuSet) Remove(cgroupPath string) error {
	if cpuCgroupPath, err := utils.GetCgroupPath(cpuset.Name(), cgroupPath, false); err == nil {
		return os.RemoveAll(cpuCgroupPath)
	} else {
		return err
	}
}

func (cpuset *CpuSet) Apply(cgroupPath string, pid int) error {
	if cpuCgroupPath, err := utils.GetCgroupPath(cpuset.Name(), cgroupPath, false); err == nil {
		if err := os.WriteFile(path.Join(cpuCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("set cgroup proc failed: %v", err)
		}
		return nil
	} else {
		return fmt.Errorf("get cgroup %s error: %v", cgroupPath, err)
	}
}

func (cpuset *CpuSet) Name() string {
	return "cpuset"
}
