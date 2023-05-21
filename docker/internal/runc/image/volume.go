package image

import (
	"docker/internal/utils/path"
	"os"
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
)

func parseVolume(volume string) (string, string) {
	dirs := strings.Split(volume, ":")
	src, dst := dirs[0], dirs[1]

	exist, err := path.PathExist(src)
	if err != nil {
		log.Infof("check path %v exist failed: %v", src, err)
	}

	if !exist {
		if err := os.Mkdir(src, 0777); err != nil {
			log.Errorf("mkdir dir %v failed: %v", src, err)
		}
	}

	return src, dst
}

func CreateVolumeLayer(mergedir, volume string) {
	hostdir, containerdir := parseVolume(volume)
	volumeContainerMountPoint := mergedir + containerdir
	exist, err := path.PathExist(volumeContainerMountPoint)
	if err != nil {
		log.Infof("check volume mount point %v exist failed: %v", volumeContainerMountPoint, err)
	}

	if !exist {
		if err := os.Mkdir(volumeContainerMountPoint, 0777); err != nil {
			log.Errorf("create volume mount point %v failed: %v", volumeContainerMountPoint, err)
		}
	}

	// @ToDo: The volume mount is not really effective
	// The issue is how to mount another directory into overlay filesystem
	cmd := exec.Command("mount", "--bind", hostdir, volumeContainerMountPoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
}

func NewOverlayFilesystemWithVolume(imagedir, lowerdir, upperdir, workdir, mergedir, volume string) {
	CreateLowerLayer(imagedir, lowerdir)
	CreateUpperLayer(upperdir)
	CreateWorkLayer(workdir)
	CreateMountPoint(lowerdir, upperdir, workdir, mergedir)
	CreateVolumeLayer(mergedir, volume)
}
