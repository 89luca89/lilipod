package cmd

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

func rmHelp(*cobra.Command) error {
	help := `Description:
  Removes one or more chroots from the host.

  Command does not remove chroots. Running or unusable chroots will not be removed without the -f option.

Usage:
  scatman rm [options] chroot [chroot...]

Examples:
  scatman rm mywebserver myflaskserver 860a4b23
  scatman rm --force --all
  scatman rm -f c684f0d469f2

Options:
  -a, --all                   Remove all chroots
  -f, --force                 Force removal of a running or unusable chroot
  -F, --force-force           Force removal of a running or unusable chroot (use SIGKILL instead of SIGTERM)`

	fmt.Println(help)

	return nil
}

// NewRmCommand removes one or more chroots from the host.
func NewRmCommand() *cobra.Command {
	var rmCommand = &cobra.Command{
		Use:              "rm [flags] IMAGE",
		Short:            "Remove one or more chroots",
		RunE:             rm,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	rmCommand.SetUsageFunc(rmHelp)
	rmCommand.Flags().SetInterspersed(false)
	rmCommand.Flags().BoolP("help", "h", false, "show help")
	rmCommand.Flags().BoolP("all", "a", false, "Remove all images")
	rmCommand.Flags().BoolP("force", "f", false, "Force removal of a running or unusable chroot")
	rmCommand.Flags().BoolP("force-force", "F", false, "Force removal of a running or unusable chroot (use SIGKILL instead of SIGTERM)")

	return rmCommand
}

func rm(cmd *cobra.Command, arguments []string) error {
	success, err := utils.EnsureRootlesskit(true)
	if err != nil {
		return err
	}

	if success {
		return nil
	}

	delAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	fforce, err := cmd.Flags().GetBool("force-force")
	if err != nil {
		return err
	}

	// if force-force, we force
	if fforce {
		force = true
	}

	if len(arguments) < 1 && !delAll {
		cmd.Help()

		return nil
	}

	// if we want to delete all, just get a list of the targets and add it to
	// the arguments.
	if delAll {
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
		targetDIR = utils.GetChrootDir(chroot)

		// check if a pid file exists.
		if utils.IsRunning(chroot, "") {
			// if force, then we try to find and kill the pid.
			if force {
				err := utils.StopChroot(chroot, fforce)
				if err != nil {
					return err
				}
			} else {
				log.Printf("Error: cannot remove chroot %s, as it is running.", chroot)

				continue
			}
		}

		// delete the targets.
		if _, err := os.Stat(targetDIR); err == nil {
			// Simple retry logic for unmount
			err := utils.Umount(targetDIR + "/rootfs")
			if err != nil {
				return err
			}

			err = os.RemoveAll(targetDIR)
			if err != nil {
				return err
			}

			log.Println("Deleted: " + chroot)
		} else {
			log.Printf("Chroot %s does not exist.\n", chroot)
		}
	}

	return nil
}
