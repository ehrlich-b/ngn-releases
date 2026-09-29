package engine

import (
	"testing"
	"time"
)

func TestTimeManagerRateLimiting(t *testing.T) {
	tm := NewTimeManager()

	// Set a very short time limit
	params := SearchParams{
		MoveTime: 100, // 100ms
	}
	tm.SetTimeControl(params, true)

	// Call ShouldStopSearch many times quickly
	callCount := 0
	start := time.Now()

	// First 1023 calls should be very fast (no actual time checking)
	for i := 0; i < 1023; i++ {
		tm.ShouldStopSearch(1)
		callCount++
	}

	fastElapsed := time.Since(start)

	// Should have taken very little time (< 10ms for 1023 calls)
	if fastElapsed > 10*time.Millisecond {
		t.Errorf("Expected fast calls to take < 10ms, took %v", fastElapsed)
	}

	// Wait for time limit to expire
	time.Sleep(150 * time.Millisecond)

	// The 1024th call should trigger actual time check and return true
	shouldStop := tm.ShouldStopSearch(1)
	if !shouldStop {
		t.Error("Should stop after time limit exceeded on 1024th call")
	}

	// Subsequent calls should return true immediately (cached result)
	shouldStopCached := tm.ShouldStopSearch(1)
	if !shouldStopCached {
		t.Error("Should stop on cached result")
	}

	t.Logf("✓ Time management rate limiting working: %d fast calls in %v", callCount, fastElapsed)
}

func TestTimeManagerAtomicCounter(t *testing.T) {
	tm := NewTimeManager()

	params := SearchParams{
		MoveTime: 1000, // 1 second
	}
	tm.SetTimeControl(params, true)

	// Verify counter starts at 0
	initialCount := tm.checkCounter
	if initialCount != 0 {
		t.Errorf("Expected initial counter to be 0, got %d", initialCount)
	}

	// Call ShouldStopSearch a few times
	for i := 0; i < 5; i++ {
		tm.ShouldStopSearch(1)
	}

	// Counter should have incremented
	finalCount := tm.checkCounter
	if finalCount != 5 {
		t.Errorf("Expected counter to be 5, got %d", finalCount)
	}

	t.Log("✓ Atomic counter working correctly")
}

func TestTimeManagerReset(t *testing.T) {
	tm := NewTimeManager()

	// Make some calls to increment counter
	params := SearchParams{MoveTime: 1000}
	tm.SetTimeControl(params, true)

	for i := 0; i < 100; i++ {
		tm.ShouldStopSearch(1)
	}

	// Counter should be non-zero
	if tm.checkCounter == 0 {
		t.Error("Expected counter to be non-zero after calls")
	}

	// Reset with new time control
	newParams := SearchParams{MoveTime: 2000}
	tm.SetTimeControl(newParams, false)

	// Counter should be reset to 0
	if tm.checkCounter != 0 {
		t.Errorf("Expected counter to be reset to 0, got %d", tm.checkCounter)
	}

	// shouldStop should be reset
	if tm.shouldStop {
		t.Error("Expected shouldStop to be reset to false")
	}

	t.Log("✓ Time manager reset working")
}
