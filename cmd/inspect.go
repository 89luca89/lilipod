package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/89luca89/scatman/pkg/pullutils"
	"github.com/89luca89/scatman/pkg/utils"
	"github.com/Jeffail/gabs/v2"
	"github.com/spf13/cobra"
)

func inspectHelp(*cobra.Command) error {
	help := `Display the configuration of object denoted by ID

Description:
  Displays the low-level information on an object identified by name or ID.

Usage:
  scatman inspect {CHROOT|IMAGE} [...]

Options:
  -t, --type string     Specify inspect-object type (Default "chroot")`

	fmt.Println(help)

	return nil
}

// NewInspectCommand will find all the processes in given chroot and will inspect them.
func NewInspectCommand() *cobra.Command {
	var inspectCommand = &cobra.Command{
		Use:              "inspect [IMAGE|CHROOT]",
		Short:            "Inspect a chroot or image",
		RunE:             inspect,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	inspectCommand.SetUsageFunc(inspectHelp)
	inspectCommand.Flags().SetInterspersed(false)
	inspectCommand.Flags().BoolP("help", "h", false, "show help")
	inspectCommand.Flags().StringP("type", "t", "chroot", "what to inspect")

	return inspectCommand
}

func inspect(cmd *cobra.Command, arguments []string) error {
	if len(arguments) < 1 {
		cmd.Help()

		return nil
	}

	success, err := utils.EnsureRootlesskit(true)
	if err != nil {
		return err
	}

	if success {
		return nil
	}

	inspectType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}

	var inspected string

	switch inspectType {
	case "chroot":
		inspected, err = inspectChroot(arguments)
	case "image":
		inspected, err = inspectImage(arguments)
	default:
		return errors.New("unsupported inspect type")
	}

	if err != nil {
		return err
	}

	fmt.Println(inspected)

	return nil
}

func inspectChroot(arguments []string) (string, error) {
	for _, chroot := range arguments {

		configPath := utils.GetChrootDir(chroot) + "/config"
		if _, err := os.Stat(configPath); err == nil {
			config, err := utils.LoadConfig(configPath)
			if err != nil {
				return "", nil
			}

			output, err := json.MarshalIndent(config, "", "    ")
			if err != nil {
				return "", nil
			}

			return string(output), nil
		}
		return "", errors.New("Chroot" + chroot + " does not exist")
	}

	return "", nil
}

func inspectImage(arguments []string) (string, error) {
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
			manifest, err := os.ReadFile(targetDIR + "/manifest.json")
			if err != nil {
				return "", err
			}

			jsonParsed, err := gabs.ParseJSON(manifest)
			if err != nil {
				return "", err
			}

			configFile := jsonParsed.Children()[0].Path("Config").Data().(string)
			config, err := os.ReadFile(targetDIR + "/" + configFile)
			if err != nil {
				return "", err
			}

			var output bytes.Buffer
			err = json.Indent(&output, config, "", "    ")
			// output, err = json.MarshalIndent(config, "", "    ")
			if err != nil {
				return "", err
			}

			return output.String(), nil
		}
		return "", errors.New("Image " + image + " does not exist")
	}

	return "", nil
}
