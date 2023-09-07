// Package main is the main package, nothing much here, just:
//   - setup of the environment
//   - setup of cobra
package main

import (
	_ "embed"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/89luca89/lilipod/cmd"
	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/spf13/cobra"
)

//go:embed pty.tar.gz
var pty []byte

func newApp() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:              "lilipod",
		Short:            "Manage containers and images",
		Version:          strings.TrimPrefix(constants.Version, "v"),
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	rootCmd.AddCommand(
		cmd.NewCpCommand(),
		cmd.NewCreateCommand(),
		cmd.NewEnterCommand(),
		cmd.NewExecCommand(),
		cmd.NewImagesCommand(),
		cmd.NewInspectCommand(),
		cmd.NewLogsCommand(),
		cmd.NewPsCommand(),
		cmd.NewPullCommand(),
		cmd.NewRenameCommand(),
		cmd.NewRmCommand(),
		cmd.NewRmiCommand(),
		cmd.NewRootlessHelperCommand(),
		cmd.NewRunCommand(),
		cmd.NewStartCommand(),
		cmd.NewStopCommand(),
		cmd.NewUpdateCommand(),
		cmd.NewVersionCommand(),
	)
	rootCmd.PersistentFlags().
		String("log-level", "", "log messages above specified level (debug, warn, warning, error)")

	return rootCmd
}

func setEnviron() error {
	_ = os.MkdirAll(utils.LilipodBinPath, 0o755)
	_ = os.MkdirAll(imageutils.ImageDir, 0o755)
	_ = os.MkdirAll(containerutils.ContainerDir, 0o755)

	path := utils.LilipodBinPath + ":" + os.Getenv("PATH")

	err := os.Setenv("PATH", path)
	if err != nil {
		return err
	}

	if os.Getenv("ROOTFUL") == "" {
		err = os.Setenv("ROOTFUL", strconv.FormatBool(os.Getuid() == 0))
		if err != nil {
			return err
		}
	}

	return utils.EnsureUNIXDependencies(pty)
}

func main() {
	err := setEnviron()
	if err != nil {
		log.Fatalf("%+v\n", err)
	}

	app := newApp()

	err = app.Execute()
	if err != nil {
		log.Fatalf("%+v\n", err)
	}
}
