// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/procutils"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/spf13/cobra"
)

// NewStartCommand will start one or more containers in input with default entrypoint command.
func NewStartCommand() *cobra.Command {
	startCommand := &cobra.Command{
		Use:              "start [flags] IMAGE",
		Short:            "Start one or more containers",
		PreRunE:          logging.Init,
		RunE:             start,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	startCommand.Flags().SetInterspersed(false)
	startCommand.Flags().BoolP("all", "a", false, "start all containers regardless of their state or configuration")
	startCommand.Flags().BoolP("help", "h", false, "show help")
	startCommand.Flags().BoolP("interactive", "i", false, "keep process in foreground")
	startCommand.Flags().BoolP("tty", "t", false, "allocate a pseudo-TTY. The default is false")

	return startCommand
}

func start(cmd *cobra.Command, arguments []string) error {
	interactive, err := cmd.Flags().GetBool("interactive")
	if err != nil {
		return err
	}

	tty, err := cmd.Flags().GetBool("tty")
	if err != nil {
		return err
	}

	parent, err := procutils.EnsureFakeRoot(interactive)
	if err != nil {
		return err
	}

	if parent {
		return nil
	}

	startAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	if len(arguments) < 1 && !startAll {
		return cmd.Help()
	}

	// if we want to delete all, just get a list of the targets and add it to
	// the arguments.
	if startAll {
		arguments = []string{}

		images, err := os.ReadDir(imageutils.ImageDir)
		if err != nil {
			return err
		}

		for _, i := range images {
			arguments = append(arguments, i.Name())
		}
	}

	for _, container := range arguments {
		// ensure a container for this name is already running
		if containerutils.IsRunning(container) {
			return fmt.Errorf("container %s is already running", container)
		}

		targetDIR := filepath.Join(containerutils.ContainerDir, container)
		if !fileutils.Exist(targetDIR) {
			targetDIR = containerutils.GetDir(container)
		}

		// save the config to file
		configPath := filepath.Join(targetDIR, "config")
		if fileutils.Exist(configPath) {
			config, err := utils.LoadConfig(configPath)
			if err != nil {
				return err
			}

			logging.LogDebug("starting: %s", container)

			err = containerutils.Start(interactive, tty, config)
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
