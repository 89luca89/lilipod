// Package containerutils contains helpers and utilities for managing and creating
// containers.
package containerutils

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/procutils"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/google/go-containerregistry/pkg/legacy"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// ContainerDir is the default location for downloaded images.
var ContainerDir = filepath.Join(utils.GetLilipodHome(), "containers")

// GetRandomName returns a 12 string char of random characters.
// Generated name will be like example_test12.
func GetRandomName() string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	string1 := make([]rune, 6)
	for i := range string1 {
		randInt, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		string1[i] = letters[randInt.Int64()]
	}

	string2 := make([]rune, 6)
	for i := range string2 {
		randInt, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		string2[i] = letters[randInt.Int64()]
	}

	return string(string1) + "_" + string(string2)
}

// GetID returns the md5sum based ID for given container.
// If a recognized ID is passed, it is returned.
func GetID(name string) string {
	if fileutils.Exist(filepath.Join(ContainerDir, name)) {
		return name
	}

	hasher := md5.New()

	_, err := io.WriteString(hasher, name)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// GetDir returns the path on the filesystem where container's rootfs and config is located.
func GetDir(name string) string {
	return filepath.Join(ContainerDir, GetID(name))
}

// GetRootfsDir returns the path on the filesystem where container's rootfs is located.
func GetRootfsDir(name string) string {
	return filepath.Join(GetDir(name), "rootfs")
}

// GetPid will return the pid of the process running the container with input id.
func GetPid(id string) (int, error) {
	id = GetID(id)
	idb := []byte(id)

	processes, err := os.ReadDir("/proc")
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return -1, err
	}

	// manually find in /proc a process that has "lilipod enter" and "id" in cmdline
	for _, proc := range processes {
		mapfile := filepath.Join("/proc", proc.Name(), "/root/run/.containerenv")

		filedata, err := fileutils.ReadFile(mapfile)
		if err != nil {
			continue
		}

		// if the maps file contains the ID of the container, we found it
		if bytes.Contains(filedata, idb) {
			pid, err := strconv.ParseInt(proc.Name(), 10, 32)
			if err != nil {
				return -1, err
			}

			return int(pid), nil
		}
	}

	return -1, fmt.Errorf("container %s is not running", id)
}

// IsRunning returns whether the container name or id is running or not.
func IsRunning(name string) bool {
	pid, err := GetPid(name)

	return pid > 0 && err == nil
}

// GetContainerInfo returns the Config of input container's name or id.
// Additional input variables can be used for filters and size info.
func GetContainerInfo(
	container string,
	size bool,
	filters map[string]string,
) (*utils.Config, error) {
	configPath := filepath.Join(ContainerDir, container, "config")
	directorySize := ""
	state := "stopped"

	config, err := utils.LoadConfig(configPath)
	if err != nil {
		// in case of invalid container, let's cleanup the mess.
		logging.LogWarning("found invalid container %s, cleaning up", container)

		return nil, exec.Command("/proc/self/exe", "rm", container).Run()
	}

	if !filterContainer(config, filters) {
		// this container does not match any filter, return nil, and no errors.
		//nolint: nilnil
		return nil, nil
	}

	isRunning := IsRunning(config.Names)
	if isRunning {
		state = "running"
	}

	if size {
		directorySize, err = fileutils.DiscUsageMegaBytes(filepath.Join(ContainerDir, container))
		if err != nil {
			return nil, err
		}
	}

	config.Status = state
	config.Size = directorySize

	return &config, nil
}

