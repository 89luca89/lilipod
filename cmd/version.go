// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/spf13/cobra"
)

// NewVersionCommand will delete an OCI image in the configured DIR.
func NewVersionCommand() *cobra.Command {
	versionCommand := &cobra.Command{
		Use:              "version",
		Short:            "Show lilipod version",
		RunE:             version,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	versionCommand.Flags().SetInterspersed(false)

	return versionCommand
}

func version(_ *cobra.Command, _ []string) error {
	fmt.Printf("lilipod version: %s\n", constants.Version)

	return nil
}
