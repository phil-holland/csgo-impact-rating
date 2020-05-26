package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/phil-holland/csgo-impact-rating/internal"
	"github.com/spf13/cobra"
)

var modelPath string

func init() {
	evaluateCmd.Flags().StringVarP(&modelPath, "model", "m", "./out/LightGBM_model.txt", "the path to the LightGBM_model.txt file to use")
	rootCmd.AddCommand(evaluateCmd)
}

var evaluateCmd = &cobra.Command{
	Use:   "evaluate [.tagged.json file]",
	Short: "Creates an output .eval.json file containing player impact ratings",
	Long:  "...",
	Run: func(cmd *cobra.Command, args []string) {
		// process the file argument
		if len(args) == 0 {
			panic("Tagged json file not supplied.")
		}
		if len(args) > 1 {
			panic("Only one json file can be supplied.")
		}
		path := args[0]
		evaluate(path)
	},
}

type Team struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func evaluate(path string) {
	// prepare a csv file
	fmt.Printf("Reading contents of json file: \"%s\"\n", path)
	jsonRaw, _ := ioutil.ReadFile(path)

	var demo internal.Demo
	err := json.Unmarshal(jsonRaw, &demo)
	if err != nil {
		panic(err.Error())
	}

	if _, err := os.Stat("./out"); os.IsNotExist(err) {
		os.Mkdir("./out", os.ModeDir)
	}

	// create a temp file to write the csv to
	file, err := ioutil.TempFile("out", "temp.*.csv")
	if err != nil {
		panic(err.Error())
	}
	defer os.Remove(file.Name())

	fmt.Printf("Writing csv to temporary file: \"%s\"\n", file.Name())
	output := internal.CSVHeader + "\n"
	for _, tick := range demo.Ticks {
		csvLine := internal.MakeCSVLine(&tick)
		output += csvLine + "\n"
	}
	file.WriteString(output)
	file.Close()

	// create a temp file to write prediction results to
	rfile, err := ioutil.TempFile("out", "temp.*.txt")
	if err != nil {
		panic(err.Error())
	}
	rfile.Close()
	defer os.Remove(rfile.Name())

	// invoke lightgbm prediction
	cmd := exec.Command("lightgbm", "task=predict", "data=\""+file.Name()+"\"",
		"header=true", "label_column=name:roundWinner", "input_model=\""+modelPath+"\"",
		"output_result=\""+rfile.Name()+"\"")
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

	var rating internal.Rating

	ratings := make(map[uint64]float64)
	names := make(map[uint64]string)
	teamIds := make(map[uint64]int)

	roundsPlayed := 0

	// loop through tagged demo ticks
	var lastPred float64
	for idx, tick := range demo.Ticks {
		// set initial ratings, and constantly update team ID map
		for _, player := range tick.Players {
			if _, ok := ratings[player.SteamID]; !ok {
				ratings[player.SteamID] = 0.0
				names[player.SteamID] = player.Name
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

		// positive if CTs benefited, negative if Ts benefited
		change := lastPred - pred

		switch tick.Type {
		case internal.TickTypeDamage:
			var flashingPlayer uint64 = 0
			var damagingPlayer uint64 = 0
			var hurtingPlayer uint64 = 0

			for _, tag := range tick.Tags {
				if tag.Action == internal.ActionFlashAssist {
					flashingPlayer = tag.Player
				} else if tag.Action == internal.ActionDamage {
					damagingPlayer = tag.Player
				} else if tag.Action == internal.ActionHurt {
					hurtingPlayer = tag.Player
				}
			}

			if flashingPlayer != 0 {
				if damagingPlayer != 0 {
					if teamIds[damagingPlayer] == tick.TeamCT.ID {
						rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
							Tick:   tick.Tick,
							Round:  tick.ScoreCT + tick.ScoreT + 1,
							Player: damagingPlayer,
							Change: change * 0.5,
							Action: internal.ActionDamage,
						})
						ratings[damagingPlayer] += change * 0.5
					} else if teamIds[damagingPlayer] == tick.TeamT.ID {
						rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
							Tick:   tick.Tick,
							Round:  tick.ScoreCT + tick.ScoreT + 1,
							Player: damagingPlayer,
							Change: -change * 0.5,
							Action: internal.ActionDamage,
						})
						ratings[damagingPlayer] -= change * 0.5
					}
				}

				if teamIds[flashingPlayer] == tick.TeamCT.ID {
					rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
						Tick:   tick.Tick,
						Round:  tick.ScoreCT + tick.ScoreT + 1,
						Player: flashingPlayer,
						Change: change * 0.5,
						Action: internal.ActionFlashAssist,
					})
					ratings[flashingPlayer] += change * 0.5
				} else if teamIds[flashingPlayer] == tick.TeamT.ID {
					rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
						Tick:   tick.Tick,
						Round:  tick.ScoreCT + tick.ScoreT + 1,
						Player: flashingPlayer,
						Change: -change * 0.5,
						Action: internal.ActionFlashAssist,
					})
					ratings[flashingPlayer] -= change * 0.5
				}
			} else {
				if damagingPlayer != 0 {
					if teamIds[damagingPlayer] == tick.TeamCT.ID {
						rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
							Tick:   tick.Tick,
							Round:  tick.ScoreCT + tick.ScoreT + 1,
							Player: damagingPlayer,
							Change: change,
							Action: internal.ActionDamage,
						})
						ratings[damagingPlayer] += change
					} else if teamIds[damagingPlayer] == tick.TeamT.ID {
						rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
							Tick:   tick.Tick,
							Round:  tick.ScoreCT + tick.ScoreT + 1,
							Player: damagingPlayer,
							Change: -change,
							Action: internal.ActionDamage,
						})
						ratings[damagingPlayer] -= change
					}
				}
			}

			if hurtingPlayer != 0 {
				if teamIds[hurtingPlayer] == tick.TeamCT.ID {
					rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
						Tick:   tick.Tick,
						Round:  tick.ScoreCT + tick.ScoreT + 1,
						Player: hurtingPlayer,
						Change: change,
						Action: internal.ActionHurt,
					})
					ratings[hurtingPlayer] += change
				} else if teamIds[hurtingPlayer] == tick.TeamT.ID {
					rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
						Tick:   tick.Tick,
						Round:  tick.ScoreCT + tick.ScoreT + 1,
						Player: hurtingPlayer,
						Change: -change,
						Action: internal.ActionHurt,
					})
					ratings[hurtingPlayer] -= change
				}
			}
		case internal.TickTypeBombDefuse:
			var defusingPlayer uint64 = 0

			for _, tag := range tick.Tags {
				if tag.Action == internal.ActionDefuse {
					defusingPlayer = tag.Player
				}
			}

			if defusingPlayer != 0 {
				if teamIds[defusingPlayer] == tick.TeamCT.ID {
					rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
						Tick:   tick.Tick,
						Round:  tick.ScoreCT + tick.ScoreT + 1,
						Player: defusingPlayer,
						Change: change,
						Action: internal.ActionDefuse,
					})
					ratings[defusingPlayer] += change
				} else if teamIds[defusingPlayer] == tick.TeamT.ID {
					rating.RatingChanges = append(rating.RatingChanges, internal.RatingChange{
						Tick:   tick.Tick,
						Round:  tick.ScoreCT + tick.ScoreT + 1,
						Player: defusingPlayer,
						Change: -change,
						Action: internal.ActionDefuse,
					})
					ratings[defusingPlayer] -= change
				}
			}
		}

		lastPred = pred
	}

	for k, v := range names {
		rating.Players = append(rating.Players, internal.RatingPlayer{
			SteamID:     k,
			Name:        v,
			TotalRating: ratings[k],
		})
	}

	// print out reports
	currentRound := 1
	roundRatings := make(map[uint64]float64)
	for k := range names {
		roundRatings[k] = 0.0
	}
	for _, change := range rating.RatingChanges {
		if change.Round > currentRound {
			fmt.Printf("\n======================== Round %d ========================\n", currentRound)
			for k, v := range names {
				fmt.Printf("> Player %s got an impact rating of: %.5f\n", v, 100.0*roundRatings[k])
				roundRatings[k] = 0.0
			}
			currentRound = change.Round
		}
		roundRatings[change.Player] += change.Change
	}

	fmt.Printf("\n======================== Overall ========================\n")
	for k, v := range names {
		fmt.Printf("> Player %s got an average impact rating of: %.5f\n", v, 100.0*ratings[k]/float64(roundsPlayed))
	}

	rating.RoundsPlayed = roundsPlayed

	outputPath := strings.Replace(path, ".tagged.json", ".rating.json", -1)
	fmt.Printf("Writing output JSON to: \"%s\"\n", outputPath)
	outputMarshalled, err := json.MarshalIndent(rating, "", "  ")
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
