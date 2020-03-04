package cmd

import (
	"fmt"
	"os"

	dem "github.com/markus-wa/demoinfocs-golang"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(tagCmd)
}

var tagCmd = &cobra.Command{
	Use:   "tag [.dem file]",
	Short: "Creates a player-tagged game state file for the input demo file",
	Long:  "...",
	Run: func(cmd *cobra.Command, args []string) {
		// process the file argument
		if len(args) == 0 {
			fmt.Println("ERROR: demo file not supplied.")
			panic("")
		}
		if len(args) > 1 {
			fmt.Println("ERROR: Only one demo file can be supplied.")
			panic("")
		}
		demoPath := args[0]

		_, err := os.Stat(demoPath)
		if os.IsNotExist(err) {
			fmt.Printf("ERROR: '%s' is not a file.\n", demoPath)
			panic("")
		}

		// start parsing the demo file
		tag(demoPath)
	},
}

func tag(demoPath string) {
	fmt.Printf("Processing demo file: '%s'\n", demoPath)

	f, err := os.Open(demoPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := dem.NewParser(f)

	err = p.ParseToEnd()
	if err != nil {
		panic(err)
	}
}
