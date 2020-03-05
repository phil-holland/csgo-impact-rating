package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	dem "github.com/markus-wa/demoinfocs-golang"
	"github.com/markus-wa/demoinfocs-golang/events"
	"github.com/phil-holland/csgo-impact-rating/internal"
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
			panic("demo file not supplied.")
		}
		if len(args) > 1 {
			panic("Only one demo file can be supplied.")
		}
		demoPath := args[0]

		_, err := os.Stat(demoPath)
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("ERROR: '%s' is not a file.\n", demoPath))
		}

		// start parsing the demo file
		tag(demoPath)
	},
}

var output internal.Demo

func tag(demoPath string) {
	fmt.Printf("Processing demo file: '%s'\n", demoPath)

	f, err := os.Open(demoPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := dem.NewParser(f)

	p.RegisterEventHandler(func(e events.RoundStart) {
		// set header fields if this is the first round
		if p.GameState().TotalRoundsPlayed() == 0 {
			internal.SetHeader(p, &output)
		}

		if internal.IsLive(p) {
			writeTick(p)
		}
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		if p.GameState().TotalRoundsPlayed() > 0 {
			// set team score values (dependent on round end event)
			internal.SetScores(p, &e, &output)

			// re-set the header at the end of each round
			internal.SetHeader(p, &output)
		}

		if internal.IsLive(p) {
			writeTick(p)
		}
	})

	p.RegisterEventHandler(func(e events.Kill) {
		var hs string
		if e.IsHeadshot {
			hs = " (HS)"
		}
		var wallBang string
		if e.PenetratedObjects > 0 {
			wallBang = " (WB)"
		}
		fmt.Printf("%s <%v%s%s> %s\n", e.Killer, e.Weapon, hs, wallBang, e.Victim)
	})

	p.RegisterEventHandler(func(e events.PlayerHurt) {
		if internal.IsLive(p) {
			//writeTick(p)
		}
	})

	err = p.ParseToEnd()
	if err != nil {
		panic(err)
	}
}

func writeTick(p *dem.Parser) {
	outputMarshalled, _ := json.Marshal(output)
	fmt.Printf("%s\n", string(outputMarshalled))
}
