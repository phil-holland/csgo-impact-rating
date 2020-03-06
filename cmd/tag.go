package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/markus-wa/demoinfocs-golang/common"

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
var roundLive bool
var roundTimes internal.RoundTimes

func tag(demoPath string) {
	fmt.Printf("Processing demo file: '%s'\n", demoPath)

	f, err := os.Open(demoPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := dem.NewParser(f)

	p.RegisterEventHandler(func(e events.RoundFreezetimeEnd) {
		// set header fields if this is the first round
		// also remove all previously saved ticks
		if p.GameState().TotalRoundsPlayed() == 0 {
			internal.SetHeader(p, &output)
			output.Ticks = nil
		}

		roundTimes.StartTick = p.GameState().IngameTick()
		roundTimes.PlantTick = 0

		roundLive = true
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		if p.GameState().TotalRoundsPlayed() > 0 {
			// set team score values (dependent on round end event)
			internal.SetScores(p, &e, &output)

			// re-set the header at the end of each round
			internal.SetHeader(p, &output)
		}

		roundLive = false
	})

	p.RegisterEventHandler(func(e events.BombPlanted) {
		if internal.IsLive(p) {
			roundTimes.PlantTick = p.GameState().IngameTick()
		}

		output.Ticks = append(output.Ticks, internal.Tick{
			Type:      internal.TickTypeBombPlant,
			GameState: internal.GetGameState(p, roundTimes),
		})
	})

	p.RegisterEventHandler(func(e events.BombDefused) {
		if internal.IsLive(p) {
			var tick internal.Tick
			tick.GameState = internal.GetGameState(p, roundTimes)
			tick.Type = internal.TickTypeBombDefuse
			tick.Tags = append(tick.Tags, internal.Tag{
				Action: internal.ActionDefuse,
				Player: e.Player.SteamID,
			})
			output.Ticks = append(output.Ticks, tick)
		}
	})

	p.RegisterEventHandler(func(e events.PlayerHurt) {
		if internal.IsLive(p) && roundLive {
			var tick internal.Tick

			tick.GameState = internal.GetGameState(p, roundTimes)
			tick.Type = internal.TickTypeDamage

			// player damaging
			if e.Attacker != nil {
				tick.Tags = append(tick.Tags, internal.Tag{
					Action: internal.ActionDamage,
					Player: e.Attacker.SteamID,
				})
			}

			// player getting hurt
			tick.Tags = append(tick.Tags, internal.Tag{
				Action: internal.ActionHurt,
				Player: e.Player.SteamID,
			})

			output.Ticks = append(output.Ticks, tick)
		}
	})

	p.RegisterEventHandler(func(e events.GamePhaseChanged) {
		if p.GameState().GamePhase() == common.GamePhaseGameEnded {
			writeOutput(demoPath + ".tagged.json")
		}
	})

	err = p.ParseToEnd()
	if err != nil {
		panic(err)
	}
}

func writeOutput(outputPath string) {
	fmt.Printf("Writing JSON output to: '%s'\n", outputPath)

	outputMarshalled, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		panic(err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	_, err = io.WriteString(file, string(outputMarshalled))
	if err != nil {
		panic(err)
	}
}
