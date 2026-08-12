package smoke

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// This file is the A→B→C topology the Gate B demonstrations run against.
//
// Three Consign nodes take part. A (did:web:lab-a.example) is the originator
// and consumer; it mints the root token and drives the stream, and needs no
// server of its own because nothing is ever submitted to it. B
// (did:web:lab-b.example) is the producer: it receives A's task, mints an
// attenuated child token and submits it to C over A2A. C
// (did:web:company-c.example) is the delegate, and is a real third node rather
// than a stand-in -- everything asserted about the hop is read off C's own wire.
//
// B additionally runs a holder-hosted artifact endpoint (§10.2), separate from
// its A2A endpoint because §8.4 makes grant_endpoint its own URL.

// consignRecipientHeader identifies the retrieving organization to the holder.
// In a real deployment that identity comes from the mTLS client certificate
// (§4.3); mTLS is out of scope here, so it is stated in a header and the
// substitution is recorded rather than hidden.
const consignRecipientHeader = "Consign-Recipient"

// consignTaskHeader names the task the retrieval is for. §10.4 scopes a grant
// to a specific recipient *and* task, so the holder needs both; without the
// task on the request the task_id conjunct could only ever be checked against
// the grant's own value, which is no check at all.
const consignTaskHeader = "Consign-Task"

const kidC = orgC + "#org-2026-08"

// childShare is the slice of its allowance node B hands to node C. It is
// deliberately more than half of max_model_calls so that two children of this
// size cannot both fit, which is what TestReDelegationRefused turns on.
func childShare(allowance Budget) Budget {
	return Budget{
		Deadline:         allowance.Deadline,
		MaxModelCalls:    6,
		MaxToolCalls:     8,
		MaxArtifactBytes: allowance.MaxArtifactBytes / 4,
	}
}

// chain holds the whole topology plus the key material and documents the
// assertions need.
type chain struct {
	Contract  Contract
	Root      Token
	Allowance Budget

	// Keys is pinned at every node, keyed by kid (§4.5). OrgKeys is the same
	// material keyed by organization DID, which is all a §12.3 receipt
	// signature names.
	Keys     Keyring
	OrgKeys  Keyring
	SignKeys map[string]ed25519.PrivateKey

	B, C     *node
	Producer *producerAgent
	Delegate *delegateAgent
}

// startChain brings up nodes B and C and mints the root token A will submit.
func startChain(t *testing.T) *chain {
	t.Helper()

	pub := Keyring{}
	orgPub := Keyring{}
	priv := map[string]ed25519.PrivateKey{}
	for org, kid := range map[string]string{orgA: kidA, orgB: kidB, orgC: kidC} {
		public, secret, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("generating %s key: %v", org, err)
		}
		pub[kid] = public
		orgPub[org] = public
		priv[org] = secret
	}

	now := time.Now()
	rfc := func(d time.Duration) string { return now.Add(d).UTC().Format(time.RFC3339) }
	allowance := Budget{
		Deadline:         rfc(20 * time.Minute),
		MaxModelCalls:    10,
		MaxToolCalls:     20,
		MaxArtifactBytes: 1 << 20,
	}

	contract := codeReviewContract(t)
	root, err := Sign(Token{
		Typ:        TokenType,
		TokenID:    "tok_root",
		Originator: orgA,
		RootTaskID: "task_019fe2",
		IssuedAt:   rfc(0),
		ExpiresAt:  rfc(30 * time.Minute),
		Caveats: Caveats{
			Contract:             []string{contract.ID},
			DataClass:            "CONSORTIUM",
			AllowedOrganizations: []string{orgA, orgB, orgC},
			AllowedTools:         []string{"git", "pytest"},
			NetworkAccess:        "deny",
			SideEffects:          "propose-only",
			MaxDelegationDepth:   1,
			AllowedDelegates:     []string{orgB, orgC},
			Budget:               allowance,
		},
	}, kidA, priv[orgA])
	if err != nil {
		t.Fatalf("minting the root token: %v", err)
	}

	delegate := &delegateAgent{org: orgC, keys: pub}
	nodeC := startNodeWith(t, "node-c", AllExtensions(), delegate)

	producer := &producerAgent{
		org:        orgB,
		kid:        kidB,
		signKey:    priv[orgB],
		keys:       pub,
		orgKeys:    orgPub,
		contract:   contract,
		consumer:   orgA,
		delegate:   orgC,
		blobs:      map[string][]byte{},
		grants:     map[string]Grant{},
		servedOnce: map[string]bool{},
	}
	producer.startArtifactEndpoint(t)
	nodeB := startNodeWith(t, "node-b", AllExtensions(), producer, &consignAcceptance{contracts: []Contract{contract}})

	// B talks to C as a client, over the same JSON-RPC binding A uses on B.
	delegateClient, err := a2aclient.NewFromCard(context.Background(), nodeC.Card)
	if err != nil {
		t.Fatalf("building node B's client for node C: %v", err)
	}
	t.Cleanup(func() { _ = delegateClient.Destroy() })
	producer.delegateClient = delegateClient

	return &chain{
		Contract: contract, Root: root, Allowance: allowance,
		Keys: pub, OrgKeys: orgPub, SignKeys: priv,
		B: nodeB, C: nodeC, Producer: producer, Delegate: delegate,
	}
}

