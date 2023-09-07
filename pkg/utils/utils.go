// Package utils contains generic helpers, utilities and structs.
package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/89luca89/lilipod/pkg/constants"
	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/schollz/progressbar/v3"
)

// Config is a struct that holds the information
// of the container we want to create.
//
// Note that this is NOT OCI COMPLIANT, lilipod is
// oci-registry and images compliant, but doesn't need
// to create oci-compliant containers.
type Config struct {
	Env        []string `json:"env"`
	Cgroup     string   `json:"cgroup"`
	Created    string   `json:"created"`
	Gidmap     string   `json:"gidmap"`
	Hostname   string   `json:"hostname"`
	ID         string   `json:"id"`
	Image      string   `json:"image"`
	Ipc        string   `json:"ipc"`
	Names      string   `json:"names"`
	Network    string   `json:"network"`
	Pid        string   `json:"pid"`
	Privileged bool     `json:"privileged"`
	Size       string   `json:"size"`
	Status     string   `json:"status"`
	Time       string   `json:"time"`
	Uidmap     string   `json:"uidmap"`
	User       string   `json:"user"`
	Userns     string   `json:"userns"`
	Workdir    string   `json:"workdir"`
	Stopsignal string   `json:"stopsignal"`
	Mounts     []string `json:"mounts"`
	Labels     []string `json:"labels"`
	// entry point related
	Entrypoint []string `json:"entrypoint"`
}

// GetDefaultTable returns the default table style we use to print out tables.
func GetDefaultTable() table.Style {
	return table.Style{
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
			DrawBorder:      false,
			SeparateColumns: true,
			SeparateFooter:  false,
			SeparateHeader:  false,
			SeparateRows:    false,
		},
	}
}

// InitConfig returns an unmarshalled config from a byte array.
func InitConfig(input []byte) (Config, error) {
	config := Config{}
	err := json.Unmarshal(input, &config)

	return config, err
}

// SaveConfig saves current config from memory to json file.
func SaveConfig(config Config, path string) error {
	file, err := json.MarshalIndent(config, "", " ")
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return err
	}

	logging.LogDebug("save config: cleaning up %s", path)
	// ensure we start from a clean file
	_ = os.Remove(path)

	logging.LogDebug("save config: writing %s", path)

	return fileutils.WriteFile(path, file, 0o644)
}

// LoadConfig loads a config from file to config struct.
func LoadConfig(path string) (Config, error) {
	file, err := fileutils.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	return InitConfig(file)
}

