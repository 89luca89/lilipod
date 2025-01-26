/* SPDX-License-Identifier: GPL-3.0-only

This file is part of the lilipod project:
   https://github.com/89luca89/lilipod

Copyright (C) 2023 lilipod contributors

lilipod is free software; you can redistribute it and/or modify it
under the terms of the GNU General Public License version 3
as published by the Free Software Foundation.

lilipod is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
General Public License for more details.

You should have received a copy of the GNU General Public License
along with lilipod; if not, see <http://www.gnu.org/licenses/>. */

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

//go:embed busybox
var busybox []byte

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

	return utils.EnsureUNIXDependencies(pty, busybox)
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
