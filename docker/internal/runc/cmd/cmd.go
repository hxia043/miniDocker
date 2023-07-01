package cmd

import (
	"docker/internal/runc/cgroups"
	"docker/internal/runc/cgroups/subsystem"
	"docker/internal/runc/container"
	"docker/internal/runc/network"
	"docker/internal/utils/id"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	_ "docker/internal/runc/nsenter"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
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

func parseCmdsFrom(context *cli.Context) []string {
	var cmds []string
	for _, arg := range context.Args() {
		cmds = append(cmds, arg)
	}

	return cmds
}

func ttyEnable(context *cli.Context) bool {
	tty := context.Bool("it")
	detach := context.Bool("d")
	if tty && detach {
		return true
	}

	return false
}

func parseContainerConfig(context *cli.Context) *containerConfig {
	return &containerConfig{
		name:           context.String("name"),
		volume:         context.String("v"),
		network:        context.String("net"),
		portmapping:    context.String("p"),
		tty:            context.Bool("it"),
		commands:       parseCmdsFrom(context),
		envs:           context.StringSlice("e"),
		resourceConfig: subsystem.NewResourceConfig(context.String("m"), context.String("cpuset"), context.String("cpushare")),
	}
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

	c, err := config.recordContainerInfo()
	if err != nil {
		return err
	}

	config.setContainerCgroup()

	if err := config.setContainerNetwork(); err != nil {
		return err
	}

	config.sendInitCommand()
	config.updateContainerInfo(c)

	return nil
}

var RunCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit
			minidocker run -it [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "it",
			Usage: "enable tty",
		},
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach",
		},
		cli.StringFlag{
			Name:  "m",
			Usage: "memory limit",
		},
		cli.StringFlag{
			Name:  "cpushare",
			Usage: "cpushare limit",
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit",
		},
		cli.StringFlag{
			Name:  "v",
			Usage: "volume",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "set env",
		},
		cli.StringFlag{
			Name:  "net",
			Usage: "container network",
		},
		cli.StringFlag{
			Name:  "p",
			Usage: "container port mapping",
		},
	},
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("minidocker: failed to get container command [%v]", context.Args())
		}

		if ttyEnable(context) {
			return fmt.Errorf("minidocker: failed to enable tty")
		}

		if err := parseContainerConfig(context).runContainer(); err != nil {
			return fmt.Errorf("minidocker: failed to run container")
		}

		return nil
	},
}

var CommitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit container to image with tar",
	Action: func(context *cli.Context) error {
		if len(context.Args()) != 1 {
			log.Errorf("wrong args for commit")
		}

		log.Infof("commit container into tar image")
		imageName := context.Args().Get(0)
		return container.RunContainerCommit(imageName)
	},
}

var StopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
	},
	Action: func(context *cli.Context) error {
		if len(context.Args()) != 0 {
			log.Errorf("wrong args for stop container")
		}

		log.Infof("stop container")
		name := context.String("name")
		return container.RunContainerStop(name)
	},
}

var RemoveCommand = cli.Command{
	Name:  "remove",
	Usage: "remove container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
	},
	Action: func(context *cli.Context) error {
		if len(context.Args()) != 0 {
			log.Errorf("wrong args for remove container")
		}

		log.Infof("remove container")
		name := context.String("name")
		return container.RunContainerRemove(name)
	},
}

var LogCommand = cli.Command{
	Name:  "log",
	Usage: "container log",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
	},
	Action: func(context *cli.Context) error {
		log.Infof("collect container log")
		name := context.String("name")
		return container.RunContainerLog(name)
	},
}

var ListCommand = cli.Command{
	Name:  "ps",
	Usage: "list container",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "a",
			Usage: "list all container",
		},
	},
	Action: func(context *cli.Context) error {
		if len(context.Args()) != 0 {
			log.Errorf("wrong args for ps")
		}

		log.Infof("list container")
		return container.RunContainerList(context.Bool("a"))
	},
}

var ExecCommand = cli.Command{
	Name:  "exec",
	Usage: "enter the contianer",
	Action: func(context *cli.Context) error {
		if pid := os.Getenv(container.ENV_EXEC_PID); pid != "" {
			log.Info("callback to pid: ", pid)
			return nil
		}

		if len(context.Args()) != 2 {
			return fmt.Errorf("missing container name or command")
		}

		containerName := context.Args().Get(0)
		var commands []string
		commands = append(commands, context.Args().Tail()...)

		return container.RunContainerExec(containerName, commands)
	},
}

var NetworkCommand = cli.Command{
	Name:  "network",
	Usage: "container network commands",
	Subcommands: []cli.Command{
		{
			Name:  "create",
			Usage: "create container network",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver",
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet",
					Usage: "container subnet",
				},
			},
			Action: func(context *cli.Context) error {
				if len(context.Args()) != 1 {
					return fmt.Errorf("wrong args %v for network create", context.Args())
				}

				subnet := context.String("subnet")
				driver := context.String("driver")
				name := context.Args().Get(0)

				if err := network.Init(); err != nil {
					return err
				}

				return network.CreateNetwork(subnet, driver, name)
			},
		},
		{
			Name:  "delete",
			Usage: "delete container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) != 1 {
					return fmt.Errorf("only container network name is needed for remove")
				}

				name := context.Args().Get(0)
				return network.DeleteNetwork(name)
			},
		},
		{
			Name:  "list",
			Usage: "list container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) != 0 {
					return fmt.Errorf("no args needed for container list")
				}

				return network.ListNetwork()
			},
		},
	},
}

var InitCommand = cli.Command{
	Name:  "init",
	Usage: `init container process`,
	Action: func(context *cli.Context) error {
		log.Infof("init container process")
		return container.RunContainerInitProcess()
	},
}

func (config *containerConfig) sendInitCommand() {
	command := strings.Join(config.commands, " ")
	log.Info("send command: ", command)

	config.writePipe.WriteString(command)
	config.writePipe.Close()
}
