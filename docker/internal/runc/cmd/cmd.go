package cmd

import (
	"docker/internal/runc/cgroups"
	"docker/internal/runc/cgroups/subsystem"
	"docker/internal/runc/container"
	"docker/internal/runc/network"
	"docker/internal/utils/id"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	_ "docker/internal/runc/nsenter"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func exitContainer(tty bool, volume string) error {
	if !tty {
		return nil
	}

	dirs := strings.Split(volume, ":")
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

func Run(tty bool, commands []string, res *subsystem.ResourceConfig, volume, name string, envs []string) {
	defer exitContainer(tty, volume)

	if name == "" {
		name = id.GenerateContainerId()
	}

	parent, writePipe := container.NewParentProcess(tty, volume, name, envs)
	if err := parent.Start(); err != nil {
		log.Error(err)
	}

	c := container.New(name, strconv.Itoa(parent.Process.Pid), strings.Join(commands, " "), container.RUNNING)
	if err := c.RecordContainerInfo(); err != nil {
		log.Error("record container info failed")
	}

	cgroupManager := cgroups.New("minidocker-cgroup")
	defer cgroupManager.Destroy()
	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)

	sendInitCommand(commands, writePipe)
	if tty {
		parent.Wait()
		c.UpdateContainerInfo(container.EXIT)
	}
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
	},
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container command")
		}

		var cmds []string
		for _, arg := range context.Args() {
			cmds = append(cmds, arg)
		}

		rc := &subsystem.ResourceConfig{
			MemoryLimit: context.String("m"),
			CpuSet:      context.String("cpuset"),
			CpuShare:    context.String("cpushare"),
		}

		volume := context.String("v")

		tty := context.Bool("it")
		detach := context.Bool("d")
		if tty && detach {
			return fmt.Errorf("it and d flag can not be provided both")
		}

		name := context.String("name")
		envs := context.StringSlice("e")
		Run(tty, cmds, rc, volume, name, envs)
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

func sendInitCommand(commands []string, writePipe *os.File) {
	command := strings.Join(commands, " ")
	log.Info("send command: ", command)

	writePipe.WriteString(command)
	writePipe.Close()
}
