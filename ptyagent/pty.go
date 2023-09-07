package main

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"

	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

type pty struct {
	wg      sync.WaitGroup
	signals chan os.Signal

	previousStdinTermios unix.Termios
	stdinIsatty          bool

	master *os.File
	slave  *os.File
}

func (p *pty) Stdin() *os.File {
	return p.slave
}

func (p *pty) Stdout() *os.File {
	return p.slave
}

func (p *pty) Stderr() *os.File {
	return p.slave
}

func (p *pty) Start() error {
	err := p.makeStdinRaw()
	if err != nil {
		return err
	}

	p.handleSignals()

	p.wg.Add(2)

	go func() {
		_, _ = io.Copy(p.master, os.Stdin)
		p.wg.Done()
	}()

	go func() {
		_, _ = io.Copy(os.Stdout, p.master)
		p.wg.Done()
	}()

	return p.inheritWindowSize()
}

func (p *pty) Terminate() {
	p.restoreStdin()

	err := p.master.Close()
	if err != nil {
		panic(err)
	}

	err = p.slave.Close()
	if err != nil {
		panic(err)
	}

	close(p.signals)
}

func (p *pty) handleSignals() {
	signal.Notify(p.signals, unix.SIGWINCH)

	go func() {
		for signal := range p.signals {
			if signal == unix.SIGWINCH {
				_ = p.inheritWindowSize()
			}
		}
	}()
}

func (p *pty) inheritWindowSize() error {
	winsz, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return err
	}

	err = unix.IoctlSetWinsize(int(p.master.Fd()), unix.TIOCSWINSZ, winsz)
	if err != nil {
		return err
	}

	return nil
}

func (p *pty) makeStdinRaw() error {
	var stdinTermios unix.Termios

	err := termios.Tcgetattr(os.Stdin.Fd(), &stdinTermios)
	// We might get ENOTTY if stdin is redirected
	if err != nil {
		var unixerr *unix.Errno

		if errors.As(err, &unixerr) {
			if *unixerr == unix.ENOTTY {
				return nil
			}

			return err
		}
	}

	p.previousStdinTermios = stdinTermios
	p.stdinIsatty = true

	termios.Cfmakeraw(&stdinTermios)

	return termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &stdinTermios)
}

func (p *pty) restoreStdin() {
	if p.stdinIsatty {
		_ = termios.Tcsetattr(os.Stdin.Fd(), termios.TCSANOW, &p.previousStdinTermios)
	}
}

// createPty will create a pseudo-terminal and return it.
func createPty() (*pty, error) {
	master, slave, err := termios.Pty()
	if err != nil {
		return nil, err
	}

	return &pty{
		master:  master,
		slave:   slave,
		signals: make(chan os.Signal, 1),
	}, nil
}
