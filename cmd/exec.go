package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

func execHelp(*cobra.Command) error {
	help := `Run a process in a running chroot

Description:
  Execute the specified command inside a running chroot.


Usage:
  scatman exec [options] chroot [COMMAND [ARG...]]

Examples:
  scatman exec -it ctrID ls
  scatman exec -it -w /tmp myCtr pwd
  scatman exec --user root ctrID ls

Options:
  -d, --detach               Run the exec session in detached mode (backgrounded)
  -e, --env stringArray      Set environment variables
  -i, --interactive          Keep STDIN open even if not attached
  -t, --tty                  Allocate a pseudo-TTY. The default is false
  -u, --user string          Sets the username or UID used and optionally the groupname or GID for the specified command
  -w, --workdir string       Working directory inside the chroot`

	fmt.Println(help)

	return nil
}

// NewExecCommand will execute a command inside a running chroot.
func NewExecCommand() *cobra.Command {
	var execCommand = &cobra.Command{
		Use:              "exec [flags] IMAGE [COMMAND] [ARG...]",
		Short:            "Exec but do not start a chroot",
		RunE:             execute,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	execCommand.SetUsageFunc(execHelp)
	execCommand.Flags().SetInterspersed(false)
	execCommand.Flags().BoolP("help", "h", false, "show help")
	execCommand.Flags().BoolP("detach", "d", false, "Run the exec session in detached mode (backgrounded)")
	execCommand.Flags().BoolP("interactive", "i", false, "Keep STDIN open even if not attached")
	execCommand.Flags().BoolP("tty", "t", false, "Allocate a pseudo-TTY. The default is false")
	execCommand.Flags().StringP("workdir", "w", "/", "Working directory inside the chroot")
	execCommand.Flags().StringP("user", "u", "root:root", "Username or UID (format: <name|uid>[:<group|gid>])")
	execCommand.Flags().StringArrayP("env", "e", nil, "Set environment variables")

	return execCommand
}

func execute(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		cmd.Help()

		return nil
	}

	detach, err := cmd.Flags().GetBool("detach")
	if err != nil {
		return err
	}

	interactive, err := cmd.Flags().GetBool("interactive")
	if err != nil {
		return err
	}

	user, err := cmd.Flags().GetString("user")
	if err != nil {
		return err
	}

	workdir, err := cmd.Flags().GetString("workdir")
	if err != nil {
		return err
	}

	env, err := cmd.Flags().GetStringArray("env")
	if err != nil {
		return err
	}

	env = append([]string{"TERM=xterm", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}, env...)

	chroot := cmd.Flags().Args()[0]
	entrypoint := cmd.Flags().Args()[1:]

	if interactive {
		detach = false
	}

	if detach {
		interactive = false
	}
	// ensure a chroot for this name is already running
	if !utils.IsRunning(chroot, "") {
		return errors.New(chroot + " is not running")
	}

	configPath := utils.GetChrootDir(chroot) + "/config"
	if _, err := os.Stat(configPath); err == nil {
		config, err := utils.LoadConfig(configPath)
		if err != nil {
			return err
		}

		config.User = user
		config.Entrypoint = entrypoint
		config.Env = append(config.Env, env...)

		log.Println("Entering: " + chroot)

		err = utils.EnterNamespace(chroot, workdir, interactive, config)
		if err != nil {
			return err
		}

		return nil
	}

	return nil
}
