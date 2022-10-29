package cmd

import (
	"fmt"
	// "errors"
	// "strings"
	"github.com/89luca89/scatman/pkg/pullutils"
	"os"
	// "os/signal"
	// "path/filepath"
	// "errors"
	"io/ioutil"
	"log"

	"github.com/spf13/cobra"
)

func buildHelp(*cobra.Command) error {
	help := `Description:
  Build an image from a dockerfile

Usage:
  scatman build [options] path

Examples:
  scatman build [options] ./dockerfile`

	fmt.Println(help)

	return nil
}


func NewBuildCommand() *cobra.Command {
	var buildCommand = &cobra.Command{
		Use:              "build [options] path",
		Short:            "Build an image from a dockerfile",
		RunE:             build,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	buildCommand.SetUsageFunc(buildHelp)
	buildCommand.Flags().SetInterspersed(false)

	return buildCommand
}

func build(cmd *cobra.Command, arguments []string) error {
	// if len(arguments) < 2 {
	// 	cmd.Help()

	// 	return nil
	// }
	// prepare to save manifest to temp file
	// file, err := ioutil.TempFile(os.TempDir(), "scatman")
	// if err != nil {
	// 	return err
	// }
	// remember delete it
	// defer os.Remove(file.Name())

	// buildkitURL = "docker.io/moby/buildkit:latest"
	// compose the manifest url startin from the input image
	// registryURL, manifestURL := pullutils.GetManifestURL(image)
	// imageBase, imageName, tag := pullutils.GetImagePrefixName(image)
 	
	// docker.io/moby/buildkit
	pullImage := pullutils.ImageInfo{
		Image:       "docker.io/moby/buildkit:latest",
		RegistryURL: "registry-1.docker.io",
		ManifestURL: "https://registry-1.docker.io/v2/moby/buildkit/manifests/latest",
		ImageBase:   "moby",
		ImageName:   "buildkit",
		ImageTag:    "latest",
	}
	token := ""
	file, err := ioutil.TempFile(os.TempDir(), "scatman")
	if err != nil {
		return err
	}
	// remember delete it
	defer os.Remove(file.Name())

	targetDIR = pullutils.GetImageDir(pullImage)
	fmt.Println("targetDir: ", targetDIR)
	token, err = pullutils.GetDockerPullAuthToken(pullImage.ImageBase + "/" + pullImage.ImageName)
	if err != nil {
		return err
	}
	err = pullutils.DownloadManifest(pullImage.ManifestURL, file, token)
	if err != nil {
		return err
	}

	log.Printf("Trying to pull %s ...\n", pullImage.Image)


	return nil;
}
