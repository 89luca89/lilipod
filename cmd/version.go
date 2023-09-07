// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/utils"
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
	out, err := exec.Command(filepath.Join(utils.LilipodBinPath, "pty"), "version").Output()
	if err != nil {
		log.Printf("%s: %v", string(out), err)

		return err
	}

	fmt.Printf("lilipod version: %s\n", constants.Version)
	fmt.Printf("lilipod pty agent version: %s\n", string(out))

	return nil
}
