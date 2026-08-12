package smoke

import (
	"errors"
	"fmt"
	"time"
)

// This file holds §7.5 budget subdivision: the producer-local ledger a holder
// keeps so that budgets are subdivided rather than shared.

// ErrBudgetExceeded is §15's E_BUDGET_EXHAUSTED seen from the delegating side:
// the child asked for more than the holder still has.
var ErrBudgetExceeded = errors.New("child budget exceeds the holder's remaining allowance")

// BudgetReport is what the `budget` namespace carries on the wire: the
// allowance a holder was given, what it has left, and what it handed on.
// §7.5 makes this producer-local and unverifiable, so it is a disclosure rather
// than a claim the consumer can check -- which is exactly why the deadline, the
// one externally observable bound, is also in it.
type BudgetReport struct {
	Allowance Budget   `json:"allowance"`
	Remaining Budget   `json:"remaining"`
	Delegated []Budget `json:"delegated,omitempty"`
}

// Ledger tracks what a holder has left of its budget caveats after delegating.
// §7.5 makes the accounting producer-local and unverifiable by the consumer,
// so this is the holder's own book, not a shared one.
type Ledger struct {
	remaining Budget
}

// NewLedger opens a ledger holding the whole of allowance.
func NewLedger(allowance Budget) *Ledger {
	return &Ledger{remaining: allowance}
}

// Remaining reports what is left to spend or delegate.
func (l *Ledger) Remaining() Budget {
	return l.remaining
}

// Subdivide deducts child from the remaining allowance, as §7.5 requires be
// done at delegation time rather than on completion. It refuses -- and deducts
// nothing -- when the child would take the holder past what it still has, which
// is what stops a node issuing children whose combined budgets exceed its own.
//
// The deadline is not deducted. §7.5 makes it the one externally observable,
// enforceable bound, so a child may hold any deadline at or before the
// holder's and it stays available to later children.
func (l *Ledger) Subdivide(child Budget) error {
	if err := notLaterDeadline(child.Deadline, l.remaining.Deadline); err != nil {
		return err
	}
	if child.MaxModelCalls > l.remaining.MaxModelCalls {
		return fmt.Errorf("%w: max_model_calls %d exceeds the remaining %d",
			ErrBudgetExceeded, child.MaxModelCalls, l.remaining.MaxModelCalls)
	}
	if child.MaxToolCalls > l.remaining.MaxToolCalls {
		return fmt.Errorf("%w: max_tool_calls %d exceeds the remaining %d",
			ErrBudgetExceeded, child.MaxToolCalls, l.remaining.MaxToolCalls)
	}
	if child.MaxArtifactBytes > l.remaining.MaxArtifactBytes {
		return fmt.Errorf("%w: max_artifact_bytes %d exceeds the remaining %d",
			ErrBudgetExceeded, child.MaxArtifactBytes, l.remaining.MaxArtifactBytes)
	}

	l.remaining.MaxModelCalls -= child.MaxModelCalls
	l.remaining.MaxToolCalls -= child.MaxToolCalls
	l.remaining.MaxArtifactBytes -= child.MaxArtifactBytes
	return nil
}

func notLaterDeadline(child, holder string) error {
	c, err := time.Parse(time.RFC3339, child)
	if err != nil {
		return fmt.Errorf("parsing child budget.deadline %q: %w", child, err)
	}
	h, err := time.Parse(time.RFC3339, holder)
	if err != nil {
		return fmt.Errorf("parsing holder budget.deadline %q: %w", holder, err)
	}
	if c.After(h) {
		return fmt.Errorf("%w: deadline %s is later than the holder's %s", ErrBudgetExceeded, child, holder)
	}
	return nil
}
