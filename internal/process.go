package internal

import (
	dem "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/events"
)

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

func SetHeader(p dem.Parser, output *Demo) {

}

type RoundTimes struct {
	StartTick  int
	PlantTick  int
	DefuseTick int
}

func GetGameState(p dem.Parser, roundTimes RoundTimes, hurtEvent *events.PlayerHurt) GameState {
	var state GameState

	state.AliveCT = 0
	state.MeanHealthCT = 0
	for _, ct := range p.GameState().TeamCounterTerrorists().Members() {
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
	for _, t := range p.GameState().TeamTerrorists().Members() {
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
		state.MeanValueCT = float64(p.GameState().TeamCounterTerrorists().CurrentEquipmentValue()) / float64(state.AliveCT)
	}

	state.MeanValueT = 0
	if state.AliveT > 0 {
		state.MeanValueT = float64(p.GameState().TeamTerrorists().CurrentEquipmentValue()) / float64(state.AliveT)
	}

	if roundTimes.PlantTick > 0 {
		// bomb has been planted
		state.RoundTime = float64(p.GameState().IngameTick()-roundTimes.PlantTick) / p.TickRate()
		state.BombPlanted = true

		if roundTimes.DefuseTick > 0 {
			// bomb has been defused
			state.BombDefused = true
		}
	} else {
		state.RoundTime = float64(p.GameState().IngameTick()-roundTimes.StartTick) / p.TickRate()
		state.BombPlanted = false
	}

	return state
}

// HasMatchFinished returns true if one of the two teams has won the match (reached 16 rounds or won in overtime)
func HasMatchFinished(score1 int, score2 int) bool {
	// detect win condition
	if score1 > 15 {
		if (score1-16)%3 == 0 && score1-score2 > 1 {
			return true
		}
	}

	if score2 > 15 {
		if (score2-16)%3 == 0 && score2-score1 > 1 {
			return true
		}
	}

	return false
}