// CreateRootfs will generate a chrootable rootfs from input oci image reference, with input name and config.
// If input image is not found it will be automatically pulled.
// This function will read the oci-image manifest and properly unpack the layers in the right order to generate
// a valid rootfs.
// Untarring process will follow the keep-id option if specified in order to ensure no permission problems.
// Generated config will be saved inside the container's dir. This will NOT be an oci-compatible container config.
func CreateRootfs(image string, name string, createConfig utils.Config, uid, gid string) error {
	logging.LogDebug("preparing rootfs for new container %s", name)

	containerDIR := GetRootfsDir(name)

	logging.LogDebug("creating %s", containerDIR)

	err := os.MkdirAll(containerDIR, os.ModePerm)
	if err != nil {
		return err
	}

	logging.LogDebug("looking up image %s", image)

	imageDir := imageutils.GetPath(image)
	if !fileutils.Exist(imageDir) {
		_, err := imageutils.Pull(image, false)
		if err != nil {
			return err
		}
	}

	logging.LogDebug("reading %s's manifest", image)

	// get manifest
	manifestFile, err := fileutils.ReadFile(filepath.Join(imageDir, "manifest.json"))
	if err != nil {
		return err
	}

	var manifest v1.Manifest

	err = json.Unmarshal(manifestFile, &manifest)
	if err != nil {
		return err
	}

	logging.LogDebug("extracting image's layers")

	for _, layer := range manifest.Layers {
		layerDigest := strings.Split(layer.Digest.String(), ":")[1] + ".tar.gz"

		logging.LogDebug("extracting layer %s in %s", layerDigest, containerDIR)

		err = fileutils.UntarFile(
			filepath.Join(imageDir, layerDigest),
			containerDIR,
			createConfig.Userns,
		)
		if err != nil {
			return err
		}
	}

	logging.LogDebug("populating default config.json")

	// get default config
	// this is useful in case we need to setup defaults like env and entrypoint
	configFile, err := fileutils.ReadFile(filepath.Join(imageDir, "config.json"))
	if err != nil {
		return err
	}

	var config legacy.LayerConfigFile

	err = json.Unmarshal(configFile, &config)
	if err != nil {
		return err
	}

	logging.LogDebug("setting up custom configs")

	logging.LogDebug("appending custom env to default image env")
	// append custom env to default image env
	createConfig.Env = append(createConfig.Env, config.Config.Env...)
	createConfig.Env = append(createConfig.Env, "HOSTNAME="+createConfig.Hostname)
	createConfig.Env = append(createConfig.Env, "TERM=xterm")

	// if empty entrypoint, default to image default entrypoint
	if len(createConfig.Entrypoint) == 0 || createConfig.Entrypoint == nil {
		logging.LogDebug("entrypoint not specified, fallbacking to default one in image manifest")

		createConfig.Entrypoint = config.Config.Cmd
	}

	createConfig.Uidmap = uid
	createConfig.Gidmap = gid

	// save the config to file
	configPath := filepath.Join(GetDir(name), "config")

	logging.LogDebug("saving config")

	err = utils.SaveConfig(createConfig, configPath)
	if err != nil {
		return err
	}

	logging.LogDebug("done")

	return nil
}

// Rename will change the name of oldContainer to newContainer.
func Rename(oldContainer string, newContainer string) error {
	logging.LogDebug("extracting IDs")

	logging.LogDebug("checking if old container %s exists", oldContainer)

	if !fileutils.Exist(GetDir(oldContainer)) {
		logging.LogError("container %s does not exist", oldContainer)

		return fmt.Errorf("container %s does not exist", oldContainer)
	}

	logging.LogDebug("checking if new container %s does not already exist", newContainer)

	if fileutils.Exist(GetDir(newContainer)) {
		logging.LogError(
			"destination name %s for container %s already exists",
			newContainer,
			oldContainer,
		)

		return fmt.Errorf(
			"destination name %s for container %s already exists",
			newContainer,
			oldContainer,
		)
	}

	logging.LogDebug("renaming %s to %s, moving %s to %s",
		oldContainer, newContainer, GetDir(oldContainer), GetDir(newContainer))

	err := os.Rename(GetDir(oldContainer), GetDir(newContainer))
	if err != nil {
		logging.LogError("cannot rename %s to %s, error: %v", oldContainer, newContainer, err)

		return fmt.Errorf("cannot rename %s to %s, error: %w", oldContainer, newContainer, err)
	}

	logging.LogDebug("adjusting config to reflect new name %s", newContainer)

	config, err := utils.LoadConfig(filepath.Join(GetDir(newContainer), "config"))
	if err != nil {
		logging.LogError("%+v", err)

		return err
	}

	config.Names = newContainer
	config.ID = GetID(newContainer)
	config.Created = time.Now().Format("2006.01.02 15:04:05")

	logging.LogDebug("saving config for %s", newContainer)

	return utils.SaveConfig(config, filepath.Join(GetDir(newContainer), "config"))
}

