package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ehrlich-b/ngn/training/nnue/k4label"
)

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ngnk4label", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "immutable sampler JSONL shard")
	output := flags.String("output", "", "new labeled JSONL shard")
	receiptPath := flags.String("receipt", "", "new run receipt JSON")
	teacher := flags.String("teacher", "", "pinned Stockfish 18 executable")
	sourceCommit := flags.String("teacher-source-commit", "", "pinned Stockfish source commit")
	bigNet := flags.String("teacher-big-net-sha256", "", "Stockfish BIG network SHA-256")
	smallNet := flags.String("teacher-small-net-sha256", "", "Stockfish SMALL network SHA-256")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || *output == "" || *receiptPath == "" ||
		*teacher == "" || *sourceCommit == "" || *bigNet == "" || *smallNet == "" {
		return errors.New("required: -input -output -receipt -teacher -teacher-source-commit -teacher-big-net-sha256 -teacher-small-net-sha256")
	}
	handshakeContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	session, err := k4label.StartSession(handshakeContext, []string{*teacher}, *sourceCommit, *bigNet, *smallNet)
	cancel()
	if err != nil {
		return err
	}
	receipt, runErr := k4label.RunShard(context.Background(), *input, *output, session)
	closeErr := session.Close()
	if runErr != nil {
		return runErr
	}
	if closeErr != nil {
		return fmt.Errorf("close teacher: %w", closeErr)
	}
	receipt.Command = append([]string{"ngnk4label"}, args...)
	if err := k4label.WriteReceiptNew(*receiptPath, receipt); err != nil {
		return fmt.Errorf("write receipt: %w", err)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(receipt)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ngnk4label:", err)
		os.Exit(1)
	}
}
