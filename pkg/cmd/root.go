package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cis",
	Short: "Container Image Sync Service",
}

func Execute() error {
	return rootCmd.Execute()
} 