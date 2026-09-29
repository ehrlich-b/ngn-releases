package engine

import "testing"

func TestHistoryPolicyIsAcceptedPolicy11(t *testing.T) {
	if got := HistoryPolicyName(); got != "11" {
		t.Fatalf("policy=%s want=11", got)
	}
}
