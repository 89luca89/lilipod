// Package procutils contains helpers and utilities for managing processes.
package procutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/logging"
)

// EnsureFakeRoot will ensure process is executed with rootless-helper.
func EnsureFakeRoot(interactive bool) (bool, error) {
	logging.LogDebug("ensuring we're either root or fake-root")

	// As files can be owned by root or fake roots, let's run either as root or
	// with rootless-helper
	if os.Getuid() == 0 &&
		(os.Getenv("ROOTFUL") != constants.TrueString ||
			os.Getenv("UNSHARED") == constants.TrueString) {
		logging.LogDebug("we're 0:0")

		return false, nil
	}

	var cmd *exec.Cmd

	// if we're running as proper root, let's do a mount unshare and continue
	if os.Getuid() == 0 &&
		os.Getenv("ROOTFUL") == constants.TrueString &&
		os.Getenv("UNSHARED") != constants.TrueString {
		unshareArgs := []string{"-m"}
		unshareArgs = append(unshareArgs, os.Args...)
		cmd = exec.Command("unshare", unshareArgs...)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "UNSHARED=true")
	} else {
		userMap, gidMap, err := GetSubIDRanges()
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return false, err
		}

		args := []string{"rootless-helper", "--log-level", logging.GetLogLevel()}
		args = append(args, os.Args...)
		cmd = exec.Command(os.Args[0], args...)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "ROOTFUL=false")
		cmd.Env = append(cmd.Env, "PARENT_UID_MAP="+strings.Join(userMap, ":"))
		cmd.Env = append(cmd.Env, "PARENT_GID_MAP="+strings.Join(gidMap, ":"))
	}

	logging.LogDebug("executing %v", cmd.Args)

	if interactive {
		err := RunWithTTY(cmd)
		if err != nil {
			logging.LogDebug("error: %+v", err)

			return false, err
		}

		return true, nil
	}

	// this is needed to completely detach a process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Foreground: false,
		Setsid:     true,
	}

	logging.LogDebug("tty not specified, using cmd.Start")

	err := cmd.Start()
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return false, err
	}

	logging.LogDebug("tty not specified, waiting for child to start")
	time.Sleep(time.Millisecond * 250)

	logging.LogDebug("tty not specified, releasing child")

	err = cmd.Process.Release()
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return false, err
	}

	return true, nil
}

// GetUIDGID will return a couple of uid/gid integers for input user.
// Input user can be in the form of username:group, or uid:gid or a mix of that.
func GetUIDGID(username string) (int, int) {
	logging.LogDebug("getting uid and gid for %s", username)

	groupname := username

	// check if we're in the format user:group or 1234:1234
	if strings.Contains(username, ":") {
		logging.LogDebug("splitting %s by ':'", username)

		split := strings.Split(username, ":")
		username = split[0]
		groupname = split[1]

		logging.LogDebug("username=%s groupname=%s", username, groupname)
	}

	logging.LogDebug("checking if input username is numeric")
	// if we're passed numeric IDs, just convert and return
	uid, err := strconv.ParseInt(username, 10, 32)
	if err != nil {
		uid = -1
	}

	logging.LogDebug("checking if input groupname is numeric")
	// if we're passed numeric IDs, just convert and return
	gid, err := strconv.ParseInt(groupname, 10, 32)
	if err != nil {
		gid = -1
	}

	// return converted IDs
	if uid > 0 && gid > 0 {
		logging.LogDebug("input username and groupname were numeric, returning %d %d", uid, gid)

		return int(uid), int(gid)
	}

	logging.LogDebug("input username and groupname were not numeric, looking up")

	// if we're here it is because they were not numbers
	// so we'll need to lookup them
	user, err := user.Lookup(username)
	if err != nil {
		return 0, 0
	}

	logging.LogDebug("parsing looked up username uid and gid")

	// if we're passed numeric IDs, just convert and return
	uid, err = strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		uid = -1
	}

	gid, err = strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		gid = -1
	}

	// return converted IDs
	if uid > 0 && gid > 0 {
		logging.LogDebug("uid and gid parsed successfully, returning %d %s", uid, gid)

		return int(uid), int(gid)
	}

	// default to root
	logging.LogDebug("parsing looked up uid/gid failed, defaulting to root 0:0")

	return 0, 0
}

// GetSubIDRanges will return a slice of subUIDs and subGIDs for
// running user.
// This function will use the "getsubids" program to discover them.
func GetSubIDRanges() ([]string, []string, error) {
	user, err := user.Current()
	if err != nil {
		logging.LogError("%v", err)

		return nil, nil, err
	}

	subUIDout, err := exec.Command("getsubids", user.Username).Output()
	if err != nil {
		logging.LogError("%v", err)

		return nil, nil, err
	}

	subUIDSlice := strings.Split(
		strings.Trim(string(subUIDout), "\n"),
		" ")[2:]

	subGIDout, err := exec.Command("getsubids", "-g", user.Username).Output()
	if err != nil {
		logging.LogError("%v", err)

		return nil, nil, err
	}

	subGIDSlice := strings.Split(
		strings.Trim(string(subGIDout), "\n"),
		" ")[2:]

	subUIDSlice = append([]string{user.Uid}, subUIDSlice...)
	subGIDSlice = append([]string{user.Gid}, subGIDSlice...)

	return subUIDSlice, subGIDSlice, nil
}

