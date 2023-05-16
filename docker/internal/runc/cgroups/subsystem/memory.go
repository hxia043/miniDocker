package subsystem

import (
	"docker/internal/utils/cgroup"
	"fmt"
	"os"
	"path"
	"strconv"
)

type Memory struct{}

func (memory *Memory) Set(cgroupPath string, resource *ResourceConfig) error {
	if memoryCgroupPath, err := cgroup.GetCgroupPath(memory.Name(), cgroupPath, true); err == nil {
		if resource.MemoryLimit != "" {
			if err := os.WriteFile(path.Join(memoryCgroupPath, "memory.limit_in_bytes"), []byte(resource.MemoryLimit), 0644); err != nil {
				return fmt.Errorf("set cgroup memory fail %v", err)
			}
		}
		return nil
	} else {
		return err
	}
}

func (memory *Memory) Remove(cgroupPath string) error {
	if cpuCgroupPath, err := cgroup.GetCgroupPath(memory.Name(), cgroupPath, false); err == nil {
		return os.RemoveAll(cpuCgroupPath)
	} else {
		return err
	}
}

func (memory *Memory) Apply(cgroupPath string, pid int) error {
	if cpuCgroupPath, err := cgroup.GetCgroupPath(memory.Name(), cgroupPath, false); err == nil {
		if err := os.WriteFile(path.Join(cpuCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("set cgroup proc failed: %v", err)
		}
		return nil
	} else {
		return fmt.Errorf("get cgroup %s error: %v", cgroupPath, err)
	}
}

func (memory *Memory) Name() string {
	return "memory"
}
