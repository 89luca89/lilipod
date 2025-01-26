// Package containerutils contains helpers and utilities for managing and creating
// containers.
package containerutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/procutils"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/moby/sys/capability"
)

// Limit access to host's kernel stuff -> /dev/null.
var linuxMaskedFiles = []string{
	"/proc/kcore",
	"/proc/keys",
	"/proc/latency_stats",
	"/proc/timer_list",
	"/proc/timer_stats",
}

// Limit access to host's kernel stuff -> empty tmpfs.
var linuxMaskedDirs = []string{
	"/proc/acpi",
	"/proc/scsi",
	"/sys/dev/block",
	"/sys/firmware",
	"/sys/fs/selinux",
	"/sys/kernel",
}

// We want access, but -ro is ok.
var linuxReadOnlyPaths = []string{
	"/proc/asound",
	"/proc/bus",
	"/proc/fs",
	"/proc/irq",
	"/proc/sys",
	"/proc/sysrq-trigger",
}

// this is a series of other files and directories that we can source from the host,
// for the containerized system to work with.
var linuxReadWritePaths = []string{
	"/dev/console",
	"/dev/full",
	"/dev/random",
	"/dev/tty",
	"/dev/urandom",
	"/dev/zero",
	"/sys",
}

// we need to setup the /sys/fs/cgroup mountpoint, by mounting a new cgroup2 filesystem.
func setupCgroupfs(conf utils.Config) error {
	logging.LogDebug("mounting new tmpfs fs on %s", "/sys/fs/cgroup")

	// blank out the mount using a tmpfs
	err := fileutils.MountTmpfs("/sys/fs/cgroup")
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting cgroups %w", err)
	}

	logging.LogDebug("mounting new cgroup fs on %s", "/sys/fs/cgroup")

	// mount a new cgroup v2 fs on it
	err = fileutils.MountCgroup("/sys/fs/cgroup")
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting cgroups %w", err)
	}

	// move our process to a dedicated scope, so that eventual init systems
	// won't encour problems with unknown PIDs
	err = os.MkdirAll("/sys/fs/cgroup/container-"+conf.Names+".scope", 0o755)
	if err != nil {
		return err
	}

	file, err := os.Create("/sys/fs/cgroup/container-" + conf.Names + ".scope/cgroup.procs")
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(file, 0)

	return err
}

// we need to setup the /dev/pts mountpoint, by mounting a new devpts filesystem
// and linking up /dev/ptmx to /dev/pts/ptmx.
func setupPTY(path string) error {
	// create pts devices
	logging.LogDebug("mounting new devpts fs on /dev/pts")

	err := fileutils.MountDevPts(filepath.Join(path, "/dev/pts"))
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting PTS %w", err)
	}

	logging.LogDebug("mounting new /dev/ptmx from /dev/pts/ptmx")

	err = fileutils.MountBind(filepath.Join(path, "dev/pts/ptmx"), filepath.Join(path, "dev/ptmx"))
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	return nil
}

// this is in case of an unprivileged container, we want to make sure that these
// mountpoints are either read-only, or masked, by being bind-mounted to /dev/null
// or to empty tmpfs.
func setupMaskedMounts(path string) error {
	logging.LogDebug("setting up read only and masked mounts")

	for _, mount := range linuxReadOnlyPaths {
		if fileutils.Exist(mount) {
			logging.LogDebug("mounting dir %s to %s as readonly", mount, filepath.Join(path, mount))

			err := fileutils.MountBindRO(mount, filepath.Join(path, mount))
			if err != nil {
				logging.LogDebug("error: %+v", err)

				return fmt.Errorf("error setting ro mount %s: %w", mount, err)
			}
		}
	}

	for _, mount := range linuxMaskedFiles {
		if fileutils.Exist(mount) {
			logging.LogDebug("mounting /dev/null to %s", filepath.Join(path, mount))

			err := fileutils.MountBindRO("/dev/null", filepath.Join(path, mount))
			if err != nil {
				logging.LogDebug("error: %+v", err)

				return fmt.Errorf("error setting masked mount file %s: %w", mount, err)
			}
		}
	}

	for _, mount := range linuxMaskedDirs {
		if fileutils.Exist(mount) {
			logging.LogDebug("mounting empty tmpfs to %s", filepath.Join(path, mount))

			err := fileutils.MountTmpfs(filepath.Join(path, mount))
			if err != nil {
				logging.LogDebug("error: %+v", err)

				return fmt.Errorf("error setting masked mount dir %s: %w", mount, err)
			}
		}
	}

	return nil
}

