package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/89luca89/scatman/pkg/pullutils"
	"golang.org/x/sys/unix"
)

// Config is a struct that holds the informations
// of the chroot we want to create.
type Config struct {
	Env      []string
	Hostname string
	Image    string
	Ipc      string
	Name     string
	Network  string
	Pid      string
	User     string
	Userns   string
	Volume   []string
	Label    []string
	// entry point related
	Entrypoint []string
}

// ScatmanExtraPath is the bin path internally used by scatman.
var ScatmanExtraPath string = os.Getenv("HOME") + "/.local/share/scatman/bin/"

// SetupRootlesskit will download and unpack rootlesskit to ScatmanExtraPath if not existing.
func SetupRootlesskit() error {
	_, err := os.Stat(ScatmanExtraPath + "rootlesskit")
	if err != nil {
		log.Println("Warning: rootlesskit not found, setting up on internal path")

		rootlesskitURL := "https://github.com/rootless-containers/rootlesskit/releases/download/v1.0.1/rootlesskit-x86_64.tar.gz"

		filepath, err := ioutil.TempFile(ScatmanExtraPath, "tar")
		if err != nil {
			return err
		}

		defer os.Remove(filepath.Name())

		err = pullutils.DownloadBlobToFile(rootlesskitURL, "", filepath.Name(), true)
		if err != nil {
			return err
		}

		err = UntarFile(filepath.Name(), ScatmanExtraPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// SetupBusybox will link the missing utility to internally managed busybox binary.
// If the binary does not exist, download it first.
func SetupBusybox(links []string) error {
	busyboxURL := "https://busybox.net/downloads/binaries/1.35.0-x86_64-linux-musl/busybox"
	filepath := ScatmanExtraPath + "/busybox"

	// if we don't already have busybox, download it.
	if _, err := os.Stat(filepath); errors.Is(err, os.ErrNotExist) {
		log.Printf("Warning: busybox not found, downloading it")

		err := pullutils.DownloadBlobToFile(busyboxURL, "", filepath, true)
		if err != nil {
			return err
		}

		err = os.Chmod(filepath, 0775)
		if err != nil {
			return err
		}
	}

	// save and restore CWD, we'll use chdir now to create symlinks.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	// enter the bin/ dir
	err = os.Chdir(ScatmanExtraPath)
	if err != nil {
		return err
	}

	// link all tools to the busybox binary
	for _, link := range links {
		_, err = os.Stat(link)
		if link != "rootlesskit" && err != nil {
			err = os.Symlink("busybox", link)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// UntarFile will untar target file to target directory.
func UntarFile(path string, target string) error {
	// first ensure we can write
	cmd := exec.Command("chmod", "+w", "-R", target)

	_, err := cmd.Output()
	if err != nil {
		return err
	}

	cmd = exec.Command("tar", "xf", path, "-C", target)

	_, err = cmd.Output()
	if err != nil {
		return err
	}

	return nil
}

// CopyFile will copy file from src to dest.
func CopyFile(src string, dest string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	fileinfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dest, input, fileinfo.Mode())
	if err != nil {
		return err
	}

	return nil
}

// EnsureRootlesskit will ensure process is executed with rootlesskit.
func EnsureRootlesskit(interactive bool) (bool, error) {
	// As files can be owned by root or fake roots, let's run either as root or
	// with rootlesskit
	if os.Getuid() != 0 {
		cmd := exec.Command("rootlesskit", os.Args...)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "PARENT_UID="+strconv.Itoa(os.Getuid()))
		cmd.Env = append(cmd.Env, "PARENT_GID="+strconv.Itoa(os.Getgid()))

		if interactive {
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Run()
			if err != nil {
				return false, err
			}

			return true, nil
		}

		err := cmd.Start()
		if err != nil {
			return false, err
		}

		time.Sleep(time.Second)

		err = cmd.Process.Release()
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

// SaveConfig saves current config from memory to json file.
func SaveConfig(config Config, path string) error {
	file, _ := json.MarshalIndent(config, "", " ")

	return os.WriteFile(path, file, 0644)
}

// LoadConfig loads a config from file to memory.
func LoadConfig(path string) (Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	data := Config{}

	err = json.Unmarshal(file, &data)
	if err != nil {
		return Config{}, err
	}

	return data, nil
}

// DiscUsageMegaBytes returns disk usage for input path in MB (rounded).
func DiscUsageMegaBytes(path string) (string, error) {
	var discUsage int64 = 0

	readSize := func(path string, file os.FileInfo, err error) error {
		if !file.IsDir() {
			discUsage += file.Size()
		}

		return nil
	}

	err := filepath.Walk(path, readSize)
	if err != nil {
		return "-", err
	}

	size := math.Round(float64(discUsage) / 1024.0 / 1024.0)

	return fmt.Sprintf("%.2f MB", size), nil
}

// GetUserIDs will search for usernames and return a couple uid:gid.
func GetUserIDs(username string) (*user.User, error) {
	u := strings.Split(username, ":")[0]

	// check if we're having an id or a username
	if _, err := strconv.Atoi(u); err == nil {
		usr, err := user.LookupId(u)
		if err != nil {
			return nil, err
		}

		return usr, nil
	}

	usr, err := user.Lookup(u)
	if err != nil {
		return nil, err
	}

	return usr, nil
}

// StringContains returns if a string is in a slice.
func StringContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Umount will force umount a destination path.
func Umount(dest string) error {
	for {
		cmd := exec.Command("mountpoint", dest)

		err := cmd.Run()
		if exitError, ok := err.(*exec.ExitError); ok {
			// umounting complete, exit
			if exitError.ExitCode() == 32 {
				break
			}
		}

		err = unix.Unmount(dest, 0)
		if err != nil {
			return err
		}

		time.Sleep(time.Millisecond * 500)
	}

	return nil
}

// Mount will perform a mount action on source, dest using flags and mode.
func Mount(source string, dest string, flags uintptr, mode string) error {
	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		src, err := os.Stat(source)
		if err != nil {
			return err
		}

		if src.IsDir() {
			err = os.MkdirAll(dest, os.ModePerm)
			if err != nil {
				return err
			}
		} else {
			emptyFile, err := os.Create(dest)
			if err != nil {
				return err
			}
			emptyFile.Close()
			flags = unix.MS_BIND
		}
	}

	if mode == "ro" {
		flags |= unix.MS_RDONLY
	}

	if err := unix.Mount(source, dest, "", flags, ""); err != nil {
		return err
	}

	return nil
}