// withExtensions attaches the seven Consign URIs as the A2A-Extensions request
// service parameter. a2aext.NewActivator is deliberately not used: it drops
// URIs the peer's card does not declare, which is the silent degradation §2.3
// forbids.
func (c *chain) withExtensions(ctx context.Context) context.Context {
	return a2aclient.AttachServiceParams(ctx, a2aclient.ServiceParams{
		a2a.SvcParamExtensions: ExtensionURIs(AllExtensions()),
	})
}

// observed is everything node A saw on the wire for one task.
type observed struct {
	Events    []a2a.Event
	Errors    []error
	Envelopes []EventEnvelope
	Artifacts []*a2a.Artifact
	Recorder  *headerRecorder

	TaskID       a2a.TaskID
	Final        a2a.TaskState
	TerminalMeta map[string]any
	Receipt      Receipt

	retrieved map[string][]byte
	accepted  bool
}

// envelope returns the single §9.1 envelope of the given type, failing if the
// stream did not carry exactly one.
func (o *observed) envelope(t *testing.T, typ string) EventEnvelope {
	t.Helper()
	var found []EventEnvelope
	for _, e := range o.Envelopes {
		if e.Type == typ {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("stream carried %d %s envelopes, want exactly 1; types seen: %v",
			len(found), typ, envelopeTypes(o.Envelopes))
	}
	return found[0]
}

func envelopeTypes(events []EventEnvelope) []string {
	types := make([]string, 0, len(events))
	for _, e := range events {
		types = append(types, e.Type)
	}
	return types
}

// Submit is node A's whole side of the task: it streams the submission over
// message/stream and collects every event, envelope and artifact the producer
// emitted, plus the response headers its own RoundTripper saw.
func (c *chain) Submit(t *testing.T) *observed {
	t.Helper()

	client, rec := newClient(t, c.B.Card)
	obs := &observed{Recorder: rec, retrieved: map[string][]byte{}}

	message := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(
		"Review the authentication service and produce a patch"))
	message.Extensions = ExtensionURIs(AllExtensions())

	for ev, err := range client.SendStreamingMessage(c.withExtensions(t.Context()), &a2a.SendMessageRequest{
		Message: message,
		Metadata: map[string]any{
			ExtContract:    jsonValue(t, c.Contract.Ref()),
			ExtAuthority:   jsonValue(t, c.Root),
			ExtBudget:      jsonValue(t, BudgetReport{Allowance: c.Allowance, Remaining: c.Allowance}),
			ExtConstraints: jsonValue(t, c.Root.Caveats),
		},
	}) {
		if err != nil {
			obs.Errors = append(obs.Errors, err)
			continue
		}
		obs.Events = append(obs.Events, ev)
		obs.collect(t, ev)
	}
	return obs
}

