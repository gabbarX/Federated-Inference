package smoke

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// consignStatePartialLiteral is §8.1's PARTIAL written the way an A2A TaskState
// is written: the shape a state-machine *extension* would take if A2A v1.0
// permitted one.
const consignStatePartialLiteral = a2a.TaskState("CONSIGN_STATE_PARTIAL")

// hangBudget is how long the literal-state stream is given to produce a
// terminal event. Two seconds is far beyond the sub-millisecond loopback round
// trip every other test in this package completes in; the coarse-state case in
// the same table finishes well inside it, which is what makes the deadline
// evidence of a hang rather than of a slow machine.
const hangBudget = 2 * time.Second

// TestLiteralConsignStateIsCarriedButNeverTerminates is step 1 of the probe. It
// asserts the OBSERVED behaviour of a literal §8.1 state value; the
// hoped-for behaviour -- that CONSIGN_STATE_PARTIAL is simply a terminal state
// A2A carries -- is false at two separate layers, and the report records the
// delta.
//
// Layer by layer:
//
//   - a2a.TaskState.UnmarshalJSON accepts the literal verbatim. It validates
//     nothing (a2a/core.go: "*ts = TaskState(s)"), so the value survives every
//     decode, including a whole a2a.Task off the wire.
//   - a2a.TaskState.Terminal() is a closed disjunction over four literals, so it
//     is false for the Consign value, and no extension point exists to widen it.
//   - That single false is what breaks the transport: internal/taskupdate.IsFinal
//     is Terminal() || INPUT_REQUIRED, so a2a-go never recognises the event as
//     ending the execution. The agent's iterator is exhausted, the event pipe's
//     writer is closed but its channel is not (internal/eventpipe/local.go:102),
//     so executionHandler.processEvents blocks in pipeReader.Read forever, the
//     execution promise never resolves, the subscription queue is never closed
//     and the SSE response is never terminated.
func TestLiteralConsignStateIsCarriedButNeverTerminates(t *testing.T) {
	t.Run("every decoder accepts the literal, and Terminal() still rejects it", func(t *testing.T) {
		var ts a2a.TaskState
		if err := json.Unmarshal([]byte(`"CONSIGN_STATE_PARTIAL"`), &ts); err != nil {
			t.Fatalf("json.Unmarshal(%q) into a2a.TaskState = %v, want nil", "CONSIGN_STATE_PARTIAL", err)
		}
		if ts != consignStatePartialLiteral {
			t.Fatalf("decoded TaskState = %q, want the literal %q back unaltered", ts, consignStatePartialLiteral)
		}
		if ts.Terminal() {
			t.Fatalf("a2a.TaskState(%q).Terminal() = true; a2a/core.go compares against four literals "+
				"and %q is not one of them", ts, ts)
		}

		// Discriminating case: the unmarshaller is not a blind pass-through. It
		// has exactly one rewriting rule, and it is not the one Consign needs.
		// Without this case, "accepted verbatim" would be unfalsifiable here.
		var unspecified a2a.TaskState
		if err := json.Unmarshal([]byte(`"TASK_STATE_UNSPECIFIED"`), &unspecified); err != nil {
			t.Fatalf("json.Unmarshal(TASK_STATE_UNSPECIFIED) = %v, want nil", err)
		}
		if unspecified != a2a.TaskStateUnspecified {
			t.Fatalf("decoded TASK_STATE_UNSPECIFIED = %q, want a2a.TaskStateUnspecified (the empty "+
				"TaskState): UnmarshalJSON rewrites this one value and no other", unspecified)
		}

		// Re-encoding is symmetric, so nothing downstream rewrites the value either.
		encoded, err := json.Marshal(consignStatePartialLiteral)
		if err != nil {
			t.Fatalf("json.Marshal(%q) = %v", consignStatePartialLiteral, err)
		}
		if string(encoded) != `"CONSIGN_STATE_PARTIAL"` {
			t.Fatalf("json.Marshal(%q) = %s, want %q", consignStatePartialLiteral, encoded, `"CONSIGN_STATE_PARTIAL"`)
		}

		// The whole object model is equally permissive: a Task carrying the
		// literal decodes without complaint.
		var task a2a.Task
		if err := json.Unmarshal([]byte(`{"id":"t1","contextId":"c1","status":{"state":"CONSIGN_STATE_PARTIAL"}}`), &task); err != nil {
			t.Fatalf("decoding an a2a.Task carrying the literal state = %v, want nil", err)
		}
		if task.Status.State != consignStatePartialLiteral {
			t.Fatalf("decoded task state = %q, want %q", task.Status.State, consignStatePartialLiteral)
		}
		t.Logf("decoded verbatim = %q, Terminal() = %v", task.Status.State, task.Status.State.Terminal())
	})

	// The table's second row is the discriminating case for the whole subtest:
	// the same agent, the same node, the same client, differing only in the
	// state literal. It terminates promptly, which is what makes the first row's
	// deadline a property of the state value rather than of this wiring.
	for _, tc := range []struct {
		name       string
		state      a2a.TaskState
		terminates bool
	}{
		{"literal Consign state", consignStatePartialLiteral, false},
		{"A2A state", a2a.TaskStateFailed, true},
	} {
		t.Run("over the JSON-RPC binding: "+tc.name, func(t *testing.T) {
			// A node declaring no extensions at all. The claim under test is a
			// claim about A2A's own state machine, so nothing Consign-specific
			// should be in the path; running it on a bare A2A node is what makes
			// the result attributable to A2A rather than to the profile's glue.
			// It also keeps the abandoned stream below clear of the response-echo
			// interceptor, which races with net/http when a client disconnects
			// mid-SSE -- see the report's concerns.
			n := startNodeWith(t, "literal-state-node", nil, literalStateAgent{state: tc.state})
			client, _ := newClient(t, n.Card)

			ctx, cancel := context.WithTimeout(t.Context(), hangBudget)
			defer cancel()

			var (
				taskID    a2a.TaskID
				states    []a2a.TaskState
				streamErr error
			)
			for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
				Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
			}) {
				if err != nil {
					streamErr = err
					break
				}
				switch v := ev.(type) {
				case *a2a.Task:
					taskID = v.ID
					states = append(states, v.Status.State)
				case *a2a.TaskStatusUpdateEvent:
					taskID = v.TaskID
					states = append(states, v.Status.State)
				default:
					t.Fatalf("unexpected event %T", ev)
				}
			}
			cancel()

			// Both rows agree on this much: the state value reaches the client
			// byte for byte, whichever value it is. Nothing coerces, substitutes
			// or drops it in the JSON-RPC binding.
			want := []a2a.TaskState{a2a.TaskStateSubmitted, tc.state}
			if !slices.Equal(states, want) {
				t.Fatalf("streamed states = %v, want %v", states, want)
			}

			if tc.terminates {
				if streamErr != nil {
					t.Fatalf("stream error = %v, want nil: a Terminal() state ends the stream", streamErr)
				}
			} else {
				if streamErr == nil {
					t.Fatalf("stream ended cleanly after %q; want it to run past the %s budget, because "+
						"taskupdate.IsFinal is false for a state Terminal() does not accept", tc.state, hangBudget)
				}
				if !errors.Is(streamErr, context.DeadlineExceeded) {
					t.Fatalf("stream error = %v, want context.DeadlineExceeded: the stream must be "+
						"unterminated, not failed for some other reason", streamErr)
				}
			}
			t.Logf("state %q: streamed %v, stream error = %v", tc.state, states, streamErr)

			// tasks/get on a fresh context: the stored task is readable even
			// while the hung execution still holds the stream open.
			stored, err := client.GetTask(t.Context(), &a2a.GetTaskRequest{ID: taskID})
			if err != nil {
				t.Fatalf("GetTask(%s) = %v", taskID, err)
			}
			if stored.Status.State != tc.state {
				t.Fatalf("GetTask(%s).status.state = %q, want %q", taskID, stored.Status.State, tc.state)
			}
			if stored.Status.State.Terminal() != tc.terminates {
				t.Fatalf("GetTask(%s).status.state.Terminal() = %v, want %v",
					taskID, stored.Status.State.Terminal(), tc.terminates)
			}
			t.Logf("GetTask(%s) = %q, Terminal() = %v", taskID, stored.Status.State, stored.Status.State.Terminal())
		})
	}
}