// SetProcessKeepIDMaps will set child process uid/gid mappings.
func SetProcessKeepIDMaps(cmd *exec.Cmd, uidMap, gidMap string) error {
	uids := strings.Split(uidMap, ":")[0]
	uidsizes := strings.Split(uidMap, ":")[2]
	gids := strings.Split(gidMap, ":")[0]
	gidsizes := strings.Split(gidMap, ":")[2]

	uid, err := strconv.ParseInt(uids, 10, 32)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	gid, err := strconv.ParseInt(gids, 10, 32)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	uidsize, err := strconv.ParseInt(uidsizes, 10, 32)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	gidsize, err := strconv.ParseInt(gidsizes, 10, 32)
	if err != nil {
		logging.LogError("%v", err)

		return err
	}

	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      1,
			Size:        int(uid),
		},
		{
			ContainerID: int(uid),
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: int(uid) + 1,
			HostID:      int(uid) + 1,
			Size:        int(uidsize - uid),
		},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      1,
			Size:        int(gid),
		},
		{
			ContainerID: int(gid),
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: int(gid) + 1,
			HostID:      int(gid) + 1,
			Size:        int(gidsize - gid),
		},
	}

	logging.LogDebug("settings uidmap %v", cmd.SysProcAttr.UidMappings)
	logging.LogDebug("settings gidmap %v", cmd.SysProcAttr.GidMappings)

	return nil
}

// IsPidRunning will return whether or not the input pid is actually alive
// and not stopped or a zombie process.
func IsPidRunning(pid int) bool {
	processPath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")

	_, err := os.Stat(processPath)
	if err != nil {
		return false
	}

	out, err := os.ReadFile(processPath)
	if err != nil {
		return false
	}

	return len(out) > 0
}

// RunWithTTY will run input cmd using main process' stdin/out/err.
func RunWithTTY(cmd *exec.Cmd) error {
	logging.LogDebug("tty specified, just use cmd.Run")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunInteractive will run input cmd using main process' stdin, but
// pipe stdout/err to main.
//
// This usually is used in combination with the ptyAgent inside a container.
func RunInteractive(cmd *exec.Cmd) error {
	logging.LogDebug("interactive but no tty, setting up pipes")

	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	// Non-blockingly echo command output to terminal
	go func() { _, _ = io.Copy(os.Stdout, stdout) }()
	go func() { _, _ = io.Copy(os.Stderr, stderr) }()

	return cmd.Wait()
}

// RunDetached will run input cmd and redurect all outputs to logfile.
// No stdin is set up.
func RunDetached(cmd *exec.Cmd, logfile string) error {
	logging.LogDebug("no interactive and no tty, setting up process log file")

	// non interactive mode, save stdout and stderr to file and disown
	cmd.SysProcAttr.Foreground = false
	cmd.SysProcAttr.Setsid = true

	// ensure we either create the file, or truncate the existing one
	// so that we always start fresh
	_, err := os.Create(logfile)
	if err != nil {
		logging.LogDebug("%v", err)
	}

	logging.LogDebug("no interactive and no tty, setting up process pipes")

	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	cmd.Stdout = io.Writer(outW)
	cmd.Stderr = io.Writer(errW)

	stdinLines := make(chan string)
	stderrLines := make(chan string)

	var wg sync.WaitGroup

	wg.Add(5)

	go func() {
		defer wg.Done()

		for line := range stdinLines {
			line := fmt.Sprintf("%d:out:%s", time.Now().Unix(), line)

			err := logging.AppendStringToFile(logfile, line)
			if err != nil {
				logging.LogError("could not log output: %v", err)
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer close(
			stdinLines,
		) // close on exit: when the scanner has no more to read, or has encountered an error.

		scanner := bufio.NewScanner(outR)

		for scanner.Scan() {
			stdinLines <- scanner.Text()
		}
	}()

	go func() {
		defer wg.Done()

		for line := range stderrLines {
			line := fmt.Sprintf("%d:err:%s", time.Now().Unix(), line)

			err := logging.AppendStringToFile(logfile, line)
			if err != nil {
				logging.LogError("could not log output: %v", err)
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer close(
			stderrLines,
		) // close on exit: when the scanner has no more to read, or has encountered an error.

		scanner := bufio.NewScanner(errR)

		for scanner.Scan() {
			stderrLines <- scanner.Text()
		}
	}()

	// keep an eye on the child process, and exit if dead
	go func() {
		defer wg.Done()

		for {
			time.Sleep(time.Second * 5)

			if !IsPidRunning(cmd.Process.Pid) {
				os.Exit(0)
			}
		}
	}()

	logging.LogDebug("no interactive and no tty, start process in background")

	err = cmd.Start()
	if err != nil {
		return err
	}

	wg.Wait()

	return cmd.Wait()
}