// collect pulls the Consign payloads out of one A2A event.
func (o *observed) collect(t *testing.T, ev a2a.Event) {
	t.Helper()
	switch e := ev.(type) {
	case *a2a.Task:
		o.TaskID = e.ID
	case *a2a.TaskArtifactUpdateEvent:
		o.Artifacts = append(o.Artifacts, e.Artifact)
		o.appendEnvelopes(t, e.Metadata)
	case *a2a.TaskStatusUpdateEvent:
		o.Final = e.Status.State
		o.appendEnvelopes(t, e.Metadata)
		if e.Status.State.Terminal() {
			o.TerminalMeta = e.Metadata
			if msg := e.Status.Message; msg != nil {
				if payload, ok := msg.Metadata[ExtReceipts]; ok {
					if err := FromMetadata(payload, &o.Receipt); err != nil {
						t.Fatalf("decoding the proposed receipt: %v", err)
					}
				}
			}
		}
	}
}

// appendEnvelopes reads §9.1 envelopes out of an A2A event's metadata. A
// Consign event rides the namespace of the extension whose payload it carries,
// so which namespace to look in is not knowable from A2A -- every Consign
// namespace has to be checked.
func (o *observed) appendEnvelopes(t *testing.T, metadata map[string]any) {
	t.Helper()
	for _, uri := range ExtensionURIs(AllExtensions()) {
		payload, ok := metadata[uri]
		if !ok {
			continue
		}
		var env EventEnvelope
		if err := FromMetadata(payload, &env); err != nil || env.Type == "" {
			continue
		}
		o.Envelopes = append(o.Envelopes, env)
	}
}

// Retrieve fetches an artifact from its holder under the grant that came with
// it (§10.2), and records the bytes node A actually received.
//
// The consumer's own grant check is here rather than in Fetch, so that Fetch
// can put a request on the wire the consumer would not have made. Without that
// separation the holder's enforcement could never be tested: every request
// would already have been screened client-side, and a holder that checked
// nothing would look identical to one that checked everything.
func (c *chain) Retrieve(t *testing.T, grant Grant, digest string) []byte {
	t.Helper()
	if err := grant.Check(orgA, grant.TaskID, digest, time.Now()); err != nil {
		t.Fatalf("the grant that arrived with %s does not authorise this consumer: %v", digest, err)
	}
	data, err := c.Fetch(t, grant.TaskID, digest, orgA)
	if err != nil {
		t.Fatalf("retrieving %s: %v", digest, err)
	}
	return data
}

