package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
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
	cols := 10
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
		input[idx*cols+8] = bToF64(tick.GameState.BombDefusing)
		input[idx*cols+9] = bToF64(tick.GameState.BombDefused)
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
	names := make(map[uint64]string)
	ids := make(map[string]uint64)
	teamIds := make(map[uint64]int)
	teamNames := make(map[int]string)
	ctTeamNames := make(map[int]string)
	tTeamNames := make(map[int]string)
	ctTeamIds := make(map[int]int)
	tTeamIds := make(map[int]int)

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
			}
			ids[player.Name] = player.SteamID
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
		ctTeamIds[roundsPlayed] = tick.TeamCT.ID
		tTeamNames[roundsPlayed] = tick.TeamT.Name
		tTeamIds[roundsPlayed] = tick.TeamT.ID

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
	playerRoundRatings := make(map[uint64]([]RoundRating))

	for k := range names {
		roundRatings[k] = 0.0

		roundDamageRatings[k] = 0.0
		roundFlashAssistRatings[k] = 0.0
		roundTradeDamageRatings[k] = 0.0
		roundRetakeRatings[k] = 0.0
		roundHurtRatings[k] = 0.0
	}

	bestRoundRating := 0.0
	bestRoundPlayer := ""
	bestRound := 0

	worstRoundRating := 0.0
	worstRoundPlayer := ""
	worstRound := 0

	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	const headerRound string = "Team \t Player \t Round Impact (%) \t|\t Damage (%) \t Flash Assists (%) \t Trade Damage (%) \t Retakes (%) \t Damage Recv. (%)"
	const borderRound string = "---- \t ------ \t ---------------- \t|\t ---------- \t ----------------- \t ---------------- \t ----------- \t ----------------"
	const entryRound string = "%s \t %s \t %.3f \t|\t %.3f \t %.3f \t %.3f \t %.3f \t %.3f\n"

	const headerOverall string = "Team \t Player \t Average Impact (%) \t|\t Damage (%) \t Flash Assists (%) \t Trade Damage (%) \t Retakes (%) \t Damage Recv. (%)"
	const borderOverall string = "---- \t ------ \t ------------------ \t|\t ---------- \t ----------------- \t ---------------- \t ----------- \t ----------------"
	const entryOverall string = "%s \t %s \t %.3f \t|\t %.3f \t %.3f \t %.3f \t %.3f \t %.3f\n"

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
		}

		if idx == len(ratingOutput.RatingChanges)-1 || ratingOutput.RatingChanges[idx+1].Round.Number >= currentRound+1 {
			if verbosity >= 2 {
				fmt.Printf("\n> Round %d [%s %d : %d %s]\n\n", currentRound, ctTeamNames[currentRound], change.Round.ScoreCT, change.Round.ScoreT, tTeamNames[currentRound])
				fmt.Fprintln(tabWriter, headerRound)
				fmt.Fprintln(tabWriter, borderRound)
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
						RetakeRating:      roundRetakeRatings[id],
						HurtRating:        roundHurtRatings[id],
					},
				})

				roundRating := roundRatings[id] * 100.0
				roundDamageRating := roundDamageRatings[id] * 100.0
				roundFlashAssistRating := roundFlashAssistRatings[id] * 100.0
				roundTradeDamageRating := roundTradeDamageRatings[id] * 100.0
				roundRetakeRating := roundRetakeRatings[id] * 100.0
				roundHurtRating := roundHurtRatings[id] * 100.0

				if verbosity >= 2 {
					fmt.Fprintf(tabWriter, entryRound, teamNames[teamIds[id]], name, roundRating, roundDamageRating,
						roundFlashAssistRating, roundTradeDamageRating, roundRetakeRating, roundHurtRating)
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
			avgRating := ratings[id] / float64(roundsPlayed) * 100.0

			avgDamageRating := damageRatings[id] / float64(roundsPlayed) * 100.0
			avgFlashAssistRating := flashAssistRatings[id] / float64(roundsPlayed) * 100.0
			avgTradeDamageRating := tradeDamageRatings[id] / float64(roundsPlayed) * 100.0
			avgRetakeRating := retakeRatings[id] / float64(roundsPlayed) * 100.0
			avgHurtRating := hurtRatings[id] / float64(roundsPlayed) * 100.0

			fmt.Fprintf(tabWriter, entryOverall, teamNames[teamIds[id]], name, avgRating, avgDamageRating,
				avgFlashAssistRating, avgTradeDamageRating, avgRetakeRating, avgHurtRating)
		}
		tabWriter.Flush()

		fmt.Printf("\n> Big Rounds:\n\n")
		fmt.Printf("%s got an Impact Rating of %.3f%% in round %d\n", bestRoundPlayer, bestRoundRating, bestRound)
		fmt.Printf("%s got an Impact Rating of %.3f%% in round %d\n\n", worstRoundPlayer, worstRoundRating, worstRound)
	}

	for k, v := range names {
		ratingOutput.Players = append(ratingOutput.Players, PlayerRating{
			SteamID: k,
			TeamID:  teamIds[k],
			Name:    v,
			OverallRating: OverallRating{
				AverageRating: ratings[k] / float64(roundsPlayed),
				RatingBreakdown: RatingBreakdown{
					DamageRating:      damageRatings[k] / float64(roundsPlayed),
					FlashAssistRating: flashAssistRatings[k] / float64(roundsPlayed),
					TradeDamageRating: tradeDamageRatings[k] / float64(roundsPlayed),
					RetakeRating:      retakeRatings[k] / float64(roundsPlayed),
					HurtRating:        hurtRatings[k] / float64(roundsPlayed),
				},
			},
			RoundRatings: playerRoundRatings[k],
		})
	}

	ctTeamID := ctTeamIds[len(ctTeamIds)-1]
	tTeamID := tTeamIds[len(tTeamIds)-1]

	ctFinalScore := ratingOutput.RatingChanges[len(ratingOutput.RatingChanges)-1].Round.ScoreCT
	tFinalScore := ratingOutput.RatingChanges[len(ratingOutput.RatingChanges)-1].Round.ScoreT

	// since the tag file reports only up to the final round, we need to add 1 to the score of the winning team
	if ctFinalScore > tFinalScore {
		ctFinalScore++
	} else if tFinalScore > ctFinalScore {
		tFinalScore++
	}

	ctTeamStartSide := false
	tTeamStartSide := true

	// work out who started on which side by how many rounds have been played
	if roundsPlayed > 15 {
		ctTeamStartSide = !ctTeamStartSide
		tTeamStartSide = !tTeamStartSide
		if roundsPlayed > 30 {
			// game has gone to overtime
			diff := roundsPlayed - 30
			otStage := int(math.Floor(float64(diff)/6.0) + 1)

			// final sides are opposite to the end of regulation on "odd" overtime stages
			if otStage%2 != 0 {
				ctTeamStartSide = !ctTeamStartSide
				tTeamStartSide = !tTeamStartSide
			}
		}
	}

	ratingOutput.Teams = append(ratingOutput.Teams, TeamRating{
		ID:           ctTeamID,
		Name:         teamNames[ctTeamID],
		StartingSide: bToInt(ctTeamStartSide),
		FinalScore:   ctFinalScore,
	})

	ratingOutput.Teams = append(ratingOutput.Teams, TeamRating{
		ID:           tTeamID,
		Name:         teamNames[tTeamID],
		StartingSide: bToInt(tTeamStartSide),
		FinalScore:   tFinalScore,
	})

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

func bToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
