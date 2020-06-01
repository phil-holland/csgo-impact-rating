package main

import (
	"fmt"
	"os"

	"github.com/phil-holland/csgo-impact-rating/internal"
	flag "github.com/spf13/pflag"
)

func main() {
	// tagging flags
	force := flag.BoolP("force", "f", false, "Force the input demo file to be tagged, even if a .tagged.json file already exists.")

	// evaluation flags
	evalSkip := flag.BoolP("eval-skip", "s", false, "Skip the evaluation process, only tag the input demo file.")
	evalModelPath := flag.StringP("eval-model", "m", "./LightGBM_model.txt", "The path to the LightGBM_model.txt file to use for evaluation.")
	evalQuiet := flag.BoolP("eval-quiet", "q", false, "Omit CLI output of evaluation summary.")
	flag.CommandLine.SortFlags = false
	flag.Parse()

	// process the file argument
	if len(flag.Args()) == 0 {
		panic("demo file not supplied.")
	}
	if len(flag.Args()) > 1 {
		panic("Only one demo file can be supplied.")
	}
	demoPath := flag.Args()[0]

	// check that the file exists
	_, err := os.Stat(demoPath)
	if os.IsNotExist(err) {
		panic(fmt.Sprintf("ERROR: '%s' is not a file.\n", demoPath))
	}

	// check if a .tagged.json file exists
	hasTaggedFile := false
	_, err = os.Stat(demoPath + ".tagged.json")
	if !os.IsNotExist(err) {
		hasTaggedFile = true
	}

	taggedFilePath := ""
	if !(*force) && hasTaggedFile {
		// if a .tagged.json file already exists, skip the tagging process
		taggedFilePath = demoPath + ".tagged.json"

		fmt.Printf("Skipping tagging process, tag file already exists at: \"%s\"\n", taggedFilePath)
	} else {
		// start parsing the demo file
		taggedFilePath = internal.TagDemo(demoPath)
		fmt.Printf("Tag file written to: \"%s\"\n", taggedFilePath)
	}

	if !(*evalSkip) {
		// start evaluating the tag file
		internal.EvaluateDemo(taggedFilePath, *evalQuiet, *evalModelPath)
	}
}
