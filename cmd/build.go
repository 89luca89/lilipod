package cmd

import (
	"fmt"
	// "strings"

	// "github.com/89luca89/scatman/pkg/utils"
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
	imageName := "docker.io/moby/buildkit:latest"
	pullArgs := []string{imageName};
	err := pull(cmd, pullArgs);
	if err != nil{
		return err
	}
	// TODO: create (with volume)
	// TODO: run 
	// TODO: store the output
	return nil;
}
