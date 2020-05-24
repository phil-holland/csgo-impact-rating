package internal

import (
	dem "github.com/markus-wa/demoinfocs-golang"
	"github.com/markus-wa/demoinfocs-golang/common"
)

// IsLive returns true if the parser is currently at a point where the gamestate
// should be saved
func IsLive(p *dem.Parser) bool {
	if !p.GameState().IsMatchStarted() {
		return false
	}

	if p.GameState().IsWarmupPeriod() {
		return false
	}

	if !(p.GameState().GamePhase() == common.GamePhaseStartGamePhase ||
		p.GameState().GamePhase() == common.GamePhaseTeamSideSwitch) {
		return false
	}

	return true
}

func SetHeader(p *dem.Parser, output *Demo) {
	team1 := p.GameState().TeamCounterTerrorists()
	team2 := p.GameState().TeamTerrorists()

	output.Team1.ID = team1.ID
	output.Team2.ID = team2.ID

	output.Team1.Name = team1.ClanName
	output.Team2.Name = team2.ClanName

	output.Team1.Flag = team1.Flag
	output.Team2.Flag = team2.Flag

	output.Players = nil
	for _, player := range p.GameState().Participants().Playing() {
		steamID := player.SteamID
		name := player.Name
		teamID := p.GameState().Team(player.Team).ID

		output.Players = append(output.Players,
			Player{SteamID: steamID, Name: name, TeamID: teamID})
	}
}

func SetScores(p *dem.Parser, output *Demo) {
	teamCt := p.GameState().TeamCounterTerrorists()
	teamT := p.GameState().TeamTerrorists()
	if output.Team1.ID == teamCt.ID {
		output.Team1.Score = teamCt.Score
		output.Team2.Score = teamT.Score
	} else if output.Team2.ID == teamCt.ID {
		output.Team2.Score = teamCt.Score
		output.Team1.Score = teamT.Score
	}
}

type RoundTimes struct {
	StartTick  int
	PlantTick  int
	DefuseTick int
}

func GetGameState(p *dem.Parser, roundTimes RoundTimes) GameState {
	var state GameState

	state.ScoreCT = p.GameState().TeamCounterTerrorists().Score
	state.ScoreT = p.GameState().TeamTerrorists().Score

	state.CTTeamID = p.GameState().TeamCounterTerrorists().ID
	state.TTeamID = p.GameState().TeamTerrorists().ID

	state.AliveCT = 0
	for _, ct := range p.GameState().TeamCounterTerrorists().Members() {
		if ct.IsAlive() {
			state.AliveCT++
		}
	}

	state.AliveT = 0
	for _, t := range p.GameState().TeamTerrorists().Members() {
		if t.IsAlive() {
			state.AliveT++
		}
	}

	// capture average health of each team
	state.MeanHealthCT = 0
	for _, ct := range p.GameState().TeamCounterTerrorists().Members() {
		if ct.IsAlive() {
			state.MeanHealthCT += float64(ct.Hp)
		}
	}
	if state.AliveCT > 0 {
		state.MeanHealthCT /= float64(state.AliveCT)
	}

	state.MeanHealthT = 0
	for _, t := range p.GameState().TeamTerrorists().Members() {
		if t.IsAlive() {
			state.MeanHealthT += float64(t.Hp)
		}
	}
	if state.AliveT > 0 {
		state.MeanHealthT /= float64(state.AliveT)
	}

	state.MeanValueCT = 0
	if state.AliveCT > 0 {
		state.MeanValueCT = float64(p.GameState().TeamCounterTerrorists().CurrentEquipmentValue()) / float64(state.AliveCT)
	}

	state.MeanValueT = 0
	if state.AliveT > 0 {
		state.MeanValueT = float64(p.GameState().TeamTerrorists().CurrentEquipmentValue()) / float64(state.AliveT)
	}

	if roundTimes.PlantTick > 0 {
		// bomb has been planted
		state.RoundTime = float64(p.GameState().IngameTick()-roundTimes.PlantTick) / p.Header().TickRate()
		state.BombPlanted = true

		if roundTimes.DefuseTick > 0 {
			// bomb has been defused
			state.BombDefused = true
		}
	} else {
		state.RoundTime = float64(p.GameState().IngameTick()-roundTimes.StartTick) / p.Header().TickRate()
		state.BombPlanted = false
	}

	return state
}

// Returns true if one of the two teams has won the match
func HasMatchFinished(p *dem.Parser) bool {
	scoreCt := p.GameState().TeamCounterTerrorists().Score
	scoreT := p.GameState().TeamTerrorists().Score

	// detect win condition
	if scoreCt > 15 {
		if (scoreCt-16)%3 == 0 && scoreCt-scoreT > 1 {
			return true
		}
	}

	if scoreT > 15 {
		if (scoreT-16)%3 == 0 && scoreT-scoreCt > 1 {
			return true
		}
	}

	return false
}