// GetDefaultConfig returns a plain container Config used to reset to defaults.
func GetDefaultConfig() Config {
	return Config{
		Env: []string{
			"TERM=xterm",
			"PATH=/.local/bin:/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		Cgroup:     constants.Private,
		Created:    "none",
		Gidmap:     "",
		Ipc:        constants.Private,
		Network:    constants.Private,
		Pid:        constants.Private,
		Privileged: false,
		Time:       constants.Private,
		Uidmap:     "",
		User:       "root:root",
		Userns:     constants.Private,
		Workdir:    "/",
		Stopsignal: "SIGTERM",
		Mounts:     []string{},
		Labels:     []string{},
		Entrypoint: []string{"/bin/sh"},
	}
}

// LilipodBinPath is the bin path internally used by lilipod.
var LilipodBinPath = filepath.Join(GetLilipodHome(), "bin")

// GetLilipodHome will return where the program will save data.
// This function will search the environment or:
//
// LILIPOD_HOME
// XDG_DATA_HOME
// HOME
//
// These variable are searched in this order.
func GetLilipodHome() string {
	if os.Getenv("LILIPOD_HOME") != "" {
		return filepath.Join(os.Getenv("LILIPOD_HOME"), "lilipod")
	}

	if os.Getenv("XDG_DATA_HOME") != "" {
		return filepath.Join(os.Getenv("XDG_DATA_HOME"), "lilipod")
	}

	return filepath.Join(os.Getenv("HOME"), ".local/share/lilipod")
}

// EnsureUNIXDependencies will link the missing utility to internally managed busybox binary.
// If the binary does not exist, download it first.
// Hard dependencies include:
//   - getsubuids
//   - newuidmap
//   - newgudmap
//
// These are needed to correctly spawn user-mapped user namespaces.
// These cannot be downloaded as they either need to be setuid binaries or
// setcap binaries.
//
// Other less crucial dependencies include:
//   - tar
//   - unshare
//   - nsenter
//
// These will be downloaded as statically compiled busybox binaries if absent.
//
// Additionally the ptyAgent will be saved into lilipod's bin directory, ready to be
// injected in the containers.
func EnsureUNIXDependencies(ptyAgent []byte) error {
	hardDependencies := []string{
		"getsubids",
		"newuidmap",
		"newgidmap",
	}

	softDependencies := []string{
		"unshare",
		"nsenter",
		"tar",
	}

	logging.LogDebug("ensuring hard dependencies")

	for _, dep := range hardDependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			logging.Log("failed to find dependency %s, can't recover.", dep)

			return err
		}
	}

	logging.LogDebug("ensuring unix dependencies")

	depFail := false

	for _, dep := range softDependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			logging.Log("failed to find dependency %s, will download busybox", dep)

			depFail = true
		}
	}

	// Setup the dependencies only if we actually don't find them
	if depFail {
		logging.LogWarning("some dependencies are not found, trying to setup busybox locally")

		return setupBusybox(softDependencies)
	}

	logging.LogDebug("ensuring agent pty")

	_, err := os.Stat(filepath.Join(LilipodBinPath, "pty"))
	ptyVersion := "none"

	if err == nil {
		out, err := exec.Command(filepath.Join(LilipodBinPath, "pty"), "version").Output()
		if err != nil {
			logging.Log("%s: %v", string(out), err)

			return err
		}

		ptyVersion = string(out)
	}

	if err != nil || ptyVersion != constants.Version {
		_ = os.MkdirAll(LilipodBinPath, os.ModePerm)

		logging.LogWarning("failed to find dependency 'pty agent', will inject it")

		err = fileutils.WriteFile(filepath.Join(LilipodBinPath, "pty.tar.gz"), ptyAgent, 0o644)
		if err != nil {
			logging.Log("failed to setup dependency 'pty agent': %v", err)

			return err
		}

		logging.LogDebug("pty agent injected, extracting")

		err = fileutils.UntarFile(
			filepath.Join(LilipodBinPath, "pty.tar.gz"),
			LilipodBinPath,
			"",
		)
		if err != nil {
			logging.Log("cannot extract agent: %v", err)
		}

		logging.LogDebug("cleanup pty agent archive")

		_ = os.Remove(filepath.Join(LilipodBinPath, "pty.tar.gz"))
	}

	_ = os.MkdirAll(filepath.Join(GetLilipodHome(), "volumes"), os.ModePerm)

	return nil
}

// ----------------------------------------------------------------------------

// setupBusybox will download the busybox statically compiled binary and
// symlink missing dependencies into LILIPOD_HOME/bin.
func setupBusybox(dependencies []string) error {
	_ = os.MkdirAll(LilipodBinPath, os.ModePerm)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Get the data
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, constants.BusyboxURL, nil)
	if err != nil {
		logging.Log("error: %+v", err)

		return err
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		logging.Log("error: %+v", err)

		return err
	}

	defer func() { _ = resp.Body.Close() }()

	// Create the file
	out, err := os.Create(filepath.Join(LilipodBinPath, "busybox"))
	if err != nil {
		logging.Log("error: %+v", err)

		return err
	}

	defer func() { _ = out.Close() }()

	bar := progressbar.NewOptions64(resp.ContentLength,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionSetDescription("Downloading busybox..."),
		progressbar.OptionOnCompletion(func() {
			println(" done ")
		}),
	)

	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		logging.Log("error: %+v", err)

		return err
	}

	err = os.Chmod(out.Name(), 0o755)
	if err != nil {
		logging.Log("error: %+v", err)

		return err
	}

	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			logging.LogDebug("linking busybox to: %s", dep)

			err = os.Symlink(filepath.Join(LilipodBinPath, "busybox"),
				filepath.Join(LilipodBinPath, dep))
			if err != nil {
				return fmt.Errorf("cannot setup dependency %s, aborting", dep)
			}
		}
	}

	return nil
}