// literalStateAgent terminates its task in the state it is given, verbatim.
type literalStateAgent struct{ state a2a.TaskState }

var _ a2asrv.AgentExecutor = literalStateAgent{}

func (a literalStateAgent) Execute(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
			return
		}
		yield(a2a.NewStatusUpdateEvent(execCtx, a.state, nil), nil)
	}
}

func (a literalStateAgent) Cancel(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

// TestTerminalOverlayRoundTrips is step 2: all five of §8.1's terminal states,
// each as a coarse A2A state plus a TerminalOverlay in the verification
// namespace, over message/stream and then read back with tasks/get.
//
// The discriminating case is the shape of the table itself. PARTIAL, UNKNOWN and
// FAILED all ride TASK_STATE_FAILED, so a consumer reading status.state gets the
// same answer for all three; the assertion after the loop pins that, and it is
// the whole reason the overlay has to exist. An implementation of TerminalStateOf
// that returned anything derived from status.state would fail it.
func TestTerminalOverlayRoundTrips(t *testing.T) {
	cases := []struct {
		consign    ConsignState
		wantCoarse a2a.TaskState
	}{
		{StateCompleted, a2a.TaskStateCompleted},
		{StateCancelled, a2a.TaskStateCanceled},
		{StatePartial, a2a.TaskStateFailed},
		{StateFailed, a2a.TaskStateFailed},
		{StateUnknown, a2a.TaskStateFailed},
	}

	coarseSeen := map[ConsignState]a2a.TaskState{}
	for _, tc := range cases {
		t.Run(string(tc.consign), func(t *testing.T) {
			coarse, err := CoarseState(tc.consign)
			if err != nil {
				t.Fatalf("CoarseState(%q) = %v", tc.consign, err)
			}
			if coarse != tc.wantCoarse {
				t.Fatalf("CoarseState(%q) = %q, want %q", tc.consign, coarse, tc.wantCoarse)
			}

			overlay := TerminalOverlay{ConsignState: tc.consign, Terminal: true}
			streamed, eventMeta, stored := runOverlayTask(t, overlayAgent{coarse: coarse, overlay: &overlay})

			if streamed != tc.wantCoarse {
				t.Fatalf("streamed terminal state = %q, want %q", streamed, tc.wantCoarse)
			}
			var onEvent TerminalOverlay
			if err := FromMetadata(eventMeta[ExtVerification], &onEvent); err != nil {
				t.Fatalf("decoding the overlay off the terminal status update: %v", err)
			}
			if onEvent != overlay {
				t.Fatalf("overlay on the streamed terminal event = %+v, want %+v", onEvent, overlay)
			}

			if stored.Status.State != tc.wantCoarse {
				t.Fatalf("GetTask().status.state = %q, want %q", stored.Status.State, tc.wantCoarse)
			}
			if !stored.Status.State.Terminal() {
				t.Fatalf("GetTask().status.state.Terminal() = false; the overlay form must be A2A-terminal, " +
					"which is exactly what the literal state of step 1 was not")
			}
			got, err := TerminalStateOf(stored)
			if err != nil {
				t.Fatalf("TerminalStateOf(stored task in %q) = %v, want %q", stored.Status.State, err, tc.consign)
			}
			if got != tc.consign {
				t.Fatalf("TerminalStateOf() = %q, want %q", got, tc.consign)
			}

			coarseSeen[tc.consign] = stored.Status.State
			t.Logf("%q rode %q; tasks/get recovered %q from task.metadata[%s]",
				tc.consign, stored.Status.State, got, ExtVerification)
		})
	}

	collapsed := []ConsignState{StatePartial, StateFailed, StateUnknown}
	for _, s := range collapsed {
		if coarseSeen[s] != coarseSeen[StatePartial] {
			t.Fatalf("%q rode %q but %q rode %q; the three are supposed to be indistinguishable "+
				"in status.state", s, coarseSeen[s], StatePartial, coarseSeen[StatePartial])
		}
	}
	t.Logf("%v are all indistinguishable in status.state (%q); only the overlay separates them",
		collapsed, coarseSeen[StatePartial])
}

// TestOverlayIsCarriedRegardlessOfTheCoarseStateChosen records that the
// transport does not constrain the PARTIAL mapping at all -- the brief's
// proposed shape, PARTIAL riding TASK_STATE_COMPLETED, round-trips just as
// faithfully as PARTIAL riding TASK_STATE_FAILED. The choice between them is a
// profile decision about what a peer that ignores the overlay is told, and it is
// CoarseState, not A2A, that makes it. This test also removes the overlay
// altogether, which is the mutation that shows the consumer-side rule is load
// bearing rather than decorative.
func TestOverlayIsCarriedRegardlessOfTheCoarseStateChosen(t *testing.T) {
	t.Run("PARTIAL over TASK_STATE_COMPLETED survives the wire but fails the profile check", func(t *testing.T) {
		overlay := TerminalOverlay{ConsignState: StatePartial, Terminal: true}
		streamed, eventMeta, stored := runOverlayTask(t, overlayAgent{coarse: a2a.TaskStateCompleted, overlay: &overlay})

		if streamed != a2a.TaskStateCompleted {
			t.Fatalf("streamed terminal state = %q, want %q", streamed, a2a.TaskStateCompleted)
		}
		var onEvent TerminalOverlay
		if err := FromMetadata(eventMeta[ExtVerification], &onEvent); err != nil {
			t.Fatalf("decoding the overlay off the terminal status update: %v", err)
		}
		if onEvent != overlay {
			t.Fatalf("overlay on the streamed terminal event = %+v, want %+v", onEvent, overlay)
		}
		var onTask TerminalOverlay
		if err := FromMetadata(stored.Metadata[ExtVerification], &onTask); err != nil {
			t.Fatalf("decoding the overlay off the stored task: %v", err)
		}
		if onTask != overlay {
			t.Fatalf("overlay on the stored task = %+v, want %+v", onTask, overlay)
		}
		t.Logf("PARTIAL rode %q intact; the transport is indifferent to which coarse state carries it", streamed)

		// ... and the profile is not indifferent.
		_, err := TerminalStateOf(stored)
		if !errors.Is(err, ErrOverlayMismatch) {
			t.Fatalf("TerminalStateOf() = %v, want %v: PARTIAL rides TASK_STATE_FAILED, and a peer "+
				"ignoring the overlay must not be told this task succeeded", err, ErrOverlayMismatch)
		}
		t.Logf("TerminalStateOf() refused it: %v", err)
	})

	t.Run("a terminal task with no overlay is a protocol violation, not a FAILED", func(t *testing.T) {
		_, _, stored := runOverlayTask(t, overlayAgent{coarse: a2a.TaskStateFailed, overlay: nil})

		if stored.Status.State != a2a.TaskStateFailed {
			t.Fatalf("GetTask().status.state = %q, want %q", stored.Status.State, a2a.TaskStateFailed)
		}
		got, err := TerminalStateOf(stored)
		if !errors.Is(err, ErrOverlayMissing) {
			t.Fatalf("TerminalStateOf() = (%q, %v), want %v: without the overlay, TASK_STATE_FAILED "+
				"could be any of §8.6's FAILED, PARTIAL or UNKNOWN, and I8 forbids resolving that", got, err, ErrOverlayMissing)
		}
		t.Logf("TerminalStateOf() refused it: %v", err)
	})
}

// runOverlayTask submits one task to a node running agent, and returns the
// terminal state seen on the stream, that terminal event's metadata, and the
// task tasks/get returns afterwards.
func runOverlayTask(t *testing.T, agent overlayAgent) (a2a.TaskState, map[string]any, *a2a.Task) {
	t.Helper()

	exts := AllExtensions()
	n := startNodeWith(t, "overlay-node", exts, agent)
	client, _ := newClient(t, n.Card)
	ctx := a2aclient.AttachServiceParams(t.Context(), a2aclient.ServiceParams{
		a2a.SvcParamExtensions: ExtensionURIs(exts),
	})

	var (
		taskID   a2a.TaskID
		terminal *a2a.TaskStatusUpdateEvent
	)
	for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
	}) {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		switch v := ev.(type) {
		case *a2a.Task:
			taskID = v.ID
		case *a2a.TaskStatusUpdateEvent:
			taskID = v.TaskID
			terminal = v
		default:
			t.Fatalf("unexpected event %T", ev)
		}
	}
	if terminal == nil {
		t.Fatalf("stream carried no status update event")
	}

	stored, err := client.GetTask(ctx, &a2a.GetTaskRequest{ID: taskID})
	if err != nil {
		t.Fatalf("GetTask(%s) = %v", taskID, err)
	}
	return terminal.Status.State, terminal.Metadata, stored
}

// overlayAgent terminates its task in a coarse A2A state, carrying the refined
// §8.1 state in the verification namespace. A nil overlay emits none, which is
// the case the consumer-side rule has to catch.
type overlayAgent struct {
	coarse  a2a.TaskState
	overlay *TerminalOverlay
}

var _ a2asrv.AgentExecutor = overlayAgent{}

func (a overlayAgent) Execute(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
			return
		}
		done := a2a.NewStatusUpdateEvent(execCtx, a.coarse, nil)
		if a.overlay != nil {
			payload, err := ToMetadata(*a.overlay)
			if err != nil {
				yield(nil, err)
				return
			}
			done.SetMeta(ExtVerification, payload)
		}
		yield(done, nil)
	}
}

func (a overlayAgent) Cancel(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}
