package smoke

import (
	"errors"
	"fmt"
	"time"
)

// This file holds §9's event envelope and the per-task sequence numbering it
// requires. A2A v1.0 has no envelope of its own -- its streaming events carry
// no type discriminator beyond their Go type and no sequence number anywhere --
// so a Consign event travels as an envelope inside the A2A event's metadata,
// keyed by the extension namespace whose payload it carries.

// The §9.3 event types this spike emits. The full table is larger; the ones
// here are the ones a submit-to-terminal round trip with one delegation hop
// produces.
const (
	EventAccepted  = "task.accepted"
	EventDelegated = "task.delegated"
	EventArtifact  = "task.artifact"
	EventCompleted = "task.completed"
)

// ErrSeqGap is what §9.2 requires a consumer to detect: a gap in seq means the
// stream is broken and must be re-subscribed rather than proceeded with.
var ErrSeqGap = errors.New("event sequence is not monotonic from 1 without gaps")

// EventEnvelope is the §9.1 envelope every Consign event carries.
type EventEnvelope struct {
	TaskID    string         `json:"task_id"`
	Seq       int            `json:"seq"`
	EmittedAt string         `json:"emitted_at"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
}

// Stream stamps a producer's events with §9.1's monotonic per-task sequence.
type Stream struct {
	taskID string
	seq    int
}

// NewStream opens the event stream for one task.
func NewStream(taskID string) *Stream {
	return &Stream{taskID: taskID}
}

// Next returns the next envelope in the stream. seq starts at 1, which is what
// §8.4 requires of task.accepted.
func (s *Stream) Next(typ string, payload map[string]any) EventEnvelope {
	s.seq++
	return EventEnvelope{
		TaskID:    s.taskID,
		Seq:       s.seq,
		EmittedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      typ,
		Payload:   payload,
	}
}

// CheckSeq applies §9.1 and §9.2 on the consumer's side: sequence numbers run
// from 1 with no gaps, in order, for a single task.
func CheckSeq(events []EventEnvelope) error {
	if len(events) == 0 {
		return fmt.Errorf("%w: no events observed", ErrSeqGap)
	}
	for i, e := range events {
		if e.Seq != i+1 {
			return fmt.Errorf("%w: event %d of the stream has seq %d", ErrSeqGap, i+1, e.Seq)
		}
		if e.TaskID != events[0].TaskID {
			return fmt.Errorf("%w: event seq %d belongs to task %s, not %s", ErrSeqGap, e.Seq, e.TaskID, events[0].TaskID)
		}
	}
	return nil
}