// Start will enter the target container.
// If tty is specified, the container will be started in interactive mode with full shell.
// If interactive only is specified, container will be started in interactive mode, but only stdin will be forwarded.
// Else the container will be started in background and all output will be saved in the logs.
func Start(interactive, tty bool, config utils.Config) error {
	logging.LogDebug("entering container")

	path := GetRootfsDir(config.ID)

	logging.LogDebug("searching pty agent")

	ptyFile, err := fileutils.ReadFile(filepath.Join(utils.LilipodBinPath, "pty"))
	if err != nil {
		logging.LogError("failed to read pty agent: %v", err)

		return err
	}

	logging.LogDebug("injecting pty agent")

	err = fileutils.WriteFile(filepath.Join(path, constants.PtyAgentPath), ptyFile, 0o755)
	if err != nil {
		logging.LogError("failed to inject pty agent: %v", err)

		return err
	}

	logging.LogDebug("pty agent injected")

	if !fileutils.Exist(filepath.Join(path, constants.PtyAgentPath)) {
		logging.LogError(
			"failed to inject agent in %s",
			filepath.Join(path, constants.PtyAgentPath),
		)

		return fmt.Errorf(
			"failed to inject agent in %s",
			filepath.Join(path, constants.PtyAgentPath),
		)
	}

	logging.LogDebug("ready to start the container")

	logging.LogDebug("ready to start the container")

	cmd, err := generateEnterCommand(config)
	if err != nil {
		logging.LogError("failed to generate enter cmd: %v", err)

		return err
	}

	logging.LogDebug("container is starting with %+v", cmd.SysProcAttr)

	logging.LogDebug("starting the container, executing %v", cmd.Args)

	// tty = interactive+tty, we allocate a terminal and pass it
	if tty {
		cmd.Args = append(cmd.Args, "--tty")

		return procutils.RunWithTTY(cmd)
	}

	// in case we want interactive mode, but no tty
	// just run the command and exchange outputs
	if interactive && !tty {
		return procutils.RunInteractive(cmd)
	}

	logfile := filepath.Join(path, "../current-logs")

	return procutils.RunDetached(cmd, logfile)
}

// Exec will enter the namespace of target container and execute the command needed.
// This function will setup an nsenter command, that will connect to the container's namespace.
func Exec(pid int, interactive bool, tty bool, config utils.Config) error {
	containerPid := strconv.Itoa(pid)

	logging.LogDebug("entering namespace of pid: %s", containerPid)
	logging.LogDebug("setting up nsenter flags")

	cmd := generateExecCommand(containerPid, tty, config)
	if tty {
		return procutils.RunWithTTY(cmd)
	}

	logging.LogDebug("tty not requested, setting up command pipes")

	// in case we want interactive mode, but no tty
	// just run the command and exchange outputs
	if interactive && !tty {
		return procutils.RunInteractive(cmd)
	}

	logfile := filepath.Join(GetDir(config.Names), "current-logs")

	return procutils.RunDetached(cmd, logfile)
}

