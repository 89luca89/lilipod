package cmd

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/89luca89/scatman/pkg/pullutils"
	"github.com/89luca89/scatman/pkg/utils"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

func imagesHelp(*cobra.Command) error {
	help := `Description:
  Lists images previously pulled to the system or created on the system.

Usage:
  scatman images`
	fmt.Println(help)

	return nil
}

// NewImagesCommand will list downloaded images with disk usage of each.
func NewImagesCommand() *cobra.Command {
	var imagesCommand = &cobra.Command{
		Use:              "images [flags] IMAGE",
		Short:            "List images in local storage",
		RunE:             images,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	imagesCommand.SetUsageFunc(imagesHelp)
	imagesCommand.Flags().SetInterspersed(false)
	imagesCommand.Flags().BoolP("help", "h", false, "show help")

	return imagesCommand
}

func images(cmd *cobra.Command, arguments []string) error {
	images, err := os.ReadDir(pullutils.ImageDir)
	if err != nil {
		return err
	}

	if len(images) == 0 {
		log.Println("No images found.")

		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.Style{
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
			DrawBorder:      true,
			SeparateColumns: true,
			SeparateFooter:  false,
			SeparateHeader:  false,
			SeparateRows:    false,
		},
	})

	t.AppendHeader(table.Row{"REPOSITORY", "TAG", "SIZE"})

	for _, img := range images {
		imageB, err := base64.StdEncoding.DecodeString(img.Name())
		if err != nil {
			return err
		}
		image := string(imageB)

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

		directorySize, err := utils.DiscUsageMegaBytes(pullutils.ImageDir + "/" + img.Name())
		if err != nil {
			return err
		}
		t.AppendRow([]interface{}{strings.Split(pullImage.Image, ":")[0], pullImage.ImageTag, directorySize})
	}

	t.Render()
	return nil
}
