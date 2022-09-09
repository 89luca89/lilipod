package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

func performBasicMounts(path string, conf utils.Config) error {
	// if we share PID, we mount host's pid, else we set
	// a new proc mount.
	if conf.Pid == "private" {
		if err := syscall.Mount("proc", path+"proc", "proc", 0, ""); err != nil {
			return errors.New("error setting PID private namespace: " + err.Error())
		}
	} else {
		if err := utils.Mount("/proc", path+"/proc", unix.MS_BIND|unix.MS_REC|unix.MS_PRIVATE, "rw"); err != nil {
			return errors.New("error setting PID host namespace: " + err.Error())
		}
	}

	// create /dev filesystem
	if err := syscall.Mount("tmpfs", path+"/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755,size=65536k"); err != nil {
		return errors.New("error setting /dev: " + err.Error())
	}

	// if we share IPC, we mount host's ipc dirs, else we set
	// a new proc mount.
	if conf.Ipc == "private" {
		if err := os.MkdirAll(path+"/dev/shm", os.ModePerm); err != nil {
			return errors.New("error creating IPC shm private namespace: " + err.Error())
		}

		if err := syscall.Mount("shm", path+"/dev/shm", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "mode=1777,size=65536k"); err != nil {
			return errors.New("error setting IPC shm private namespace: " + err.Error())
		}

		if err := os.MkdirAll(path+"/dev/mqueue", os.ModePerm); err != nil {
			return errors.New("error creating IPC mqueue private namespace: " + err.Error())
		}

		if err := syscall.Mount("mqueue", path+"/dev/mqueue", "mqueue", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
			return errors.New("error setting IPC mqueue private namespace: " + err.Error())
		}
	} else {
		if err := utils.Mount("/dev/shm", path+"/dev/shm", unix.MS_BIND|unix.MS_REC|unix.MS_PRIVATE, "rw"); err != nil {
			return errors.New("error setting IPC shm namespace: " + err.Error())
		}

		if err := utils.Mount("/dev/mqueue", path+"/dev/mqueue", unix.MS_BIND|unix.MS_REC|unix.MS_PRIVATE, "rw"); err != nil {
			return errors.New("error setting IPC mqueue namespace: " + err.Error())
		}
	}

	// this is a series of other files that we can source from the host, for
	// the chrooted system to work with.
	otherMounts := []string{
		"/sys",
		"/dev/null",
		"/dev/random",
		"/dev/full",
		"/dev/tty",
		"/dev/zero",
		"/dev/urandom",
		"/dev/console",
	}

	for _, mount := range otherMounts {
		if err := utils.Mount(mount, path+mount, unix.MS_BIND|unix.MS_REC|unix.MS_PRIVATE, "rw"); err != nil {
			return errors.New("error setting " + mount + ": " + err.Error())
		}
	}

	if conf.Network == "host" {
		if err := utils.CopyFile("/etc/resolv.conf", path+"/etc/resolv.conf"); err != nil {
			return errors.New("error setting host network DNS: " + err.Error())
		}
	}

	return nil
}

func performCustomMounts(path string, conf utils.Config) error {
	for _, volume := range conf.Volume {
		if strings.Compare(volume, "") == 0 {
			continue
		}

		modeU := unix.MS_BIND | unix.MS_REC | unix.MS_PRIVATE
		mode := "rw"
		mountings := strings.Split(volume, ":")

		if len(mountings) <= 1 {
			// we got an anonymous mount, let's do it
			os.MkdirAll(path+volume, os.ModePerm)
			if err := syscall.Mount("/dev/shm", path+volume, "tmpfs", 0, ""); err != nil {
				return errors.New("error creating anonymous mount: " + err.Error())
			}

			continue
		}

		if len(mountings) > 1 {
			mode = mountings[1]
		}

		source := mountings[0]
		dest := path + mountings[1]

		err := utils.Mount(source, dest, uintptr(modeU), mode)
		if err != nil {
			return err
		}
	}

	return nil
}

func performPTSMount(path string) error {
	// create pts devices
	err := os.MkdirAll(path+"dev/pts", os.ModePerm)
	if err != nil {
		return errors.New("error setting PTS: " + err.Error())
	}

	if err := syscall.Mount("devpts", path+"dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=0620,gid=5"); err != nil {
		return errors.New("error setting PTS: " + err.Error())
	}

	return nil
}

// NewEnterCommand is the forked/child process that is launched after creating the Namespaces.
// This will effectively enter the chroot.
func NewEnterCommand() *cobra.Command {
	var enterCommand = &cobra.Command{
		Use:              "enter chroot",
		RunE:             enter,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	enterCommand.Flags().SetInterspersed(false)

	return enterCommand
}

func enter(cobraCmd *cobra.Command, arguments []string) error {
	path := arguments[0]
	workdir := arguments[1]
	m := arguments[2]
	config := arguments[3]

	var conf utils.Config

	err := json.Unmarshal([]byte(config), &conf)
	if err != nil {
		return errors.New("Invalid config")
	}

	mainProc, err := strconv.ParseBool(m)
	if err != nil {
		return err
	}

	// Perform mountpoints only if we're the main process for this chroot
	if mainProc {

		// this section will make sure that mounts are privare in this mount
		// namespace, so that even with root we do not have pending mounts.
		if err := utils.Mount(path, path, unix.MS_BIND|unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
			return errors.New("error setting private mount: " + err.Error())
		}

		if err := utils.Mount("", path, unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
			return errors.New("error setting private mount: " + err.Error())
		}
		// -----
		err := performBasicMounts(path, conf)
		if err != nil {
			return err
		}

		err = performCustomMounts(path, conf)
		if err != nil {
			return err
		}

		err = performPTSMount(path)
		if err != nil {
			return err
		}

		// then we set up the hostname.
		if err := syscall.Sethostname([]byte(conf.Hostname)); err != nil {
			return errors.New("error setting UTS namespace")
		}

		// setting this file ensures compatibility and gives back some info
		infoFile, err := os.Create(path + "/run/.containerenv")
		if err != nil {
			return err
		}
		defer infoFile.Close()

		_, err = infoFile.WriteString("name=\"" + conf.Name + "\"")
		if err != nil {
			return err
		}
	}
	// first we set up chroot.
	if path != "" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			if err := syscall.Chroot(path); err != nil {
				return err
			}

			if err := syscall.Chdir(workdir); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	command := conf.Entrypoint[0]
	args := conf.Entrypoint[1:]
	cmd := exec.Command(command, args...)
	cmd.Env = append(cmd.Env, conf.Env...)

	usr, err := utils.GetUserIDs(conf.User)
	if err != nil {
		return err
	}
	// become the user that is set in create or in exec
	uid, _ := strconv.Atoi(usr.Uid)
	gid, _ := strconv.Atoi(usr.Gid)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
