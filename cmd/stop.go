package cmd

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

func stopHelp(*cobra.Command) error {
	help := `Stop one or more chroots

Description:
  Stops one or more running chroots.

Usage:
  scatman stop [options] chroot [chroot...]

Examples:
  scatman stop ctrID
  scatman stop --all
  scatman stop --force test-chroot

Options:
  -a, --all                   Stop all running chroots
  -f, --force                   Stop all running chroots (use SIGKILL instead of SIGTERM)`

	fmt.Println(help)

	return nil
}

// NewStopCommand will find all the processes in given chroot and will stop them.
func NewStopCommand() *cobra.Command {
	var stopCommand = &cobra.Command{
		Use:              "stop [flags] IMAGE",
		Short:            "Remove one or more chroots",
		RunE:             stop,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	stopCommand.SetUsageFunc(stopHelp)
	stopCommand.Flags().SetInterspersed(false)
	stopCommand.Flags().BoolP("help", "h", false, "show help")
	stopCommand.Flags().BoolP("all", "a", false, "Remove all images")
	stopCommand.Flags().BoolP("force", "f", false, "Force removal of a running or unusable chroot")

	return stopCommand
}

func stop(cmd *cobra.Command, arguments []string) error {
	stopAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	if len(arguments) < 1 && !stopAll {
		cmd.Help()

		return nil
	}

	// if we want to delete all, just get a list of the targets and add it to
	// the arguments.
	if stopAll {
		arguments = []string{}

		chroots, err := os.ReadDir(utils.ChrootDir)
		if err != nil {
			return err
		}

		for _, i := range chroots {
			name, err := base64.StdEncoding.DecodeString(i.Name())
			if err != nil {
				return err
			}

			arguments = append(arguments, string(name))
		}
	}

	for _, chroot := range arguments {
		// delete the targets.
		targetDIR = utils.GetChrootDir(chroot)
		if _, err := os.Stat(targetDIR); err == nil {
			log.Printf("Stopping %s...\n", chroot)

			err := utils.StopChroot(chroot, force)
			if err != nil {
				log.Fatal(err)
			}

			log.Println("Stopped: " + chroot)
		} else {
			log.Printf("Chroot %s does not exist.\n", chroot)
		}
	}

	return nil
}
