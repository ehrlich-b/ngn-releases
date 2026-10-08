package claim

import (
	"os/exec"
	"strings"
)

// Set once by the command before any sessions start. It is part of the receipt.
var WindowsBridge Artifact

func sessionLaunch(a Artifact) []string {
	if strings.HasSuffix(strings.ToLower(a.Path), ".exe") {
		return []string{WindowsBridge.Path, a.Path}
	}
	return []string{a.Path}
}

func sessionCommand(a Artifact) *exec.Cmd {
	argv := sessionLaunch(a)
	return exec.CommandContext(ProcessContext, argv[0], argv[1:]...)
}

func schedulingFor(a Artifact) string {
	if strings.HasSuffix(strings.ToLower(a.Path), ".exe") {
		return "Windows job affinity 0xffff (0-15), Idle priority, kill on bridge close"
	}
	return "inherited taskset 0-15, nice 19; GOMAXPROCS=1"
}
