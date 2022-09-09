package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/89luca89/scatman/pkg/pullutils"
	"github.com/Jeffail/gabs/v2"
	"github.com/spf13/cobra"
)

var targetDIR string

func pullHelp(*cobra.Command) error {
	help := `Description:
  Pulls an image from a registry and stores it locally.

  An image can be pulled by tag or digest. If a tag is not specified, the image with the 'latest' tag is pulled.

Usage:
  scatman pull [options] IMAGE [IMAGE...]`
	fmt.Println(help)

	return nil
}

// NewPullCommand will pull a new container image from a registry.
func NewPullCommand() *cobra.Command {
	var pullCommand = &cobra.Command{
		Use:              "pull [flags] IMAGE",
		Short:            "Pull an image from a registry",
		RunE:             pull,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	pullCommand.SetUsageFunc(pullHelp)
	pullCommand.Flags().SetInterspersed(false)
	pullCommand.Flags().BoolP("help", "h", false, "show help")

	return pullCommand
}

func handleSingleManifest(jsonParsed *gabs.Container, pullImage pullutils.ImageInfo) error {
	var err error

	configDigest, ok := jsonParsed.Path("config.digest").Data().(string)
	if !ok {
		return errors.New(jsonParsed.String())
	}

	configFileName := targetDIR + "/" + strings.Split(configDigest, ":")[1] + ".json"
	configURL := "https://" + pullImage.RegistryURL + "/v2/" + pullImage.ImageBase +
		"/" + pullImage.ImageName + "/blobs/" + configDigest

	// docker requires a token
	token := ""
	if strings.Contains(pullImage.ManifestURL, "docker") {
		token, err = pullutils.GetDockerPullAuthToken(pullImage.ImageBase + "/" + pullImage.ImageName)
		if err != nil {
			return err
		}
	}

	err = pullutils.DownloadBlobToFile(configURL, token, configFileName, false)
	if err != nil {
		return err
	}

	log.Printf("Copying config %s done", strings.Split(configDigest, ":")[1][:12])

	layers := jsonParsed.Path("layers").Children()
	layerID := ""
	layerArray := ""

	for index, layer := range layers {
		layerMediaType, ok := layer.Path("mediaType").Data().(string)
		if !ok {
			return errors.New(jsonParsed.String())
		}

		layerDigest, ok := layer.Path("digest").Data().(string)
		if !ok {
			return errors.New(jsonParsed.String())
		}

		// save the previous layer's ID
		parentLayerID := layerID

		// create a new fake layer ID based on this layer's digest and the previous layer's fake ID
		composedID := parentLayerID + "\n" + layerDigest
		sum := sha256.Sum256([]byte(composedID))
		layerID = hex.EncodeToString(sum[:])
		// this accounts for the possibility that an image contains the same layer
		// twice (and thus has a duplicate digest value)

		err = os.MkdirAll(targetDIR+layerID, os.ModePerm)
		if err != nil {
			return err
		}

		versionFile, err := os.Create(targetDIR + layerID + "/VERSION")
		if err != nil {
			return err
		}
		defer versionFile.Close()

		_, err = versionFile.WriteString("1.0\n")
		if err != nil {
			return err
		}

		// create JSON file if it does not exist.
		if _, err := os.Stat(targetDIR + layerID + "/json"); errors.Is(err, os.ErrNotExist) {
			idString := `"id": "` + layerID + `",`
			if parentLayerID != "" {
				idString = idString + "\n\t" + `"parent": "` + parentLayerID + `",`
			}

			jsonContent := fmt.Sprintf(pullutils.StarterJSON, idString)

			jsonFile, err := os.Create(targetDIR + layerID + "/json")
			if err != nil {
				return err
			}
			defer jsonFile.Close()

			_, err = jsonFile.WriteString(jsonContent)
			if err != nil {
				return err
			}
		}

		// download the layer now!
		if layerMediaType == "application/vnd.docker.image.rootfs.diff.tar.gzip" {
			layerTarFile := targetDIR + layerID + "/layer" + strconv.Itoa(index) + ".tar"

			// if layer exists, skip and continue
			if _, err := os.Stat(layerTarFile); err == nil {
				log.Printf("Copying blob %s skipped: already exists\n", layerID[:12])

				continue
			}

			// docker.io reuires a bearer token to be fetched
			token := ""
			if strings.Contains(pullImage.ManifestURL, "docker") {
				token, err = pullutils.GetDockerPullAuthToken(pullImage.ImageBase + "/" + pullImage.ImageName)
				if err != nil {
					return err
				}
			}

			tarURL := "https://" + pullImage.RegistryURL + "/v2/" + pullImage.ImageBase +
				"/" + pullImage.ImageName + "/blobs/" + layerDigest

			err := pullutils.DownloadBlobToFile(tarURL, token, layerTarFile, true)
			if err != nil {
				return err
			}
		} else {
			return errors.New("Error: unknown layer media type")
		}

		// add the layer to the array of layers in the manifest.
		// only add a comma if we're not the last
		layerArray += "\t\t\"" + layerID + "/layer" + strconv.Itoa(index) + ".tar" + "\""
		if index < len(layers)-1 {
			layerArray += ","
		}

		layerArray += "\n"
	}

	log.Println("Writing manifest to image destination")

	// save manifest to targetDIR for this image.
	jsonContent := fmt.Sprintf(pullutils.ManifestJSON,
		filepath.Base(configFileName),
		pullImage.Image,
		layerArray)

	jsonFile, err := os.Create(targetDIR + "/manifest.json")
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	_, err = jsonFile.WriteString(jsonContent)
	if err != nil {
		return err
	}

	fmt.Println(strings.Split(configDigest, ":")[1])

	return nil
}

func interpretManifest(jsonPath string, pullImage pullutils.ImageInfo) error {
	inputFile, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	jsonParsed, err := gabs.ParseJSON(inputFile)
	if err != nil {
		return err
	}

	// check if we're facing a single or a "fat" manifest
	mediaType, ok := jsonParsed.Path("mediaType").Data().(string)
	if !ok {
		return errors.New(jsonParsed.String())
	}

	if mediaType == "application/vnd.docker.distribution.manifest.v2+json" {
		err = handleSingleManifest(jsonParsed, pullImage)
	} else {
		return errors.New("Error: unknown manifest type")
	}

	return err
}

// SetupCloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the OS. We then handle this by calling
// our clean up procedure and exiting the program.
func setupCloseHandler(dir string) {
	// signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	abortCh := make(chan os.Signal, 2)
	signal.Notify(abortCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-abortCh
		log.Println("\nInterrupted. Cleaning up.")
		os.RemoveAll(dir)
		os.Exit(0)
	}()
}

// Pull will download an OCI image in the configured DIR.
func pull(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		cmd.Help()

		return nil
	}

	for _, image := range arguments {
		// prepare to save manifest to temp file
		file, err := ioutil.TempFile(os.TempDir(), "scatman")
		if err != nil {
			return err
		}
		// remember delete it
		defer os.Remove(file.Name())

		// compose the manifest url startin from the input image
		registryURL, manifestURL := pullutils.GetManifestURL(image)
		imageBase, imageName, tag := pullutils.GetImagePrefixName(image)

		pullImage := pullutils.ImageInfo{
			Image:       image,
			RegistryURL: registryURL,
			ManifestURL: manifestURL,
			ImageBase:   imageBase,
			ImageName:   imageName,
			ImageTag:    tag,
		}

		targetDIR = pullutils.GetImageDir(pullImage)

		if _, err := os.Stat(targetDIR); errors.Is(err, os.ErrNotExist) {
			err := os.MkdirAll(targetDIR, os.ModePerm)
			if err != nil {
				return err
			}
		}

		// handle interrupts. so we can clean up empty dirs.
		setupCloseHandler(targetDIR)

		// docker.io reuires a bearer token to be fetched
		token := ""
		if strings.Contains(pullImage.ManifestURL, "docker") {
			token, err = pullutils.GetDockerPullAuthToken(pullImage.ImageBase + "/" + pullImage.ImageName)
			if err != nil {
				return err
			}
		}

		// download manifest to file
		err = pullutils.DownloadManifest(pullImage.ManifestURL, file, token)
		if err != nil {
			return err
		}

		log.Printf("Trying to pull %s ...\n", image)
		// interpre downloaded manifest, and download all the blobs
		err = interpretManifest(file.Name(), pullImage)
		if err != nil {
			os.RemoveAll(targetDIR)

			return err
		}
	}

	return nil
}
