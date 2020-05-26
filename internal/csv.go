package internal

import "strconv"

const CSVHeader string = "roundWinner,aliveCt,aliveT,bombDefused,bombPlanted,meanHealthCt,meanHealthT,meanValueCT,meanValueT,roundTime"

func MakeCSVLine(tick *Tick) string {
	roundWinner := strconv.FormatInt(int64(tick.RoundWinner), 10)

	aliveCt := strconv.FormatInt(int64(tick.GameState.AliveCT), 10)
	aliveT := strconv.FormatInt(int64(tick.GameState.AliveT), 10)

	bombDefused := "0"
	if tick.GameState.BombDefused {
		bombDefused = "1"
	}

	bombPlanted := "0"
	if tick.GameState.BombPlanted {
		bombPlanted = "1"
	}

	meanHealthCt := strconv.FormatFloat(tick.GameState.MeanHealthCT, 'f', 4, 64)
	meanHealthT := strconv.FormatFloat(tick.GameState.MeanHealthT, 'f', 4, 64)

	meanValueCt := strconv.FormatFloat(tick.GameState.MeanValueCT, 'f', 4, 64)
	meanValueT := strconv.FormatFloat(tick.GameState.MeanValueT, 'f', 4, 64)

	roundTime := strconv.FormatFloat(tick.GameState.RoundTime, 'f', 4, 64)

	return (roundWinner + "," + aliveCt + "," + aliveT + "," + bombDefused + "," + bombPlanted +
		"," + meanHealthCt + "," + meanHealthT + "," + meanValueCt + "," + meanValueT + "," + roundTime)
}
