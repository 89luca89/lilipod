package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/89luca89/scatman/pkg/utils"
	"github.com/spf13/cobra"
)

func logsHelp(*cobra.Command) error {
	help := `Description:
Description:
  Retrieves logs for one or more chroots.

  This does not guarantee execution order when combined with scatman run (i.e., your run may not have generated any logs at the time you execute scatman logs).


Usage:
  scatman logs [options] chroot [chroot...]

Examples:
  scatman logs ctrID
  scatman logs --tail 2 mywebserver
  scatman logs --follow --since 10m ctrID
  scatman logs mywebserver mydbserver

Options:
  -f, --follow         Follow log output.  The default is false
  --tail int           Output the specified number of LINES at the end of the logs.  Defaults to print all lines.`
	fmt.Println(help)

	return nil
}

// NewLogsCommand will list running chroots optionally with disk usage of each.
func NewLogsCommand() *cobra.Command {
	var logsCommand = &cobra.Command{
		Use:              "logs [flags] CHROOT",
		Short:            "Fetch the logs of one or more ",
		RunE:             logs,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	logsCommand.SetUsageFunc(logsHelp)
	logsCommand.Flags().SetInterspersed(false)
	logsCommand.Flags().BoolP("help", "h", false, "show help")
	logsCommand.Flags().BoolP("follow", "f", false, "Follow log output.")
	logsCommand.Flags().IntP("tail", "", math.MaxInt32, "Output the specified number of LINES at the end of the logs")

	return logsCommand
}

func getLastLine(file *os.File) int {
	reader := bufio.NewReader(file)
	reader.ReadLine()
	lastLineSize := 0
	for {
		line, _, err := reader.ReadLine()

		if err == io.EOF {
			break
		}

		lastLineSize = len(line)
	}
	return lastLineSize
}

func logs(cmd *cobra.Command, arguments []string) error {
	follow, err := cmd.Flags().GetBool("follow")
	if err != nil {
		return err
	}

	tail, err := cmd.Flags().GetInt("tail")
	if err != nil {
		return err
	}

	if len(arguments) < 1 {
		cmd.Help()

		return nil
	}

	if len(arguments) > 1 {
		return errors.New("You need to select one target at time")
	}

	target := utils.GetChrootDir(arguments[0]) + "/current-logs"

	tailCmd := []string{"tail", "-n", strconv.Itoa(tail)}
	if follow {
		tailCmd = append(tailCmd, "-f")
	}

	tailCmd = append(tailCmd, target)
	command := exec.Command(tailCmd[0], tailCmd[1:]...)

	var waitGroup sync.WaitGroup

	waitGroup.Add(2)

	stdout, _ := command.StdoutPipe()
	defer stdout.Close()

	stderr, _ := command.StderrPipe()
	defer stderr.Close()

	command.Start()
	if err != nil {
		return err
	}
	stdoutScan := bufio.NewScanner(stdout)
	stderrScan := bufio.NewScanner(stderr)
	// async fetch stdout
	go func() {
		defer waitGroup.Done()

		for stdoutScan.Scan() {
			println(stdoutScan.Text())
		}
	}()
	// async fetch stderr
	go func() {
		defer waitGroup.Done()

		for stderrScan.Scan() {
			println(stderrScan.Text())
		}
	}()

	waitGroup.Wait()

	return nil
}
