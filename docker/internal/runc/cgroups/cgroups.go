package cgroups

import (
	"docker/internal/runc/cgroups/subsystem"
)

type CgroupManager struct {
	Path string
}

func (cm *CgroupManager) Destroy() {
	for _, subsystem := range subsystem.Subsystems {
		subsystem.Remove(cm.Path)
	}
}

func (cm *CgroupManager) Set(res *subsystem.ResourceConfig) {
	for _, subsystem := range subsystem.Subsystems {
		subsystem.Set(cm.Path, res)
	}
}

func (cm *CgroupManager) Apply(pid int) {
	for _, subsystem := range subsystem.Subsystems {
		subsystem.Apply(cm.Path, pid)
	}
}

func New(path string) *CgroupManager {
	return &CgroupManager{
		Path: path,
	}
}
