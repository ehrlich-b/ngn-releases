package main

import (
	"fmt"
	"os"
)

func main() { fmt.Fprintln(os.Stderr, "claimbridge must be cross-built for Windows/amd64"); os.Exit(1) }