// Fetch goes straight to the holder's §10.2 endpoint as the named organization,
// with no consumer-side screening, and returns whatever the holder decides.
func (c *chain) Fetch(t *testing.T, taskID, digest, as string) ([]byte, error) {
	t.Helper()
	url := c.Producer.grantEndpoint + "/" + strings.TrimPrefix(digest, casPrefix)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building retrieval request: %v", err)
	}
	req.Header.Set(consignRecipientHeader, as)
	req.Header.Set(consignTaskHeader, taskID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("holder returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// RetrieveResult fetches every artifact the stream announced, then the result
// document itself, and returns the decoded result alongside the bytes a
// consumer-side validation run has to work from.
func (c *chain) RetrieveResult(t *testing.T, obs *observed, resultRef string) (Result, map[string][]byte) {
	t.Helper()
	for _, art := range obs.Artifacts {
		var carried ArtifactPayload
		if err := FromMetadata(art.Metadata[ExtArtifacts], &carried); err != nil {
			t.Fatalf("decoding artifact metadata: %v", err)
		}
		data := c.Retrieve(t, carried.Grant, carried.Record.Digest)
		obs.retrieved[carried.Record.Digest] = data
	}

	raw, ok := obs.retrieved[resultRef]
	if !ok {
		t.Fatalf("the stream announced result_ref %s but no artifact carried it", resultRef)
	}
	if err := VerifyCAS(resultRef, raw); err != nil {
		t.Fatalf("result document: %v", err)
	}
	var result Result
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decoding the result document: %v", err)
	}
	obs.accepted = AcceptResult(result, obs.retrieved) == nil
	return result, obs.retrieved
}

// Validate is node A's whole §11.3 pass: pull the result reference off the
// terminal event, retrieve and digest-verify every artifact, and decide
// acceptance on that evidence. It has to happen before A can witness units of
// its own -- verified_completions is defined as coming from the consumer's own
// validator run (§12.2), so before this runs there is nothing to witness.
func (c *chain) Validate(t *testing.T, obs *observed) Result {
	t.Helper()
	completed := obs.envelope(t, EventCompleted)
	resultRef, _ := completed.Payload["result_ref"].(string)
	if resultRef == "" {
		t.Fatalf("task.completed payload has no result_ref: %v", completed.Payload)
	}
	result, _ := c.RetrieveResult(t, obs, resultRef)
	return result
}

// WitnessedUnits is node A's own accounting (§12.2): bytes it actually
// retrieved, completions its own validation run accepted, and a lease duration
// taken from the producer's own accepted and completed event timestamps, which
// both parties can see.
func (o *observed) WitnessedUnits(t *testing.T) Units {
	t.Helper()
	var bytes int64
	for _, data := range o.retrieved {
		bytes += int64(len(data))
	}
	completions := 0
	if o.accepted {
		completions = 1
	}
	accepted, completed := o.envelope(t, EventAccepted), o.envelope(t, EventCompleted)
	from, err := time.Parse(time.RFC3339Nano, accepted.EmittedAt)
	if err != nil {
		t.Fatalf("parsing task.accepted emitted_at: %v", err)
	}
	to, err := time.Parse(time.RFC3339Nano, completed.EmittedAt)
	if err != nil {
		t.Fatalf("parsing task.completed emitted_at: %v", err)
	}
	return Units{
		LeaseSeconds:        int(to.Sub(from).Seconds()),
		TasksAccepted:       1,
		VerifiedCompletions: completions,
		ArtifactBytesServed: bytes,
	}
}

// CoSignAndReturn is node A's last step: it counter-signs the producer's
// proposal and returns the co-signed receipt to node B in a final A2A Message,
// then reads back the copy node B verified off its own wire.
func (c *chain) CoSignAndReturn(t *testing.T, obs *observed, proposed Receipt) Receipt {
	t.Helper()
	cosigned, err := proposed.SignedBy(orgA, c.SignKeys[orgA])
	if err != nil {
		t.Fatalf("counter-signing: %v", err)
	}

	message := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("receipt"))
	message.Extensions = []string{ExtReceipts}
	message.SetMeta(ExtReceipts, jsonValue(t, cosigned))

	client, _ := newClient(t, c.B.Card)
	res, err := client.SendMessage(c.withExtensions(t.Context()), &a2a.SendMessageRequest{Message: message})
	if err != nil {
		t.Fatalf("returning the co-signed receipt: %v", err)
	}
	reply, ok := res.(*a2a.Message)
	if !ok {
		t.Fatalf("node B answered the receipt with %T, want *a2a.Message", res)
	}
	var readBack Receipt
	if err := FromMetadata(reply.Metadata[ExtReceipts], &readBack); err != nil {
		t.Fatalf("decoding the receipt node B read back: %v", err)
	}
	return readBack
}

// producerAgent is node B: it accepts A's task, delegates part of it to C, and
// publishes artifacts, a result and a receipt proposal.
type producerAgent struct {
	org, kid string
	signKey  ed25519.PrivateKey
	keys     Keyring
	orgKeys  Keyring
	contract Contract
	consumer string
	delegate string

	delegateClient *a2aclient.Client
	grantEndpoint  string

	mu     sync.Mutex
	blobs  map[string][]byte
	grants map[string]Grant
	// servedOnce records which digests have already been counted, so that
	// artifact_bytes_served means "distinct artifact bytes transferred for this
	// task" on both sides. The consumer's own tally is a map keyed by digest and
	// therefore dedupes; without the same rule here the two would disagree the
	// moment anything is fetched twice, and §12.2's whole premise is that both
	// parties can arrive at the same number independently.
	servedOnce map[string]bool
	served     int64
}

var _ a2asrv.AgentExecutor = (*producerAgent)(nil)

