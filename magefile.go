//go:build mage
// +build mage

package main

import (
	"github.com/magefile/mage/sh"
)

var Default = All

func All() {
	Clean()
	Pty()
	Lilipod()
}

func Clean() error {
	if err := sh.Rm("lilipod"); err != nil {
		return err
	}
	if err := sh.Rm("pty"); err != nil {
		return err
	}
	if err := sh.Rm("pty.tar.gz"); err != nil {
		return err
	}
	return nil
}

func Lilipod() error {
	if err := sh.Rm("lilipod"); err != nil {
		return err
	}
	env := map[string]string{"CGO_ENABLED": "0"}
	if err := sh.RunWith(env, "go", "build", "-mod", "vendor", "-ldflags=-s -w -X 'github.com/89luca89/lilipod/pkg/constants.Version=$${RELEASE_VERSION:-0.0.0}'", "-o", "lilipod", "main.go"); err != nil {
		return err
	}
	return nil
}

func Coverage() error {
	if err := sh.Rm("coverage/*"); err != nil {
		return err
	}
	if err := sh.Run("mkdir", "-p", "coverage"); err != nil {
		return err
	}
	env := map[string]string{"CGO_ENABLED": "0"}
	if err := sh.RunWith(env, "go", "build", "-mod", "vendor", "-cover", "-o", "coverage/pty", "ptyagent/main.go", "ptyagent/pty.go"); err != nil {
		return err
	}
	if err := sh.Rm("pty"); err != nil {
		return err
	}
	if err := sh.Rm("pty.tar.gz"); err != nil {
		return err
	}
	if err := sh.RunWith(env, "go", "build", "-mod", "vendor", "-gcflags=all=-l -B -C", "-ldflags=-s -w", "-o", "pty", "ptyagent/main.go", "ptyagent/pty.go"); err != nil {
		return err
	}
	if err := sh.Run("tar", "czfv", "pty.tar.gz", "pty"); err != nil {
		return err
	}
	if err := sh.RunWith(env, "go", "build", "-mod", "vendor", "-cover", "-o", "coverage/lilipod", "main.go"); err != nil {
		return err
	}
	return nil
}

func Pty() error {
	if err := sh.Rm("pty"); err != nil {
		return err
	}
	if err := sh.Rm("pty.tar.gz"); err != nil {
		return err
	}
	env := map[string]string{"CGO_ENABLED": "0"}
	if err := sh.RunWith(env, "go", "build", "-mod", "vendor", "-gcflags=all=-l -B -C", "-ldflags=-s -w -X 'main.version=$${RELEASE_VERSION:-0.0.0}'", "-o", "pty", "ptyagent/main.go", "ptyagent/pty.go"); err != nil {
		return err
	}
	if err := sh.Run("tar", "czfv", "pty.tar.gz", "pty"); err != nil {
		return err
	}
	return nil
}
