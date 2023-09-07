// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/spf13/cobra"
)

// NewExecCommand will execute a command inside a running container.
func NewExecCommand() *cobra.Command {
	execCommand := &cobra.Command{
		Use:              "exec [flags] IMAGE [COMMAND] [ARG...]",
		Short:            "Exec but do not start a container",
		PreRunE:          logging.Init,
		RunE:             execute,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	execCommand.Flags().SetInterspersed(false)
	execCommand.Flags().BoolP("detach", "d", false, "run the exec session in detached mode (backgrounded)")
	execCommand.Flags().BoolP("help", "h", false, "show help")
	execCommand.Flags().BoolP("interactive", "i", false, "keep STDIN open even if not attached")
	execCommand.Flags().BoolP("tty", "t", false, "allocate a pseudo-TTY. The default is false")
	//nolint:lll
	execCommand.Flags().StringArrayP("env", "e", nil, "set environment variables in container (default [PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin,TERM=xterm])")
	execCommand.Flags().StringP("user", "u", "root:root", "username or UID (format: <name|uid>[:<group|gid>])")
	execCommand.Flags().StringP("workdir", "w", "/", "working directory inside the container")

	// This does nothing, it's here for CLI compatibility with podman/docker
	execCommand.Flags().String("detach-keys", "", "")

	return execCommand
}

func execute(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		return cmd.Help()
	}

	detach, err := cmd.Flags().GetBool("detach")
	if err != nil {
		return err
	}

	interactive, err := cmd.Flags().GetBool("interactive")
	if err != nil {
		return err
	}

	tty, err := cmd.Flags().GetBool("tty")
	if err != nil {
		return err
	}

	user, err := cmd.Flags().GetString("user")
	if err != nil {
		return err
	}

	workdir, err := cmd.Flags().GetString("workdir")
	if err != nil {
		return err
	}

	env, err := cmd.Flags().GetStringArray("env")
	if err != nil {
		return err
	}

	env = append(
		[]string{"TERM=xterm", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		env...)

	container := cmd.Flags().Args()[0]
	entrypoint := cmd.Flags().Args()[1:]

	if len(entrypoint) == 0 {
		return fmt.Errorf("entrypoint command empty, please specify one")
	}

	if interactive || tty {
		detach = false
	}

	if detach {
		interactive = false
		tty = false
	}

	if !fileutils.Exist(containerutils.GetDir(container)) {
		return fmt.Errorf("container %s does not exist", container)
	}

	// ensure a container for this name is already running
	containerPid, err := containerutils.GetPid(container)
	if err != nil || containerPid < 1 {
		return fmt.Errorf("container %s is not running", container)
	}

	configPath := filepath.Join(containerutils.GetDir(container), "config")
	if fileutils.Exist(configPath) {
		config, err := utils.LoadConfig(configPath)
		if err != nil {
			return err
		}

		logging.LogDebug("entering: %s", container)

		config.User = user
		config.Entrypoint = entrypoint
		config.Env = append(config.Env, env...)
		config.Workdir = workdir

		err = containerutils.Exec(containerPid, interactive, tty, config)
		if err != nil {
			return err
		}

		return nil
	}

	return nil
}
