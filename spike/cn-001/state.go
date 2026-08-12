package smoke

import (
	"errors"
	"fmt"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// This file holds the overlay §8.1 needs once step 1 of the state probe has
// established that a literal Consign state cannot ride status.state: a coarse
// A2A state that A2A itself understands, plus the refined Consign state carried
// in the verification namespace's metadata.
//
// Only the terminal half of §8.1 is modelled. The non-terminal states already
// have carriers: CREATED/SUBMITTED are the mandatory first a2a.Task, ACCEPTED
// and RUNNING are TASK_STATE_WORKING plus a §9.1 envelope, and §8.1 already
// calls VERIFYING consumer-side, so it never appears on the wire at all.

// ConsignState is a §8.1 terminal task state.
type ConsignState string

const (
	StateCompleted ConsignState = "COMPLETED"
	StatePartial   ConsignState = "PARTIAL"
	StateFailed    ConsignState = "FAILED"
	StateCancelled ConsignState = "CANCELLED"
	StateUnknown   ConsignState = "UNKNOWN"
)

var (
	// ErrNotTerminalState is returned for a value that is not one of §8.1's five
	// terminal states.
	ErrNotTerminalState = errors.New("not a §8.1 terminal state")
	// ErrOverlayMissing is what a consumer sees when a peer terminates a task
	// without saying which Consign state it terminated in. It is a protocol
	// violation, not a default: absent the overlay there is no safe reading of
	// TASK_STATE_FAILED, because §8.6's FAILED asserts side_effect_state: none
	// while UNKNOWN asserts the opposite is unestablished.
	ErrOverlayMissing = errors.New("terminal task carries no Consign state overlay")
	// ErrOverlayMismatch is returned when the overlay names a Consign state that
	// does not map onto the coarse A2A state it arrived on.
	ErrOverlayMismatch = errors.New("Consign state overlay contradicts the A2A task state")
)

// TerminalOverlay is the payload a producer attaches under ExtVerification to
// the A2A status update that terminates a task.
type TerminalOverlay struct {
	ConsignState ConsignState `json:"consign_state"`
	Terminal     bool         `json:"terminal"`
}

// CoarseState maps a §8.1 terminal state onto the A2A v1.0 TaskState that
// carries it.
//
// PARTIAL and UNKNOWN map onto TASK_STATE_FAILED rather than TASK_STATE_COMPLETED
// because the mapping has to be safe under the reading a peer gets when it
// ignores the overlay entirely: FAILED understates a partial success, whereas
// COMPLETED would report an unmet acceptance criterion (§8.6) as a success. A2A's
// own FAILED means only "execution failed"; it makes no side-effect claim, so it
// does not contradict UNKNOWN the way Consign's own FAILED would.
func CoarseState(s ConsignState) (a2a.TaskState, error) {
	switch s {
	case StateCompleted:
		return a2a.TaskStateCompleted, nil
	case StateCancelled:
		return a2a.TaskStateCanceled, nil
	case StatePartial, StateFailed, StateUnknown:
		return a2a.TaskStateFailed, nil
	}
	return a2a.TaskStateUnspecified, fmt.Errorf("%w: %q", ErrNotTerminalState, s)
}

// TerminalStateOf is the consumer-side rule the overlay forces on every Consign
// node: read the §8.1 state off the overlay, never off status.state. Reading
// status.state directly cannot distinguish PARTIAL, UNKNOWN and FAILED, since
// CoarseState collapses all three onto TASK_STATE_FAILED.
//
// The cross-check against CoarseState is what makes the coarse state and the
// overlay a single fact rather than two independent ones a producer could
// contradict itself with.
func TerminalStateOf(task *a2a.Task) (ConsignState, error) {
	if !task.Status.State.Terminal() {
		return "", fmt.Errorf("task %s is in %q, which is not terminal", task.ID, task.Status.State)
	}
	payload, ok := task.Metadata[ExtVerification]
	if !ok {
		return "", fmt.Errorf("%w: task %s in %q", ErrOverlayMissing, task.ID, task.Status.State)
	}
	var overlay TerminalOverlay
	if err := FromMetadata(payload, &overlay); err != nil {
		return "", err
	}
	if !overlay.Terminal {
		return "", fmt.Errorf("%w: task %s carries a non-terminal overlay on a terminal A2A state",
			ErrOverlayMismatch, task.ID)
	}
	coarse, err := CoarseState(overlay.ConsignState)
	if err != nil {
		return "", err
	}
	if coarse != task.Status.State {
		return "", fmt.Errorf("%w: task %s says %q, which rides %q, but arrived on %q",
			ErrOverlayMismatch, task.ID, overlay.ConsignState, coarse, task.Status.State)
	}
	return overlay.ConsignState, nil
}
