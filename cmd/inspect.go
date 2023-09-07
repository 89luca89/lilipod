// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/spf13/cobra"
)

// NewInspectCommand will give information about a container or an image.
func NewInspectCommand() *cobra.Command {
	inspectCommand := &cobra.Command{
		Use:              "inspect [IMAGE|CONTAINER]",
		Short:            "Inspect a container or image",
		PreRunE:          logging.Init,
		RunE:             inspect,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	inspectCommand.Flags().SetInterspersed(false)
	inspectCommand.Flags().BoolP("help", "h", false, "show help")
	inspectCommand.Flags().BoolP("size", "s", false, "show container size")
	inspectCommand.Flags().String("format", "", "pretty-print output using a Go template")
	inspectCommand.Flags().StringP("type", "t", "container", "specify inspect-object type")

	return inspectCommand
}

func inspect(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		return cmd.Help()
	}

	inspectType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}

	size, err := cmd.Flags().GetBool("size")
	if err != nil {
		return err
	}

	if size && inspectType == "image" {
		return fmt.Errorf("size is not supported for type image")
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	if format != "" && !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	format = strings.ReplaceAll(format, ".State.Status", ".Status")
	format = strings.ReplaceAll(format, ".Config.Env", ".Env")

	var output string

	switch inspectType {
	case "container":
		output, err = containerutils.Inspect(arguments, size, format)
	case "image":
		output, err = imageutils.Inspect(arguments, format)
	default:
		return errors.New("unsupported inspect type")
	}

	if err != nil || output == "" {
		return fmt.Errorf("no such object: %v", arguments)
	}

	fmt.Println(output)

	return nil
}
