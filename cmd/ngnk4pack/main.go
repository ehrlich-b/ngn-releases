package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ehrlich-b/ngn/training/nnue/k4pack"
)

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ngnk4pack", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "completed ngnk4label JSONL shard")
	output := flags.String("output", "", "new Bullet-format shard")
	receiptPath := flags.String("receipt", "", "new conversion receipt")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || *output == "" || *receiptPath == "" {
		return errors.New("required: -input -output -receipt")
	}
	command := append([]string{"ngnk4pack"}, args...)
	receipt, err := k4pack.Pack(*input, *output, *receiptPath, command)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(receipt)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ngnk4pack:", err)
		os.Exit(1)
	}
}
