package container

import (
	utils "docker/internal/utils/pipe"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
)

func NewParentProcess(tty bool) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := utils.NewPipe()
	if err != nil {
		log.Errorf("new pipe failed: %v", err)
		return nil, nil
	}

	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}

	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
	}

	cmd.ExtraFiles = []*os.File{readPipe}
	return cmd, writePipe
}

func readUserCommands() []string {
	pipe := os.NewFile(uintptr(3), "pipe")
	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("read pipe error %v", err)
		return nil
	}

	return strings.Split(string(msg), " ")
}

func RunContainerInitProcess() error {
	commands := readUserCommands()
	if len(commands) == 0 {
		return fmt.Errorf("get empty command")
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	command, err := exec.LookPath(commands[0])
	if err != nil {
		return fmt.Errorf("look command %v failed: %v", command, err)
	}

	if err := syscall.Exec(command, commands[0:], os.Environ()); err != nil {
		log.Errorf(err.Error())
	}

	return nil
}
