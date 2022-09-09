package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/89luca89/scatman/pkg/pullutils"
	"github.com/Jeffail/gabs/v2"
)

// ChrootDir is the default location for downloaded images.
var ChrootDir string = os.Getenv("HOME") + `/.local/share/scatman/chroots/`

// GetChrootDir returns a base64 encoded string of the full name of the chroot.
func GetChrootDir(name string) string {
	targetDIR := ChrootDir
	targetDIR += base64.StdEncoding.EncodeToString([]byte(name))
	targetDIR += "/"

	return targetDIR
}

// GetRootfsDir returns a base64 encoded string of the full name of the chroot rootfs.
func GetRootfsDir(name string) string {
	targetDIR := ChrootDir
	targetDIR += base64.StdEncoding.EncodeToString([]byte(name))
	targetDIR += "/rootfs/"

	return targetDIR
}

// PrepareRootfs will find the image passed as argument, and set the chroot
// directory named as the passed chroot name.
func PrepareRootfs(image string, name string, userns string) error {
	chrootDIR := GetRootfsDir(name)

	err := os.MkdirAll(chrootDIR, os.ModePerm)
	if err != nil {
		return err
	}

	if !strings.Contains(image, ":") {
		// no tags provided default to latest
		log.Println("No tags found, defaulting to latest.")

		image += ":latest"
	}

	// compose the manifest url startin from the input image
	registryURL, manifestURL := pullutils.GetManifestURL(image)
	imageBase, imageName, tag := pullutils.GetImagePrefixName(image)

	pullImage := pullutils.ImageInfo{
		Image:       image,
		RegistryURL: registryURL,
		ManifestURL: manifestURL,
		ImageBase:   imageBase,
		ImageName:   imageName,
		ImageTag:    tag,
	}

	imageDIR := pullutils.GetImageDir(pullImage)

	// get manifest.json of the image
	jsonFile, err := os.ReadFile(imageDIR + "/manifest.json")
	if err != nil {
		os.RemoveAll(GetChrootDir(name))
		return errors.New("Image not found")
	}

	jsonParsed, err := gabs.ParseJSON(jsonFile)
	if err != nil {
		os.RemoveAll(GetChrootDir(name))
		return err
	}

	// get layers list
	layers := jsonParsed.Children()[0].Path("Layers")
	for _, layer := range layers.Children() {
		layerTar := imageDIR + layer.Data().(string)

		log.Printf("Extracting %s...\n", layer.Data().(string))

		err := UntarFile(layerTar, chrootDIR)
		if err != nil {
			os.RemoveAll(GetChrootDir(name))
			return err
		}
	}

	log.Println("Extraction complete.")

	return nil
}

// FixChrootPermissionKeepID will set rootfs ownership to be root inside a keep-id environment.
func FixChrootPermissionKeepID(chrootDIR string) error {
	// this is our child process that will enter the chroot effectively
	cmd := exec.Command("chown", "-R", "0:0", chrootDIR)
	// Set Namespaces with generated value, this value will keep-id with the host.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: 0, Gid: 0},
		Cloneflags:                 syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
		GidMappingsEnableSetgroups: true,
		Pdeathsig:                  syscall.SIGKILL,
	}

	uid, err := strconv.ParseInt(os.Getenv("PARENT_UID"), 10, 32)
	if err != nil {
		return err
	}

	gid, err := strconv.ParseInt(os.Getenv("PARENT_GID"), 10, 32)
	if err != nil {
		return err
	}
	setProcessKeepIDMaps(cmd, int(uid), int(gid))

	fmt.Println(cmd, cmd.SysProcAttr, os.Getuid(), os.Getegid())

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func unique(s []string) []string {
	inResult := make(map[string]bool)

	var result []string

	for _, str := range s {
		if _, ok := inResult[str]; !ok {
			inResult[str] = true

			result = append(result, str)
		}
	}

	return result
}

func findChrootSubProcesses(name string) ([]string, error) {
	pids := []string{}
	processDir := "/proc/" + name + "/task"
	err := filepath.Walk(processDir, func(path string, info os.FileInfo, e error) error {
		// check if it is a regular file (not dir)
		if info.Mode().IsRegular() && info.Name() == "children" {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			pid := strings.Trim(string(b), " ")
			if len(pid) > 0 {
				// we have other subprocesses! Let's recurse
				pids = append(pids, pid)
				subPids, err := findChrootSubProcesses(pid)
				if err != nil {
					return err
				}
				pids = append(pids, subPids...)
			}
		}

		return nil
	})

	if err != nil {
		return pids, err
	}

	return pids, nil
}

func findChrootProcesses(name string) ([]string, error) {
	result := []string{}
	nameb64 := base64.StdEncoding.EncodeToString([]byte(name))

	pids, err := os.ReadDir("/proc")
	if err != nil {
		log.Fatal(err)
	}

	for _, pid := range pids {
		cmdline := "/proc/" + pid.Name() + "/cmdline"
		if _, err := os.Stat(cmdline); err == nil {
			b, err := os.ReadFile(cmdline)
			if err != nil {
				return result, err
			}

			match, err := regexp.Match(nameb64, b)
			if err != nil {
				return result, err
			}

			if match {
				result = append(result, filepath.Base(filepath.Dir(cmdline)))
			}
		}
	}

	if err != nil {
		return result, err
	}

	for _, pid := range result {
		subPids, err := findChrootSubProcesses(pid)

		if err != nil {
			return result, err
		}

		result = append(result, subPids...)
	}

	result = unique(result)

	return result, nil
}

