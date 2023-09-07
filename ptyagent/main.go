// Package main of ptyagent. This program is used to run input process instantiating
// a working PTY.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

var version = "development"

func main() {
	if os.Args[1] == "version" {
		fmt.Println(version)

		return
	}

	pty, err := createPty()
	if err != nil {
		log.Fatal(err)
	}

	err = pty.Start()
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM

	if cmd.Stdout == nil {
		cmd.Stdout = pty.Stdout()
	}

	if cmd.Stderr == nil {
		cmd.Stderr = pty.Stderr()
	}

	if cmd.Stdin == nil {
		cmd.Stdin = pty.Stdin()
	}

	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGTERM,
		syscall.SIGINT)

	go func() {
		<-channel

		if pty != nil {
			pty.Terminate()
			pty = nil
		}

		_ = cmd.Process.Signal(syscall.SIGKILL)
	}()

	err = cmd.Run()
	if err != nil {
		if pty != nil {
			pty.Terminate()
			pty = nil
		}

		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			os.Exit(exiterr.ExitCode())
		}

		log.Fatal(err)
	}

	if pty != nil {
		pty.Terminate()
		pty = nil
	}
}
