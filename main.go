package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/phil-holland/csgo-impact-rating/internal"
	flag "github.com/spf13/pflag"
)

const version string = "0.4"

func usage() {
	fmt.Printf("Usage: csgo-impact-rating [OPTION]... [DEMO_FILE (.dem)]\n\n")
	fmt.Printf("Tags DEMO_FILE, creating a '.tagged.json' file in the same directory, which is\n")
	fmt.Printf("subsequently evaluated, producing an Impact Rating report which is written to\n")
	fmt.Printf("the console and a '.rating.json' file.\n")

	fmt.Printf("\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func main() {
	// tagging flags
	force := flag.BoolP("force", "f", false, "Force the input demo file to be tagged, even if a\n.tagged.json file already exists.")

	// evaluation flags
	evalSkip := flag.BoolP("eval-skip", "s", false, "Skip the evaluation process, only tag the input\ndemo file.")
	evalModelPath := flag.StringP("eval-model", "m", "", "The path to the LightGBM_model.txt file to use for\nevaluation. If omitted, the application looks for\na file named \"LightGBM_model.txt\" in the same\ndirectory as the executable.")
	evalVerbosity := flag.IntP("eval-verbosity", "v", 2, "Evaluation console verbosity level:\n 0 = do not print a report\n 1 = print only overall rating\n 2 = print overall & per-round ratings")
	flag.CommandLine.SortFlags = false
	flag.ErrHelp = fmt.Errorf("version: %s", version)
	flag.Usage = usage
	flag.Parse()

	if *evalModelPath == "" {
		// get parent directory of executable
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exPath := filepath.Dir(ex)
		*evalModelPath = filepath.Join(exPath, "LightGBM_model.txt")
	}

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
		// check that the model file exists
		_, err = os.Stat(*evalModelPath)
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("ERROR: LightGBM model not loaded - '%s' does not exist.\n", *evalModelPath))
		}

		// start evaluating the tag file
		internal.EvaluateDemo(taggedFilePath, *evalVerbosity, *evalModelPath)
	}
}