// startArtifactEndpoint brings up the holder-hosted retrieval surface of §10.2.
// It is a separate server from the A2A endpoint because §8.4 makes
// grant_endpoint its own URL, and only GET is implemented: §10.2's PUT, HEAD
// and byte-range surface says nothing about A2A extension expressibility, which
// is what this spike is for.
func (p *producerAgent) startArtifactEndpoint(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		digest := casPrefix + strings.TrimPrefix(req.URL.Path, "/")
		recipient := req.Header.Get(consignRecipientHeader)
		taskID := req.Header.Get(consignTaskHeader)

		p.mu.Lock()
		data, known := p.blobs[digest]
		grant, granted := p.grants[digest]
		p.mu.Unlock()

		// §10.4 places the authorization on the holder, so every conjunct is
		// checked here and not merely by the well-behaved consumer: the grant
		// must exist, and must name this recipient, this task and this digest,
		// and must not have expired.
		if !known || !granted || grant.Check(recipient, taskID, digest, time.Now()) != nil {
			// §8.5 and §10.4: one coarse code, and no hint about which conjunct
			// failed or whether the digest exists at all.
			rw.WriteHeader(http.StatusForbidden)
			_, _ = rw.Write([]byte(CodeGrantInvalid))
			return
		}

		p.mu.Lock()
		if !p.servedOnce[digest] {
			p.servedOnce[digest] = true
			p.served += int64(len(data))
		}
		p.mu.Unlock()
		_, _ = rw.Write(data)
	}))
	t.Cleanup(srv.Close)
	p.grantEndpoint = srv.URL
}

func (p *producerAgent) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		if msg := execCtx.Message; msg != nil {
			if payload, ok := msg.Metadata[ExtReceipts]; ok {
				yield(p.settleReceipt(execCtx, payload))
				return
			}
		}
		for ev, err := range p.runTask(ctx, execCtx) {
			if !yield(ev, err) {
				return
			}
		}
	}
}

