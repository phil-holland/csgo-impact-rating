package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/dmitryikh/leaves"
)

// EvaluateDemo processes a .tagged.json file, producing an Impact Rating report which is written to
// the console and a '.rating.json' file
func EvaluateDemo(taggedFilePath string, verbosity int, modelPath string) {
	// load in the tagged json
	fmt.Printf("Reading contents of json file: \"%s\"\n", taggedFilePath)
	jsonRaw, _ := ioutil.ReadFile(taggedFilePath)

	var demo TaggedDemo
	err := json.Unmarshal(jsonRaw, &demo)
	if err != nil {
		panic(err)
	}

	// build the input float slice
	cols := 9
	input := make([]float64, len(demo.Ticks)*cols)
	for idx, tick := range demo.Ticks {
		input[idx*cols] = float64(tick.GameState.AliveCT)
		input[idx*cols+1] = float64(tick.GameState.AliveT)
		input[idx*cols+2] = float64(tick.GameState.MeanHealthCT)
		input[idx*cols+3] = float64(tick.GameState.MeanHealthT)
		input[idx*cols+4] = float64(tick.GameState.MeanValueCT)
		input[idx*cols+5] = float64(tick.GameState.MeanValueT)
		input[idx*cols+6] = float64(tick.GameState.RoundTime)
		input[idx*cols+7] = float64(tick.GameState.BombTime)
		input[idx*cols+8] = bToF64(tick.GameState.BombDefused)
	}

	// load the LightGBM model in using leaves
	fmt.Printf("Loading LightGBM model from \"%s\"\n", modelPath)
	model, err := leaves.LGEnsembleFromFile(modelPath, true)
	if err != nil {
		panic(err)
	}
	fmt.Printf("LightGBM model loaded successfully\n")

	preds := make([]float64, len(demo.Ticks))
	model.PredictDense(input, len(demo.Ticks), cols, preds, 0, 1)

	var ratingOutput Rating = Rating{
		RatingMetadata: RatingMetadata{
			Version: Version,
		},
	}

	// create a load of maps to hold cumulative player rating values
	ratings := make(map[uint64]float64)
	damageRatings := make(map[uint64]float64)
	flashAssistRatings := make(map[uint64]float64)
	tradeDamageRatings := make(map[uint64]float64)
	defuseRatings := make(map[uint64]float64)
	defusedOnRatings := make(map[uint64]float64)
	hurtRatings := make(map[uint64]float64)
	aliveRatings := make(map[uint64]float64)
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
				defusedOnRatings[player.SteamID] = 0.0
				hurtRatings[player.SteamID] = 0.0
				aliveRatings[player.SteamID] = 0.0
				names[player.SteamID] = player.Name
				ids[player.Name] = player.SteamID
			}
			teamIds[player.SteamID] = player.TeamID
		}

		// update rounds played
		if tick.ScoreCT+tick.ScoreT+1 > roundsPlayed {
			roundsPlayed = tick.ScoreCT + tick.ScoreT + 1
		}

		// get the prediction for this tick
		pred := preds[idx]

		// amend the prediction if this is a round end tick - this fixes any issues
		// with "alive" actions
		if tick.Type == TickTimeExpired {
			if tick.RoundWinner == 0 {
				pred = 0.0
			} else if tick.RoundWinner == 1 {
				pred = 1.0
			}
		}

		// append to the round outcome prediction slice
		ratingOutput.RoundOutcomePredictions = append(ratingOutput.RoundOutcomePredictions, RoundOutcomePrediction{
			Tick:              tick.Tick,
			Round:             Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
			OutcomePrediction: pred,
		})

		// positive if CTs benefited, negative if Ts benefited
		change := lastPred - pred

		switch tick.Type {
		case TickDamage:
			var flashingPlayer uint64
			var teamFlash bool
			var damagingPlayer uint64
			var hurtingPlayer uint64
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

			if flashingPlayer != 0 {
				// was this a teamflash?
				if teamIds[flashingPlayer] == teamIds[hurtingPlayer] {
					teamFlash = true
				}
			}

			splitChange := change
			if flashingPlayer != 0 && !teamFlash && damagingPlayer != 0 && len(tradedPlayers) > 0 {
				// flash assist + trade damage
				splitChange /= 3.0
			} else if damagingPlayer != 0 && len(tradedPlayers) > 0 {
				// just trade damage
				splitChange /= 2.0
			} else if flashingPlayer != 0 && !teamFlash && damagingPlayer != 0 {
				// just flash assist
				splitChange /= 2.0
			}

			if damagingPlayer != 0 {
				if teamIds[damagingPlayer] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: damagingPlayer,
						Change: splitChange,
						Action: ActionDamage,
					})
					ratings[damagingPlayer] += splitChange
					damageRatings[damagingPlayer] += splitChange
				} else if teamIds[damagingPlayer] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: damagingPlayer,
						Change: -splitChange,
						Action: ActionDamage,
					})
					ratings[damagingPlayer] -= splitChange
					damageRatings[damagingPlayer] -= splitChange
				}
			}

			if flashingPlayer != 0 && !teamFlash {
				if teamIds[flashingPlayer] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: flashingPlayer,
						Change: splitChange,
						Action: ActionFlashAssist,
					})
					ratings[flashingPlayer] += splitChange
					flashAssistRatings[flashingPlayer] += splitChange
				} else if teamIds[flashingPlayer] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: flashingPlayer,
						Change: -splitChange,
						Action: ActionFlashAssist,
					})
					ratings[flashingPlayer] -= splitChange
					flashAssistRatings[flashingPlayer] -= splitChange
				}
			}

			avgChange := splitChange / float64(len(tradedPlayers))
			for _, tp := range tradedPlayers {
				if teamIds[tp] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: tp,
						Change: avgChange,
						Action: ActionTradeDamage,
					})
					ratings[tp] += avgChange
					tradeDamageRatings[tp] += avgChange
				} else if teamIds[tp] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: tp,
						Change: -avgChange,
						Action: ActionTradeDamage,
					})
					ratings[tp] -= avgChange
					tradeDamageRatings[tp] -= avgChange
				}
			}

			if hurtingPlayer != 0 {
				splitChange := change
				if flashingPlayer != 0 && teamFlash {
					// player was teamflashed
					splitChange /= 2.0
				}

				if teamIds[hurtingPlayer] == tick.TeamCT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: hurtingPlayer,
						Change: splitChange,
						Action: ActionHurt,
					})
					ratings[hurtingPlayer] += splitChange
					hurtRatings[hurtingPlayer] += splitChange
				} else if teamIds[hurtingPlayer] == tick.TeamT.ID {
					ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
						Tick:   tick.Tick,
						Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
						Player: hurtingPlayer,
						Change: -splitChange,
						Action: ActionHurt,
					})
					ratings[hurtingPlayer] -= splitChange
					hurtRatings[hurtingPlayer] -= splitChange
				}

				if flashingPlayer != 0 && teamFlash {
					if teamIds[flashingPlayer] == tick.TeamCT.ID {
						ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
							Tick:   tick.Tick,
							Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
							Player: flashingPlayer,
							Change: splitChange,
							Action: ActionFlashAssist,
						})
						ratings[flashingPlayer] += splitChange
						flashAssistRatings[flashingPlayer] += splitChange
					} else if teamIds[flashingPlayer] == tick.TeamT.ID {
						ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
							Tick:   tick.Tick,
							Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
							Player: flashingPlayer,
							Change: -splitChange,
							Action: ActionFlashAssist,
						})
						ratings[flashingPlayer] -= splitChange
						flashAssistRatings[flashingPlayer] -= splitChange
					}
				}
			}
		case TickBombDefuse:
			var defusingPlayer uint64
			var defusedOnPlayers []uint64

			for _, tag := range tick.Tags {
				if tag.Action == ActionDefuse {
					defusingPlayer = tag.Player
				} else if tag.Action == ActionDefusedOn {
					defusedOnPlayers = append(defusedOnPlayers, tag.Player)
				}
			}

			if defusingPlayer != 0 {
				// player has to be a ct
				ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
					Tick:   tick.Tick,
					Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
					Player: defusingPlayer,
					Change: change,
					Action: ActionDefuse,
				})
				ratings[defusingPlayer] += change
				defuseRatings[defusingPlayer] += change
			}

			avgChange := change / float64(len(defusedOnPlayers))
			for _, dop := range defusedOnPlayers {
				// player has to be a t
				ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
					Tick:   tick.Tick,
					Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
					Player: dop,
					Change: -avgChange,
					Action: ActionDefusedOn,
				})
				ratings[dop] -= avgChange
				defusedOnRatings[dop] -= avgChange
			}
		default:
			var alivePlayersCT []uint64
			var alivePlayersT []uint64

			for _, tag := range tick.Tags {
				if tag.Action == ActionAlive {
					if teamIds[tag.Player] == tick.TeamCT.ID {
						alivePlayersCT = append(alivePlayersCT, tag.Player)
					} else if teamIds[tag.Player] == tick.TeamT.ID {
						alivePlayersT = append(alivePlayersT, tag.Player)
					}
				}
			}

			avgChangeCT := change / float64(len(alivePlayersCT))
			avgChangeT := change / float64(len(alivePlayersT))
			for _, ap := range alivePlayersCT {
				ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
					Tick:   tick.Tick,
					Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
					Player: ap,
					Change: avgChangeCT,
					Action: ActionAlive,
				})
				ratings[ap] += avgChangeCT
				aliveRatings[ap] += avgChangeCT
			}

			for _, ap := range alivePlayersT {
				ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
					Tick:   tick.Tick,
					Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
					Player: ap,
					Change: -avgChangeT,
					Action: ActionAlive,
				})
				ratings[ap] -= avgChangeT
				aliveRatings[ap] -= avgChangeT
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
	roundDefusedOnRatings := make(map[uint64]float64)
	roundHurtRatings := make(map[uint64]float64)
	roundAliveRatings := make(map[uint64]float64)
	playerRoundRatings := make(map[uint64]([]RoundRating))

	for k := range names {
		roundRatings[k] = 0.0

		roundDamageRatings[k] = 0.0
		roundFlashAssistRatings[k] = 0.0
		roundTradeDamageRatings[k] = 0.0
		roundDefuseRatings[k] = 0.0
		roundDefusedOnRatings[k] = 0.0
		roundHurtRatings[k] = 0.0
		roundAliveRatings[k] = 0.0
	}

	bestRoundRating := 0.0
	bestRoundPlayer := ""
	bestRound := 0

	worstRoundRating := 0.0
	worstRoundPlayer := ""
	worstRound := 0

	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

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
		case ActionDefusedOn:
			roundDefusedOnRatings[change.Player] += change.Change
		case ActionHurt:
			roundHurtRatings[change.Player] += change.Change
		case ActionAlive:
			roundAliveRatings[change.Player] += change.Change
		}

		if idx == len(ratingOutput.RatingChanges)-1 || ratingOutput.RatingChanges[idx+1].Round.Number >= currentRound+1 {
			if verbosity >= 2 {
				fmt.Printf("\n> Round %d [%d:%d]\n\n", currentRound, change.Round.ScoreCT, change.Round.ScoreT)
				fmt.Fprintln(tabWriter, "Player \t Round Impact (%) \t|\t Damage (%) \t Flash Assists (%) \t Trade Damage (%) \t Defuses (%) \t Defuses On (%) \t Damage Recv. (%) \t Alive (%)")
				fmt.Fprintln(tabWriter, "------ \t ---------------- \t|\t ---------- \t ----------------- \t ---------------- \t ----------- \t -------------- \t ---------------- \t ---------")
			}
			for _, name := range playerNames {
				id := ids[name]

				if _, ok := playerRoundRatings[id]; !ok {
					playerRoundRatings[id] = make([]RoundRating, 0)
				}
				playerRoundRatings[id] = append(playerRoundRatings[id], RoundRating{
					Round:       Round{Number: currentRound, ScoreCT: change.Round.ScoreCT, ScoreT: change.Round.ScoreT},
					TotalRating: roundRatings[id],
					RatingBreakdown: RatingBreakdown{
						DamageRating:      roundDamageRatings[id],
						FlashAssistRating: roundFlashAssistRatings[id],
						TradeDamageRating: roundTradeDamageRatings[id],
						DefuseRating:      roundDefuseRatings[id],
						HurtRating:        roundHurtRatings[id],
						AliveRating:       roundAliveRatings[id],
					},
				})

				r := 100.0 * roundRatings[id]
				dr := 100.0 * roundDamageRatings[id]
				far := 100.0 * roundFlashAssistRatings[id]
				tdr := 100.0 * roundTradeDamageRatings[id]
				der := 100.0 * roundDefuseRatings[id]
				deor := 100.0 * roundDefusedOnRatings[id]
				hr := 100.0 * roundHurtRatings[id]
				ar := 100.0 * roundAliveRatings[id]
				if verbosity >= 2 {
					fmt.Fprintf(tabWriter, "%s \t %.3f \t|\t %.3f \t %.3f \t %.3f \t %.3f \t %.3f \t %.3f \t %.3f\n",
						name, r, dr, far, tdr, der, deor, hr, ar)
				}
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
				roundRatings[id] = 0.0
				roundDamageRatings[id] = 0.0
				roundFlashAssistRatings[id] = 0.0
				roundTradeDamageRatings[id] = 0.0
				roundDefuseRatings[id] = 0.0
				roundDefusedOnRatings[id] = 0.0
				roundHurtRatings[id] = 0.0
				roundAliveRatings[id] = 0.0
			}
			currentRound++
			tabWriter.Flush()
		}
	}

	if verbosity >= 1 {
		fmt.Printf("\n> Overall:\n\n")
		fmt.Fprintln(tabWriter, "Player \t Average Impact (%) \t|\t Damage (%) \t Flash Assists (%) \t Trade Damage (%) \t Defuses (%) \t Defuses On (%) \t Damage Recv. (%) \t Alive (%)")
		fmt.Fprintln(tabWriter, "------ \t ------------------ \t|\t ---------- \t ----------------- \t ---------------- \t ----------- \t -------------- \t ---------------- \t ---------")
		for _, name := range playerNames {
			id := ids[name]
			avgRating := 100.0 * ratings[id] / float64(roundsPlayed)

			avgDamageRating := 100.0 * damageRatings[id] / float64(roundsPlayed)
			avgFlashAssistRating := 100.0 * flashAssistRatings[id] / float64(roundsPlayed)
			avgTradeDamageRating := 100.0 * tradeDamageRatings[id] / float64(roundsPlayed)
			avgDefuseRating := 100.0 * defuseRatings[id] / float64(roundsPlayed)
			avgDefusedOnRating := 100.0 * defusedOnRatings[id] / float64(roundsPlayed)
			avgHurtRating := 100.0 * hurtRatings[id] / float64(roundsPlayed)
			avgAliveRating := 100.0 * aliveRatings[id] / float64(roundsPlayed)

			fmt.Fprintf(tabWriter, "%s \t %.3f \t|\t %.3f \t %.3f \t %.3f \t %.3f \t %.3f \t %.3f \t %.3f\n",
				name, avgRating, avgDamageRating, avgFlashAssistRating, avgTradeDamageRating, avgDefuseRating, avgDefusedOnRating, avgHurtRating, avgAliveRating)
		}
		tabWriter.Flush()

		fmt.Printf("\n> Big Rounds:\n\n")
		fmt.Printf("%s got an Impact Rating of %.3f in round %d\n", bestRoundPlayer, bestRoundRating, bestRound)
		fmt.Printf("%s got an Impact Rating of %.3f in round %d\n\n", worstRoundPlayer, worstRoundRating, worstRound)
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
					AliveRating:       aliveRatings[k] / float64(roundsPlayed),
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

func bToF64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
