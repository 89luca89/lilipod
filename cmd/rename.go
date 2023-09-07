// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/procutils"
	"github.com/spf13/cobra"
)

// NewRenameCommand will copy a file to or from a container.
func NewRenameCommand() *cobra.Command {
	renameCommand := &cobra.Command{
		Use:              "rename OLD_NAME NEW_NAME",
		Short:            "Rename a container",
		PreRunE:          logging.Init,
		RunE:             rename,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	renameCommand.Flags().SetInterspersed(false)

	return renameCommand
}

func rename(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 2 {
		return cmd.Help()
	}

	success, err := procutils.EnsureFakeRoot(true)
	if err != nil {
		return err
	}

	if success {
		return nil
	}

	container := arguments[0]
	newName := arguments[1]

	return containerutils.Rename(container, newName)
}