// Stop will find all the processes in given container and will stop them.
func Stop(name string, force bool, timeout int, signal string) error {
	logging.LogDebug("stopping container %s", name)

	if signal == "" {
		signal = "SIGTERM"
	}

	containerPid, err := GetPid(name)
	if err != nil {
		return err
	}

	logging.LogDebug("container pid is %d", containerPid)
	logging.LogDebug("terminating pid: %d", containerPid)

	if force {
		logging.LogDebug("killing process with pid: %d", containerPid)

		return exec.Command("kill", "-SIGKILL", strconv.Itoa(containerPid)).Run()
	}

	logging.LogDebug("sending SIGTERM to pid: %d", containerPid)

	err = exec.Command("kill", "-"+signal, strconv.Itoa(containerPid)).Run()
	if err != nil {
		return err
	}

	for {
		if timeout <= 0 {
			logging.LogWarning("timeout exceeded, force killing")

			return exec.Command("kill", "-SIGKILL", strconv.Itoa(containerPid)).Run()
		}

		time.Sleep(time.Second)

		containerPid, _ = GetPid(name)
		if containerPid < 1 {
			break
		}

		timeout--
	}

	return nil
}

// Inspect will return a JSON or a formatted string describing the input containers.
func Inspect(containers []string, size bool, format string) (string, error) {
	result := ""

	for _, container := range containers {
		container = GetID(container)

		configPath := filepath.Join(ContainerDir, container, "config")

		config, err := utils.LoadConfig(configPath)
		if err != nil {
			return "", err
		}

		config.Status = "stopped"

		if IsRunning(config.Names) {
			config.Status = "running"
		}

		if size {
			directorySize, err := fileutils.DiscUsageMegaBytes(
				filepath.Join(ContainerDir, container),
			)
			if err != nil {
				return "", err
			}

			config.Size = directorySize
		}

		// Go-template string
		if format != "" {
			tmpl, err := template.New("format").Parse(format)
			if err != nil {
				return "", err
			}

			var out bytes.Buffer

			err = tmpl.Execute(&out, config)
			if err != nil {
				return "", err
			}

			result += out.String()

			continue
		}
		// else we do json dump

		out, err := json.MarshalIndent(config, " ", " ")
		if err != nil {
			return "", err
		}

		result += string(out) + "\n"
	}

	return result, nil
}

// ----------------------------------------------------------------------------

// generateEnterCommand will generate a "lilipod enter" command to be executed.
// this command will respect the container's namespace configuration and will
// let you execute an entrypoint in target rootfs and namespace.
//
// This process will already be inside a new namespace for each private namespace
// specified in the container's config.
// This process will also have user-uid/gid mapping performed respecting the container's
// specified config.
//
// Example nsenter command:
//
// /proc/self/exe --log-level debug enter--config "{JSON_OF_CONFIG}" --tty
//
// Output command is ready to be executed.
func generateEnterCommand(config utils.Config) (*exec.Cmd, error) {
	logging.LogDebug("validating config")

	configArg, err := json.Marshal(config)
	if err != nil {
		return nil, errors.New("invalid config")
	}

	// this is our child process that will enter the container effectively
	cmd := exec.Command("/proc/self/exe",
		"--log-level", logging.GetLogLevel(),
		"enter",
		"--config", string(configArg))

	cloneFlags := syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS

	if config.Userns == constants.KeepID &&
		os.Getenv("ROOTFUL") != constants.TrueString {
		cloneFlags |= syscall.CLONE_NEWUSER
	}

	if config.Ipc == constants.Private {
		cloneFlags |= syscall.CLONE_NEWIPC
	}

	if config.Network == constants.Private {
		cloneFlags |= syscall.CLONE_NEWNET
	}

	if config.Pid == constants.Private {
		cloneFlags |= syscall.CLONE_NEWPID
	}

	if config.Cgroup == constants.Private {
		cloneFlags |= syscall.CLONE_NEWCGROUP
	}

	if config.Time == constants.Private {
		cloneFlags |= syscall.CLONE_NEWTIME
	}

	// Set Namespaces with generated value, this value will keep-id with the host.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential:                 &syscall.Credential{Uid: 0, Gid: 0},
		Cloneflags:                 uintptr(cloneFlags),
		GidMappingsEnableSetgroups: true,

		Setsid:     true,
		Foreground: false,
		Pdeathsig:  syscall.SIGTERM,
	}

	if config.Userns == constants.KeepID &&
		os.Getenv("ROOTFUL") != constants.TrueString {
		logging.LogDebug("setting up uidmaps")

		uidMaps := config.Uidmap
		if uidMaps == "" {
			logging.LogWarning("cannot find uidMaps, defaulting to 1000:100000:65536")

			uidMaps = "1000:100000:65536"
		}

		logging.LogDebug("setting up gidmaps")

		gidMaps := config.Gidmap
		if gidMaps == "" {
			logging.LogWarning("cannot find gidMaps, defaulting to 1000:100000:65536")

			gidMaps = "1000:100000:65536"
		}

		logging.LogDebug("keep-id passed, setting process UID/GID maps")

		err := procutils.SetProcessKeepIDMaps(cmd, uidMaps, gidMaps)
		if err != nil {
			return nil, err
		}
	}

	return cmd, nil
}

