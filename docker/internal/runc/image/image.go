package image

import (
	"docker/internal/utils/path"
	"os"
	"os/exec"

	log "github.com/Sirupsen/logrus"
)

func CreateLowerLayer(imagedir, lowerdir string) {
	exist, err := path.PathExist(lowerdir)
	if err != nil {
		log.Infof("check path exist failed: %v", err)
	}
	if !exist {
		if err := os.Mkdir(lowerdir, 0777); err != nil {
			log.Errorf("mkdir dir %v failed: %v", lowerdir, err)
		}
		if _, err := exec.Command("tar", "-xvf", imagedir, "-C", lowerdir).CombinedOutput(); err != nil {
			log.Errorf("untar %v failed: %v", imagedir, err)
		}
	}
}

func CreateUpperLayer(upperdir string) {
	if err := os.Mkdir(upperdir, 0777); err != nil {
		log.Errorf("create upper layer %v failed: %v", upperdir, err)
	}
}

func CreateWorkLayer(workdir string) {
	if err := os.Mkdir(workdir, 0777); err != nil {
		log.Errorf("create work layer %v failed: %v", workdir, err)
	}
}

func CreateMountPoint(lowerdir, upperdir, workdir, mergedir string) {
	if err := os.Mkdir(mergedir, 0777); err != nil {
		log.Errorf("create merge directory %v failed: %v", mergedir, err)
	}

	dirs := "lowerdir=" + lowerdir + ",upperdir=" + upperdir + ",workdir=" + workdir
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", dirs, mergedir)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
}

func NewOverlayFilesystem(imagedir, lowerdir, upperdir, workdir, mergedir string) {
	CreateLowerLayer(imagedir, lowerdir)
	CreateUpperLayer(upperdir)
	CreateWorkLayer(workdir)
	CreateMountPoint(lowerdir, upperdir, workdir, mergedir)
}