func (p *producerAgent) Cancel(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

func (p *producerAgent) runTask(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		now := time.Now()

		var token Token
		if err := FromMetadata(execCtx.Metadata[ExtAuthority], &token); err != nil {
			yield(nil, err)
			return
		}
		// §7.3, in full, against the token that actually arrived.
		if err := VerifyChain(p.keys, p.org, token, now); err != nil {
			yield(nil, consignRefusal(a2a.ErrUnauthorized, "capability token not accepted", CodeTokenInvalid))
			return
		}
		var declaredBudget BudgetReport
		if err := FromMetadata(execCtx.Metadata[ExtBudget], &declaredBudget); err != nil {
			yield(nil, err)
			return
		}
		// The signed token is authoritative; the budget namespace is a
		// disclosure of the same numbers and must not disagree with it.
		if declaredBudget.Allowance != token.Caveats.Budget {
			yield(nil, consignRefusal(a2a.ErrInvalidParams, "budget disclosure does not match the token", CodeSchemaInvalid))
			return
		}

		stream := NewStream(string(execCtx.TaskID))
		if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
			return
		}

		// §8.4: acceptance carries the lease, the stream cursor and the grant
		// endpoint, and is seq 1. activated_extensions is not part of §8.4; it
		// is a2asrv's own activation record, carried here so the consumer can
		// compare it with the A2A-Extensions response header.
		activated := []any{}
		if ext, ok := a2asrv.ExtensionsFrom(ctx); ok {
			for _, uri := range ext.ActivatedURIs() {
				activated = append(activated, uri)
			}
		}
		accepted := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil)
		acceptedEnv := stream.Next(EventAccepted, map[string]any{
			"lease": map[string]any{
				"lease_id":          "lease_" + string(execCtx.TaskID)[:8],
				"expires_at":        now.Add(10 * time.Minute).UTC().Format(time.RFC3339),
				"heartbeat_seconds": 10,
			},
			"node_id":              p.org + "#node-1",
			"grant_endpoint":       p.grantEndpoint,
			"stream":               map[string]any{"cursor": 1},
			"activated_extensions": activated,
		})
		accepted.SetMeta(ExtContract, mustMeta(acceptedEnv))
		if !yield(accepted, nil) {
			return
		}

		ledger := NewLedger(token.Caveats.Budget)
		share := childShare(token.Caveats.Budget)
		child, err := MintChild(token, ledger, p.delegate, "tok_child", p.kid, share, p.signKey, now)
		if err != nil {
			yield(nil, err)
			return
		}

		childTaskID, childClaim, err := p.submitToDelegate(ctx, child)
		if err != nil {
			yield(nil, err)
			return
		}

		// §7.6: the disclosure is required on every re-delegation, and it names
		// the child task the delegate really created.
		delegated := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil)
		delegated.SetMeta(ExtAuthority, mustMeta(stream.Next(EventDelegated, map[string]any{
			"child_task_id":   string(childTaskID),
			"delegate_org":    p.delegate,
			"remaining_depth": child.Caveats.MaxDelegationDepth,
		})))
		if !yield(delegated, nil) {
			return
		}

		patch := []byte("--- a/auth.go\n+++ b/auth.go\n@@ rotate the session id on login\n")
		report := []byte(`{"tests": 41, "failed": 0, "delegate": "` + p.delegate + `"}`)

		result := Result{
			Schema:        ResultType,
			TaskID:        string(execCtx.TaskID),
			Contract:      p.contract.Ref(),
			Verifiability: p.contract.Verifiability,
			Output:        map[string]any{"summary": "session fixation fixed by rotating the session id"},
			Artifacts: []ResultArtifact{
				{Role: "patch", Ref: CASRef(patch), Label: "CONSORTIUM"},
				{Role: "test_report", Ref: CASRef(report), Label: "CONSORTIUM"},
			},
			Claims: []Claim{
				{
					ID:                "c1",
					Statement:         "Session fixation is fixed by rotating the session id on login",
					Evidence:          []string{CASRef(report)},
					ProducerValidated: true,
				},
				childClaim,
			},
			Provenance: Provenance{
				Chain:     []string{p.consumer, p.org, p.delegate},
				ToolsUsed: token.Caveats.AllowedTools,
			},
			Usage: Usage{WallSeconds: 1, DeclaredModelCalls: 2, DeclaredToolCalls: 3},
		}
		// The result envelope is content-addressed like any other artifact, so
		// what the task.completed event names is the digest of these bytes and
		// not an inline copy (§9.3, §11.2).
		resultDoc, err := json.Marshal(result)
		if err != nil {
			yield(nil, err)
			return
		}

		var published int64
		for _, blob := range []struct {
			role, mediaType string
			data            []byte
		}{
			{"patch", "application/vnd.consign.patch", patch},
			{"test_report", "application/json", report},
			{"result", "application/json", resultDoc},
		} {
			ev, size := p.publish(execCtx, stream, blob.role, blob.mediaType, blob.data, now)
			published += size
			if !yield(ev, nil) {
				return
			}
		}

		remaining := ledger.Remaining()
		completedEnv := stream.Next(EventCompleted, map[string]any{
			"result_ref":       CASRef(resultDoc),
			"verifiability":    result.Verifiability,
			"provenance_chain": []any{p.consumer, p.org, p.delegate},
			"usage":            mustMeta(result.Usage),
		})
		completed := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil)
		completed.SetMeta(ExtVerification, mustMeta(completedEnv))
		completed.SetMeta(ExtBudget, mustMeta(BudgetReport{
			Allowance: token.Caveats.Budget,
			Remaining: remaining,
			Delegated: []Budget{share},
		}))

		// §12.2's units, proposed by the producer from what it can witness.
		//
		// lease_seconds is taken from the producer's own task.accepted and
		// task.completed timestamps rather than from a private clock, because
		// §12.2 admits it only as a unit *both* parties witness -- and the only
		// way the consumer can witness it is from those same two events on the
		// stream. verified_completions is the one unit the producer cannot see
		// for itself, so it is proposed optimistically and stands only if the
		// consumer's own validation run agrees before counter-signing (§12.3).
		receipt := Receipt{
			Typ:           ReceiptType,
			TaskID:        string(execCtx.TaskID),
			Consumer:      p.consumer,
			Producer:      p.org,
			TerminalState: "COMPLETED",
			Units: Units{
				LeaseSeconds:        elapsedSeconds(acceptedEnv, completedEnv),
				TasksAccepted:       1,
				VerifiedCompletions: 1,
				ArtifactBytesServed: published,
			},
		}
		signed, err := receipt.SignedBy(p.org, p.signKey)
		if err != nil {
			yield(nil, err)
			return
		}
		final := a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("receipt proposal"))
		final.Extensions = []string{ExtReceipts}
		final.SetMeta(ExtReceipts, mustMeta(signed))
		completed.Status.Message = final

		yield(completed, nil)
	}
}

