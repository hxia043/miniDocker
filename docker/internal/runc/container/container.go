package container

import (
	"docker/internal/runc/image"
	"docker/internal/utils/pipe"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
)

const (
	imagedir = "/root/go/src/miniDocker/docker/cmd/docker-nwmgmt.tar"
	lowerdir = "/root/go/src/miniDocker/docker/cmd/docker-nwmgmt"
	Upperdir = "/root/go/src/miniDocker/docker/cmd/diff"
	Workdir  = "/root/go/src/miniDocker/docker/cmd/work"
	Mergedir = "/root/go/src/miniDocker/docker/cmd/merged"
)

func NewParentProcess(tty bool, volume string) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := pipe.NewPipe()
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

	if volume != "" && len(strings.Split(volume, ":")) == 2 {
		image.NewOverlayFilesystemWithVolume(imagedir, lowerdir, Upperdir, Workdir, Mergedir, volume)
	} else {
		image.NewOverlayFilesystem(imagedir, lowerdir, Upperdir, Workdir, Mergedir)
	}

	cmd.Dir = Mergedir
	return cmd, writePipe
}

func pivotRoot(root string) error {
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return err
	}

	// Mount the new root as a new filesystem
	if err := syscall.Mount(root, root, "", syscall.MS_BIND|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("mount rootfs to itself failed: %v", err)
	}

	// Create a new directory for the old root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0700); err != nil {
		return err
	}

	// Pivot the root directory
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}

	// Change the current working directory to "/"
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir %v", err)
	}

	// Unmount the old root
	pivotDir = filepath.Join("/", ".pivot_root")
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("umount pivot_root dir %v", err)
	}

	return os.Remove(pivotDir)
}

func setupMount() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("get current location failed: %v", err)
	}

	log.Infof("current locaion: %v", pwd)
	if err := pivotRoot(pwd); err != nil {
		log.Errorf("change root filesystem failed: %v", err)
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
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

func RunContainerCommit(imageName string) error {
	imageTar := imageName + ".tar"
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", Mergedir, ".").CombinedOutput(); err != nil {
		log.Errorf("commit container into %v failed: %v", imageTar, err)
	}

	return nil
}

func RunContainerInitProcess() error {
	commands := readUserCommands()
	if len(commands) == 0 {
		return fmt.Errorf("get empty command")
	}

	setupMount()

	command, err := exec.LookPath(commands[0])
	if err != nil {
		return fmt.Errorf("look command %v failed: %v", command, err)
	}

	if err := syscall.Exec(command, commands[0:], os.Environ()); err != nil {
		log.Errorf(err.Error())
	}

	return nil
}
