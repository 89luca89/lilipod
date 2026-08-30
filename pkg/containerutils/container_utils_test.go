package containerutils

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWaitForPidfdReturnsAfterProcessExit(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	cmd := exec.Command("/usr/bin/cat")
	cmd.Stdin = reader
	if err := cmd.Start(); err != nil {
		reader.Close()
		t.Fatal(err)
	}
	reader.Close()

	pidfd, err := unix.PidfdOpen(cmd.Process.Pid, 0)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	defer unix.Close(pidfd)

	if err := cmd.Process.Kill(); err != nil {
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatal(err)
		}
	}

	exited, err := waitForPidfd(pidfd, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !exited {
		t.Fatal("process exit was not detected")
	}
}

func TestWaitForPidfdHonorsTimeout(t *testing.T) {
	pidfd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(pidfd)

	const timeout = 20 * time.Millisecond
	started := time.Now()
	exited, err := waitForPidfd(pidfd, timeout)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if exited {
		t.Fatal("running process reported as exited")
	}
	if elapsed < timeout || elapsed > time.Second {
		t.Fatalf("wait lasted %s, want %s..1s", elapsed, timeout)
	}
}

func TestStopPidfdEscalatesAndWaitsForExit(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinWriter.Close()
	readyReader, readyWriter, err := os.Pipe()
	if err != nil {
		stdinReader.Close()
		t.Fatal(err)
	}

	cmd := exec.Command("/bin/sh", "-c", "trap '' TERM; printf x >&3; exec /usr/bin/cat")
	cmd.Stdin = stdinReader
	cmd.ExtraFiles = []*os.File{readyWriter}
	if err := cmd.Start(); err != nil {
		stdinReader.Close()
		readyReader.Close()
		readyWriter.Close()
		t.Fatal(err)
	}
	stdinReader.Close()
	readyWriter.Close()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	var ready [1]byte
	if _, err := readyReader.Read(ready[:]); err != nil {
		readyReader.Close()
		t.Fatal(err)
	}
	readyReader.Close()

	pidfd, err := unix.PidfdOpen(cmd.Process.Pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(pidfd)

	const timeout = 20 * time.Millisecond
	started := time.Now()
	if err := stopPidfd(pidfd, false, timeout); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	if elapsed < timeout || elapsed > time.Second {
		t.Fatalf("stop lasted %s, want %s..1s", elapsed, timeout)
	}

	exited, err := waitForPidfd(pidfd, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !exited {
		t.Fatal("stop returned before the process exited")
	}
}