// this will setup all the basic mountpoints needed for the container to work.
// depending on the container's config, it will either bind-mount host's directories
// (eg in case of pid==host, it will bind-mount host's /proc) or by mounting new
// instances of filesystems (eg in case of pid==private, it will mount a new procfs).
//
// Setup mountpoints are:
//   - /proc
//   - /dev
//   - /dev/shm
//   - /dev/mqueue
//   - /tmp
//   - /etc/resolv.conf
//   - linuxReadWritePaths
func setupMounts(path string, conf utils.Config) error {
	logging.LogDebug("setting up basic mountpoints")

	// if we share PID, we mount host's pid, else we set
	// a new proc mount.
	if conf.Pid == constants.Private {
		logging.LogDebug("setting up private PID namespace")
		logging.LogDebug("mounting new profs on /proc")

		err := fileutils.MountProc(filepath.Join(path, "proc"))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("error setting PID private namespace: %w", err)
		}
	} else {
		logging.LogDebug("setting up shared PID namespace")
		logging.LogDebug("mounting host's /proc on %s", filepath.Join(path, "proc"))

		err := fileutils.MountBind("/proc", filepath.Join(path, "proc"))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("error setting PID host namespace: %w", err)
		}
	}

	// create /dev filesystem
	logging.LogDebug("mounting host's /dev on %s", filepath.Join(path, "dev"))

	err := fileutils.MountBind("/dev", filepath.Join(path, "/dev"))
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting /dev: %w", err)
	}

	logging.LogDebug("setting up tmps on /tmp")

	// create /tmp
	err = fileutils.MountTmpfs(filepath.Join(path, "/tmp"))
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting /tmp: %w", err)
	}

	// if we share IPC, we mount host's ipc dirs
	if conf.Ipc == constants.Private {
		logging.LogDebug("setting up private IPC namespace")
		logging.LogDebug("mounting new tmpfs on /dev/shm")

		err := fileutils.MountShm(filepath.Join(path, "/dev/shm"))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("error setting IPC private namespace - /dev/shm: %w", err)
		}

		logging.LogDebug("mounting new mqueue fs on /dev/mqueue")

		err = fileutils.MountMqueue(filepath.Join(path, "/dev/mqueue"))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("error setting IPC private namespace - /dev/mqueue: %w", err)
		}
	} else {
		logging.LogDebug("setting up shared IPC namespace")
		logging.LogDebug("mounting host's /dev/shm on %s", filepath.Join(path, "/dev/shm"))

		err := fileutils.MountBind("/dev/shm", filepath.Join(path, "/dev/shm"))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("error setting IPC namespace - /dev/shm: %w", err)
		}

		logging.LogDebug("mounting host's /dev/mqueue on %s", filepath.Join(path, "/dev/mqueue"))

		err = fileutils.MountBind("/dev/mqueue", filepath.Join(path, "/dev/mqueue"))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("error setting IPC namespace - /dev/mqueue: %w", err)
		}
	}

	if conf.Network == constants.Host {
		logging.LogDebug("coping host's /dev/resolv.conf on %s", filepath.Join(path, "/etc/"))

		err = fileutils.MountBind("/etc/resolv.conf", filepath.Join(path, "/etc/resolv.conf"))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("error setting DNS: %w", err)
		}
	}

	for _, mount := range linuxReadWritePaths {
		if fileutils.Exist(mount) {
			logging.LogDebug("mounting host's %s on %s", mount, filepath.Join(path, mount))

			err := fileutils.MountBind(mount, filepath.Join(path, mount))
			if err != nil {
				logging.LogDebug("error: %+v", err)

				return fmt.Errorf("error setting rw mount %s: %w", mount, err)
			}
		}
	}

	// unprivileged has less access to filesystem kernel and so on
	if !conf.Privileged {
		logging.LogDebug("container is not privileged, setting up masked mounts")

		err = setupMaskedMounts(path)
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return err
		}
	}

	return nil
}