// publish stores an artifact at the holder, grants the consumer access to it,
// and returns the A2A artifact event announcing it by reference.
func (p *producerAgent) publish(execCtx *a2asrv.ExecutorContext, stream *Stream, role, mediaType string, data []byte, now time.Time) (*a2a.TaskArtifactUpdateEvent, int64) {
	digest := CASRef(data)

	record := ArtifactRecord{
		Digest:         digest,
		MediaType:      mediaType,
		Size:           int64(len(data)),
		Label:          "CONSORTIUM",
		Producer:       p.org,
		AssignedBy:     p.org,
		AssignedAt:     now.UTC().Format(time.RFC3339),
		RetentionUntil: now.Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}
	grant := Grant{
		Typ:       GrantType,
		TaskID:    string(execCtx.TaskID),
		Recipient: p.consumer,
		Digests:   []string{digest},
		ExpiresAt: now.Add(45 * time.Minute).UTC().Format(time.RFC3339),
	}

	// The holder keeps the grant it issued, because §10.4 makes the holder the
	// enforcement point. The copy that travels with the artifact tells the
	// consumer what it may ask for; this copy decides what it gets.
	p.mu.Lock()
	p.blobs[digest] = data
	p.grants[digest] = grant
	p.mu.Unlock()

	// The artifact itself carries the §10 payload and names the namespace in
	// artifact.extensions[]; the event carries the §9.1 envelope.
	//
	// The single part is a URL into the holder's grant endpoint, not the bytes.
	// A2A leaves no choice about there being a part at all --
	// internal/taskupdate/manager.go rejects an artifact with none -- but a URL
	// part is a reference, so §9.3's "reference only" survives.
	ev := a2a.NewArtifactEvent(execCtx, a2a.NewFileURLPart(
		a2a.URL(p.grantEndpoint+"/"+strings.TrimPrefix(digest, casPrefix)), mediaType))
	ev.Artifact.Name = role
	ev.Artifact.Extensions = []string{ExtArtifacts}
	ev.Artifact.SetMeta(ExtArtifacts, mustMeta(ArtifactPayload{Record: record, Grant: grant}))
	ev.LastChunk = true
	ev.SetMeta(ExtArtifacts, mustMeta(stream.Next(EventArtifact, map[string]any{
		"artifact_ref": digest,
		"media_type":   mediaType,
		"digest":       digest,
		"label":        record.Label,
		"size":         record.Size,
	})))
	return ev, record.Size
}

// submitToDelegate is the hop: a real A2A message/send from node B to node C,
// carrying the attenuated child token.
func (p *producerAgent) submitToDelegate(ctx context.Context, child Token) (a2a.TaskID, Claim, error) {
	childMessage := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("Run the test suite against the patch"))
	childMessage.Extensions = ExtensionURIs(AllExtensions())

	childMeta, err := ToMetadata(child)
	if err != nil {
		return "", Claim{}, err
	}
	contractMeta, err := ToMetadata(p.contract.Ref())
	if err != nil {
		return "", Claim{}, err
	}
	budgetMeta, err := ToMetadata(BudgetReport{Allowance: child.Caveats.Budget, Remaining: child.Caveats.Budget})
	if err != nil {
		return "", Claim{}, err
	}
	constraintsMeta, err := ToMetadata(child.Caveats)
	if err != nil {
		return "", Claim{}, err
	}

	ctx = a2aclient.AttachServiceParams(ctx, a2aclient.ServiceParams{
		a2a.SvcParamExtensions: ExtensionURIs(AllExtensions()),
	})
	res, err := p.delegateClient.SendMessage(ctx, &a2a.SendMessageRequest{
		Message: childMessage,
		Metadata: map[string]any{
			ExtContract:    contractMeta,
			ExtAuthority:   childMeta,
			ExtBudget:      budgetMeta,
			ExtConstraints: constraintsMeta,
		},
	})
	if err != nil {
		return "", Claim{}, fmt.Errorf("delegating to %s: %w", p.delegate, err)
	}
	reply, ok := res.(*a2a.Message)
	if !ok {
		return "", Claim{}, fmt.Errorf("delegate answered with %T, want *a2a.Message", res)
	}
	var claim Claim
	if err := FromMetadata(reply.Metadata[ExtVerification], &claim); err != nil {
		return "", Claim{}, fmt.Errorf("decoding the delegate's claim: %w", err)
	}
	return reply.TaskID, claim, nil
}

