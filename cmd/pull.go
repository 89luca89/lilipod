// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"

	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/spf13/cobra"
)

// NewPullCommand will pull a new OCI image from a registry.
func NewPullCommand() *cobra.Command {
	pullCommand := &cobra.Command{
		Use:              "pull [flags] IMAGE:TAG",
		Short:            "Pull an image from a registry",
		PreRunE:          logging.Init,
		RunE:             pull,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	pullCommand.Flags().SetInterspersed(false)
	pullCommand.Flags().BoolP("help", "h", false, "show help")
	pullCommand.Flags().BoolP("quiet", "q", false, "suppress output")

	return pullCommand
}

// Pull will download an OCI image in the configured DIR.
func pull(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		return cmd.Help()
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}

	for _, image := range arguments {
		id, err := imageutils.Pull(image, quiet)
		if err != nil {
			return err
		}

		fmt.Println(id)
	}

	return nil
}
