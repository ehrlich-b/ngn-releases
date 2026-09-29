package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ehrlich-b/ngn/training/nnue/k4finalize"
)

func run(args []string) error {
	if len(args) == 0 || (args[0] != "pilot" && args[0] != "main" && args[0] != "probe5m") {
		return errors.New("usage: ngnk4finalize pilot|probe5m|main -sampler MANIFEST -labels DIR -packed DIR -output NEW_DIR [-pilot PILOT_MANIFEST]")
	}
	mode := args[0]
	flags := flag.NewFlagSet("ngnk4finalize "+mode, flag.ContinueOnError)
	sampler := flags.String("sampler", "", "completed ngnk4sample manifest")
	labels := flags.String("labels", "", "directory containing SHARD.labels.jsonl")
	packed := flags.String("packed", "", "directory containing SHARD.bf and SHARD.pack.json")
	output := flags.String("output", "", "new finalized output directory")
	pilot := flags.String("pilot", "", "completed pilot finalizer manifest (probe5m/main only)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *sampler == "" || *labels == "" || *packed == "" || *output == "" ||
		(mode != "pilot" && *pilot == "") || (mode == "pilot" && *pilot != "") {
		return errors.New("required: -sampler -labels -packed -output; probe5m/main also require -pilot")
	}
	command := append([]string{"ngnk4finalize"}, args...)
	var manifest k4finalize.Manifest
	var err error
	if mode == "pilot" {
		manifest, err = k4finalize.FinalizePilot(*sampler, *labels, *packed, *output, command)
	} else if mode == "main" {
		manifest, err = k4finalize.FinalizeMain(*sampler, *pilot, *labels, *packed, *output, command)
	} else {
		manifest, err = k4finalize.FinalizeProbe5M(*sampler, *pilot, *labels, *packed, *output, command)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(manifest)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ngnk4finalize:", err)
		os.Exit(1)
	}
}
