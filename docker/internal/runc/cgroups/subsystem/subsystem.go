package subsystem

type ResourceConfig struct {
	MemoryLimit string
	CpuSet      string
	CpuShare    string
}

type subsystem interface {
	Set(cgroupPath string, resource *ResourceConfig) error
	Remove(cgroupPath string) error
	Apply(cgoupPath string, pid int) error
}

var Subsystems = []subsystem{
	&Cpu{},
	&CpuSet{},
	&Memory{},
}

func NewResourceConfig(memory, cpuset, cpushare string) *ResourceConfig {
	return &ResourceConfig{
		MemoryLimit: memory,
		CpuSet:      cpuset,
		CpuShare:    cpushare,
	}
}
