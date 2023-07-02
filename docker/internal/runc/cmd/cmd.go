package cmd

import (
	"docker/internal/runc/cgroups/subsystem"
	"docker/internal/runc/container"
	"docker/internal/runc/network"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "docker/internal/runc/nsenter"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

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
			return errors.New("minidocker: wrong args for commit")
		}

		log.Infof("minidocker: commit container into tar image")
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
			return errors.New("minidocker: wrong args for stop container")
		}

		log.Infof("minidocker: stop container")
		return container.RunContainerStop(context.String("name"))
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
			return errors.New("minidocker: wrong args for remove container")
		}

		log.Infof("minidocker: remove container")
		return container.RunContainerRemove(context.String("name"))
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
		log.Infof("minidocker: collect container log")
		return container.RunContainerLog(context.String("name"))
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
			return errors.New("minidocker: wrong args for ps")
		}

		log.Infof("minidocker: list container")
		return container.RunContainerList(context.Bool("a"))
	},
}

var ExecCommand = cli.Command{
	Name:  "exec",
	Usage: "enter the contianer",
	Action: func(context *cli.Context) error {
		if pid := os.Getenv(container.ENV_EXEC_PID); pid != "" {
			log.Info("minidocker: callback to pid: ", pid)
			return nil
		}

		if len(context.Args()) != 2 {
			return fmt.Errorf("minidocker: missing container name or command")
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
					return fmt.Errorf("minidocker: wrong args %v for network create", context.Args())
				}

				if err := network.Init(); err != nil {
					return err
				}

				name := context.Args().Get(0)
				return network.CreateNetwork(context.String("subnet"), context.String("driver"), name)
			},
		},
		{
			Name:  "delete",
			Usage: "delete container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) != 1 {
					return fmt.Errorf("minidocker: only container network name is needed for remove")
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
					return fmt.Errorf("minidocker: no args needed for container list")
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
		log.Infof("minidocker: init container process")
		return container.RunContainerInitProcess()
	},
}

func (config *containerConfig) sendInitCommand() {
	command := strings.Join(config.commands, " ")
	log.Info("minidocker: send command: ", command)

	config.writePipe.WriteString(command)
	config.writePipe.Close()
}
