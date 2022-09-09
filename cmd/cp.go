package cmd

import (
	"fmt"
	"strings"

	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

func cpHelp(*cobra.Command) error {
	help := `Description:
  Copy the contents of SRC_PATH to the DEST_PATH.

  You can copy from the chroot's file system to the local machine or the reverse, from the local filesystem to the chroot. If "-" is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT. The chroot can be a running or stopped chroot. The SRC_PATH or DEST_PATH can be a file or a directory.


Usage:
  scatman cp [options] [chroot:]SRC_PATH [chroot:]DEST_PATH

Examples:
  scatman cp [options] [chroot:]SRC_PATH [chroot:]DEST_PATH`

	fmt.Println(help)

	return nil
}

// NewCpCommand will delete an OCI image in the configured DIR.
func NewCpCommand() *cobra.Command {
	var cpCommand = &cobra.Command{
		Use:              "cp [options] [chroot:]SRC_PATH [chroot:]DEST_PATH",
		Short:            "Copy files/folders between a chroot and the local filesystem",
		RunE:             cp,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	cpCommand.SetUsageFunc(cpHelp)
	cpCommand.Flags().SetInterspersed(false)

	return cpCommand
}

func cp(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 2 {
		cmd.Help()

		return nil
	}

	success, err := utils.EnsureRootlesskit(true)
	if err != nil {
		return err
	}

	if success {
		return nil
	}

	src := arguments[0]
	dest := arguments[1]

	if strings.Contains(src, ":") {
		chroot := strings.Split(src, ":")[0]
		file := strings.Split(src, ":")[1]

		src = utils.GetRootfsDir(chroot) + file
	}

	if strings.Contains(dest, ":") {
		chroot := strings.Split(dest, ":")[0]
		file := strings.Split(dest, ":")[1]

		dest = utils.GetRootfsDir(chroot) + file
	}

	return utils.CopyFile(src, dest)
}
