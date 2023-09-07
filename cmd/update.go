// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/spf13/cobra"
)

// NewUpdateCommand will update a new container environment ready to use.
func NewUpdateCommand() *cobra.Command {
	updateCommand := &cobra.Command{
		Use:              "update [flags] IMAGE [COMMAND] [ARG...]",
		Short:            "Update but do not start a container",
		PreRunE:          logging.Init,
		RunE:             update,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	updateCommand.Flags().SetInterspersed(false)
	updateCommand.Flags().Bool("config-dump", false, "show configuration for container")
	updateCommand.Flags().Bool("help", false, "show help")
	updateCommand.Flags().Bool("config-reset", false, "reset blank configuration for container")
	updateCommand.Flags().String("cgroup", "", "cgroup namespace to use")
	updateCommand.Flags().String("entrypoint", "", "overwrite command to execute when starting the container")
	updateCommand.Flags().String("ipc", "", "IPC namespace to use")
	updateCommand.Flags().String("network", "", "connect a container to a network")
	updateCommand.Flags().String("pid", "", "pid namespace to use")
	updateCommand.Flags().String("privileged", "", "Give extended privileges to the container")
	updateCommand.Flags().String("time", "", "time namespace to use")
	updateCommand.Flags().String("userns", "", "user namespace to use")
	//nolint:lll
	updateCommand.Flags().StringArrayP("env", "e", nil, "set environment variables in container (default [PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin,TERM=xterm])")
	updateCommand.Flags().StringArrayP("label", "", nil, "set metadata on container")
	updateCommand.Flags().StringArrayP("volume", "v", nil, "bind mount a volume into the container")
	updateCommand.Flags().StringP("hostname", "h", "", "set container hostname")

	return updateCommand
}

func update(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		return cmd.Help()
	}

	container := cmd.Flags().Args()[0]

	dump, err := cmd.Flags().GetBool("config-dump")
	if err != nil {
		return err
	}

	reset, err := cmd.Flags().GetBool("config-reset")
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

	network, err := cmd.Flags().GetString("network")
	if err != nil {
		return err
	}

	cgroup, err := cmd.Flags().GetString("cgroup")
	if err != nil {
		return err
	}

	time, err := cmd.Flags().GetString("time")
	if err != nil {
		return err
	}

	pid, err := cmd.Flags().GetString("pid")
	if err != nil {
		return err
	}

	privileged, err := cmd.Flags().GetString("privileged")
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

	if !fileutils.Exist(filepath.Join(containerutils.GetDir(container), "config")) {
		return fmt.Errorf("container %s does not exist", container)
	}

	configfile, err := fileutils.ReadFile(filepath.Join(containerutils.GetDir(container), "config"))
	if err != nil {
		logging.LogError("%+v", err)

		return err
	}

	config, err := utils.InitConfig(configfile)
	if err != nil {
		logging.LogError("%+v", err)

		return err
	}

	if dump {
		fmt.Println(string(configfile))

		return nil
	}

	if containerutils.IsRunning(container) {
		return fmt.Errorf("container %s is running, stop it first", container)
	}

	if reset {
		logging.LogDebug("resetting container %s to default config", config.Names)

		defConf := utils.GetDefaultConfig()
		defConf.ID = config.ID
		defConf.Names = config.Names
		defConf.Image = config.Image
		defConf.Hostname = config.Hostname
		defConf.Userns = config.Userns
		defConf.ID = containerutils.GetID(container)

		return utils.SaveConfig(defConf, filepath.Join(containerutils.GetDir(container), "config"))
	}

	if cmd.Flags().Lookup("entrypoint").Changed {
		config.Entrypoint = strings.Split(entrypoint, " ")
	}

	if cmd.Flags().Lookup("privileged").Changed {
		config.Privileged, err = strconv.ParseBool(privileged)
		if err != nil {
			return err
		}
	}

	if cmd.Flags().Lookup("ipc").Changed {
		config.Ipc = ipc
	}

	if cmd.Flags().Lookup("network").Changed {
		config.Network = network
	}

	if cmd.Flags().Lookup("cgroup").Changed {
		config.Cgroup = cgroup
	}

	if cmd.Flags().Lookup("time").Changed {
		config.Time = time
	}

	if cmd.Flags().Lookup("pid").Changed {
		config.Pid = pid
	}

	if cmd.Flags().Lookup("userns").Changed {
		return fmt.Errorf("userns cannot be changed after creation")
	}

	if cmd.Flags().Lookup("hostname").Changed {
		config.Hostname = hostname
	}

	if cmd.Flags().Lookup("env").Changed {
		config.Env = env
	}

	if cmd.Flags().Lookup("volume").Changed {
		config.Mounts = volume
	}

	if cmd.Flags().Lookup("label").Changed {
		config.Labels = label
	}

	logging.LogDebug(
		"saving config to %s",
		filepath.Join(containerutils.GetDir(container), "config"),
	)

	err = utils.SaveConfig(config, filepath.Join(containerutils.GetDir(container), "config"))
	if err != nil {
		return err
	}

	logging.LogDebug("configured %s successfully", container)
	logging.LogWarning("please stop %s and start again to take effect", container)

	fmt.Println(container)

	return nil
}
