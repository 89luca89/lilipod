// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/procutils"
	"github.com/89luca89/lilipod/pkg/utils"
	imgName "github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
)

// NewCreateCommand will create a new container environment ready to use.
func NewCreateCommand() *cobra.Command {
	createCommand := &cobra.Command{
		Use:              "create [flags] IMAGE [COMMAND] [ARG...]",
		Short:            "Create but do not start a container",
		PreRunE:          logging.Init,
		RunE:             create,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	createCommand.Flags().SetInterspersed(false)
	createCommand.Flags().Bool("help", false, "show help")
	createCommand.Flags().Bool("privileged", false, "give extended privileges to the container")
	createCommand.Flags().Bool("pull", false, "pull image before running")
	createCommand.Flags().String("cgroupns", constants.Private, "cgroup namespace to use")
	createCommand.Flags().String("entrypoint", "", "overwrite command to execute when starting the container")
	createCommand.Flags().String("ipc", constants.Private, "IPC namespace to use")
	createCommand.Flags().String("name", containerutils.GetRandomName(), "Assign a name to the container")
	createCommand.Flags().String("network", constants.Private, "connect a container to a network")
	createCommand.Flags().String("pid", constants.Private, "pid namespace to use")
	createCommand.Flags().String("time", constants.Private, "time namespace to use")
	createCommand.Flags().String("userns", constants.KeepID, "user namespace to use")
	createCommand.Flags().String("stop-signal", "SIGTERM", "signal to stop the container")
	//nolint:lll
	createCommand.Flags().StringArrayP("env", "e", nil, "set environment variables in container (default [PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin,TERM=xterm])")
	createCommand.Flags().StringArrayP("label", "", nil, "set metadata on container")
	createCommand.Flags().StringArrayP("volume", "v", nil, "bind mount a volume into the container")
	createCommand.Flags().StringArrayP("mount", "", nil, "perform a mount into the container")
	createCommand.Flags().StringP("hostname", "h", "", "set container hostname")
	createCommand.Flags().StringP("user", "u", "root:root", "username or UID (format: <name|uid>[:<group|gid>])")

	// This does nothing, it's here for CLI compatibility with podman/docker
	createCommand.Flags().String("security-opt", "", "")
	createCommand.Flags().String("pids-limit", "", "")
	_ = createCommand.Flags().MarkHidden("security-opt")
	_ = createCommand.Flags().MarkHidden("pids-limit")

	return createCommand
}

func create(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		return cmd.Help()
	}

	success, err := procutils.EnsureFakeRoot(true)
	if err != nil {
		return err
	}

	if success {
		return nil
	}

	pull, err := cmd.Flags().GetBool("pull")
	if err != nil {
		return err
	}

	privileged, err := cmd.Flags().GetBool("privileged")
	if err != nil {
		return err
	}

	hostname, err := cmd.Flags().GetString("hostname")
	if err != nil {
		return err
	}

	ipc, err := cmd.Flags().GetString("ipc")
	if err != nil {
		return err
	}

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}

	network, err := cmd.Flags().GetString("network")
	if err != nil {
		return err
	}

	cgroup, err := cmd.Flags().GetString("cgroupns")
	if err != nil {
		return err
	}

	timens, err := cmd.Flags().GetString("time")
	if err != nil {
		return err
	}

	pid, err := cmd.Flags().GetString("pid")
	if err != nil {
		return err
	}

	user, err := cmd.Flags().GetString("user")
	if err != nil {
		return err
	}

	userns, err := cmd.Flags().GetString("userns")
	if err != nil {
		return err
	}

	stopsignal, err := cmd.Flags().GetString("stop-signal")
	if err != nil {
		return err
	}

	entrypoint, err := cmd.Flags().GetString("entrypoint")
	if err != nil {
		return err
	}

	env, err := cmd.Flags().GetStringArray("env")
	if err != nil {
		return err
	}

	label, err := cmd.Flags().GetStringArray("label")
	if err != nil {
		return err
	}

	volume, err := cmd.Flags().GetStringArray("volume")
	if err != nil {
		return err
	}

	mount, err := cmd.Flags().GetStringArray("mount")
	if err != nil {
		return err
	}

	// default hostname to name if not specified.
	if hostname == "" {
		hostname = name
	}

	image := cmd.Flags().Args()[0]

	if pull {
		logging.LogDebug("pulling image: %s", image)

		_, err := imageutils.Pull(image, false)
		if err != nil {
			return err
		}
	}

	if !fileutils.Exist(imageutils.GetPath(image)) {
		ref, err := imgName.ParseReference(image)
		if err == nil {
			image = ref.Name()
		}
	}

	args := cmd.Flags().Args()[1:]
	if len(args) == 0 {
		args = nil
	}

	configEntrypoint := []string{}
	if entrypoint != "" {
		configEntrypoint = append(configEntrypoint, entrypoint)
	}

	if os.Getenv("ROOTFUL") == constants.TrueString && userns == constants.KeepID {
		return fmt.Errorf("cannot use userns=keep-id in rootful mode, use private for it")
	}

	uid := os.Getenv("PARENT_UID_MAP")
	gid := os.Getenv("PARENT_GID_MAP")

	createConfig := utils.Config{
		ID:         containerutils.GetID(name),
		Env:        env,
		Cgroup:     cgroup,
		Created:    time.Now().Format("2006.01.02 15:04:05"),
		Hostname:   hostname,
		Image:      image,
		Ipc:        ipc,
		Names:      name,
		Network:    network,
		Pid:        pid,
		Privileged: privileged,
		Time:       timens,
		User:       user,
		Userns:     userns,
		Workdir:    "/",
		Stopsignal: stopsignal,
		Mounts:     append(mount, volume...),
		Labels:     utils.ListToMap(label),
		// entry point related
		Entrypoint: append(configEntrypoint, args...),
	}

	if fileutils.Exist(filepath.Join(containerutils.GetDir(name), "config")) {
		return fmt.Errorf("container %s already exists", name)
	}

	logging.LogDebug("preparing rootfs for: %s", name)

	err = containerutils.CreateRootfs(image, name, createConfig, uid, gid)
	if err != nil {
		return err
	}

	fmt.Println(containerutils.GetID(name))

	return nil
}