// here we setup the custom mounts/volumes specified during creation. Reference
// config is utils.Config.Mounts.
// Specified mounts are in the form of src:dest:mode
// For anonymous mountpoints, we create an empty dir in LILIPOD_HOME/volumes/ID/path.
func setupVolumes(path string, conf utils.Config) error {
	for _, volume := range conf.Mounts {
		if strings.Compare(volume, "") == 0 {
			continue
		}

		mounts := strings.Split(volume, ",")
		// case of --mount type=xxx,source=xxx,destination=xxx,readonly=xxxx,bind-propagation=xxxx
		if len(mounts) > 1 {
			logging.LogDebug("setting up mount %s", volume)

			readonly := false
			destination := ""
			mountType := ""
			propagation := ""
			source := ""

			for _, mount := range mounts {
				if strings.HasPrefix(mount, "type") {
					mountType = strings.Split(mount, "=")[1]
				}

				if strings.HasPrefix(mount, "source") {
					source = strings.Split(mount, "=")[1]
				}

				if strings.HasPrefix(mount, "destination") {
					destination = filepath.Join(path, strings.Split(mount, "=")[1])
				}

				if strings.HasPrefix(mount, "readonly") {
					readonly = true
				}

				if strings.HasPrefix(mount, "bind-propagation") {
					propagation = strings.Split(mount, "=")[1]
				}
			}

			mountprop := 0

			if readonly {
				mountprop |= syscall.MS_RDONLY
			}

			switch propagation {
			case "private":
				mountprop |= syscall.MS_PRIVATE
			case "rprivate":
				mountprop |= syscall.MS_REC | syscall.MS_PRIVATE
			case "rshared":
				mountprop |= syscall.MS_REC | syscall.MS_SHARED
			case "rslave":
				mountprop |= syscall.MS_REC | syscall.MS_SLAVE
			case "shared":
				mountprop |= syscall.MS_SHARED
			case "slave":
				mountprop |= syscall.MS_SLAVE
			}

			switch mountType {
			case "tmpfs":
				err := fileutils.MountTmpfs(destination)
				if err != nil {
					logging.LogDebug("error: %+v", err)

					return fmt.Errorf("error creating tmpfs mount %s: %w", volume, err)
				}
			case "bind":
				mountprop |= syscall.MS_BIND | syscall.MS_REC

				err := fileutils.Mount(source, destination, uintptr(mountprop))
				if err != nil {
					logging.LogDebug("error: %+v", err)

					return fmt.Errorf("error creating bind mount %s: %w", volume, err)
				}
			}

			// mount finished, skip to next entry
			continue
		}

		// here it means it's not a mount but a volume
		logging.LogDebug("setting up volume mount %s", volume)

		mode := "rw"
		mountings := strings.Split(volume, ":")

		// case of --volume a, anonymous mount
		if len(mountings) <= 1 {
			logging.LogDebug("setting up anonymous mount: %s. mounting empty tmps", volume)

			src := filepath.Join(utils.GetLilipodHome(), "volumes", conf.ID, volume)
			dest := filepath.Join(path, volume)

			// we now create the volume in LILIPOD_HOME
			err := os.MkdirAll(src, os.ModePerm)
			if err != nil {
				logging.LogDebug("error: %+v", err)

				return fmt.Errorf("error creating anonyous mount %s: %w", volume, err)
			}

			err = fileutils.MountBind(src, dest)
			if err != nil {
				logging.LogDebug("error: %+v", err)

				return fmt.Errorf("error creating anonyous mount %s: %w", volume, err)
			}

			continue
		}

		// case of --volume a:b:mode
		if len(mountings) > 2 {
			mode = mountings[2]
		}

		modeUint := syscall.MS_BIND | syscall.MS_REC

		if strings.Contains(mode, "rslave") {
			modeUint |= syscall.MS_SLAVE | syscall.MS_REC
		}

		if strings.Contains(mode, "rshared") {
			modeUint |= syscall.MS_SHARED | syscall.MS_REC
		}

		if strings.Contains(mode, "rprivate") {
			modeUint |= syscall.MS_PRIVATE | syscall.MS_REC
		}

		source := mountings[0]
		dest := filepath.Join(path, mountings[1])

		logging.LogDebug("setting up mount: %s on %s as %s", source, dest, mode)

		if !fileutils.Exist(source) {
			logging.LogDebug("path %s does not exist on host", source)

			return fmt.Errorf("path %s does not exist on host", source)
		}

		err := fileutils.Mount(source, dest, uintptr(modeUint))
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("failed to mount %s on %s: %w", source, dest, err)
		}
	}

	return nil
}

