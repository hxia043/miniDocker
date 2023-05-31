package cmd

import (
	"docker/internal/runc/cgroups"
	"docker/internal/runc/cgroups/subsystem"
	"docker/internal/runc/container"
	"fmt"
	"os"
	"strings"
	"syscall"

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

func Run(tty bool, commands []string, res *subsystem.ResourceConfig, volume string) {
	defer exitContainer(tty, volume)

	parent, writePipe := container.NewParentProcess(tty, volume)
	if err := parent.Start(); err != nil {
		log.Error(err)
	}

	cgroupManager := cgroups.New("minidocker-cgroup")
	defer cgroupManager.Destroy()
	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)

	sendInitCommand(commands, writePipe)
	if tty {
		parent.Wait()
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

		Run(tty, cmds, rc, volume)
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
	log.Infof("send command: ", command)

	writePipe.WriteString(command)
	writePipe.Close()
}
