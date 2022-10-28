package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/89luca89/scatman/cmd"
	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

// -ldflags  "-X main.Version=1.1.1"

var skibidi string = `
____ _  _ _    ___  _  ___  _ ___  ___  _   _  ___  _ ___
[__  |_/  | __ |__] |  |  \ | |__] |__]  \_/   |  \ | |__]
___] | \_ |    |__] |  |__/ | |__] |__]   |    |__/ | |__]
_   _____  ___  ____  ___  _  _ ___   ___  _  _ ___
 \_/ |  |  |  \ |__|  |  \ |  | |__]  |  \ |  | |__]
  |  |__|  |__/ |  |  |__/ |__| |__]  |__/ |__| |__]

    .~7~.              .!P&@@@@@@&P~    .?G&@@@@@@&#5^             .?B&B?.
  ^#@@57!^           ~G@@@@@@@@@@@@@&?~G@@@@@@@@@@@@@@&5:         .7^^J@@@?
 !@@@:            .Y&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@B~            .&@@Y
:@@@Y           ~G@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#!           J@@@!
P@@@?        :Y&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@&?.        7@@@#
#@@@B     .7#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@P^     :&@@@&
G@@@@#J?5#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@BPG#@@@@@#
~@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@!
 B@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#^ 7&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@5
 .#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#~     ?&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@G
   5@@@@@@@@@@@@@@@@@@@@@@@@@@@#J.         ^5&@@@@@@@@@@@@@@@@@@@@@@@@@@&7
    :Y&@@@@@@@@@@@@@@@@@@@@BJ~.               .!P#@@@@@@@@@@@@@@@@@@@@B~
       .~YB&@@@@@@@&#BP?~:                        .:~JPB#&&&@@@&&#P7^.

Scatman version: %s
`

// Version of scatman. This should be overwritten at compile time.
var Version string = "0.0.1"

var dependencies []string = []string{
	"chmod",
	"mountpoint",
	"nsenter",
	"ps",
	"rootlesskit",
	"tail",
	"tar",
}

func help(*cobra.Command) error {
	help := `Usage:
  scatman [options] [command]

Available Commands:
  build       Build an image from a dockerfile
  cp          Copy files/folders between a chroot and the local filesystem
  create      Create but do not start a chroot
  exec        Run a process in a running chroot
  help        Help about any command
  images      List images in local storage
  logs        Fetch the logs of one or more chroots
  ps          List chroots
  pull        Pull an image from a registry
  rm          Remove one or more chroots
  rmi         Removes one or more images from local storage
  start       Start one or more chroots
  stop        Stop one or more chroots
  version     Display the scatman version information
`

	fmt.Println(help)

	return nil
}
func newApp() (*cobra.Command, error) {
	var rootCmd = &cobra.Command{
		Use:              "scatman",
		Short:            "Manage chroots and images",
		Version:          strings.TrimPrefix(Version, "v"),
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	rootCmd.SetUsageFunc(help)
	rootCmd.AddCommand(
		cmd.NewBuildCommand(),
		cmd.NewCreateCommand(),
		cmd.NewEnterCommand(),
		cmd.NewExecCommand(),
		cmd.NewImagesCommand(),
		cmd.NewInspectCommand(),
		cmd.NewLogsCommand(),
		cmd.NewPsCommand(),
		cmd.NewPullCommand(),
		cmd.NewRmCommand(),
		cmd.NewRmiCommand(),
		cmd.NewStartCommand(),
		cmd.NewStopCommand(),
		cmd.NewCpCommand(),
	)

	return rootCmd, nil
}

func setEnviron() error {
	err := os.MkdirAll(utils.ScatmanExtraPath, os.ModePerm)
	if err != nil {
		return err
	}

	err = utils.SetupRootlesskit()
	if err != nil {
		return err
	}

	err = utils.SetupBusybox(dependencies)
	if err != nil {
		return err
	}

	path := os.Getenv("PATH") + ":" + utils.ScatmanExtraPath

	err = os.Setenv("PATH", path)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := setEnviron()
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	if len(os.Args) >1  && os.Args[1] == "sing" {
		fmt.Printf(skibidi, Version)
		os.Exit(0)
	}

	app, err := newApp()
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	err = app.Execute()
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}
