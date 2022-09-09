package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/89luca89/scatman/pkg/utils"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

func psHelp(*cobra.Command) error {
	help := `Description:
  Prints out information about the chroots

Usage:
  scatman ps [options]

Examples:
  scatman ps
  scatman ps -a

Options:
  -a, --all              Show all the chroots, default is only running chroots
  -s, --size             Display the total file sizes
  -f, --filter           Filter output based on conditions given`
	fmt.Println(help)

	return nil
}

// NewPsCommand will list running chroots optionally with disk usage of each.
func NewPsCommand() *cobra.Command {
	var psCommand = &cobra.Command{
		Use:              "ps [flags] IMAGE",
		Short:            "List chroots",
		RunE:             ps,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	psCommand.SetUsageFunc(psHelp)
	psCommand.Flags().SetInterspersed(false)
	psCommand.Flags().BoolP("help", "h", false, "show help")
	psCommand.Flags().BoolP("all", "a", false, "Show all chroots")
	psCommand.Flags().BoolP("size", "s", false, "Display the total file sizes")
	psCommand.Flags().BoolP("noheading", "", false, "Do not print headers")
	psCommand.Flags().StringP("filter", "f", "", "Filter output based on conditions given")

	return psCommand
}

func ps(cmd *cobra.Command, arguments []string) error {
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	size, err := cmd.Flags().GetBool("size")
	if err != nil {
		return err
	}

	noheading, err := cmd.Flags().GetBool("noheading")
	if err != nil {
		return err
	}

	filter, err := cmd.Flags().GetString("filter")
	if err != nil {
		return err
	}

	chroots, err := os.ReadDir(utils.ChrootDir)

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.Style{
		Name: "psStyle",
		Box: table.BoxStyle{
			BottomLeft:       "",
			BottomRight:      "",
			BottomSeparator:  "",
			Left:             "",
			LeftSeparator:    "",
			MiddleHorizontal: "",
			MiddleSeparator:  "",
			MiddleVertical:   "	",
			PaddingLeft:      "",
			PaddingRight:     "",
			Right:            "",
			RightSeparator:   "",
			TopLeft:          "",
			TopRight:         "",
			TopSeparator:     "",
			UnfinishedRow:    "",
		},
		Format: table.FormatOptions{
			Footer: text.FormatUpper,
			Header: text.FormatUpper,
			Row:    text.FormatDefault,
		},
		Options: table.Options{
			DrawBorder:      true,
			SeparateColumns: true,
			SeparateFooter:  false,
			SeparateHeader:  false,
			SeparateRows:    false,
		},
	})

	if !noheading {
		if !size {
			t.AppendHeader(table.Row{"IMAGE", "COMMAND", "CREATED", "STATUS", "LABELS", "NAMES"})
		} else {
			t.AppendHeader(table.Row{"IMAGE", "COMMAND", "CREATED", "STATUS", "LABELS", "NAMES", "SIZE"})
		}
	}
	//
	// IsRunning can take the output of `ps ax` in input in order to speed up
	// the execution. We don't need to call `ps ax` for each individual chroot.
	output, err := exec.Command("ps", "ax").Output()
	if err != nil {
		return err
	}
	// but we need it as string
	running := string(output)

	for _, chroot := range chroots {

		configPath := utils.ChrootDir + chroot.Name() + "/config"
		config, err := utils.LoadConfig(configPath)
		if err != nil {
			return err
		}

		state := "created"
		isRunning := utils.IsRunning(string(config.Name), running)

		labels := strings.Join(config.Label, ",")

		if isRunning {
			state = "running"
		} else {
			state = "stopped"
		}

		command := config.Entrypoint[0]
		if len(config.Entrypoint) > 1 {
			command += "..."
		}

		if (isRunning || all) && checkFilter(filter, config, state) {
			if size {
				directorySize, err := utils.DiscUsageMegaBytes(utils.ChrootDir + "/" + chroot.Name())
				if err != nil {
					return err
				}
				t.AppendRow([]interface{}{config.Image, command, "", state, labels, config.Name, directorySize})
			} else {
				t.AppendRow([]interface{}{config.Image, command, "", state, labels, config.Name})
			}
		}
	}

	t.Render()
	return nil
}

func checkFilter(filter string, conf utils.Config, running string) bool {
	if filter == "" {
		return true
	}

	validFilters := []string{
		"status",
		"label",
		"name",
	}

	what := strings.Split(filter, "=")[0]
	condition := strings.Join(strings.Split(filter, "=")[1:], "=")
	// check if this is a valid filter
	if !utils.StringContains(validFilters, what) {
		panic("Invalid filter, choose one of: " + strings.Join(validFilters, " "))
	}

	if what == "status" {
		return condition == running
	}
	if what == "label" {
		return utils.StringContains(conf.Label, condition)
	}
	if what == "name" {
		return conf.Name == condition
	}

	return false
}
