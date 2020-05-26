package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/phil-holland/csgo-impact-rating/internal"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(prepareCmd)
}

var prepareCmd = &cobra.Command{
	Use:   "prepare [.tagged.json file / dir of .tagged.json files]",
	Short: "Creates a csv file for use with LightGBM from all tagged json files provided",
	Long:  "...",
	Run: func(cmd *cobra.Command, args []string) {
		// process the file argument
		if len(args) == 0 {
			panic("Tagged json file not supplied.")
		}
		if len(args) > 1 {
			panic("Only one json file/directory can be supplied.")
		}
		path := args[0]
		prepare(path)
	},
}

func prepare(path string) {
	var files []string

	fi, err := os.Stat(path)
	if err != nil {
		panic(err)
	}
	switch mode := fi.Mode(); {
	case mode.IsDir():
		fmt.Printf("Directory path given: \"%s\"\n", path)
		f, err := filepath.Glob(path + "/*.tagged.json")
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, f...)
	case mode.IsRegular():
		fmt.Printf("Single file given: \"%s\"\n", path)
		files = append(files, path)
	}

	fmt.Printf("Processing %d tagged json file(s)\n", len(files))

	output := internal.CSVHeader + "\n"

	for _, file := range files {
		fmt.Printf("Processing file: \"%s\"\n", file)
		jsonRaw, _ := ioutil.ReadFile(file)

		var demo internal.Demo
		err := json.Unmarshal(jsonRaw, &demo)
		if err != nil {
			panic(err)
		}
		for _, tick := range demo.Ticks {
			csvLine := internal.MakeCSVLine(&tick)
			output += csvLine + "\n"
		}
	}

	if _, err := os.Stat("./out"); os.IsNotExist(err) {
		os.Mkdir("./out", os.ModeDir)
	}

	outPath := "./out/" + time.Now().Format("2006-01-02--15-04-05") + ".csv"
	fmt.Printf("Writing output csv to \"%s\"\n", outPath)

	file, err := os.Create(outPath)
	if err != nil {
		return
	}
	defer file.Close()

	file.WriteString(output)
}
