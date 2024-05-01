// Package fileutils contains utilities and helpers to manage and manipulate files.
package fileutils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/procutils"
)

// eadFile will return the content of input file or error.
// This is a linux-only implementation using syscalls for performance benefits.
func ReadFile(path string) ([]byte, error) {
	var stat syscall.Stat_t

	// ensure that file exists
	err := syscall.Stat(path, &stat)
	if err != nil {
		return nil, err
	}

	// and that I can open it
	fd, err := syscall.Open(path, syscall.O_RDONLY, uint32(os.ModePerm))
	if err != nil {
		logging.LogDebug("%v", err)

		return nil, err
	}

	defer func() { _ = syscall.Close(fd) }()

	fileLenght := 10000
	if stat.Size > 0 {
		fileLenght = int(stat.Size)
	}

	filedata := make([]byte, fileLenght)

	_, err = syscall.Read(fd, filedata)
	if err != nil {
		logging.LogError("%v", err)

		return nil, err
	}

	return filedata, nil
}

// WriteFile will write the content in input to file in path or error.
// This is a linux-only implementation using syscalls for performance benefits.
func WriteFile(path string, content []byte, perm uint32) error {
	var fd int

	var stat syscall.Stat_t
	// ensure that file exists
	err := syscall.Stat(path, &stat)
	if err != nil {
		fd, err = syscall.Creat(path, perm)
		if err != nil {
			logging.LogError("%v", err)

			return err
		}
	} else {
		fd, err = syscall.Open(path, syscall.O_RDWR, perm)
		if err != nil {
			logging.LogError("%v", err)

			return err
		}

		err = syscall.Chmod(path, perm)
		if err != nil {
			logging.LogError("%v", err)

			return err
		}
	}

	defer func() { _ = syscall.Close(fd) }()

	_, err = syscall.Write(fd, content)

	return err
}

// GetFileDigest will return the sha256sum of input file. Empty if error occurs.
func GetFileDigest(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}

	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return ""
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// CheckFileDigest will compare input digest to the checksum of input file.
// Returns whether the input digest is equal to the input file's one.
func CheckFileDigest(path string, digest string) bool {
	checksum := GetFileDigest(path)

	logging.LogDebug("input checksum is: %s", "sha256:"+checksum)
	logging.LogDebug("expected checksum is: %s", digest)

	return "sha256:"+checksum == digest
}

// Exist returns if a path exists or not.
func Exist(path string) bool {
	var stat syscall.Stat_t
	err := syscall.Stat(path, &stat)

	return err == nil
}

// CopyFileContainer will copy file from src to dest.
func CopyFileContainer(src string, dest string) error {
	input, err := ReadFile(src)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	fileinfo, err := os.Stat(src)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	destinfo, err := os.Stat(dest)
	if err != nil {
		logging.LogDebug("%v", err)

		_, err = os.Create(dest)
		if err != nil {
			logging.LogError("%v", err)

			return err
		}

		destinfo, err = os.Stat(dest)
		if err != nil {
			logging.LogError("%v", err)

			return err
		}
	}

	if destinfo.IsDir() {
		dest = filepath.Join(dest, filepath.Base(src))
	}

	err = WriteFile(dest, input, uint32(fileinfo.Mode()))
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	return nil
}

// DiscUsageMegaBytes returns disk usage for input path in MB (rounded).
func DiscUsageMegaBytes(path string) (string, error) {
	var discUsage int64

	readSize := func(path string, file os.FileInfo, err error) error {
		if !file.IsDir() {
			discUsage += file.Size()
		}

		return nil
	}

	err := filepath.Walk(path, readSize)
	if err != nil {
		logging.LogError("%v", err)

		return "", err
	}

	size := math.Round(float64(discUsage) / 1024.0 / 1024.0)

	return fmt.Sprintf("%.2f MB", size), nil
}

// Umount will force umount a destination path.
func Umount(dest string) error {
	for {
		if !ismountpoint(dest) {
			logging.LogDebug("%s not a mountpoint", dest)

			break
		}

		err := syscall.Unmount(dest, 0)
		if err != nil {
			logging.LogError("%v", err)

			return err
		}

		time.Sleep(time.Millisecond * 500)
	}

	return nil
}

// ismountpoint will return whether the input path is a mounpoint or not.
// This function will parse the /proc/mounts file and search for input path.
func ismountpoint(path string) bool {
	mounts, err := ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	lines := strings.Split(string(mounts), "\n")

	for _, line := range lines {
		if len(strings.Split(line, " ")) > 1 {
			mountpoint := strings.Split(line, " ")[1]
			if mountpoint == path {
				return true
			}
		}
	}

	return false
}

// Mount will bind-mount src to dest, using input mode.
func Mount(src, dest string, mode uintptr) error {
	logging.LogDebug("ensuring source point %s exists", src)

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	logging.LogDebug("ensuring destination point %s exists", dest)

	if info.IsDir() {
		_ = os.MkdirAll(dest, 0o755)
	} else {
		file, _ := os.Create(dest)

		defer func() { _ = file.Close() }()
	}

	logging.LogDebug("mounting %s on %s as bind, with mode %s", src, dest, mode)

	return syscall.Mount(src,
		dest,
		"bind",
		mode,
		"")
}

