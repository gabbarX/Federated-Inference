package smoke

import (
	"context"
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// TestFullRoundTrip is demonstration 1: submit, accept, event stream, terminal
// result, over message/stream, with all seven Consign URIs observed active on
// the wire.
//
// "Without an A2A protocol violation" is asserted concretely, because the
// phrase is otherwise unfalsifiable. It means, here: the stream yields no
// error; every event is one of the four event types A2A v1.0 defines; the
// stream begins with a Task as a2a-go requires and ends in a state
// a2a.TaskState.Terminal() accepts; and a subsequent tasks/get over the same
// binding returns that same task in that same terminal state, so nothing the
// Consign payloads did prevented the SDK from storing a well-formed task.
func TestFullRoundTrip(t *testing.T) {
	c := startChain(t)
	obs := c.Submit(t)

	t.Run("the stream completes with no A2A protocol violation", func(t *testing.T) {
		if len(obs.Errors) != 0 {
			t.Fatalf("stream yielded %d errors, first = %v", len(obs.Errors), obs.Errors[0])
		}
		if len(obs.Events) == 0 {
			t.Fatalf("stream yielded no events")
		}
		if _, ok := obs.Events[0].(*a2a.Task); !ok {
			t.Fatalf("first stream event is %T, want *a2a.Task", obs.Events[0])
		}
		for i, ev := range obs.Events {
			switch ev.(type) {
			case *a2a.Task, *a2a.Message, *a2a.TaskStatusUpdateEvent, *a2a.TaskArtifactUpdateEvent:
			default:
				t.Fatalf("event %d is %T, which is not an A2A v1.0 event type", i, ev)
			}
		}
		if obs.Final != a2a.TaskStateCompleted {
			t.Fatalf("terminal state = %s, want %s", obs.Final, a2a.TaskStateCompleted)
		}
		if !obs.Final.Terminal() {
			t.Fatalf("a2a.TaskState(%s).Terminal() = false", obs.Final)
		}

		client, _ := newClient(t, c.B.Card)
		stored, err := client.GetTask(c.withExtensions(t.Context()), &a2a.GetTaskRequest{ID: obs.TaskID})
		if err != nil {
			t.Fatalf("GetTask(%s) after the stream = %v", obs.TaskID, err)
		}
		if stored.Status.State != obs.Final {
			t.Fatalf("stored task state = %s, streamed terminal state = %s", stored.Status.State, obs.Final)
		}
		t.Logf("stream: %d events, terminal %s, stored task has %d artifacts",
			len(obs.Events), obs.Final, len(stored.Artifacts))
	})

	t.Run("all seven URIs are active on the wire", func(t *testing.T) {
		want := sorted(ExtensionURIs(AllExtensions()))

		// Observed on the response, off the client's own RoundTripper.
		got := observedActivatedURIs(t, obs.Recorder)
		if !slices.Equal(got, want) {
			t.Fatalf("streaming %s response header = %v, want the seven declared %v",
				a2a.SvcParamExtensions, got, want)
		}

		// Observed in-band, from a2asrv's own activation bookkeeping: the
		// producer read Extensions().ActivatedURIs() inside its executor and put
		// it in the task.accepted payload. The header above is written by outer
		// middleware that never sees the CallContext, so the two are independent
		// observations and their agreement is the evidence that the header
		// reports activation rather than replaying the request.
		accepted := obs.envelope(t, EventAccepted)
		inBand := stringsOf(t, accepted.Payload["activated_extensions"])
		if !slices.Equal(sorted(inBand), want) {
			t.Fatalf("a2asrv ActivatedURIs() seen inside the executor = %v, want %v", inBand, want)
		}
		t.Logf("activated in-band by a2asrv = %v", inBand)
	})

	t.Run("the Consign event sequence is complete and gapless", func(t *testing.T) {
		if err := CheckSeq(obs.Envelopes); err != nil {
			t.Fatalf("CheckSeq() over the observed §9.1 envelopes = %v, want nil", err)
		}
		types := []string{}
		for _, e := range obs.Envelopes {
			types = append(types, e.Type)
		}
		want := []string{EventAccepted, EventDelegated, EventArtifact, EventArtifact, EventArtifact, EventCompleted}
		if !slices.Equal(types, want) {
			t.Fatalf("observed Consign event types = %v, want %v", types, want)
		}
		if obs.Envelopes[0].Type != EventAccepted || obs.Envelopes[0].Seq != 1 {
			t.Fatalf("first Consign event = %s at seq %d, want %s at seq 1 (§8.4)",
				obs.Envelopes[0].Type, obs.Envelopes[0].Seq, EventAccepted)
		}
		t.Logf("Consign events = %v", types)
	})

	t.Run("artifacts carry their §10 record and grant on the A2A artifact", func(t *testing.T) {
		if len(obs.Artifacts) != 3 {
			t.Fatalf("stream carried %d artifacts, want 3", len(obs.Artifacts))
		}
		for _, art := range obs.Artifacts {
			if !slices.Contains(art.Extensions, ExtArtifacts) {
				t.Fatalf("artifact %s does not name %s in artifact.extensions[]", art.ID, ExtArtifacts)
			}
			var carried ArtifactPayload
			if err := FromMetadata(art.Metadata[ExtArtifacts], &carried); err != nil {
				t.Fatalf("artifact %s metadata: %v", art.ID, err)
			}
			if carried.Record.Label != "CONSORTIUM" {
				t.Fatalf("artifact %s label = %q, want CONSORTIUM", art.ID, carried.Record.Label)
			}
			// §9.3: task.artifact is a reference, so no bytes ride the stream.
			// A2A requires at least one part
			// (internal/taskupdate/manager.go: "artifact cannot be empty"), so
			// the reference is that part: a URL into the holder's grant
			// endpoint, never a Raw or Text part carrying content.
			if len(art.Parts) != 1 {
				t.Fatalf("artifact %s carries %d parts, want exactly 1 reference part", art.ID, len(art.Parts))
			}
			if url := string(art.Parts[0].URL()); url == "" {
				t.Fatalf("artifact %s part is %T, want a URL reference; §9.3 forbids bytes on the stream",
					art.ID, art.Parts[0].Content)
			}
			if art.Parts[0].Raw() != nil || art.Parts[0].Text() != "" {
				t.Fatalf("artifact %s part carries content, not a reference", art.ID)
			}

			// §10.1: retrieve from the holder and verify the address.
			data := c.Retrieve(t, carried.Grant, carried.Record.Digest)
			if strings.Contains(string(art.Parts[0].URL()), string(data)) {
				t.Fatalf("artifact %s smuggles its bytes into the reference", art.ID)
			}
			if err := VerifyCAS(carried.Record.Digest, data); err != nil {
				t.Fatalf("artifact %s: %v", art.ID, err)
			}
			if int64(len(data)) != carried.Record.Size {
				t.Fatalf("artifact %s: retrieved %d bytes, record says %d", art.ID, len(data), carried.Record.Size)
			}
		}
		t.Logf("retrieved and digest-verified %d holder-hosted artifacts", len(obs.Artifacts))
	})

	t.Run("the terminal result is accepted on the consumer's own evidence", func(t *testing.T) {
		completed := obs.envelope(t, EventCompleted)
		resultRef, _ := completed.Payload["result_ref"].(string)
		if resultRef == "" {
			t.Fatalf("task.completed payload has no result_ref: %v", completed.Payload)
		}

		result, retrieved := c.RetrieveResult(t, obs, resultRef)
		if result.Contract != c.Contract.Ref() {
			t.Fatalf("result contract = %+v, want %+v", result.Contract, c.Contract.Ref())
		}
		if err := AcceptResult(result, retrieved); err != nil {
			t.Fatalf("AcceptResult() on the delivered result = %v, want nil", err)
		}
		if len(result.Claims) == 0 {
			t.Fatalf("result carries no claims")
		}
		// §11.3: the producer says it validated, and that is not why the result
		// was accepted -- AcceptResult never reads the flag.
		if !result.Claims[0].ProducerValidated {
			t.Fatalf("fixture is wrong: the claim must assert producer_validated")
		}
		t.Logf("accepted result for %s: %d claims over %d artifacts, provenance %v",
			result.TaskID, len(result.Claims), len(result.Artifacts), result.Provenance.Chain)
	})
}

// TestStreamingActivationIsNotEchoedByTheInterceptor characterizes what happens
// to extension negotiation on the message/stream path, which is the question
// the round trip above answers only once the profile has worked around it.
//
// The node below is wired exactly as Task 2 wired its nodes: the response echo
// lives in consignExtensions.After. On message/send that works. On
// message/stream it cannot, and the mechanism is not subtle --
// jsonrpcHandler.handleStreamingRequest calls sseWriter.WriteHeaders(), which
// ends in WriteHeader(http.StatusOK), before it dispatches to the handler, so
// every interceptor runs after the response headers are committed.
//
// The test pins both halves: activation genuinely happens (a2asrv's own
// ActivatedURIs() is non-empty inside the executor) and the header genuinely
// does not arrive. It is the loss of the echo that is being characterized, not
// a loss of negotiation.
func TestStreamingActivationIsNotEchoedByTheInterceptor(t *testing.T) {
	exts := AllExtensions()
	n := startNodeCustom(t, "interceptor-echo-only", exts, activationReporter{}, false)
	client, rec := newClient(t, n.Card)
	ctx := a2aclient.AttachServiceParams(t.Context(), a2aclient.ServiceParams{
		a2a.SvcParamExtensions: ExtensionURIs(exts),
	})

	var reported []string
	for ev, err := range client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
	}) {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		if status, ok := ev.(*a2a.TaskStatusUpdateEvent); ok && status.Metadata != nil {
			reported = stringsOf(t, status.Metadata["activated"])
		}
	}

	if got := sorted(reported); !slices.Equal(got, sorted(ExtensionURIs(exts))) {
		t.Fatalf("a2asrv ActivatedURIs() inside the executor = %v, want all seven; "+
			"activation itself must still work on the streaming path", got)
	}
	streamed := rec.Last().Values(a2a.SvcParamExtensions)
	if len(streamed) != 0 {
		t.Fatalf("streaming %s response header = %v, want empty: the interceptor cannot write it "+
			"after sseWriter.WriteHeaders() has committed the response", a2a.SvcParamExtensions, streamed)
	}
	t.Logf("streaming: activated in-band = %d URIs, %s response header = %v",
		len(reported), a2a.SvcParamExtensions, streamed)

	// Same node, same interceptor, non-streaming: the echo arrives. This is what
	// makes the empty header above a property of the streaming binding rather
	// than of this node's wiring.
	if _, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
	}); err != nil {
		t.Fatalf("SendMessage() = %v", err)
	}
	sent := rec.Last().Values(a2a.SvcParamExtensions)
	if len(sent) != len(exts) {
		t.Fatalf("non-streaming %s response header = %v, want the seven activated URIs",
			a2a.SvcParamExtensions, sent)
	}
	t.Logf("non-streaming: %s response header carries %d URIs", a2a.SvcParamExtensions, len(sent))
}

// activationReporter is the smallest agent that reports what a2asrv considers
// activated, so the characterization test can separate activation from echo.
type activationReporter struct{}

func (activationReporter) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		activated := []any{}
		if ext, ok := a2asrv.ExtensionsFrom(ctx); ok {
			for _, uri := range ext.ActivatedURIs() {
				activated = append(activated, uri)
			}
		}
		if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
			return
		}
		done := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil)
		done.SetMeta("activated", activated)
		yield(done, nil)
	}
}

func (activationReporter) Cancel(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

// stringsOf reads a []any of strings back out of a metadata payload, which is
// the only list shape a2asrv's task store permits (see ToMetadata).
func stringsOf(t *testing.T, v any) []string {
	t.Helper()
	items, ok := v.([]any)
	if !ok {
		t.Fatalf("metadata value is %T, want []any", v)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("metadata list element is %T, want string", item)
		}
		out = append(out, s)
	}
	return out
}
