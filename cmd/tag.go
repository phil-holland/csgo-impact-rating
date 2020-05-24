package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb/v3"
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
var alreadyWritten bool

func tag(demoPath string) {
	fmt.Printf("Processing demo file: '%s'\n", demoPath)

	f, err := os.Open(demoPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	tmpl := `{{ green "Progress:" }} {{ bar . "[" "#" "#" "." "]"}} {{speed .}} {{percent .}}`
	bar := pb.ProgressBarTemplate(tmpl).Start64(100)

	p.RegisterEventHandler(func(e events.RoundFreezetimeEnd) {
		// set header fields if this is the first round
		// also remove all previously saved ticks
		if p.GameState().TeamCounterTerrorists().Score == 0 && p.GameState().TeamTerrorists().Score == 0 {
			internal.SetHeader(p, &output)
			output.Ticks = nil
		}

		roundTimes.StartTick = p.GameState().IngameTick()
		roundTimes.PlantTick = 0
		roundTimes.DefuseTick = 0

		roundLive = true
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		bar.SetCurrent(int64(p.Progress() * 100))
		roundLive = false
	})

	p.RegisterEventHandler(func(e events.ScoreUpdated) {
		if p.GameState().TotalRoundsPlayed() > 0 {
			// set team score values (dependent on round end event)
			internal.SetScores(p, &output)

			// re-set the header at the end of each round
			internal.SetHeader(p, &output)
		}

		// write output at the end of the final round
		if internal.HasMatchFinished(p) {
			bar.SetCurrent(100)
			bar.Finish()
			writeOutput(demoPath + ".tagged.json")
		}
	})

	p.RegisterEventHandler(func(e events.BombPlanted) {
		if internal.IsLive(p) {
			roundTimes.PlantTick = p.GameState().IngameTick()
		}

		output.Ticks = append(output.Ticks, internal.Tick{
			Type:      internal.TickTypeBombPlant,
			Tick:      p.CurrentFrame(),
			GameState: internal.GetGameState(p, roundTimes),
		})
	})

	p.RegisterEventHandler(func(e events.BombDefused) {
		if internal.IsLive(p) {
			roundTimes.DefuseTick = p.GameState().IngameTick()

			var tick internal.Tick
			tick.GameState = internal.GetGameState(p, roundTimes)
			tick.Type = internal.TickTypeBombDefuse
			tick.Tick = p.CurrentFrame()
			tick.Tags = append(tick.Tags, internal.Tag{
				Action: internal.ActionDefuse,
				Player: e.Player.SteamID,
			})
			output.Ticks = append(output.Ticks, tick)
		}
	})

	p.RegisterEventHandler(func(e events.PlayerHurt) {
		if internal.IsLive(p) && roundLive {
			e.Player.Hp = e.Health

			var tick internal.Tick

			tick.GameState = internal.GetGameState(p, roundTimes)
			tick.Type = internal.TickTypeDamage
			tick.Tick = p.CurrentFrame()

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

	err = p.ParseToEnd()
	if err != nil {
		panic(err)
	}
	bar.SetCurrent(100)
	bar.Finish()
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
