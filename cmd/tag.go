package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb/v3"
	dem "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs"
	events "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/events"
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

func tag(demoPath string) {
	var output internal.Demo
	var roundLive bool
	var roundTimes internal.RoundTimes
	var tickBuffer []internal.Tick
	var lastKillTick int
	var lastTScore int = -1
	var lastCtScore int = -1
	var matchFinished bool

	// map from player id -> the id of the player who last flashed them (could be teammates)
	var lastFlashedPlayer map[uint64]uint64 = make(map[uint64]uint64)

	// map from player1 id -> (map of player2 ids of last tick where player 1 damaged player 2)
	var lastDamageTick map[uint64](map[uint64]int) = make(map[uint64](map[uint64]int))

	fmt.Printf("Tagging demo file: '%s'\n", demoPath)

	f, err := os.Open(demoPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	tmpl := `{{ green "Progress:" }} {{ bar . "[" "#" "#" "." "]"}} {{speed .}} {{percent .}}`
	bar := pb.ProgressBarTemplate(tmpl).Start64(100)

	p.RegisterEventHandler(func(e events.RoundFreezetimeEnd) {
		if matchFinished {
			return
		}

		teamCt := p.GameState().TeamCounterTerrorists()
		teamT := p.GameState().TeamTerrorists()

		// empty ticks if this is round 1 (fixes weird warmups)
		if teamCt.Score() == 0 && teamT.Score() == 0 {
			output.Ticks = nil
			tickBuffer = nil
		}

		// empty tick buffer if the score at the start of this round is the same as something that's been played already
		if lastTScore == teamT.Score() && lastCtScore == teamCt.Score() {
			tickBuffer = nil
		}

		lastTScore = teamT.Score()
		lastCtScore = teamCt.Score()

		if tickBuffer != nil {
			output.Ticks = append(output.Ticks, tickBuffer...)
			tickBuffer = nil
			writeOutput(&output, demoPath+".tagged.json")
		}

		roundTimes.StartTick = p.GameState().IngameTick()
		roundTimes.PlantTick = 0
		roundTimes.DefuseTick = 0

		roundLive = true

		tick := createTick(&p)
		tick.Type = internal.TickTypeRoundStart
		tick.GameState = internal.GetGameState(p, roundTimes, nil)
		tickBuffer = append(tickBuffer, tick)
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		if matchFinished {
			return
		}

		bar.SetCurrent(int64(p.Progress() * 100))

		roundLive = false
		switch e.Reason {
		case events.RoundEndReasonTargetBombed, events.RoundEndReasonBombDefused, events.RoundEndReasonCTWin, events.RoundEndReasonTerroristsWin, events.RoundEndReasonTargetSaved:
			var winningTeam uint
			if p.GameState().Team(e.Winner) == p.GameState().TeamCounterTerrorists() {
				winningTeam = 0
				matchFinished = internal.HasMatchFinished(lastCtScore+1, lastTScore)
			} else if p.GameState().Team(e.Winner) == p.GameState().TeamTerrorists() {
				winningTeam = 1
				matchFinished = internal.HasMatchFinished(lastCtScore, lastTScore+1)
			}

			for idx := range tickBuffer {
				tickBuffer[idx].RoundWinner = winningTeam
			}
		default:
			tickBuffer = nil
		}
	})

	p.RegisterEventHandler(func(e events.BombPlanted) {
		if matchFinished {
			return
		}

		if internal.IsLive(&p) {
			roundTimes.PlantTick = p.GameState().IngameTick()
		}

		tick := createTick(&p)
		tick.Type = internal.TickTypeBombPlant

		tick.GameState = internal.GetGameState(p, roundTimes, nil)

		tickBuffer = append(tickBuffer, tick)
	})

	p.RegisterEventHandler(func(e events.BombDefused) {
		if matchFinished {
			return
		}

		if internal.IsLive(&p) {
			roundTimes.DefuseTick = p.GameState().IngameTick()

			tick := createTick(&p)
			tick.GameState = internal.GetGameState(p, roundTimes, nil)
			tick.Type = internal.TickTypeBombDefuse
			tick.Tags = append(tick.Tags, internal.Tag{
				Action: internal.ActionDefuse,
				Player: e.Player.SteamID64,
			})
			tickBuffer = append(tickBuffer, tick)
		}
	})

	p.RegisterEventHandler(func(e events.ItemPickup) {
		if matchFinished {
			return
		}

		if internal.IsLive(&p) && roundLive && e.Weapon.String() != "C4" {
			tick := createTick(&p)

			tick.GameState = internal.GetGameState(p, roundTimes, nil)
			tick.Type = internal.TickTypeItemPickup

			tickBuffer = append(tickBuffer, tick)
		}
	})

	p.RegisterEventHandler(func(e events.ItemDrop) {
		if matchFinished {
			return
		}

		if internal.IsLive(&p) && roundLive && p.CurrentFrame() != lastKillTick && e.Weapon.String() != "C4" {
			tick := createTick(&p)

			tick.GameState = internal.GetGameState(p, roundTimes, nil)
			tick.Type = internal.TickTypeItemDrop

			tickBuffer = append(tickBuffer, tick)
		}
	})

	p.RegisterEventHandler(func(e events.PlayerFlashed) {
		if matchFinished {
			return
		}

		// update the last flashed player map
		if e.FlashDuration().Seconds() >= 1.0 {
			lastFlashedPlayer[e.Player.SteamID64] = e.Attacker.SteamID64
		}
	})

	p.RegisterEventHandler(func(e events.PlayerHurt) {
		if matchFinished {
			return
		}

		if internal.IsLive(&p) && roundLive {
			// create the pre-damage tick
			pretick := createTick(&p)
			pretick.GameState = internal.GetGameState(p, roundTimes, nil)
			pretick.Type = internal.TickTypePreDamage
			tickBuffer = append(tickBuffer, pretick)

			tick := createTick(&p)
			tick.GameState = internal.GetGameState(p, roundTimes, &e)
			tick.Type = internal.TickTypeDamage

			// player damaging
			if e.Attacker != nil {
				tick.Tags = append(tick.Tags, internal.Tag{
					Action: internal.ActionDamage,
					Player: e.Attacker.SteamID64,
				})
			}

			if e.Player.FlashDurationTime() >= 1.0 {
				if val, ok := lastFlashedPlayer[e.Player.SteamID64]; ok {
					tick.Tags = append(tick.Tags, internal.Tag{
						Action: internal.ActionFlashAssist,
						Player: val,
					})
				}
			}

			if e.Attacker != nil {
				if _, ok := lastDamageTick[e.Attacker.SteamID64]; !ok {
					lastDamageTick[e.Attacker.SteamID64] = make(map[uint64]int)
				}
				lastDamageTick[e.Attacker.SteamID64][e.Player.SteamID64] = p.CurrentFrame()
			}

			if _, ok := lastDamageTick[e.Player.SteamID64]; ok {
				for id, t := range lastDamageTick[e.Player.SteamID64] {
					if float64(p.CurrentFrame()-t)*p.TickTime().Seconds() <= 2.0 {
						tick.Tags = append(tick.Tags, internal.Tag{
							Action: internal.ActionTradeDamage,
							Player: id,
						})
					}
				}
			}

			tick.Tags = append(tick.Tags, internal.Tag{
				Action: internal.ActionHurt,
				Player: e.Player.SteamID64,
			})
			tickBuffer = append(tickBuffer, tick)

			if e.Health <= 0 {
				lastKillTick = p.CurrentFrame()
			}
		}
	})

	err = p.ParseToEnd()
	if err != nil {
		panic(err)
	}

	if tickBuffer != nil {
		output.Ticks = append(output.Ticks, tickBuffer...)
		tickBuffer = nil
		writeOutput(&output, demoPath+".tagged.json")
	}

	bar.SetCurrent(100)
	bar.Finish()
}

func createTick(p *dem.Parser) internal.Tick {
	var tick internal.Tick

	tick.ScoreCT = (*p).GameState().TeamCounterTerrorists().Score()
	tick.ScoreT = (*p).GameState().TeamTerrorists().Score()

	teamCt := (*p).GameState().TeamCounterTerrorists()
	teamT := (*p).GameState().TeamTerrorists()

	tick.TeamCT.ID = teamCt.ID()
	tick.TeamT.ID = teamT.ID()

	tick.TeamCT.Name = teamCt.ClanName()
	tick.TeamT.Name = teamT.ClanName()

	tick.Players = nil
	for _, player := range (*p).GameState().Participants().Playing() {
		steamID := player.SteamID64
		name := player.Name
		teamID := (*p).GameState().Team(player.Team).ID()

		tick.Players = append(tick.Players,
			internal.Player{SteamID: steamID, Name: name, TeamID: teamID})
	}

	tick.Tick = (*p).CurrentFrame()

	return tick
}

func writeOutput(output *internal.Demo, outputPath string) {
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
