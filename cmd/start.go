package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/89luca89/scatman/pkg/pullutils"
	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

func startHelp(*cobra.Command) error {
	help := `Description:
  Starts one or more chroots.  The chroot name or ID can be used.

Usage:
  scatman start [options] chroot [chroot...]

Examples:
  scatman start --latest
  scatman start 860a4b231279 5421ab43b45
  scatman start --interactive chrootName

Options:
      --all                 Start all chroots regardless of their state or configuration
  -i, --interactive         Keep process in foreground`

	fmt.Println(help)

	return nil
}

// NewStartCommand will start one or more chroots in input with default entrypoint command.
func NewStartCommand() *cobra.Command {
	var startCommand = &cobra.Command{
		Use:              "start [flags] IMAGE",
		Short:            "Start one or more chroots",
		RunE:             start,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	startCommand.SetUsageFunc(startHelp)
	startCommand.Flags().SetInterspersed(false)
	startCommand.Flags().BoolP("help", "h", false, "show help")
	startCommand.Flags().BoolP("all", "a", false, "Start all chroots regardless of their state or configuration")
	startCommand.Flags().BoolP("interactive", "i", false, "Keep process in foreground")

	return startCommand
}

func start(cmd *cobra.Command, arguments []string) error {
	startAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	interactive, err := cmd.Flags().GetBool("interactive")
	if err != nil {
		return err
	}

	if len(arguments) < 1 && !startAll {
		cmd.Help()

		return nil
	}

	success, err := utils.EnsureRootlesskit(interactive)
	if err != nil {
		return err
	}

	if success {
		return nil
	}

	// if we want to delete all, just get a list of the targets and add it to
	// the arguments.
	if startAll {
		arguments = []string{}

		images, err := os.ReadDir(pullutils.ImageDir)
		if err != nil {
			return err
		}

		for _, i := range images {
			arguments = append(arguments, i.Name())
		}
	}

	for _, chroot := range arguments {
		// save the config to file
		configPath := utils.GetChrootDir(chroot) + "/config"
		if _, err := os.Stat(configPath); err == nil {
			config, err := utils.LoadConfig(configPath)
			if err != nil {
				return err
			}

			log.Println("Starting: " + chroot)

			err = utils.EnterChroot(utils.GetRootfsDir(chroot), "/", interactive, config)
			if err != nil {
				return err
			}
		} else {
			log.Printf("Chroot %s does not exist.\n", chroot)
		}
	}

	return nil
}
