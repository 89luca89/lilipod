// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/procutils"
	"github.com/spf13/cobra"
)

// NewCpCommand will copy a file to or from a container.
func NewCpCommand() *cobra.Command {
	cpCommand := &cobra.Command{
		Use:              "cp [options] [container:]SRC_PATH [container:]DEST_PATH",
		Short:            "Copy files/folders between a container and the local filesystem",
		PreRunE:          logging.Init,
		RunE:             cp,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	cpCommand.Flags().SetInterspersed(false)

	return cpCommand
}

func cp(cmd *cobra.Command, arguments []string) error {
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

	src := arguments[0]
	dest := arguments[1]

	if strings.Contains(src, ":") {
		container := strings.Split(src, ":")[0]
		file := strings.Split(src, ":")[1]

		src = filepath.Join(containerutils.GetRootfsDir(container), file)

		if !fileutils.Exist(containerutils.GetDir(container)) {
			return fmt.Errorf("container %s does not exist", container)
		}
	}

	if strings.Contains(dest, ":") {
		container := strings.Split(dest, ":")[0]
		file := strings.Split(dest, ":")[1]

		dest = filepath.Join(containerutils.GetRootfsDir(container), file)

		if !fileutils.Exist(containerutils.GetDir(container)) {
			return fmt.Errorf("container %s does not exist", container)
		}
	}

	return fileutils.CopyFileContainer(src, dest)
}