// generateExecCommand will generate an nsenter command to be executed.
// this command will respect the container's namespace configuration and will
// let you execute an entrypoint in target namespace.
//
// Example nsenter command:
//
//	nsenter -m -u -U --preserve-credentials -i -n -p -S 1000 -G 1000 \
//	    -r/proc/11111/root -w/proc/11111/root/tmp/ -t 11111 command-to-run
func generateExecCommand(containerPid string, tty bool, config utils.Config) *exec.Cmd {
	args := []string{"-m", "-u", "-U", "--preserve-credentials"}

	if config.Ipc == constants.Private {
		args = append(args, "-i")
	}

	if config.Network == constants.Private {
		args = append(args, "-n")
	}

	if config.Pid == constants.Private {
		args = append(args, "-p")
	}

	uid, gid := procutils.GetUIDGID(config.User)

	args = append(args, []string{"-S", strconv.FormatInt(int64(uid), 10)}...)
	args = append(args, []string{"-G", strconv.FormatInt(int64(gid), 10)}...)
	args = append(args, []string{"-r" + filepath.Join("/proc", containerPid, "root")}...)
	args = append(
		args,
		[]string{"-w" + filepath.Join("/proc", containerPid, "root", config.Workdir)}...)
	args = append(args, []string{"-t", containerPid}...)

	logging.LogDebug("nsenter flags: %v", args)

	if tty {
		logging.LogDebug(
			"tty requested, execute command with agent: %s %v",
			constants.PtyAgentPath,
			config.Entrypoint,
		)

		args = append(args, []string{constants.PtyAgentPath}...)
	}

	args = append(args, config.Entrypoint...)

	logging.LogDebug("executing nsenter: %s %v", "nsenter", args)

	cmd := exec.Command("nsenter", args...)
	cmd.Env = config.Env

	return cmd
}

// filterContainer will return true if a specified container's config respects
// the input filter. False otherwise.
func filterContainer(config utils.Config, filters map[string]string) bool {
	if len(filters) == 0 {
		logging.LogDebug("no filter specified, return always true")

		return true
	}

	matched := 0
	filterLen := len(filters)

	for name, filter := range filters {
		switch name {
		case "label":
			labels := strings.Split(filter, constants.FilterSeparator)
			for _, containerLabel := range config.Labels {
				for _, filterLabel := range labels {
					logging.LogDebug("filtering label: %s, %s", containerLabel, filterLabel)

					if containerLabel == filterLabel {
						matched++
					}
				}
			}
		case "status":
			logging.LogDebug("filtering status: %s, %s", config.Status, filter)

			if config.Status == filter {
				matched++
			}
		case "name":
			logging.LogDebug("filtering names: %s, %s", config.Names, filter)

			if config.Names == filter {
				matched++
			}
		case "id":
			logging.LogDebug("filtering IDs: %s, %s", config.ID, filter)

			if config.ID == filter {
				matched++
			}
		default:
			logging.LogWarning("invalid filter %s, skipping", name)
			logging.LogWarning("valid filters are: label, status, name, id")
		}
	}

	return matched >= filterLen
}
