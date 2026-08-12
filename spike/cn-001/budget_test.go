package smoke

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestBudgetSubdivision is the offline half of demonstration 3: §7.5's rule
// that budgets are subdivided rather than shared. A holder deducts a child's
// budget from its own remaining allowance at delegation time and must not issue
// children whose combined budgets exceed that allowance.
func TestBudgetSubdivision(t *testing.T) {
	now := time.Now()
	rfc := func(d time.Duration) string { return now.Add(d).UTC().Format(time.RFC3339) }

	allowance := Budget{
		Deadline:         rfc(20 * time.Minute),
		MaxModelCalls:    10,
		MaxToolCalls:     20,
		MaxArtifactBytes: 1 << 20,
	}

	t.Run("a fresh ledger holds its whole allowance", func(t *testing.T) {
		l := NewLedger(allowance)
		if got := l.Remaining(); got != allowance {
			t.Fatalf("Remaining() on a fresh ledger = %+v, want %+v", got, allowance)
		}
	})

	t.Run("delegating deducts at delegation time", func(t *testing.T) {
		l := NewLedger(allowance)
		child := Budget{Deadline: rfc(15 * time.Minute), MaxModelCalls: 4, MaxToolCalls: 6, MaxArtifactBytes: 4096}
		if err := l.Subdivide(child); err != nil {
			t.Fatalf("Subdivide(%+v) = %v, want nil", child, err)
		}
		want := Budget{Deadline: allowance.Deadline, MaxModelCalls: 6, MaxToolCalls: 14, MaxArtifactBytes: 1<<20 - 4096}
		if got := l.Remaining(); got != want {
			t.Fatalf("Remaining() after one child = %+v, want %+v", got, want)
		}
		// The deadline is wall-clock and externally observable (§7.5), so it is
		// bounded by the parent's rather than consumed by the child.
		if l.Remaining().Deadline != allowance.Deadline {
			t.Fatalf("Remaining().Deadline = %q, want the allowance's %q -- deadlines are not spent",
				l.Remaining().Deadline, allowance.Deadline)
		}
	})

	t.Run("combined children may not exceed the allowance", func(t *testing.T) {
		l := NewLedger(allowance)
		first := Budget{Deadline: rfc(15 * time.Minute), MaxModelCalls: 4, MaxToolCalls: 6, MaxArtifactBytes: 4096}
		if err := l.Subdivide(first); err != nil {
			t.Fatalf("Subdivide(first) = %v, want nil", err)
		}

		// 4 + 7 = 11 > 10, though 7 on its own is well inside the allowance --
		// which is the whole point: the bound is the *remaining* value, not the
		// original one.
		second := Budget{Deadline: rfc(15 * time.Minute), MaxModelCalls: 7, MaxToolCalls: 6, MaxArtifactBytes: 4096}
		err := l.Subdivide(second)
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("Subdivide(second) = %v, want %v", err, ErrBudgetExceeded)
		}
		if !strings.Contains(err.Error(), "max_model_calls") {
			t.Fatalf("rejection does not name the exhausted unit: %v", err)
		}
		t.Logf("rejected: %v", err)

		if got := l.Remaining().MaxModelCalls; got != 6 {
			t.Fatalf("Remaining().MaxModelCalls after a refused child = %d, want 6 -- a refused subdivision must not deduct", got)
		}
	})

	// Discriminating case: a ledger that refused everything after the first
	// child, or that compared against the original allowance, would be caught
	// here. 4 + 6 exactly exhausts the allowance and must be permitted.
	t.Run("a child that exactly exhausts the remainder is permitted", func(t *testing.T) {
		l := NewLedger(allowance)
		if err := l.Subdivide(Budget{Deadline: rfc(15 * time.Minute), MaxModelCalls: 4, MaxToolCalls: 6, MaxArtifactBytes: 4096}); err != nil {
			t.Fatalf("Subdivide(first) = %v, want nil", err)
		}
		if err := l.Subdivide(Budget{Deadline: rfc(15 * time.Minute), MaxModelCalls: 6, MaxToolCalls: 14, MaxArtifactBytes: 1<<20 - 4096}); err != nil {
			t.Fatalf("Subdivide(second) exactly exhausting the remainder = %v, want nil", err)
		}
		want := Budget{Deadline: allowance.Deadline}
		if got := l.Remaining(); got != want {
			t.Fatalf("Remaining() after exhausting = %+v, want %+v", got, want)
		}
	})

	t.Run("a child deadline beyond the allowance is refused", func(t *testing.T) {
		l := NewLedger(allowance)
		err := l.Subdivide(Budget{Deadline: rfc(25 * time.Minute), MaxModelCalls: 1, MaxToolCalls: 1, MaxArtifactBytes: 1})
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("Subdivide() with a later deadline = %v, want %v", err, ErrBudgetExceeded)
		}
		if !strings.Contains(err.Error(), "deadline") {
			t.Fatalf("rejection does not name the deadline: %v", err)
		}
		t.Logf("rejected: %v", err)
	})
}
