// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"os"

	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/spf13/cobra"
)

// NewRmiCommand will delete an OCI image in the configured DIR.
func NewRmiCommand() *cobra.Command {
	rmiCommand := &cobra.Command{
		Use:              "rmi [flags] IMAGE:TAG",
		Short:            "Removes one or more images from local storage",
		PreRunE:          logging.Init,
		RunE:             rmi,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	rmiCommand.Flags().SetInterspersed(false)
	rmiCommand.Flags().BoolP("all", "a", false, "remove all images")
	rmiCommand.Flags().BoolP("help", "h", false, "show help")

	return rmiCommand
}

func rmi(cmd *cobra.Command, arguments []string) error {
	delAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	if len(arguments) < 1 && !delAll {
		return cmd.Help()
	}

	if delAll {
		return os.RemoveAll(imageutils.ImageDir)
	}

	for _, img := range arguments {
		targetDIR := imageutils.GetPath(img)
		if !fileutils.Exist(targetDIR) {
			return fmt.Errorf("image %s not found", img)
		}

		logging.LogDebug("deleting: %s", img)

		err = os.RemoveAll(targetDIR)
		if err != nil {
			return err
		}

		fmt.Println(img)
	}

	return nil
}
