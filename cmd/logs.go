// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/spf13/cobra"
)

// NewLogsCommand will output all logs of a target container.
func NewLogsCommand() *cobra.Command {
	logsCommand := &cobra.Command{
		Use:              "logs [flags] container",
		Short:            "Fetch the logs of one or more ",
		PreRunE:          logging.Init,
		RunE:             logs,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	logsCommand.Flags().SetInterspersed(false)
	logsCommand.Flags().BoolP("follow", "f", false, "follow log output.  The default is false")
	logsCommand.Flags().BoolP("timestamps", "t", false, "show timestamps")
	logsCommand.Flags().String("since", "", "show logs since input timestamp")
	logsCommand.Flags().String("until", "", "show logs until input timestamp")
	logsCommand.Flags().BoolP("help", "h", false, "show help")

	return logsCommand
}

func logs(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		return cmd.Help()
	}

	container := arguments[0]

	if !fileutils.Exist(containerutils.GetDir(container)) {
		return fmt.Errorf("container %s does not exist", container)
	}

	follow, err := cmd.Flags().GetBool("follow")
	if err != nil {
		return err
	}

	timestamps, err := cmd.Flags().GetBool("timestamps")
	if err != nil {
		return err
	}

	since, err := cmd.Flags().GetString("since")
	if err != nil {
		return err
	}

	until, err := cmd.Flags().GetString("until")
	if err != nil {
		return err
	}

	file, err := os.Open(containerutils.GetDir(container) + "/current-logs")
	if err != nil {
		return err
	}

	defer func() { _ = file.Close() }()

	return logging.ReadLog(file, convert(since), convert(until), follow, timestamps)
}

// convert input string into a unix timestamp int64.
func convert(input string) int64 {
	var result time.Time

	var err error

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02Z07:00",
		"2006-01-02",
	}

	for _, format := range formats {
		result, err = time.Parse(format, input)
		if err == nil {
			break
		}
	}

	return result.Unix()
}