// settleReceipt handles the consumer's returned co-signed receipt: both
// signatures must verify, and the units must match what this node actually
// served.
func (p *producerAgent) settleReceipt(execCtx *a2asrv.ExecutorContext, payload any) (a2a.Event, error) {
	var receipt Receipt
	if err := FromMetadata(payload, &receipt); err != nil {
		return nil, err
	}
	if err := VerifyReceipt(p.orgKeys, receipt); err != nil {
		return nil, consignRefusal(a2a.ErrInvalidParams, "receipt not accepted", CodeSchemaInvalid)
	}
	p.mu.Lock()
	served := p.served
	p.mu.Unlock()
	if receipt.Units.ArtifactBytesServed != served {
		return nil, consignRefusal(a2a.ErrInvalidParams, "receipt units not accepted", CodeSchemaInvalid)
	}

	reply := a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("receipt settled"))
	reply.Extensions = []string{ExtReceipts}
	reply.SetMeta(ExtReceipts, mustMeta(receipt))
	return reply, nil
}

// delegateAgent is node C: it verifies the child token it was handed and
// answers with one claim. It records what it received so the delegation hop can
// be asserted from the delegate's own side.
type delegateAgent struct {
	org  string
	keys Keyring

	mu     sync.Mutex
	taskID a2a.TaskID
	token  Token
}

var _ a2asrv.AgentExecutor = (*delegateAgent)(nil)

// receivedTask is what node C saw.
type receivedTask struct {
	TaskID a2a.TaskID
	Token  Token
}

// Received returns what node C recorded, failing if it was never called.
func (d *delegateAgent) Received(t *testing.T) receivedTask {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	return receivedTask{TaskID: d.taskID, Token: d.token}
}

func (d *delegateAgent) Execute(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		var token Token
		if err := FromMetadata(execCtx.Metadata[ExtAuthority], &token); err != nil {
			yield(nil, err)
			return
		}
		if err := VerifyChain(d.keys, d.org, token, time.Now()); err != nil {
			yield(nil, consignRefusal(a2a.ErrUnauthorized, "capability token not accepted", CodeTokenInvalid))
			return
		}

		d.mu.Lock()
		d.taskID, d.token = execCtx.TaskID, token
		d.mu.Unlock()

		reply := a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("suite passed"))
		reply.Extensions = ExtensionURIs(AllExtensions())
		reply.SetMeta(ExtVerification, mustMeta(Claim{
			ID:                "c2",
			Statement:         "The test suite passes against the patch",
			Evidence:          []string{},
			ProducerValidated: true,
		}))
		yield(reply, nil)
	}
}

func (d *delegateAgent) Cancel(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

// elapsedSeconds is the whole-second gap between two §9.1 envelopes. Both
// parties compute lease_seconds this way, from the same two emitted_at values
// on the same stream, so agreement is a property of the events rather than of
// either party's clock.
func elapsedSeconds(from, to EventEnvelope) int {
	start, err := time.Parse(time.RFC3339Nano, from.EmittedAt)
	if err != nil {
		return 0
	}
	end, err := time.Parse(time.RFC3339Nano, to.EmittedAt)
	if err != nil {
		return 0
	}
	return int(end.Sub(start).Seconds())
}

// mustMeta converts a Consign payload for carriage in A2A metadata. It panics
// rather than returning an error because every call site passes a value built
// from Consign's own types, which cannot fail to marshal; a2asrv's panic
// handler would surface it as a failed task if that ever stopped being true.
func mustMeta(v any) map[string]any {
	m, err := ToMetadata(v)
	if err != nil {
		panic(err)
	}
	return m
}
