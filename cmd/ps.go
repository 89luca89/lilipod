// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/containerutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

// NewPsCommand will list running containers optionally with disk usage of each.
func NewPsCommand() *cobra.Command {
	psCommand := &cobra.Command{
		Use:              "ps [flags] IMAGE",
		Short:            "List containers",
		PreRunE:          logging.Init,
		RunE:             ps,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	psCommand.Flags().SetInterspersed(false)
	psCommand.Flags().BoolP("all", "a", false, "show all containers")
	psCommand.Flags().BoolP("help", "h", false, "show help")
	psCommand.Flags().BoolP("no-trunc", "", false, "do not truncate data")
	psCommand.Flags().BoolP("noheading", "", false, "do not print headers")
	psCommand.Flags().BoolP("quiet", "q", false, "display only container IDs")
	psCommand.Flags().BoolP("size", "s", false, "display the total file sizes")
	psCommand.Flags().String("format", "", "pretty-print output using a Go template")
	psCommand.Flags().
		StringArrayP("filter", "f", []string{}, "filter output based on conditions given")

	return psCommand
}

func ps(cmd *cobra.Command, _ []string) error {
	var filters map[string]string

	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}

	filterInput, err := cmd.Flags().GetStringArray("filter")
	if err != nil {
		return err
	}

	filters = make(map[string]string)

	for _, filter := range filterInput {
		name := strings.Split(filter, "=")[0]
		value := strings.Join(strings.Split(filter, "=")[1:], "=")

		switch name {
		case "label":
			if filters[name] != "" {
				filters[name] = filters[name] + constants.FilterSeparator + value
			} else {
				filters[name] = value
			}
		case "status":
			filters[name] = value
		case "name":
			filters[name] = value
		case "id":
			filters[name] = value
		default:
			logging.LogWarning("invalid filter %s, skipping", name)
			logging.LogWarning("valid filters are: label, status, name, id")
		}
	}

	size, err := cmd.Flags().GetBool("size")
	if err != nil {
		return err
	}

	noheading, err := cmd.Flags().GetBool("noheading")
	if err != nil {
		return err
	}

	notrunc, err := cmd.Flags().GetBool("no-trunc")
	if err != nil {
		return err
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	containers, err := os.ReadDir(containerutils.ContainerDir)
	if err != nil {
		logging.Log("no containers found")

		//nolint: nilerr
		return nil
	}

	psTable := table.NewWriter()
	psTable.SetOutputMirror(os.Stdout)
	psTable.SetStyle(utils.GetDefaultTable())

	for _, container := range containers {
		if quiet {
			fmt.Println(containerutils.GetID(container.Name()))

			continue
		}

		err = doContainerRow(psTable, container.Name(), format, size, notrunc, all, filters)
		if err != nil {
			return err
		}
	}

	// render only if we're not using a go template string
	if format == "" && !quiet {
		if !noheading {
			if !size {
				psTable.AppendHeader(
					table.Row{
						"CONTAINER ID",
						"IMAGE",
						"COMMAND",
						"CREATED",
						"STATUS",
						"LABELS",
						"NAMES",
					},
				)
			} else {
				psTable.AppendHeader(table.Row{"CONTAINER ID", "IMAGE", "COMMAND", "CREATED", "STATUS", "LABELS", "NAMES", "SIZE"})
			}
		}

		psTable.Render()
	}

	return nil
}

func doContainerRow(
	psTable table.Writer,
	container, format string,
	size, notrunc, all bool,
	filters map[string]string,
) error {
	config, err := containerutils.GetContainerInfo(container, size, filters)
	if err != nil {
		return err
	}

	if config == nil {
		return nil
	}

	// Go-template string
	if format != "" {
		tmpl, err := template.New("format").Parse(format)
		if err != nil {
			return err
		}

		var out bytes.Buffer

		err = tmpl.Execute(&out, config)
		if err != nil {
			return err
		}

		fmt.Println(out.String())

		return nil
	}
	// continue with table

	labels := strings.Join(config.Labels, ",")
	if len(labels) > 16 && !notrunc {
		labels = labels[:15] + "..."
	}

	// truncate long commands
	command := strings.Join(config.Entrypoint, " ")
	if len(command) > 16 && !notrunc {
		command = command[:15] + "..."
	}

	if config.Status == "running" || all {
		if size {
			psTable.AppendRow(
				[]interface{}{
					container,
					config.Image,
					command,
					config.Created,
					config.Status,
					labels,
					config.Names,
					config.Size,
				},
			)
		} else {
			psTable.AppendRow([]interface{}{
				container,
				config.Image,
				command,
				config.Created,
				config.Status,
				labels,
				config.Names,
			})
		}
	}

	return nil
}
