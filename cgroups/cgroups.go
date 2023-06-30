package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
)

const (
	exeCommand                 = "/proc/self/exe"
	cgroupMemoryHierarchyMount = "/sys/fs/cgroup/memory"

	isolateNamespace uintptr = syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS
	localNamespace   uintptr = 0
)

var (
	startupCommand = "sh"
	startupArgs    = []string{"-c", `stress --vm-bytes 200m --vm-keep -m 1`}
)

func newCommand(namespace uintptr, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: namespace}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func runCommandInNamespace(namespace uintptr, name string, args ...string) error {
	cmd := newCommand(namespace, name, args...)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func isRunExe(args []string, name string) bool {
	return args[0] == name
}

func setCgroupFor(pid int, cgroupPath string) {
	os.Mkdir(path.Join(cgroupPath, "testmemorylimit"), 0755)
	ioutil.WriteFile(path.Join(cgroupPath, "testmemorylimit", "tasks"), []byte(strconv.Itoa(pid)), 0644)
	ioutil.WriteFile(path.Join(cgroupPath, "testmemorylimit", "memory.limit_in_bytes"), []byte("500m"), 0644)
}

func main() {
	if isRunExe(os.Args, exeCommand) {
		if err := runCommandInNamespace(localNamespace, startupCommand, startupArgs...); err != nil {
			os.Exit(1)
		}
	}

	cmd := newCommand(isolateNamespace, exeCommand)
	if err := cmd.Start(); err != nil {
		os.Exit(1)
	} else {
		setCgroupFor(cmd.Process.Pid, cgroupMemoryHierarchyMount)
	}

	cmd.Process.Wait()
}