// SetupRootfs will set up the rootfs defined in conf into path.
// This will also populate container's /run/.containerenv.
func SetupRootfs(conf utils.Config) error {
	path := GetRootfsDir(conf.ID)
	// this section will make sure that mounts are private in this mount
	// namespace, so that even with root we do not have pending mounts.
	logging.LogDebug("remounting %s as private", path)

	err := syscall.Mount(path, path, "", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting private mount: %s. %v", path, err.Error())
	}

	logging.LogDebug("remounting %s as rprivate", path)

	err = syscall.Mount("", path, "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting private mount: %s. %v", path, err.Error())
	}

	err = syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting private mount: %s. %v", path, err.Error())
	}

	// -----------------------------------------------------------------------

	logging.LogDebug("setting up basic mounts")

	err = setupMounts(path, conf)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	logging.LogDebug("setting up volumes")

	err = setupVolumes(path, conf)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	logging.LogDebug("setting up PTY %s", path)

	// setup the pty,
	err = setupPTY(path)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("setup pty: %w", err)
	}

	logging.LogDebug("populating /run/.containerenv")

	// setting this file ensures compatibility and gives back some info
	infoFile, err := os.Create(filepath.Join(path, "/run/.containerenv"))
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	defer func() { _ = infoFile.Close() }()

	info := fmt.Sprintf(`engine="%s"
name="%s"
id="%s"
image="%s"
imageid="%s"
`, "lilipod-"+constants.Version, conf.Names, GetID(conf.Names), conf.Image, imageutils.GetID(conf.Image))

	_, err = infoFile.WriteString(info)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	return nil
}

// PivotRoot will perform pivot root syscall into path.
func PivotRoot(path string) error {
	// first we set up pivotroot.
	if !fileutils.Exist(path) {
		logging.LogDebug("pivotroot: rootfs %s does not exist", path)

		return fmt.Errorf("pivotroot: rootfs %s does not exist", path)
	}

	logging.LogDebug("initializing pivotroot")

	tmpDir := filepath.Join(path, "/")
	pivotDir := filepath.Join(tmpDir, ".pivot_root")

	logging.LogDebug("pivotroot: mkdir %s", tmpDir)

	_ = os.Remove(tmpDir)
	_ = os.Remove(pivotDir)

	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("pivotroot: can't create tmp dir %s, error %w", tmpDir, err)
	}

	logging.LogDebug("pivotroot: mkdir %s", pivotDir)

	err = os.Mkdir(pivotDir, os.ModePerm)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("pivotroot: can't create pivot_root dir %s, error %w", pivotDir, err)
	}

	logging.LogDebug("pivotroot: pivot from %s to %s", path, pivotDir)

	err = syscall.PivotRoot(path, pivotDir)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("pivotroot: %w", err)
	}

	// path to pivot dir now changed, update
	pivotDir = filepath.Join("/", filepath.Base(pivotDir))

	logging.LogDebug("pivotroot: umount %s", pivotDir)

	err = syscall.Unmount(pivotDir, syscall.MNT_DETACH)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("unmount pivot_root dir %w", err)
	}

	logging.LogDebug("pivotroot: delete %s", pivotDir)

	err = os.Remove(pivotDir)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("cleanup pivot_root dir %w", err)
	}

	return nil
}

