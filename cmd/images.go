// Package cmd contains all the cobra commands for the CLI application.
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/89luca89/lilipod/pkg/fileutils"
	"github.com/89luca89/lilipod/pkg/imageutils"
	"github.com/89luca89/lilipod/pkg/logging"
	"github.com/89luca89/lilipod/pkg/utils"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

// NewImagesCommand will list downloaded images with disk usage of each.
func NewImagesCommand() *cobra.Command {
	imagesCommand := &cobra.Command{
		Use:              "images [flags] IMAGE",
		Short:            "List images in local storage",
		PreRunE:          logging.Init,
		RunE:             images,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
	}

	imagesCommand.Flags().SetInterspersed(false)
	imagesCommand.Flags().Bool("digests", false, "display image digests")
	imagesCommand.Flags().BoolP("help", "h", false, "show help")
	imagesCommand.Flags().BoolP("no-trunc", "", false, "do not truncate data")
	imagesCommand.Flags().BoolP("quiet", "q", false, "display only image IDs")

	return imagesCommand
}

func images(cmd *cobra.Command, _ []string) error {
	images, err := os.ReadDir(imageutils.ImageDir)
	if err != nil {
		logging.Log("no images found")

		//nolint: nilerr
		return nil
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}

	notrunc, err := cmd.Flags().GetBool("no-trunc")
	if err != nil {
		return err
	}

	digest, err := cmd.Flags().GetBool("digests")
	if err != nil {
		return err
	}

	imageTable := table.NewWriter()
	imageTable.SetOutputMirror(os.Stdout)
	imageTable.SetStyle(utils.GetDefaultTable())

	if digest {
		imageTable.AppendHeader(table.Row{"REPOSITORY", "TAG", "DIGEST", "IMAGE ID", "SIZE"})
	} else {
		imageTable.AppendHeader(table.Row{"REPOSITORY", "TAG", "IMAGE ID", "SIZE"})
	}

	for _, img := range images {
		err = doImageRow(imageTable, img.Name(), quiet, notrunc, digest)
		if err != nil {
			return err
		}
	}

	if !quiet {
		imageTable.Render()
	}

	return nil
}

func doImageRow(imageTable table.Writer, image string, quiet, notrunc, digest bool) error {
	if quiet {
		fmt.Println(imageutils.GetID(image))

		return nil
	}

	imageFile, err := fileutils.ReadFile(filepath.Join(imageutils.ImageDir, image, "image_name"))
	if err != nil {
		logging.LogWarning("found invalid image %s, cleaning up", image)

		err = os.RemoveAll(imageutils.GetPath(image))
		if err != nil {
			logging.LogError("%+v", err)

			return err
		}

		return nil
	}

	imageName := string(bytes.Split(imageFile, []byte(":"))[0])
	imageTag := string(bytes.Split(imageFile, []byte(":"))[1])

	directorySize, err := fileutils.DiscUsageMegaBytes(filepath.Join(imageutils.ImageDir, image))
	if err != nil {
		return err
	}

	if digest {
		checksum := fileutils.GetFileDigest(
			filepath.Join(imageutils.GetPath(image), "manifest.json"),
		)
		if !notrunc {
			checksum = checksum[:12]
		}

		imageTable.AppendRow(
			[]interface{}{
				imageName,
				imageTag,
				"sha256:" + checksum,
				imageutils.GetID(image),
				directorySize,
			},
		)

		return nil
	}

	imageTable.AppendRow([]interface{}{imageName, imageTag, imageutils.GetID(image), directorySize})

	return nil
}