// MountCgroup will mount a new cgroup/cgroup2 fs on dest.
func MountCgroup(dest string) error {
	logging.LogDebug("ensuring destination point %s exists", dest)

	return syscall.Mount("cgroup2",
		dest,
		"cgroup2",
		0,
		"")
}

// MountShm will mount a new shm tmpfs to dest path.
// Said mount will be created with mode: noexec,nosuid,nodev,mode=1777,size=65536k.
func MountShm(dest string) error {
	logging.LogDebug("ensuring destination point %s exists", dest)

	_ = os.MkdirAll(dest, 0o777)

	logging.LogDebug(
		"mounting new shm on %s, with mode noexec,nosuid,nodev,mode=1777,size=65536k",
		dest,
	)

	return syscall.Mount("shm",
		dest,
		"tmpfs",
		syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV,
		"mode=1777,size=65536k")
}

// MountMqueue will mount a new mqueue tmpfs in dest path.
// Said mount will be created with mode: noexec,nosuid,nodev.
func MountMqueue(dest string) error {
	logging.LogDebug("ensuring destination point %s exists", dest)

	_ = os.MkdirAll(dest, 0o777)

	logging.LogDebug("mounting new mqueue on %s, with mode noexec,nosuid,nodev", dest)

	return syscall.Mount("mqueue",
		dest,
		"mqueue",
		syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV,
		"")
}

// MountTmpfs will mount a new tmpfs in dest path.
func MountTmpfs(dest string) error {
	logging.LogDebug("ensuring destination point %s exists", dest)

	_ = os.MkdirAll(dest, 0o777)

	logging.LogDebug("mounting new tmpfs on %s", dest)

	return syscall.Mount("tmpfs",
		dest,
		"tmpfs",
		uintptr(0),
		"")
}

// MountProc will mount a new procfs in dest path.
// Said mount will be created with mode: noexec,nosuid,nodev.
func MountProc(dest string) error {
	logging.LogDebug("ensuring destination point %s exists", dest)

	_ = os.MkdirAll(dest, 0o755)

	return syscall.Mount("proc",
		dest,
		"proc",
		syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV,
		"")
}

// MountDevPts will mount a new devpts in dest path.
// Said mount will be created with mode: noexec,nosuid,newinstance,ptmxmode=0666,mode=0620.
func MountDevPts(dest string) error {
	logging.LogDebug("ensuring destination point %s exists", dest)

	_ = os.MkdirAll(dest, 0o755)

	logging.LogDebug(
		"mounting new devpts on %s with mode noexec,nosuid,newinstance,ptmxmode=0666,mode=0620",
		dest,
	)

	return syscall.Mount("devpts",
		dest,
		"devpts",
		syscall.MS_NOEXEC|syscall.MS_NOSUID,
		"newinstance,ptmxmode=0666,mode=0620")
}

// MountBind will bind-mount src path in dest path.
// Said mount will be created with mode: rbind,rprivate.
func MountBind(src, dest string) error {
	logging.LogDebug("performing bind mount %s %s", src, dest)

	return Mount(src, dest, syscall.MS_BIND|syscall.MS_REC|syscall.MS_PRIVATE)
}

// MountBindRO will bind-mount read-only src path in dest path.
// Said mount will be created with mode: rbind,rprivate,ro,nosuid,noexec,nodev.
func MountBindRO(src, dest string) error {
	logging.LogDebug("performing read-only bind mount %s %s", src, dest)

	return Mount(src,
		dest,
		syscall.MS_BIND|syscall.MS_REC|syscall.MS_RDONLY|syscall.MS_NOSUID|
			syscall.MS_NOEXEC|syscall.MS_NODEV|syscall.MS_PRIVATE)
}

// UntarFile will untar target file to target directory.
// If userns is specified and it is keep-id, it will perform the
// untarring in a new user namespace with user id maps set, in order to prevent
// permission errors.
func UntarFile(path string, target string, userns string) error {
	// first ensure we can write
	err := syscall.Access(path, 2)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	if userns != constants.KeepID {
		cmd := exec.Command("tar", "--exclude=dev/*", "-xf", path, "-C", target)
		logging.LogDebug("no keep-id specified, simply perform %v", cmd.Args)

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, string(out))
		}

		return nil
	}

	command := "/bin/sh"
	args := []string{
		"-c",
		"mkdir -p " + target + " &&" +
			"chown -R root:root " + target + " &&" +
			"tar --exclude=dev/* -xf " + path + " -C " + target,
	}

	cmd := exec.Command(command, args...)

	// we need to unpack using keep-id in order to keep consistency
	cloneFlags := syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS
	cloneFlags |= syscall.CLONE_NEWUSER
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential:                 &syscall.Credential{Uid: 0, Gid: 0},
		Cloneflags:                 uintptr(cloneFlags),
		GidMappingsEnableSetgroups: true,

		Pdeathsig: syscall.SIGTERM,
	}

	uid := os.Getenv("PARENT_UID_MAP")
	gid := os.Getenv("PARENT_GID_MAP")

	logging.LogDebug("setting up keep-id %s, %s", uid, gid)

	err = procutils.SetProcessKeepIDMaps(cmd, uid, gid)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	return cmd.Run()
}