func setProcessKeepIDMaps(cmd *exec.Cmd, uid int, gid int) {
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      1,
			Size:        uid,
		},
		{
			ContainerID: uid,
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: uid + 1,
			HostID:      uid + 1,
			Size:        64536,
		},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      1,
			Size:        gid,
		},
		{
			ContainerID: gid,
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: gid + 1,
			HostID:      gid + 1,
			Size:        64536,
		},
	}
}

// IsRunning returns wether the chroot is running or not.
// As a little optimization, we could take in input an output of an already
// executed `ps ax` command, in order to speed up.
func IsRunning(name string, running string) bool {
	// just check is a process that contains the chroot name exist
	nameb64 := base64.StdEncoding.EncodeToString([]byte(name))

	if running == "" {
		output, err := exec.Command("ps", "ax").Output()
		if err != nil {
			return false
		}

		running = string(output)
	}

	return strings.Contains(running, nameb64)
}

// GetChrootPid will return the pid of the process generating the chroot.
func GetChrootPid(name string) (string, error) {
	pids, err := findChrootProcesses(name)
	if err != nil {
		return "", err
	}

	return pids[0], nil
}

// StopChroot will find all the processes in given chroot and will
// stop them.
func StopChroot(name string, force bool) error {
	// walk in /proc to find all processes and
	// subprocesses of the chroot
	pids, err := findChrootProcesses(name)
	if err != nil {
		return err
	}

	// now loop in reverse to kill from child back to parents.
	for i := len(pids) - 1; i >= 0; i-- {
		log.Println("Killing pid:", pids[i])

		pid, err := strconv.ParseInt(pids[i], 10, 32)
		if err != nil {
			return err
		}

		proc, err := os.FindProcess(int(pid))
		if err != nil {
			log.Println(err)
		}

		// Kill the process
		if force {
			err = proc.Signal(syscall.SIGKILL)
		} else {
			err = proc.Signal(syscall.SIGTERM)
		}

		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

// EnterNamespace will enter the namespace of target chroot.
func EnterNamespace(chroot string, workdir string, interactive bool, config Config) error {
	chrootPid, err := GetChrootPid(chroot)
	if err != nil {
		return err
	}

	configArg, err := json.Marshal(config)
	if err != nil {
		return errors.New("Invalid config")
	}

	currentWorkingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command("nsenter", "--wd="+currentWorkingDirectory, "-U", "-m", "-u", "-t", chrootPid,
		os.Args[0], "enter", GetRootfsDir(chroot), workdir, "false", string(configArg))
	// in case we want interactive mode, we use RUN and we attach to the command.
	if interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	// else just start it in background.
	err = cmd.Start()
	if err != nil {
		return err
	}

	return nil
}

// EnterChroot will enter the target chroot.
func EnterChroot(path string, workdir string, interactive bool, config Config) error {
	configArg, err := json.Marshal(config)
	if err != nil {
		return errors.New("Invalid config")
	}

	var uid, gid int64

	usrFile, err := os.ReadFile(path + "../user")
	if err != nil {
		log.Printf("Warning: cannot find userfile, defaulting to 1000:1000")

		uid = 1000
		gid = 1000
	}

	uid, err = strconv.ParseInt(strings.Split(string(usrFile), ":")[0], 10, 32)
	if err != nil {
		return err
	}

	gid, err = strconv.ParseInt(strings.Split(string(usrFile), ":")[1], 10, 32)
	if err != nil {
		return err
	}

	// this is our child process that will enter the chroot effectively
	cmd := exec.Command(os.Args[0], "enter", path, workdir, "true", string(configArg))

	cloneFlags := syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS

	if config.Userns == "keep-id" {
		cloneFlags |= syscall.CLONE_NEWUSER
	}
	if config.Ipc == "private" {
		cloneFlags |= syscall.CLONE_NEWIPC
	}

	if config.Network == "private" {
		cloneFlags |= syscall.CLONE_NEWNET
	}

	if config.Pid == "private" {
		cloneFlags |= syscall.CLONE_NEWPID
	}
	// Set Namespaces with generated value, this value will keep-id with the host.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential:                 &syscall.Credential{Uid: 0, Gid: 0},
		Cloneflags:                 uintptr(cloneFlags),
		GidMappingsEnableSetgroups: true,

		Pdeathsig: syscall.SIGKILL,
	}

	if config.Userns == "keep-id" {
		setProcessKeepIDMaps(cmd, int(uid), int(gid))
	}
	// in case we want interactive mode, we use RUN and we attach to the command.
	if interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// non interactive mode, save stdout and stderr to file and disown
		logfile, err := os.Create(path + "../current-logs")
		if err != nil {
			return err
		}

		cmd.Stdout = logfile
		cmd.Stderr = logfile
	}
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
