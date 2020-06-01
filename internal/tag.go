package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb/v3"
	dem "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/events"
)

// TagDemo processes the input demo file, creating a '.tagged.json' file in the same directory
func TagDemo(demoPath string) string {
	var output Demo
	var roundLive bool
	var roundTimes RoundTimes
	var tickBuffer []Tick
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
		tick.Type = TickTypeRoundStart
		tick.GameState = GetGameState(&p, roundTimes, nil)
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
				matchFinished = HasMatchFinished(lastCtScore+1, lastTScore, 15)
			} else if p.GameState().Team(e.Winner) == p.GameState().TeamTerrorists() {
				winningTeam = 1
				matchFinished = HasMatchFinished(lastCtScore, lastTScore+1, 15)
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

		if IsLive(&p) {
			roundTimes.PlantTick = p.GameState().IngameTick()
		}

		tick := createTick(&p)
		tick.Type = TickTypeBombPlant

		tick.GameState = GetGameState(&p, roundTimes, nil)

		tickBuffer = append(tickBuffer, tick)
	})

	p.RegisterEventHandler(func(e events.BombDefused) {
		if matchFinished {
			return
		}

		if IsLive(&p) {
			// create two ticks, one pre defuse before the actual defuse
			preTick := createTick(&p)
			preTick.GameState = GetGameState(&p, roundTimes, nil)
			preTick.Type = TickTypePreBombDefuse
			tickBuffer = append(tickBuffer, preTick)

			roundTimes.DefuseTick = p.GameState().IngameTick()

			tick := createTick(&p)
			tick.GameState = GetGameState(&p, roundTimes, nil)
			tick.Type = TickTypeBombDefuse

			// add tag for the actual defuser
			tick.Tags = append(tick.Tags, Tag{
				Action: ActionDefuse,
				Player: e.Player.SteamID64,
			})

			// add tag for each T alive when the bomb is defused
			for _, t := range p.GameState().TeamTerrorists().Members() {
				if t.IsAlive() {
					tick.Tags = append(tick.Tags, Tag{
						Action: ActionDefusedOn,
						Player: t.SteamID64,
					})
				}
			}

			tickBuffer = append(tickBuffer, tick)
		}
	})

	p.RegisterEventHandler(func(e events.ItemPickup) {
		if matchFinished {
			return
		}

		if IsLive(&p) && roundLive && e.Weapon.String() != "C4" {
			tick := createTick(&p)

			tick.GameState = GetGameState(&p, roundTimes, nil)
			tick.Type = TickTypeItemPickup

			tickBuffer = append(tickBuffer, tick)
		}
	})

	p.RegisterEventHandler(func(e events.ItemDrop) {
		if matchFinished {
			return
		}

		if IsLive(&p) && roundLive && p.CurrentFrame() != lastKillTick && e.Weapon.String() != "C4" {
			tick := createTick(&p)

			tick.GameState = GetGameState(&p, roundTimes, nil)
			tick.Type = TickTypeItemDrop

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

		if IsLive(&p) && roundLive {
			// create the pre-damage tick
			pretick := createTick(&p)
			pretick.GameState = GetGameState(&p, roundTimes, nil)
			pretick.Type = TickTypePreDamage
			tickBuffer = append(tickBuffer, pretick)

			tick := createTick(&p)
			tick.GameState = GetGameState(&p, roundTimes, &e)
			tick.Type = TickTypeDamage

			// player damaging
			if e.Attacker != nil {
				tick.Tags = append(tick.Tags, Tag{
					Action: ActionDamage,
					Player: e.Attacker.SteamID64,
				})
			}

			if e.Player.FlashDurationTime() >= 1.0 {
				if val, ok := lastFlashedPlayer[e.Player.SteamID64]; ok {
					tick.Tags = append(tick.Tags, Tag{
						Action: ActionFlashAssist,
						Player: val,
					})
				}
			}

			if e.Attacker != nil {
				// only register players on opposing teams
				if p.GameState().Team(e.Attacker.Team).ID() != p.GameState().Team(e.Player.Team).ID() {
					if _, ok := lastDamageTick[e.Attacker.SteamID64]; !ok {
						lastDamageTick[e.Attacker.SteamID64] = make(map[uint64]int)
					}
					lastDamageTick[e.Attacker.SteamID64][e.Player.SteamID64] = p.CurrentFrame()
				}
			}

			if _, ok := lastDamageTick[e.Player.SteamID64]; ok {
				for id, t := range lastDamageTick[e.Player.SteamID64] {
					if float64(p.CurrentFrame()-t)*p.TickTime().Seconds() <= 2.0 && e.Attacker.SteamID64 != id {
						tick.Tags = append(tick.Tags, Tag{
							Action: ActionTradeDamage,
							Player: id,
						})
					}
				}
			}

			tick.Tags = append(tick.Tags, Tag{
				Action: ActionHurt,
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
		fmt.Printf("WARNING: Demo was not parsed successfully - output may not contain data for the whole match")
	}

	if tickBuffer != nil {
		output.Ticks = append(output.Ticks, tickBuffer...)
		tickBuffer = nil
		writeOutput(&output, demoPath+".tagged.json")
	}

	bar.SetCurrent(100)
	bar.Finish()

	return demoPath + ".tagged.json"
}

func createTick(p *dem.Parser) Tick {
	var tick Tick

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
			Player{SteamID: steamID, Name: name, TeamID: teamID})
	}

	tick.Tick = (*p).CurrentFrame()

	return tick
}

func writeOutput(output *Demo, outputPath string) {
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

// IsLive returns true if the parser is currently at a point where the gamestate
// should be saved
func IsLive(p *dem.Parser) bool {
	if !(*p).GameState().IsMatchStarted() {
		return false
	}

	if (*p).GameState().IsWarmupPeriod() {
		return false
	}

	if !((*p).GameState().GamePhase() == common.GamePhaseStartGamePhase ||
		(*p).GameState().GamePhase() == common.GamePhaseTeamSideSwitch) {
		return false
	}

	return true
}

// GetGameState serialises the current state of the round using only the features we care about
func GetGameState(p *dem.Parser, roundTimes RoundTimes, hurtEvent *events.PlayerHurt) GameState {
	var state GameState

	state.AliveCT = 0
	state.MeanHealthCT = 0
	for _, ct := range (*p).GameState().TeamCounterTerrorists().Members() {
		health := ct.Health()

		if hurtEvent != nil {
			if ct.SteamID64 == hurtEvent.Player.SteamID64 {
				health -= hurtEvent.HealthDamage
			}
		}

		if health > 0 {
			state.AliveCT++
			state.MeanHealthCT += float64(health)
		}
	}
	if state.AliveCT > 0 {
		state.MeanHealthCT /= float64(state.AliveCT)
	}

	state.AliveT = 0
	state.MeanHealthT = 0
	for _, t := range (*p).GameState().TeamTerrorists().Members() {
		health := t.Health()

		if hurtEvent != nil {
			if t.SteamID64 == hurtEvent.Player.SteamID64 {
				health -= hurtEvent.HealthDamage
			}
		}

		if health > 0 {
			state.AliveT++
			state.MeanHealthT += float64(health)
		}
	}
	if state.AliveT > 0 {
		state.MeanHealthT /= float64(state.AliveT)
	}

	state.MeanValueCT = 0
	if state.AliveCT > 0 {
		state.MeanValueCT = float64((*p).GameState().TeamCounterTerrorists().CurrentEquipmentValue()) / float64(state.AliveCT)
	}

	state.MeanValueT = 0
	if state.AliveT > 0 {
		state.MeanValueT = float64((*p).GameState().TeamTerrorists().CurrentEquipmentValue()) / float64(state.AliveT)
	}

	if roundTimes.PlantTick > 0 {
		// bomb has been planted
		state.RoundTime = float64((*p).GameState().IngameTick()-roundTimes.PlantTick) / (*p).TickRate()
		state.BombPlanted = true

		if roundTimes.DefuseTick > 0 {
			// bomb has been defused
			state.BombDefused = true
		}
	} else {
		state.RoundTime = float64((*p).GameState().IngameTick()-roundTimes.StartTick) / (*p).TickRate()
		state.BombPlanted = false
	}

	return state
}

// HasMatchFinished returns true if one of the two teams has won the match (reached (mr+1) rounds or won in overtime)
func HasMatchFinished(score1 int, score2 int, mr int) bool {
	if score1 > mr {
		if (score1-(mr+1))%3 == 0 && score1-score2 > 1 {
			return true
		}
	}

	if score2 > mr {
		if (score2-(mr+1))%3 == 0 && score2-score1 > 1 {
			return true
		}
	}

	return false
}
