// Package logging will handle multi-level logging for the application.
package logging

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// loglevel represents the logging level chosen for the run.
// Defaults to warn.
var loglevel int

const (
	mute  = 0
	err   = 1
	warn  = 2
	debug = 3
	trace = 4
)

const (
	errorString   = "[error] "
	warningString = "[warn] "
	debugString   = "[debug] "
)

const (
	reset  = "\033[0;0m"
	red    = "\033[1;31m"
	green  = "\033[1;32m"
	yellow = "\033[1;33m"
)

var levels = map[int]string{
	0: "mute",
	1: "error",
	2: "warn",
	3: "debug",
	4: "trace",
}

// Init will initialize the logging to the input level.
// This is meant to be ran as a PreRunE function in cobra.
func Init(cmd *cobra.Command, _ []string) error {
	flag, flagErr := cmd.Flags().GetString("log-level")
	if flagErr != nil {
		return flagErr
	}

	level := strings.ToLower(flag)

	switch level {
	case levels[err]:
		loglevel = err
	case levels[warn]:
		loglevel = warn
	case levels[debug]:
		loglevel = debug
	case levels[trace]:
		loglevel = trace
	case levels[mute]:
		loglevel = mute
	default:
		loglevel = warn
	}

	return nil
}

// GetLogLevel returns the logging level currently set.
func GetLogLevel() string {
	return levels[loglevel]
}

// ReadLog will read input file and print.
// File will be read from since (timestamp) to until (timestamp).
// File will be continuously read if follow is true. (like tail -f)
// Timestamps for each line will be shown if timestamps is true.
// Lines will be printed to stderr or stdout based on where they were meant to be.
//
//	Line structure: timestamp:stdout:line
func ReadLog(file io.Reader, since, until int64, follow, timestamps bool) error {
	reader := bufio.NewReader(file)

	if until <= 0 {
		until = math.MaxInt
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// without this sleep you would hogg the CPU
				time.Sleep(250 * time.Millisecond)

				// if we're in follow mode, let's keep cycling,
				// this will simulate a "tail -f"
				if follow {
					continue
				}

				// file finished, let's exit
				break
			}

			LogDebug("error: %+v", err)

			return err
		}

		// Line has a structure:
		//    timestamp_unix:output:message
		// output can be stderr or stdout, so we'll need to respect that.
		stamp := strings.Split(line, ":")[0]
		where := strings.Split(line, ":")[1]
		content := strings.Join(strings.Split(line, ":")[2:], ":")

		timestamp, err := strconv.ParseInt(stamp, 10, 64)
		if err != nil {
			LogDebug("error: %+v", err)

			return err
		}

		// If we have to print timestamps, let's add them to the message, using
		// a nice format encoding.
		if timestamps {
			content = time.Unix(timestamp, 0).Format(time.RFC3339Nano) + " " + content
		}

		// Ensure we're printing only if the timestamp is between since and until.
		// If none is specified, since is 0 and until is MAX_INT, so we'll always print.
		if timestamp >= since && timestamp <= until {
			// print to stderr if needd
			if where == "err" {
				fmt.Fprintf(os.Stderr, "%s", content)

				continue
			}
			// else print to stdout
			fmt.Fprintf(os.Stdout, "%s", content)
		}
	}

	return nil
}

// AppendStringToFile will append input string onto input file.
func AppendStringToFile(path string, input string) error {
	if !strings.HasSuffix(input, "\n") {
		input += "\n"
	}

	// We'll open the file in append only mode using syscall, so that our writes
	// are automatically added in append.
	fd, err := syscall.Open(path,
		syscall.O_APPEND|syscall.O_CREAT|syscall.O_WRONLY, uint32(os.ModePerm))
	if err != nil {
		LogDebug("error: %+v", err)

		return err
	}

	defer func() { _ = syscall.Close(fd) }()

	written, err := syscall.Write(fd, []byte(input))
	if len(input) != written {
		return fmt.Errorf("incomplete string write to file")
	}

	if err != nil {
		LogDebug("error: %+v", err)

		return err
	}

	// always remember to close the fd!
	return syscall.Close(fd)
}

// LogError will create an error log in the form of:
// callerfile.go:line [error] message...
func LogError(format string, v ...any) {
	filteredLog(err, red+errorString+reset+format, v...)
}

// LogWarning will create a warning log in the form of:
// callerfile.go:line [warn] message...
func LogWarning(format string, v ...any) {
	filteredLog(warn, yellow+warningString+reset+format, v...)
}

// LogDebug will create a debug log in the form of:
// callerfile.go:line [debug] message...
func LogDebug(format string, v ...any) {
	filteredLog(debug, green+debugString+reset+format, v...)
}

// Log will create a plain log for input string.
func Log(format string, v ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", v...)
}

// print logs only if level is <= than the globally set level.
func filteredLog(level int, format string, inputs ...any) {
	if level <= loglevel {
		// try to add the filename:line
		_, file, line, ok := runtime.Caller(2)
		if ok {
			file = filepath.Base(file)

			format = file + ":" + strconv.Itoa(line) + " " + format
		}

		fmt.Fprintf(os.Stderr, format+"\n", inputs...)
	}
}