// RunContainer will start specified container in path, with tty if enabled.
// This will:
//   - SetupRootfs
//   - PivotRoot
//   - Set Hostname according to input config
//   - Set UID/GID according to input config
//   - execve the entrypoint
func RunContainer(tty bool, conf utils.Config) error {
	// setup mounts and stuff
	logging.LogDebug("setting up rootfs in: %s", GetRootfsDir(conf.ID))

	err := SetupRootfs(conf)
	if err != nil {
		logging.LogError("error: %+v", err)

		return fmt.Errorf("setup rootfs: %w", err)
	}

	err = PivotRoot(GetRootfsDir(conf.ID))
	if err != nil {
		logging.LogError("error: %+v", err)

		return err
	}

	// setup cgrups if private
	if conf.Cgroup != constants.Host {
		err = setupCgroupfs(conf)
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return fmt.Errorf("setup cgroupfs: %w", err)
		}
	}

	logging.LogDebug("chdir to workdir: %s", conf.Workdir)

	err = syscall.Chdir(conf.Workdir)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	logging.LogDebug("setting container hostname to %s", conf.Hostname)

	// then we set up the hostname.
	err = syscall.Sethostname([]byte(conf.Hostname))
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return fmt.Errorf("error setting hostname for namespace: %w", err)
	}

	logging.LogDebug("become user: %s", conf.User)

	// become the user that we're reuired to be
	uid, gid := procutils.GetUIDGID(conf.User)

	err = syscall.Setgid(gid)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	err = syscall.Setuid(uid)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	logging.LogDebug("setting up env variables")

	for _, v := range conf.Env {
		en := strings.Split(v, "=")

		err = os.Setenv(en[0], en[1])
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return err
		}
	}

	command := conf.Entrypoint[0]

	commandPath, err := exec.LookPath(command)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	if err := setCapabilities(keepCaps...); err != nil {
		fmt.Fprintf(os.Stderr, "error setting capabilities for process: %v\n", err)
		os.Exit(1)
	}

	if tty {
		args := append([]string{constants.PtyAgentPath}, conf.Entrypoint...)

		logging.LogDebug("tty requested, execute entrypoint with agent: %s", args)

		return syscall.Exec(constants.PtyAgentPath, args, conf.Env)
	}

	logging.LogDebug("execute entrypoint: %s", conf.Entrypoint)

	return syscall.Exec(commandPath, conf.Entrypoint, conf.Env)
}

var keepCaps = []string{
	"chown",
	"dac_override",
	"fsetid",
	"fowner",
	"mknod",
	"net_raw",
	"setgid",
	"setuid",
	"setfcap",
	"setpcap",
	"net_bind_service",
	"sys_chroot",
	"kill",
	"audit_write",
}

func setCapabilities(keepCaps ...string) error {
	caps, err := capability.NewPid2(0)
	if err != nil {
		return fmt.Errorf("reading capabilities of current process: %w", err)
	}

	knownCapsList := capability.ListKnown()

	for _, capSpec := range keepCaps {
		// nocap
		capToSet := capability.Cap(-1)

		for _, c := range knownCapsList {
			if strings.EqualFold(c.String(), capSpec) {
				capToSet = c

				break
			}
		}

		caps.Set(capability.BOUNDING, capToSet)
		caps.Set(capability.EFFECTIVE, capToSet)
		caps.Set(capability.PERMITTED, capToSet)
	}

	if err = caps.Apply(capability.CAPS | capability.BOUNDS | capability.AMBS); err != nil {
		return fmt.Errorf("setting capabilities: %w", err)
	}

	_, err = capability.NewPid2(0)
	if err != nil {
		return fmt.Errorf("reading capabilities of current process: %w", err)
	}

	return nil
}
