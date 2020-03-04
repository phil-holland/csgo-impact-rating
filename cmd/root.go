package cmd

import (
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "csgo-impact-rating",
		Short: "...",
		Long:  "...",
	}
)

func Execute() error {
	return rootCmd.Execute()
}
