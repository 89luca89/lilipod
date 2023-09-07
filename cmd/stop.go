// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/spf13/cobra"
)

// NewStopCommand will find all the processes in given container and will stop them.
func NewStopCommand() *cobra.Command {
	stopCommand := &cobra.Command{
		Use:              "stop [flags] IMAGE",
		Short:            "Remove one or more containers",
		PreRunE:          logging.Init,
		RunE:             stop,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	stopCommand.Flags().SetInterspersed(false)
	stopCommand.Flags().BoolP("all", "a", false, "stop all running containers")
	stopCommand.Flags().BoolP("force", "f", false, "force stop running container (use SIGKILL instead of SIGTERM)")
	stopCommand.Flags().BoolP("help", "h", false, "show help")
	stopCommand.Flags().IntP("timeout", "t", 10, "seconds to wait before forcefully exiting the container")

	return stopCommand
}

func stop(cmd *cobra.Command, arguments []string) error {
	stopAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	timeout, err := cmd.Flags().GetInt("timeout")
	if err != nil {
		return err
	}

	if len(arguments) < 1 && !stopAll {
		return cmd.Help()
	}

	// if we want to delete all, just get a list of the targets and add it to
	// the arguments.
	if stopAll {
		arguments = []string{}

		containers, err := os.ReadDir(containerutils.ContainerDir)
		if err != nil {
			return err
		}

		for _, i := range containers {
			arguments = append(arguments, i.Name())
		}
	}

	for _, container := range arguments {
		// delete the targets.
		targetDIR := filepath.Join(containerutils.ContainerDir, container)
		if !fileutils.Exist(targetDIR) {
			targetDIR = containerutils.GetDir(container)
		}

		if fileutils.Exist(targetDIR) {
			logging.LogDebug("stopping: %s", container)

			pid, _ := containerutils.GetPid(container)
			if pid < 1 {
				logging.LogDebug("container %s already stopped", container)

				return nil
			}

			configPath := filepath.Join(containerutils.ContainerDir, containerutils.GetID(container), "config")

			config, err := utils.LoadConfig(configPath)
			if err != nil {
				// in case of invalid container, let's cleanup the mess.
				logging.LogWarning("found invalid container %s, cleaning up", container)

				return exec.Command("/proc/self/exe", "rm", container).Run()
			}

			err = containerutils.Stop(container, force, timeout, config.Stopsignal)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("container %s does not exist", container)
		}

		fmt.Println(container)
	}

	return nil
}
