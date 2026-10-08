package main

import (
	"github.com/ehrlich-b/ngn/internal/claim"
	"os/exec"
	"strings"
)

func sourceCommit(root string) (string, error) {
	b, e := exec.CommandContext(claim.ProcessContext, "git", "-C", root, "rev-parse", "HEAD").Output()
	return strings.TrimSpace(string(b)), e
}
