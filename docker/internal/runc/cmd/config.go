package cmd

import (
	"docker/internal/runc/cgroups"
	"docker/internal/runc/cgroups/subsystem"
	"docker/internal/runc/container"
	"docker/internal/runc/network"
	"docker/internal/utils/id"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
)

type containerConfig struct {
	name           string
	volume         string
	network        string
	portmapping    string
	tty            bool
	commands       []string
	envs           []string
	resourceConfig *subsystem.ResourceConfig
	parent         *exec.Cmd
	writePipe      *os.File
}

func (config *containerConfig) exitContainer() error {
	if !config.tty {
		return nil
	}

	dirs := strings.Split(config.volume, ":")
	if len(dirs) == 2 {
		volumeContainerDir := dirs[1]
		volumeContainerMountPoint := container.Mergedir + volumeContainerDir
		if err := syscall.Unmount(volumeContainerMountPoint, 0); err != nil {
			log.Error(err)
			return err
		}
	}

	if err := syscall.Unmount(container.Mergedir, 0); err != nil {
		log.Error(err)
		return err
	}

	if err := os.RemoveAll(container.Mergedir); err != nil {
		log.Errorf("remove merge layer dir %v failed: %v", container.Mergedir, err)
	}

	if err := os.RemoveAll(container.Workdir); err != nil {
		log.Errorf("remove work layer dir %v failed: %v", container.Upperdir, err)
	}

	if err := os.RemoveAll(container.Upperdir); err != nil {
		log.Errorf("remove upper layer dir %v failed: %v", container.Upperdir, err)
	}

	if err := syscall.Unmount("/proc", 0); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func (config *containerConfig) setContainerName() {
	if config.name == "" {
		config.name = id.GenerateContainerId()
	}
}

func (config *containerConfig) startupParentProcess() error {
	parent, writePipe := container.NewParentProcess(config.tty, config.volume, config.name, config.envs)
	if err := parent.Start(); err != nil {
		return err
	}

	config.parent, config.writePipe = parent, writePipe
	return nil
}

func (config *containerConfig) recordContainerInfo() (*container.Container, error) {
	c := container.New(config.name, strconv.Itoa(config.parent.Process.Pid), strings.Join(config.commands, " "), container.RUNNING)
	if err := c.RecordContainerInfo(); err != nil {
		return nil, err
	}

	return c, nil
}

func (config *containerConfig) setContainerCgroup() {
	cgroupManager := cgroups.New("minidocker-cgroup")
	defer cgroupManager.Destroy()
	cgroupManager.Set(config.resourceConfig)
	cgroupManager.Apply(config.parent.Process.Pid)
}

func (config *containerConfig) setContainerNetwork() error {
	if config.network != "" {
		if err := network.Init(); err != nil {
			return err
		}

		if err := network.ConnectNetwork(config.name, config.network, config.portmapping, strconv.Itoa(config.parent.Process.Pid)); err != nil {
			return err
		}
	}

	return nil
}

func (config *containerConfig) updateContainerInfo(c *container.Container) {
	if config.tty {
		config.parent.Wait()
		c.UpdateContainerInfo(container.EXIT)
	}
}

func (config *containerConfig) runContainer() error {
	defer config.exitContainer()

	config.setContainerName()

	if err := config.startupParentProcess(); err != nil {
		return err
	}

	container, err := config.recordContainerInfo()
	if err != nil {
		return err
	}

	config.setContainerCgroup()

	if err := config.setContainerNetwork(); err != nil {
		return err
	}

	config.sendInitCommand()
	config.updateContainerInfo(container)

	return nil
}
