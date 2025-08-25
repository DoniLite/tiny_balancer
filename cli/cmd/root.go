package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "mogoly",
	Short: "My custom CLI tool",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Hello from my CLI!")
	},
}
