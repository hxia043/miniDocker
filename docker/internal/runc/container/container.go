package container

import (
	"bufio"
	"docker/internal/runc/image"
	"docker/internal/utils/cmdtable"
	"docker/internal/utils/path"
	"docker/internal/utils/pipe"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gosuri/uitable"
)

const (
	imagedir = "/root/go/src/miniDocker/docker/cmd/docker-nwmgmt.tar"
	lowerdir = "/root/go/src/miniDocker/docker/cmd/docker-nwmgmt"
	Upperdir = "/root/go/src/miniDocker/docker/cmd/diff"
	Workdir  = "/root/go/src/miniDocker/docker/cmd/work"
	Mergedir = "/root/go/src/miniDocker/docker/cmd/merged"

	RUNNING = "running"
	STOP    = "stop"
	EXIT    = "exit"

	defaultContainerInfoPath = "/var/run/minidocker"
	configName               = "config.json"
	logName                  = "container.log"

	ENV_EXEC_PID = "minidocker_pid"
	ENV_EXEC_CMD = "minidocker_cmd"
)

type Container struct {
	Pid         string `json:"pid"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Command     string `json:"command"`
	CreatedTime string `json:"created"`
	config      string
}

func NewParentProcess(tty bool, volume, name string, envs []string) (*exec.Cmd, *os.File) {
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
	} else {
		containerlog := fmt.Sprintf("%s/%s/%s", defaultContainerInfoPath, name, logName)
		file, _ := os.OpenFile(containerlog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
		defer func() { file.Close() }()

		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Stdout = file
	}

	cmd.ExtraFiles = []*os.File{readPipe}

	cmd.Env = append(os.Environ(), envs...)
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

func RunContainerLog(name string) error {
	containerlog := fmt.Sprintf("%s/%s/%s", defaultContainerInfoPath, name, logName)
	exist, err := path.PathExist(containerlog)
	if err != nil {
		return err
	}
	if !exist {
		return fmt.Errorf("container log doesn't exist in %v", containerlog)
	}

	content, err := os.ReadFile(containerlog)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(os.Stdout, string(content)); err != nil {
		return err
	}

	return nil
}

func RunContainerRemove(name string) error {
	config := fmt.Sprintf("%s/%s/%s", defaultContainerInfoPath, name, configName)
	content, _ := os.ReadFile(config)
	var container Container
	json.Unmarshal(content, &container)

	if container.Status == RUNNING {
		if err := RunContainerStop(name); err != nil {
			return err
		}
	}

	containerpath := fmt.Sprintf("%s/%s", defaultContainerInfoPath, name)
	if err := os.RemoveAll(containerpath); err != nil {
		return err
	}

	return nil
}

func RunContainerStop(name string) error {
	config := fmt.Sprintf("%s/%s/%s", defaultContainerInfoPath, name, configName)
	content, _ := os.ReadFile(config)
	var container Container
	if err := json.Unmarshal(content, &container); err != nil {
		log.Errorf("json Unmarshal failed: %v", err)
		return err
	}

	pid, _ := strconv.Atoi(container.Pid)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return err
	}

	container.Status = STOP
	file, _ := os.OpenFile(config, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0622)

	newContainerInfo, _ := json.MarshalIndent(container, "", "    ")
	w := bufio.NewWriter(file)
	w.WriteString(string(newContainerInfo))
	w.Flush()

	return nil
}

func RunContainerCommit(imageName string) error {
	imageTar := imageName + ".tar"
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", Mergedir, ".").CombinedOutput(); err != nil {
		log.Errorf("commit container into %v failed: %v", imageTar, err)
	}

	return nil
}

func getContainerInfo(file os.FileInfo) (*Container, error) {
	name := file.Name()
	config := fmt.Sprintf("%s/%s/%s", defaultContainerInfoPath, name, configName)
	content, err := os.ReadFile(config)
	if err != nil {
		return nil, err
	}

	var container Container
	if err := json.Unmarshal(content, &container); err != nil {
		log.Errorf("json Unmarshal failed: %v", err)
		return nil, err
	}

	return &container, nil
}

func RunContainerList(flag bool) error {
	files, err := ioutil.ReadDir(defaultContainerInfoPath)
	if err != nil {
		log.Errorf("read container config failed: %v", err)
		return err
	}

	var containers []*Container
	for _, file := range files {
		c, err := getContainerInfo(file)
		if err != nil {
			log.Errorf("get container info failed: %v", err)
			continue
		}

		if flag {
			containers = append(containers, c)
		} else {
			if c.Status == RUNNING {
				containers = append(containers, c)
			}
		}
	}

	table := uitable.New()
	table.AddRow("NAME", "COMMAND", "CREATED", "STATUS", "PID")
	for _, container := range containers {
		table.AddRow(container.Name, container.Command, container.CreatedTime, container.Status, container.Pid)
	}

	return cmdtable.EncodeTable(os.Stdout, table)
}

func getContainerPid(name string) (string, error) {
	config := fmt.Sprintf("%s/%s/%s", defaultContainerInfoPath, name, configName)
	content, _ := os.ReadFile(config)

	var container Container
	if err := json.Unmarshal(content, &container); err != nil {
		return "", fmt.Errorf("json Unmarshal failed: %v", err)
	}

	return container.Pid, nil
}

func RunContainerExec(containerName string, commands []string) error {
	pid, err := getContainerPid(containerName)
	if err != nil {
		return fmt.Errorf("get container pid failed: %v", err)
	}

	log.Infof("minidocker pid: %v", pid)
	log.Infof("minidocker command: %v", strings.Join(commands, " "))

	os.Setenv(ENV_EXEC_PID, pid)
	os.Setenv(ENV_EXEC_CMD, strings.Join(commands, " "))

	cmd := exec.Command("/proc/self/exe", "exec")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	pidEnvBytes, err := os.ReadFile(fmt.Sprintf("/proc/%s/environ", pid))
	if err != nil {
		return fmt.Errorf("get %s env failed", err)
	}
	pidenvs := strings.Split(string(pidEnvBytes), "\u0000")
	cmd.Env = append(os.Environ(), pidenvs...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run exec failed: %v", err)
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

func (c *Container) UpdateContainerInfo(status string) {
	file, err := os.OpenFile(c.config, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0622)
	if err != nil {
		log.Errorf("open container config failed: %v", err)
	}

	if status != RUNNING {
		c.Status = status
	}

	newContainerInfo, _ := json.MarshalIndent(c, "", "    ")
	w := bufio.NewWriter(file)
	w.WriteString(string(newContainerInfo))
	w.Flush()
}

func (c *Container) RecordContainerInfo() error {
	containerInfo, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		log.Errorf("record container info failed: %v", err)
		return err
	}

	containerInfoPath := fmt.Sprintf("%s/%s", defaultContainerInfoPath, c.Name)
	if err := os.MkdirAll(containerInfoPath, 0622); err != nil {
		log.Errorf("create container path failed: %v", err)
		return err
	}

	c.config = fmt.Sprintf("%s/%s", containerInfoPath, configName)
	file, err := os.Create(c.config)
	defer func() {
		if err := file.Close(); err != nil {
			return
		}
	}()

	if err != nil {
		log.Errorf("create container path failed: %v", err)
		return err
	}

	if _, err := file.WriteString(string(containerInfo)); err != nil {
		log.Errorf("write container config failed: %v", err)
		return err
	}

	return nil
}

func New(name, pid, command, status string) *Container {
	return &Container{
		Name:        name,
		Pid:         pid,
		Command:     command,
		Status:      status,
		CreatedTime: time.Now().Format(time.RFC3339),
	}
}
