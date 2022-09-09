package cmd

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/89luca89/scatman/pkg/pullutils"
	"github.com/spf13/cobra"
)

func rmiHelp(*cobra.Command) error {
	help := `Removes one or more images from local storage

Description:
  Removes one or more previously pulled or locally created images.

Usage:
  scatman rmi [options] IMAGE [IMAGE...]

Examples:
  scatman rmi imageID
  scatman rmi --force alpine
  scatman rmi c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7

Options:
  -a, --all      Remove all images`

	fmt.Println(help)

	return nil
}

// NewRmiCommand will delete an OCI image in the configured DIR.
func NewRmiCommand() *cobra.Command {
	var rmiCommand = &cobra.Command{
		Use:              "rmi [flags] IMAGE",
		Short:            "Removes one or more images from local storage",
		RunE:             rmi,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	rmiCommand.SetUsageFunc(rmiHelp)
	rmiCommand.Flags().SetInterspersed(false)
	rmiCommand.Flags().BoolP("help", "h", false, "show help")
	rmiCommand.Flags().BoolP("all", "a", false, "Remove all images")

	return rmiCommand
}

func rmi(cmd *cobra.Command, arguments []string) error {
	delAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	if len(arguments) < 1 && !delAll {
		cmd.Help()

		return nil
	}

	// if we want to delete all, just get a list of the targets and add it to
	// the arguments.
	if delAll {
		arguments = []string{}

		images, err := os.ReadDir(pullutils.ImageDir)
		if err != nil {
			return err
		}

		for _, i := range images {
			image, err := base64.StdEncoding.DecodeString(i.Name())
			if err != nil {
				return err
			}

			arguments = append(arguments, string(image))
		}
	}

	for _, image := range arguments {
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

		// delete the targets.
		if _, err := os.Stat(targetDIR); err == nil {
			err := os.RemoveAll(targetDIR)
			if err != nil {
				return err
			}

			log.Println("Deleted: " + image)
		} else {
			log.Printf("Image %s does not exist.\n", image)
		}
	}

	return nil
}
