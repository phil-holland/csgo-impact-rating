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
// TODO: take in output path as a parameter to enable easier testing
func TagDemo(demoPath string, pretty bool) string {
	var output TaggedDemo = TaggedDemo{
		TaggedDemoMetadata: TaggedDemoMetadata{
			Version: Version,
		},
		Ticks: make([]Tick, 0),
	}
	var roundLive bool
	var startTick int
	var plantTick int
	var defuseTick int
	var tickBuffer []Tick
	var lastKillTick int
	var lastTScore int = -1
	var lastCtScore int = -1
	var matchFinished bool
	var totalProblemsEncountered int
	var roundProblemsEncountered int

	// map from player id -> the id of the player who last flashed them (could be teammates)
	var lastFlashedPlayer map[uint64]uint64 = make(map[uint64]uint64)

	// map from player1 id -> (map of player2 ids of last tick where player 1 damaged player 2)
	var lastDamageTick map[uint64](map[uint64]int) = make(map[uint64](map[uint64]int))

	fmt.Printf("Tagging demo file: \"%s\"\n", demoPath)

	f, err := os.Open(demoPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	defer p.Close()

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

		if tickBuffer != nil && roundProblemsEncountered == 0 {
			output.Ticks = append(output.Ticks, tickBuffer...)
			writeTaggedDemo(&output, demoPath+".tagged.json", pretty)
		}
		tickBuffer = nil

		startTick = p.GameState().IngameTick()
		plantTick = 0
		defuseTick = 0
		roundProblemsEncountered = 0

		roundLive = true

		tick := createTick(&p)
		tick.Type = TickRoundStart
		tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		tickBuffer = append(tickBuffer, tick)
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		if matchFinished {
			return
		}

		// find out whether the round has ended from time running out
		if plantTick == 0 && len(tickBuffer) > 0 &&
			tickBuffer[len(tickBuffer)-1].GameState.AliveCT > 0 &&
			tickBuffer[len(tickBuffer)-1].GameState.AliveT > 0 {
			tick := createTick(&p)
			tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
			tick.Type = TickTimeExpired
			for _, p := range p.GameState().Participants().Playing() {
				if p.IsAlive() {
					tick.Tags = append(tick.Tags, Tag{
						Action: ActionAlive,
						Player: p.SteamID64,
					})
				}
			}
			tickBuffer = append(tickBuffer, tick)
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
		if matchFinished || !IsLive(&p) || !roundLive {
			return
		}

		plantTick = p.GameState().IngameTick()
		tick := createTick(&p)
		tick.Type = TickBombPlant
		tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		tickBuffer = append(tickBuffer, tick)
	})

	p.RegisterEventHandler(func(e events.BombDefused) {
		if matchFinished || !IsLive(&p) || !roundLive {
			return
		}

		roundLive = false

		// create two ticks, one pre defuse before the actual defuse
		preTick := createTick(&p)
		preTick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		preTick.Type = TickPreBombDefuse
		tickBuffer = append(tickBuffer, preTick)

		defuseTick = p.GameState().IngameTick()

		tick := createTick(&p)
		tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		tick.Type = TickBombDefuse

		// add tag for each of the players alive when the bomb is defused
		for _, p := range p.GameState().Participants().Playing() {
			if p.IsAlive() {
				tick.Tags = append(tick.Tags, Tag{
					Action: ActionRetake,
					Player: p.SteamID64,
				})
			}
		}

		tickBuffer = append(tickBuffer, tick)

	})

	p.RegisterEventHandler(func(e events.BombExplode) {
		if matchFinished || !IsLive(&p) || !roundLive {
			return
		}

		roundLive = false

		tick := createTick(&p)
		for _, p := range p.GameState().Participants().Playing() {
			if p.IsAlive() {
				tick.Tags = append(tick.Tags, Tag{
					Action: ActionAlive,
					Player: p.SteamID64,
				})
			}
		}
		tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		tick.Type = TickBombExplode

		tickBuffer = append(tickBuffer, tick)
	})

	p.RegisterEventHandler(func(e events.ItemPickup) {
		if matchFinished || !IsLive(&p) || !roundLive || e.Weapon.String() == "C4" {
			return
		}

		tick := createTick(&p)
		tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		tick.Type = TickItemPickUp
		tickBuffer = append(tickBuffer, tick)
	})

	p.RegisterEventHandler(func(e events.ItemDrop) {
		if matchFinished || !IsLive(&p) || !roundLive || p.CurrentFrame() == lastKillTick || e.Weapon.String() == "C4" {
			return
		}
		tick := createTick(&p)
		tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		tick.Type = TickItemDrop
		tickBuffer = append(tickBuffer, tick)
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
		if matchFinished || !IsLive(&p) || !roundLive {
			return
		}

		// create the pre-damage tick
		pretick := createTick(&p)
		pretick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, nil)
		pretick.Type = TickPreDamage
		tickBuffer = append(tickBuffer, pretick)

		tick := createTick(&p)
		tick.GameState = GetGameState(&p, startTick, plantTick, defuseTick, &e)
		tick.Type = TickDamage

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

		// register any valid trade damage
		if _, ok := lastDamageTick[e.Player.SteamID64]; ok {
			for id, t := range lastDamageTick[e.Player.SteamID64] {
				if float64(p.CurrentFrame()-t)*p.TickTime().Seconds() <= 2.0 {
					// don't tag trade damage from the same person who's attacking
					if e.Attacker != nil {
						if e.Attacker.SteamID64 == id {
							continue
						}
					}
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

		// check if the round has ended from a kill
		if tick.GameState.AliveCT == 0 {
			roundLive = false
		}
		if tick.GameState.AliveT == 0 && tick.GameState.BombTime == 0.0 {
			roundLive = false
		}
	})

	// parse the demo file tick-by-tick - record any problem ticks, but keep
	// parsing if any issues are encountered
	for ok, err := p.ParseNextFrame(); ok; ok, err = p.ParseNextFrame() {
		if err != nil {
			roundProblemsEncountered++
			totalProblemsEncountered++

			panic(err.Error())
		}
	}

	if tickBuffer != nil {
		output.Ticks = append(output.Ticks, tickBuffer...)
		tickBuffer = nil
		writeTaggedDemo(&output, demoPath+".tagged.json", pretty)
	}

	bar.SetCurrent(100)
	bar.Finish()

	if totalProblemsEncountered > 0 {
		fmt.Printf("WARNING: %d unexpected issues were encountered whilst parsing the demo file - output may be missing data from some rounds.\n",
			totalProblemsEncountered)
	}

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

func writeTaggedDemo(output *TaggedDemo, outputPath string, pretty bool) {
	var outputMarshalled []byte
	var err error

	if pretty {
		outputMarshalled, err = json.MarshalIndent(output, "", "  ")
		if err != nil {
			panic(err)
		}
	} else {
		outputMarshalled, err = json.Marshal(output)
		if err != nil {
			panic(err)
		}
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
func GetGameState(p *dem.Parser, startTick int, plantTick int, defuseTick int, hurtEvent *events.PlayerHurt) GameState {
	var state GameState

	state.AliveCT = 0
	state.MeanHealthCT = 0
	state.MeanValueCT = 0
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
			state.MeanValueCT += float64(ct.EquipmentValueCurrent())
		}
	}
	if state.AliveCT > 0 {
		state.MeanHealthCT /= float64(state.AliveCT)
		state.MeanValueCT /= float64(state.AliveCT)
	}

	state.AliveT = 0
	state.MeanHealthT = 0
	state.MeanValueT = 0
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
			state.MeanValueT += float64(t.EquipmentValueCurrent())
		}
	}
	if state.AliveT > 0 {
		state.MeanHealthT /= float64(state.AliveT)
		state.MeanValueT /= float64(state.AliveT)
	}

	state.RoundTime = float64((*p).GameState().IngameTick()-startTick) / (*p).TickRate()

	if plantTick > 0 {
		// bomb has been planted
		state.BombTime = float64((*p).GameState().IngameTick()-plantTick) / (*p).TickRate()

		if defuseTick > 0 {
			// bomb has been defused
			state.BombDefused = true
		}
	} else {
		state.BombTime = 0.0
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
