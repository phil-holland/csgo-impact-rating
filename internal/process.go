package internal

import (
	dem "github.com/markus-wa/demoinfocs-golang"
	"github.com/markus-wa/demoinfocs-golang/common"
	"github.com/markus-wa/demoinfocs-golang/events"
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

func SetScores(p *dem.Parser, e *events.RoundEnd, output *Demo) {
	teamCt := p.GameState().TeamCounterTerrorists()
	teamT := p.GameState().TeamTerrorists()

	if e.Winner == teamCt.Team() {
		if output.Team1.ID == teamCt.ID {
			output.Team1.Score = teamCt.Score + 1
			output.Team2.Score = teamT.Score
		} else if output.Team2.ID == teamCt.ID {
			output.Team2.Score = teamCt.Score + 1
			output.Team1.Score = teamT.Score
		}
	} else if e.Winner == teamT.Team() {
		if output.Team1.ID == teamT.ID {
			output.Team1.Score = teamT.Score + 1
			output.Team2.Score = teamCt.Score
		} else if output.Team2.ID == teamT.ID {
			output.Team2.Score = teamT.Score + 1
			output.Team1.Score = teamCt.Score
		}
	}
}
