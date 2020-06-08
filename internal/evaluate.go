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
	retakeRatings := make(map[uint64]float64)
	hurtRatings := make(map[uint64]float64)
	aliveRatings := make(map[uint64]float64)
	names := make(map[uint64]string)
	ids := make(map[string]uint64)
	teamIds := make(map[uint64]int)
	teamNames := make(map[int]string)
	ctTeamNames := make(map[int]string)
	tTeamNames := make(map[int]string)

	startCtTeam := 0
	startTTeam := 0

	roundsPlayed := 0

	// loop through tagged demo ticks
	var lastPred float64
	for idx, tick := range demo.Ticks {
		// set initial ratings, and constantly update team ID map
		for _, player := range tick.Players {
			if player.SteamID == 0 {
				// this is a bot, so ignore
				continue
			}

			if _, ok := ratings[player.SteamID]; !ok {
				ratings[player.SteamID] = 0.0
				damageRatings[player.SteamID] = 0.0
				flashAssistRatings[player.SteamID] = 0.0
				tradeDamageRatings[player.SteamID] = 0.0
				defuseRatings[player.SteamID] = 0.0
				retakeRatings[player.SteamID] = 0.0
				hurtRatings[player.SteamID] = 0.0
				aliveRatings[player.SteamID] = 0.0
				ids[player.Name] = player.SteamID
			}
			names[player.SteamID] = player.Name
			teamIds[player.SteamID] = player.TeamID
		}

		// set team names
		if idx == 0 {
			startCtTeam = tick.TeamCT.ID
			startTTeam = tick.TeamT.ID
		}
		teamNames[tick.TeamCT.ID] = tick.TeamCT.Name
		teamNames[tick.TeamT.ID] = tick.TeamT.Name

		// update rounds played
		if tick.ScoreCT+tick.ScoreT+1 > roundsPlayed {
			roundsPlayed = tick.ScoreCT + tick.ScoreT + 1
		}

		ctTeamNames[roundsPlayed] = tick.TeamCT.Name
		tTeamNames[roundsPlayed] = tick.TeamT.Name

		// get the prediction for this tick
		pred := preds[idx]

		// amend the prediction if this is a time expired tick
		if tick.Type == TickTimeExpired {
			if tick.RoundWinner == 0 {
				// should never be anything different, but make sure
				pred = 0.0
			}
		}

		// amend the prediction if this is a bomb explode tick
		if tick.Type == TickBombExplode {
			if tick.RoundWinner == 1 {
				// should never be anything different, but make sure
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
			var retakingPlayers []uint64
			var defusedOnPlayers []uint64

			for _, tag := range tick.Tags {
				if tag.Action == ActionRetake {
					if teamIds[tag.Player] == tick.TeamCT.ID {
						retakingPlayers = append(retakingPlayers, tag.Player)
					} else if teamIds[tag.Player] == tick.TeamT.ID {
						defusedOnPlayers = append(defusedOnPlayers, tag.Player)
					}
				}
			}

			avgChangeCT := change / float64(len(retakingPlayers))
			avgChangeT := change / float64(len(defusedOnPlayers))

			for _, rp := range retakingPlayers {
				// player has to be a ct
				ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
					Tick:   tick.Tick,
					Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
					Player: rp,
					Change: avgChangeCT,
					Action: ActionRetake,
				})
				ratings[rp] += avgChangeCT
				retakeRatings[rp] += avgChangeCT
			}

			for _, dop := range defusedOnPlayers {
				// player has to be a t
				ratingOutput.RatingChanges = append(ratingOutput.RatingChanges, RatingChange{
					Tick:   tick.Tick,
					Round:  Round{Number: roundsPlayed, ScoreCT: tick.ScoreCT, ScoreT: tick.ScoreT},
					Player: dop,
					Change: -avgChangeT,
					Action: ActionRetake,
				})
				ratings[dop] -= avgChangeT
				retakeRatings[dop] -= avgChangeT
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

	playerIds := make([]uint64, len(ids))
	for _, id := range ids {
		playerIds = append(playerIds, id)
	}
	sort.Slice(playerIds, func(i, j int) bool { return playerIds[i] < playerIds[j] })

	playerNames := make([]string, len(ids))
	ctMark := 0
	tMark := len(ids) - 1
	for _, id := range playerIds {
		if teamIds[id] == startCtTeam {
			playerNames[ctMark] = names[id]
			ctMark++
		} else if teamIds[id] == startTTeam {
			playerNames[tMark] = names[id]
			tMark--
		}
	}

	currentRound := 1
	roundRatings := make(map[uint64]float64)
	roundDamageRatings := make(map[uint64]float64)
	roundFlashAssistRatings := make(map[uint64]float64)
	roundTradeDamageRatings := make(map[uint64]float64)
	roundRetakeRatings := make(map[uint64]float64)
	roundHurtRatings := make(map[uint64]float64)
	roundAliveRatings := make(map[uint64]float64)
	playerRoundRatings := make(map[uint64]([]RoundRating))

	for k := range names {
		roundRatings[k] = 0.0

		roundDamageRatings[k] = 0.0
		roundFlashAssistRatings[k] = 0.0
		roundTradeDamageRatings[k] = 0.0
		roundRetakeRatings[k] = 0.0
		roundHurtRatings[k] = 0.0
		roundAliveRatings[k] = 0.0
	}

	bestRoundRating := 0.0
	bestRoundPlayer := ""
	bestRound := 0

	worstRoundRating := 0.0
	worstRoundPlayer := ""
	worstRound := 0

	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	const headerRound string = "Team \t Player \t Round Impact (%) \t|\t Damage (%) \t Flash Assists (%) \t Trade Damage (%) \t Retakes (%) \t Damage Recv. (%) \t Alive (%)"
	const borderRound string = "---- \t ------ \t ---------------- \t|\t ---------- \t ----------------- \t ---------------- \t ----------- \t ---------------- \t ---------"
	const entryRound string = "%s \t %s \t %.3f \t|\t %.3f \t %.3f \t %.3f \t %.3f \t %.3f \t %.3f\n"

	const headerOverall string = "Team \t Player \t Average Impact (%) \t|\t Damage (%) \t Flash Assists (%) \t Trade Damage (%) \t Retakes (%) \t Damage Recv. (%) \t Alive (%)"
	const borderOverall string = "---- \t ------ \t ------------------ \t|\t ---------- \t ----------------- \t ---------------- \t ----------- \t ---------------- \t ---------"
	const entryOverall string = "%s \t %s \t %.3f \t|\t %.3f \t %.3f \t %.3f \t %.3f \t %.3f \t %.3f\n"

	for idx, change := range ratingOutput.RatingChanges {
		roundRatings[change.Player] += change.Change
		switch change.Action {
		case ActionDamage:
			roundDamageRatings[change.Player] += change.Change
		case ActionFlashAssist:
			roundFlashAssistRatings[change.Player] += change.Change
		case ActionTradeDamage:
			roundTradeDamageRatings[change.Player] += change.Change
		case ActionRetake:
			roundRetakeRatings[change.Player] += change.Change
		case ActionHurt:
			roundHurtRatings[change.Player] += change.Change
		case ActionAlive:
			roundAliveRatings[change.Player] += change.Change
		}

		if idx == len(ratingOutput.RatingChanges)-1 || ratingOutput.RatingChanges[idx+1].Round.Number >= currentRound+1 {
			if verbosity >= 2 {
				fmt.Printf("\n> Round %d [%s %d : %d %s]\n\n", currentRound, ctTeamNames[currentRound], change.Round.ScoreCT, change.Round.ScoreT, tTeamNames[currentRound])
				fmt.Fprintln(tabWriter, headerRound)
				fmt.Fprintln(tabWriter, borderRound)
			}
			// TODO: add team ratings
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
						RetakeRating:      roundRetakeRatings[id],
						HurtRating:        roundHurtRatings[id],
						AliveRating:       roundAliveRatings[id],
					},
				})

				roundRating := 100.0 * roundRatings[id]
				roundDamageRating := 100.0 * roundDamageRatings[id]
				roundFlashAssistRating := 100.0 * roundFlashAssistRatings[id]
				roundTradeDamageRating := 100.0 * roundTradeDamageRatings[id]
				roundRetakeRating := 100.0 * roundRetakeRatings[id]
				roundHurtRating := 100.0 * roundHurtRatings[id]
				roundAliveRating := 100.0 * roundAliveRatings[id]

				if verbosity >= 2 {
					fmt.Fprintf(tabWriter, entryRound, teamNames[teamIds[id]], name, roundRating, roundDamageRating,
						roundFlashAssistRating, roundTradeDamageRating, roundRetakeRating, roundHurtRating, roundAliveRating)
				}
				if roundRating > bestRoundRating {
					bestRoundRating = roundRating
					bestRoundPlayer = name
					bestRound = currentRound
				}
				if roundRating < worstRoundRating {
					worstRoundRating = roundRating
					worstRoundPlayer = name
					worstRound = currentRound
				}
				roundRatings[id] = 0.0
				roundDamageRatings[id] = 0.0
				roundFlashAssistRatings[id] = 0.0
				roundTradeDamageRatings[id] = 0.0
				roundRetakeRatings[id] = 0.0
				roundHurtRatings[id] = 0.0
				roundAliveRatings[id] = 0.0
			}
			currentRound++
			tabWriter.Flush()
		}
	}

	if verbosity >= 1 {
		fmt.Printf("\n> Overall:\n\n")
		fmt.Fprintln(tabWriter, headerOverall)
		fmt.Fprintln(tabWriter, borderOverall)
		for _, name := range playerNames {
			id := ids[name]
			avgRating := 100.0 * ratings[id] / float64(roundsPlayed)

			avgDamageRating := 100.0 * damageRatings[id] / float64(roundsPlayed)
			avgFlashAssistRating := 100.0 * flashAssistRatings[id] / float64(roundsPlayed)
			avgTradeDamageRating := 100.0 * tradeDamageRatings[id] / float64(roundsPlayed)
			avgRetakeRating := 100.0 * retakeRatings[id] / float64(roundsPlayed)
			avgHurtRating := 100.0 * hurtRatings[id] / float64(roundsPlayed)
			avgAliveRating := 100.0 * aliveRatings[id] / float64(roundsPlayed)

			fmt.Fprintf(tabWriter, entryOverall, teamNames[teamIds[id]], name, avgRating, avgDamageRating,
				avgFlashAssistRating, avgTradeDamageRating, avgRetakeRating, avgHurtRating, avgAliveRating)
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
					RetakeRating:      retakeRatings[k] / float64(roundsPlayed),
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
