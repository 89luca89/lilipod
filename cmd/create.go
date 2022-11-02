package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/89luca89/scatman/pkg/createutils"
	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

func createHelp(*cobra.Command) error {
	help := `Description:
  Creates a new chroot from the given image or storage and prepares it for running the specified command.

  You can then start it at any time with the scatman start <chroot_name> command.
  If no COMMAND is passed, it will default to /bin/sh.

Usage:
  scatman create [options] IMAGE [COMMAND [ARG...]]

Examples:
  scatman create alpine ls
  scatman create --name myctr --volume /opt:/opt:rw alpine ls

Options:
  -e, --env stringArray                          Set environment variables in chroot (default [PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin,TERM=xterm])
  -h, --hostname string                          Set chroot hostname
      --ipc string                               IPC namespace to use (host,private)
      --name string                              Assign a name to the chroot
      --network stringArray                      Connect a chroot to a network (host,private)
      --pid string                               PID namespace to use (host,private)
  -u, --user string                              Username or UID (format: <name|uid>[:<group|gid>])
      --userns string                            User namespace to use (host,private)
  -v, --volume stringArray                       Bind mount a volume into the chroot`

	fmt.Println(help)

	return nil
}

// NewCreateCommand will create a new chroot environment ready to use.
func NewCreateCommand() *cobra.Command {
	var createCommand = &cobra.Command{
		Use:              "create [flags] IMAGE [COMMAND] [ARG...]",
		Short:            "Create but do not start a chroot",
		RunE:             create,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	createCommand.SetUsageFunc(createHelp)
	createCommand.Flags().SetInterspersed(false)
	createCommand.Flags().Bool("help", false, "show help")
	createCommand.Flags().String("ipc", "private", "IPC namespace to use")
	createCommand.Flags().String("name", createutils.GetRandomName(), "Assign a name to the chroot")
	createCommand.Flags().String("network", "private", "Connect a chroot to a network")
	createCommand.Flags().String("pid", "private", "PID namespace to use")
	createCommand.Flags().String("userns", "private", "User namespace to use")
	createCommand.Flags().StringP("hostname", "h", "", "Set chroot hostname")
	createCommand.Flags().StringP("user", "u", "root:root", "Username or UID (format: <name|uid>[:<group|gid>])")
	createCommand.Flags().StringArrayP("env", "e", nil, "Set environment variables in chroot")
	createCommand.Flags().StringArrayP("volume", "v", nil, "Bind mount a volume into the chroot")
	createCommand.Flags().StringArrayP("label", "", nil, "Set metadata on container")

	return createCommand
}

func create(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
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

	hostname, err := cmd.Flags().GetString("hostname")
	if err != nil {
		return err
	}

	ipc, err := cmd.Flags().GetString("ipc")
	if err != nil {
		return err
	}

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}

	network, err := cmd.Flags().GetString("network")
	if err != nil {
		return err
	}

	pid, err := cmd.Flags().GetString("pid")
	if err != nil {
		return err
	}

	user, err := cmd.Flags().GetString("user")
	if err != nil {
		return err
	}

	userns, err := cmd.Flags().GetString("userns")
	if err != nil {
		return err
	}

	env, err := cmd.Flags().GetStringArray("env")
	if err != nil {
		return err
	}
	env = append([]string{"TERM=xterm", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}, env...)

	label, err := cmd.Flags().GetStringArray("label")
	if err != nil {
		return err
	}

	volume, err := cmd.Flags().GetStringArray("volume")
	if err != nil {
		return err
	}

	// default hostname to name if not specified.
	if hostname == "" {
		hostname = name
	}

	image := cmd.Flags().Args()[0]

	if(!isImageLocal(image)){
		pullArgs := []string{image};
		err := pull(cmd, pullArgs);
		if err != nil{
			log.Printf("ERROR: %v", err)
			return err
		}
	}

	entrypoint := cmd.Flags().Args()[1:]
	if len(entrypoint) == 0 {
		entrypoint = []string{"/bin/sh"}
	}

	uid := os.Getenv("PARENT_UID")
	gid := os.Getenv("PARENT_GID")

	if uid == "0" {
		userns = "private"
	}

	createConfig := utils.Config{
		Env:      env,
		Hostname: hostname,
		Image:    image,
		Ipc:      ipc,
		Name:     name,
		Network:  network,
		Pid:      pid,
		User:     user,
		Userns:   userns,
		Volume:   volume,
		Label:    label,
		// entry point related
		Entrypoint: entrypoint,
	}

	if _, err := os.Stat(utils.GetChrootDir(name) + "/config"); err == nil {
		return errors.New(name + " already exists.")
	}

	// prepare chroot directory if it does not exist.
	if _, err := os.Stat(utils.GetRootfsDir(name) + "/root"); errors.Is(err, os.ErrNotExist) {
		err := utils.PrepareRootfs(image, name, userns)
		if err != nil {
			return err
		}
	}
	// save the config to file
	configPath := utils.GetChrootDir(name) + "/config"

	err = utils.SaveConfig(createConfig, configPath)
	if err != nil {
		return err
	}

	// save creator's uid:gid value in file
	usr := uid + ":" + gid

	err = os.WriteFile(utils.GetChrootDir(name)+"/user", []byte(usr), 0644)
	if err != nil {
		return err
	}

	if userns == "keep-id" {
		log.Printf("Fixing %s permissions.", name)
		err = utils.FixChrootPermissionKeepID(utils.GetRootfsDir(name))
		if err != nil {
			return err
		}
	}


	log.Printf("Created %s successfully.", name)

	return nil
}
