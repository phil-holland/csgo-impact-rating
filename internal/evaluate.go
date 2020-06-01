package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

func EvaluateDemo(taggedFilePath string, quiet bool, modelPath string) {
	// prepare a csv file
	fmt.Printf("Reading contents of json file: \"%s\"\n", taggedFilePath)
	jsonRaw, _ := ioutil.ReadFile(taggedFilePath)

	var demo Demo
	err := json.Unmarshal(jsonRaw, &demo)
	if err != nil {
		panic(err.Error())
	}

	// create a temp file to write the csv to
	file, err := ioutil.TempFile(".", "temp.*.csv")
	if err != nil {
		panic(err.Error())
	}
	defer os.Remove(file.Name())

	fmt.Printf("Writing csv to temporary file: \"%s\"\n", file.Name())
	output := "roundWinner,aliveCt,aliveT,bombDefused,bombPlanted,meanHealthCt,meanHealthT,meanValueCT,meanValueT,roundTime\n"
	for _, tick := range demo.Ticks {
		csvLine := makeCSVLine(&tick)
		output += csvLine + "\n"
	}
	file.WriteString(output)
	file.Close()

	// create a temp file to write prediction results to
	rfile, err := ioutil.TempFile(".", "temp.*.txt")
	if err != nil {
		panic(err.Error())
	}
	rfile.Close()
	defer os.Remove(rfile.Name())

	// invoke lightgbm prediction
	cmd := exec.Command("lightgbm", "task=predict", "data=\""+file.Name()+"\"",
		"header=true", "label_column=name:roundWinner", "input_model=\""+modelPath+"\"",
		"output_result=\""+rfile.Name()+"\"")
	fmt.Printf("Running command: %s\n", cmd.String())
	stdout, err := cmd.Output()
	if err != nil {
		panic(err.Error())
	}
	fmt.Print(string(stdout))
	fmt.Printf("Output results written to temporary txt file: \"%s\"\n", rfile.Name())

	// read prediction results in
	results, err := ioutil.ReadFile(rfile.Name())
	if err != nil {
		panic(err.Error())
	}
	lines := strings.Split(string(results), "\n")

	var ratingOutput Rating

	ratings := make(map[uint64]float64)
	damageRatings := make(map[uint64]float64)
	flashAssistRatings := make(map[uint64]float64)
	tradeDamageRatings := make(map[uint64]float64)
	defuseRatings := make(map[uint64]float64)
	hurtRatings := make(map[uint64]float64)
	names := make(map[uint64]string)
	ids := make(map[string]uint64)
	teamIds := make(map[uint64]int)

	roundsPlayed := 0

	// loop through tagged demo ticks
	var lastPred float64
	for idx, tick := range demo.Ticks {
		// set initial ratings, and constantly update team ID map
		for _, player := range tick.Players {
			if _, ok := ratings[player.SteamID]; !ok {
				ratings[player.SteamID] = 0.0
				damageRatings[player.SteamID] = 0.0
				flashAssistRatings[player.SteamID] = 0.0
				tradeDamageRatings[player.SteamID] = 0.0
				defuseRatings[player.SteamID] = 0.0
				hurtRatings[player.SteamID] = 0.0
				names[player.SteamID] = player.Name
				ids[player.Name] = player.SteamID
			}
			teamIds[player.SteamID] = player.TeamID
		}

		// update rounds played
		if tick.ScoreCT+tick.ScoreT+1 > roundsPlayed {
			roundsPlayed = tick.ScoreCT + tick.ScoreT + 1
		}

		pred, err := strconv.ParseFloat(lines[idx], 64)
		if err != nil {
			panic(err.Error())
		}

		// amend the prediction if the bomb has been defused - certain ct win
		if tick.GameState.BombDefused {
			pred = 0.0
		}

		// append to the round outcome prediction slice
		ratingOutput.RoundOutcomePredictions = append(ratingOutput.RoundOutcomePredictions, RoundOutcomePrediction{
			Tick:              tick.Tick,
			Round:             roundsPlayed,
			OutcomePrediction: pred,
		})

		// positive if CTs benefited, negative if Ts benefited
		change := lastPred - pred

		switch tick.Type {
		case TickTypeDamage:
			var flashingPlayer uint64 = 0
			var damagingPlayer uint64 = 0
			var hurtingPlayer uint64 = 0
			var tradedPlayers []uint64

			for _, tag := range tick.Tags {
				if tag.Action == ActionFlashAssist {
					flashingPlayer = tag.Player
				} else if tag.Action == ActionDamage {
					damagingPlayer = tag.Player
				} else if tag.Action == ActionHurt {
					hurtingPlayer = tag.Player
				} else if tag.Action == ActionTradeDamage {
					tradedPlayers = append(tradedPlayers, tag.Player)
				}
			}

			splitChange := change
			if flashingPlayer != 0 && damagingPlayer != 0 && len(tradedPlayers) > 0 {
				splitChange /= 3.0
			} else if damagingPlayer != 0 && len(tradedPlayers) > 0 {
				splitChange /= 2.0
			} else if flashingPlayer != 0 && damagingPlayer != 0 {
				splitChange /= 2.0
			}

			if damagingPlayer != 0 {
				if teamIds[damagingPlayer] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: damagingPlayer,
						Change: splitChange,
						Action: ActionDamage,
					})
					ratings[damagingPlayer] += splitChange
					damageRatings[damagingPlayer] += splitChange
				} else if teamIds[damagingPlayer] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: damagingPlayer,
						Change: -splitChange,
						Action: ActionDamage,
					})
					ratings[damagingPlayer] -= splitChange
					damageRatings[damagingPlayer] -= splitChange
				}
			}

			if flashingPlayer != 0 {
				if teamIds[flashingPlayer] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: flashingPlayer,
						Change: splitChange,
						Action: ActionFlashAssist,
					})
					ratings[flashingPlayer] += splitChange
					flashAssistRatings[flashingPlayer] += splitChange
				} else if teamIds[flashingPlayer] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: flashingPlayer,
						Change: -splitChange,
						Action: ActionFlashAssist,
					})
					ratings[flashingPlayer] -= splitChange
					flashAssistRatings[flashingPlayer] -= splitChange
				}
			}

			if len(tradedPlayers) > 0 {
				for _, tp := range tradedPlayers {
					if teamIds[tp] == tick.TeamCT.ID {
						ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
							Tick:   tick.Tick,
							Round:  roundsPlayed,
							Player: tp,
							Change: splitChange / float64(len(tradedPlayers)),
							Action: ActionTradeDamage,
						})
						ratings[tp] += splitChange
						tradeDamageRatings[tp] += splitChange
					} else if teamIds[tp] == tick.TeamT.ID {
						ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
							Tick:   tick.Tick,
							Round:  roundsPlayed,
							Player: tp,
							Change: -splitChange / float64(len(tradedPlayers)),
							Action: ActionTradeDamage,
						})
						ratings[tp] -= splitChange
						tradeDamageRatings[tp] -= splitChange
					}
				}
			}

			if hurtingPlayer != 0 {
				if teamIds[hurtingPlayer] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: hurtingPlayer,
						Change: change,
						Action: ActionHurt,
					})
					ratings[hurtingPlayer] += change
					hurtRatings[hurtingPlayer] += change
				} else if teamIds[hurtingPlayer] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: hurtingPlayer,
						Change: -change,
						Action: ActionHurt,
					})
					ratings[hurtingPlayer] -= change
					hurtRatings[hurtingPlayer] -= change
				}
			}
		case TickTypeBombDefuse:
			var defusingPlayer uint64 = 0

			for _, tag := range tick.Tags {
				if tag.Action == ActionDefuse {
					defusingPlayer = tag.Player
				}
			}

			if defusingPlayer != 0 {
				if teamIds[defusingPlayer] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: defusingPlayer,
						Change: change,
						Action: ActionDefuse,
					})
					ratings[defusingPlayer] += change
					defuseRatings[defusingPlayer] += change
				} else if teamIds[defusingPlayer] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  roundsPlayed,
						Player: defusingPlayer,
						Change: -change,
						Action: ActionDefuse,
					})
					ratings[defusingPlayer] -= change
					defuseRatings[defusingPlayer] -= change
				}
			}
		}

		lastPred = pred
	}

	ratingOutput.RoundsPlayed = roundsPlayed

	playerNames := make([]string, 0)
	for k := range ids {
		playerNames = append(playerNames, k)
	}
	sort.Strings(playerNames)

	currentRound := 1
	roundRatings := make(map[uint64]float64)
	roundDamageRatings := make(map[uint64]float64)
	roundFlashAssistRatings := make(map[uint64]float64)
	roundTradeDamageRatings := make(map[uint64]float64)
	roundDefuseRatings := make(map[uint64]float64)
	roundHurtRatings := make(map[uint64]float64)
	playerRoundRatings := make(map[uint64]([]RoundRating))

	for k := range names {
		roundRatings[k] = 0.0

		roundDamageRatings[k] = 0.0
		roundFlashAssistRatings[k] = 0.0
		roundTradeDamageRatings[k] = 0.0
		roundDefuseRatings[k] = 0.0
		roundHurtRatings[k] = 0.0
	}

	bestRoundRating := 0.0
	bestRoundPlayer := ""
	bestRound := 0

	worstRoundRating := 0.0
	worstRoundPlayer := ""
	worstRound := 0

	for idx, change := range ratingOutput.RatingChanges {
		roundRatings[change.Player] += change.Change
		switch change.Action {
		case ActionDamage:
			roundDamageRatings[change.Player] += change.Change
		case ActionFlashAssist:
			roundFlashAssistRatings[change.Player] += change.Change
		case ActionTradeDamage:
			roundTradeDamageRatings[change.Player] += change.Change
		case ActionDefuse:
			roundDefuseRatings[change.Player] += change.Change
		case ActionHurt:
			roundHurtRatings[change.Player] += change.Change
		}

		if idx == len(ratingOutput.RatingChanges)-1 || ratingOutput.RatingChanges[idx+1].Round >= currentRound+1 {
			if !quiet {
				fmt.Printf("\n[ Round %d ]\n", currentRound)
			}
			for _, name := range playerNames {
				id := ids[name]

				if _, ok := playerRoundRatings[id]; !ok {
					playerRoundRatings[id] = make([]RoundRating, 0)
				}
				playerRoundRatings[id] = append(playerRoundRatings[id], RoundRating{
					Round:       currentRound,
					TotalRating: roundRatings[id],
					RatingBreakdown: RatingBreakdown{
						DamageRating:      roundDamageRatings[id],
						FlashAssistRating: roundFlashAssistRatings[id],
						TradeDamageRating: roundTradeDamageRatings[id],
						DefuseRating:      roundDefuseRatings[id],
						HurtRating:        roundHurtRatings[id],
					},
				})

				if !quiet {
					r := 100.0 * roundRatings[id]
					dr := 100.0 * roundDamageRatings[id]
					far := 100.0 * roundFlashAssistRatings[id]
					tdr := 100.0 * roundTradeDamageRatings[id]
					der := 100.0 * roundDefuseRatings[id]
					hr := 100.0 * roundHurtRatings[id]
					fmt.Printf("> Player \"%s\" got an Impact Rating of: [%.3f] (dmg: %.3f, flash: %.3f, trd: %.3f, def: %.3f, hurt: %.3f)\n",
						name, r, dr, far, tdr, der, hr)
					if r > bestRoundRating {
						bestRoundRating = r
						bestRoundPlayer = name
						bestRound = currentRound
					}
					if r < worstRoundRating {
						worstRoundRating = r
						worstRoundPlayer = name
						worstRound = currentRound
					}
				}
				roundRatings[id] = 0.0
				roundDamageRatings[id] = 0.0
				roundFlashAssistRatings[id] = 0.0
				roundTradeDamageRatings[id] = 0.0
				roundDefuseRatings[id] = 0.0
				roundHurtRatings[id] = 0.0
			}
			currentRound++
		}
	}

	if !quiet {
		fmt.Printf("\n[ Overall ]\n")
		for _, name := range playerNames {
			id := ids[name]
			avgRating := 100.0 * ratings[id] / float64(roundsPlayed)

			avgDamageRating := 100.0 * damageRatings[id] / float64(roundsPlayed)
			avgFlashAssistRating := 100.0 * flashAssistRatings[id] / float64(roundsPlayed)
			avgTradeDamageRating := 100.0 * tradeDamageRatings[id] / float64(roundsPlayed)
			avgDefuseRating := 100.0 * defuseRatings[id] / float64(roundsPlayed)
			avgHurtRating := 100.0 * hurtRatings[id] / float64(roundsPlayed)

			fmt.Printf("> Player \"%s\" got an average Impact Rating of: [%.3f] (dmg: %.3f, flash: %.3f, trd: %.3f, def: %.3f, hurt: %.3f)\n",
				name, avgRating, avgDamageRating, avgFlashAssistRating, avgTradeDamageRating, avgDefuseRating, avgHurtRating)
		}

		fmt.Printf("\n[ Big Rounds ]\n")
		fmt.Printf("> Player %s got an Impact Rating of [%.3f] in round %d\n", bestRoundPlayer, bestRoundRating, bestRound)
		fmt.Printf("> Player %s got an Impact Rating of [%.3f] in round %d\n\n", worstRoundPlayer, worstRoundRating, worstRound)
	}

	for k, v := range names {
		ratingOutput.Players = append(ratingOutput.Players, PlayerRating{
			SteamID: k,
			Name:    v,
			OverallRating: OverallRating{
				AverageRating: ratings[k] / float64(roundsPlayed),
				RatingBreakdown: RatingBreakdown{
					DamageRating:      damageRatings[k] / float64(roundsPlayed),
					FlashAssistRating: flashAssistRatings[k] / float64(roundsPlayed),
					TradeDamageRating: tradeDamageRatings[k] / float64(roundsPlayed),
					DefuseRating:      defuseRatings[k] / float64(roundsPlayed),
					HurtRating:        hurtRatings[k] / float64(roundsPlayed),
				},
			},
			RoundRatings: playerRoundRatings[k],
		})
	}

	// write final output json
	outputPath := strings.Replace(taggedFilePath, ".tagged.json", ".rating.json", -1)
	fmt.Printf("Writing output JSON to: \"%s\"\n", outputPath)
	outputMarshalled, err := json.MarshalIndent(ratingOutput, "", "  ")
	if err != nil {
		panic(err)
	}
	outFile, err := os.Create(outputPath)
	if err != nil {
		panic(err)
	}
	defer outFile.Close()

	_, err = io.WriteString(outFile, string(outputMarshalled))
	if err != nil {
		panic(err)
	}
}

func makeCSVLine(tick *Tick) string {
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
