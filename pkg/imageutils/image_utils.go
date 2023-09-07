// Package imageutils contains helpers and utilities for managing and pulling
// images.
package imageutils

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/legacy"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/schollz/progressbar/v3"
)

// ImageDir is the default location for downloaded images.
var ImageDir = filepath.Join(utils.GetLilipodHome(), "images")

// GetID returns the md5sum based ID for given image.
// If a recognized ID is passed, it is returned.
func GetID(image string) string {
	// if an ID is already passed, just return
	if fileutils.Exist(filepath.Join(ImageDir, image)) {
		return image
	}

	// Normalize the name with full length registry
	ref, err := name.ParseReference(image)
	if err == nil {
		image = ref.Name()
	}

	hasher := md5.New()

	_, err = hasher.Write([]byte(image))
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// GetPath returns the path for given image name or id.
func GetPath(name string) string {
	return filepath.Join(ImageDir, GetID(name))
}

// Pull will pull a given image and save it to ImageDir.
// This function uses github.com/google/go-containerregistry/pkg/crane to pull
// the image's manifest, and performs the downloading of each layer separately.
// Each layer is deduplicated between images in order to save space, using hardlinks.
// If quiet is specified, no output nor progress will be shown.
func Pull(image string, quiet bool) (string, error) {
	// First we try to get the fully qualified uri of the image
	// eg alpine:latest -> index.docker.io/library/alpine:latest
	ref, err := name.ParseReference(image)
	if err == nil {
		image = ref.Name()
	}

	if !quiet {
		fmt.Printf("pulling image manifest: %s\n", image)
	}
	// Pull will just get us the v1.Image struct, from
	// which we get all the information we need
	imageManifest, err := crane.Pull(image)
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	// We get the layers
	layers, err := imageManifest.Layers()
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	// Prepare the image path
	targetDIR := GetPath(image)
	if !fileutils.Exist(targetDIR) {
		err := os.MkdirAll(targetDIR, os.ModePerm)
		if err != nil {
			logging.LogError("%+v", err)

			return "", err
		}
	}

	keepFiles := []string{}
	// Now we download the layers
	for _, layer := range layers {
		fileName, err := downloadLayer(targetDIR, quiet, layer)
		if err != nil {
			logging.LogError("%+v", err)

			return "", err
		}

		keepFiles = append(keepFiles, fileName)
	}

	logging.LogDebug("%d layers successfully saved", len(layers))
	logging.LogDebug("cleaning up unwanded files")

	fileList, err := os.ReadDir(targetDIR)
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	for _, file := range fileList {
		if !strings.Contains(
			strings.Join(keepFiles, ":"),
			filepath.Base(file.Name()),
		) {
			logging.LogDebug("found unwanted file %s, removing", file.Name())

			err = os.Remove(filepath.Join(targetDIR, file.Name()))
			if err != nil {
				logging.LogError("%+v", err)

				return "", err
			}
		}
	}

	if !quiet {
		fmt.Printf("saving manifest for %s\n", image)
	}
	// we save the manifest.json for later use. This contains
	// the information on how the layers are ordered and
	// how to unpack them
	rawManifest, err := imageManifest.RawManifest()
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	err = fileutils.WriteFile(filepath.Join(targetDIR, "manifest.json"), rawManifest, 0o644)
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	if !quiet {
		fmt.Printf("saving config for %s\n", image)
	}

	// The config.json file is also saved, indicating lots of information
	// about the image, like default env, entrypoint and so on
	rawConfig, err := imageManifest.RawConfigFile()
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	err = fileutils.WriteFile(filepath.Join(targetDIR, "config.json"), rawConfig, 0o644)
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	if !quiet {
		fmt.Printf("saving metadata for %s\n", image)
	}
	// We also save the fully qualified name to retrieve it later
	err = fileutils.WriteFile(filepath.Join(targetDIR, "image_name"), []byte(image), 0o644)
	if err != nil {
		logging.LogError("%+v", err)

		return "", err
	}

	if !quiet {
		fmt.Println("done")
	}

	return GetID(image), nil
}

// Inspect will return a JSON or a formatted string describing the input images.
func Inspect(images []string, format string) (string, error) {
	result := ""

	for _, image := range images {
		image := GetPath(image)

		configFile, err := fileutils.ReadFile(filepath.Join(image, "config.json"))
		if err != nil {
			return "", err
		}

		var config legacy.LayerConfigFile

		err = json.Unmarshal(configFile, &config)
		if err != nil {
			return "", err
		}

		// Go-template string
		if format != "" {
			tmpl, err := template.New("format").Parse(format)
			if err != nil {
				return "", err
			}

			var out bytes.Buffer

			err = tmpl.Execute(&out, config)
			if err != nil {
				return "", err
			}

			result += out.String()

			continue
		}
		// else we do json dump

		out, err := json.MarshalIndent(config, " ", " ")
		if err != nil {
			return "", err
		}

		result += string(out) + "\n"
	}

	return result, nil
}

// ----------------------------------------------------------------------------

// downloadLayer will download input layer into targetDIR.
// downloadLayer will first searc hexisting images inside the ImageDir in order
// to find matching layers, and hardlink them in order to save disk space.
//
// Each layer download is verified in order to ensure no corrupted downloads occur.
func downloadLayer(targetDIR string, quiet bool, layer v1.Layer) (string, error) {
	// we use this as a path to download layers, in order to
	// verify them and ensure we do not leave broken files
	tmpdir := filepath.Join(targetDIR, ".temp")

	// always cleanup before
	_ = os.RemoveAll(tmpdir)

	err := os.MkdirAll(tmpdir, 0o750)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return "", err
	}

	// and after
	defer func() { _ = os.RemoveAll(tmpdir) }()

	layerDigest, _ := layer.Digest()

	layerFileName := strings.Split(layerDigest.String(), ":")[1] + ".tar.gz"

	if !quiet {
		logging.Log("pulling layer %s", layerFileName)
	}

	// If a layer already exists, exit
	if fileutils.Exist(filepath.Join(targetDIR, layerFileName)) &&
		fileutils.CheckFileDigest(filepath.Join(targetDIR, layerFileName), layerDigest.String()) {
		if !quiet {
			logging.Log("layer %s already exists, skipping", layerFileName)
		}

		return layerFileName, nil
	}

	// But if a layer with the same name/digest exists in another directory
	// let's deduplicate the disk usage by using hardlinks
	matchingLayers := findExistingLayer(ImageDir, layerFileName)
	if len(matchingLayers) > 0 &&
		fileutils.CheckFileDigest(matchingLayers[0], layerDigest.String()) {
		if !quiet {
			logging.Log("layer %s already exists, linking", layerFileName)
		}

		return layerFileName, os.Link(matchingLayers[0], filepath.Join(targetDIR, layerFileName))
	}

	// Else we proceed with the download of the layer
	savedLayer, err := os.Create(filepath.Join(tmpdir, layerFileName))
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return "", err
	}

	defer func() { _ = savedLayer.Close() }()

	tarLayer, err := layer.Compressed()
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return "", err
	}

	layerSize, err := layer.Size()
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return "", err
	}

	bar := progressbar.NewOptions64(layerSize,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionSetVisibility(!quiet),
		progressbar.OptionSetDescription("Copying blob "+layerDigest.String()),
		progressbar.OptionOnCompletion(func() {
			println("")
			logging.Log("saving layer %s done", layerDigest.String())
		}),
	)

	_, err = io.Copy(io.MultiWriter(savedLayer, bar), tarLayer)
	if err != nil {
		logging.LogDebug("error: %+v", err)

		return "", err
	}

	// always verify if the download was correctly done by
	// checking the digest of the file
	if fileutils.CheckFileDigest(filepath.Join(tmpdir, layerFileName), layerDigest.String()) {
		err = os.Rename(filepath.Join(tmpdir, layerFileName),
			filepath.Join(targetDIR, layerFileName))

		logging.LogDebug("successfully checked layer: %s", layerFileName)

		return layerFileName, err
	}

	return "", fmt.Errorf("error getting layer")
}

// findExistingLayer is useful to find layers with matching name/digest in order to
// deduplicate disk usage by using hardlinks later.
func findExistingLayer(targetDIR, filename string) []string {
	var matchingFiles []string

	_ = filepath.WalkDir(targetDIR, func(name string, dirEntry fs.DirEntry, err error) error {
		if dirEntry.Name() == filename {
			matchingFiles = append(matchingFiles, name)
		}

		return nil
	})

	return matchingFiles
}
