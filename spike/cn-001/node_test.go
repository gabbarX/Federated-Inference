package smoke

import (
	"context"
	"iter"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// wellKnownCardPath is where A2A v1.0 publishes the public Agent Card.
const wellKnownCardPath = "/.well-known/agent-card.json"

// jsonValue renders a typed Consign payload into the JSON-generic form A2A
// metadata accepts. See ToMetadata for why the conversion is mandatory.
func jsonValue(t *testing.T, v any) map[string]any {
	t.Helper()
	m, err := ToMetadata(v)
	if err != nil {
		t.Fatalf("converting %T to metadata: %v", v, err)
	}
	return m
}

// consignCard builds an Agent Card declaring the given Consign extensions.
func consignCard(name, url string, exts []a2a.AgentExtension) *a2a.AgentCard {
	return &a2a.AgentCard{
		Name:        name,
		Description: "CN-001 spike node",
		Version:     "0.0.1",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(url, a2a.TransportProtocolJSONRPC),
		},
		// Streaming must be declared for the SSE path to be reachable at all:
		// a2aclient.SendStreamingMessage silently falls back to SendMessage when
		// the card says otherwise, and defaultRequestHandler refuses the call.
		Capabilities:       a2a.AgentCapabilities{Extensions: exts, Streaming: true},
		DefaultInputModes:  []string{"application/json"},
		DefaultOutputModes: []string{"application/json"},
		Skills:             []a2a.AgentSkill{},
	}
}

// echoExecutor is the agent behind each spike node. It replies with a single
// agent Message and copies the Consign extension metadata it received back
// into that Message's metadata, so a client can observe round-trip fidelity.
type echoExecutor struct{}

var _ a2asrv.AgentExecutor = echoExecutor{}

func (echoExecutor) Execute(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		reply := a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("ack"))
		// Copy each Consign namespace's payload straight back, untouched. The
		// value is whatever encoding/json decoded from the request, so nothing
		// this node does not model can be dropped along the way.
		for _, uri := range ExtensionURIs(AllExtensions()) {
			if payload, ok := execCtx.Metadata[uri]; ok {
				reply.SetMeta(uri, payload)
			}
		}
		yield(reply, nil)
	}
}

func (echoExecutor) Cancel(_ context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

// node is one in-process a2asrv server on loopback HTTP.
type node struct {
	Card *a2a.AgentCard
	URL  string
}

// startNode brings up an a2asrv JSON-RPC server plus its public Agent Card on
// loopback HTTP, declaring exts in capabilities.extensions[], behind the echo
// executor.
func startNode(t *testing.T, name string, exts []a2a.AgentExtension) *node {
	t.Helper()
	return startNodeWith(t, name, exts, echoExecutor{})
}

// startNodeWith is startNode with a caller-supplied agent and any extra call
// interceptors, which is what the three-node delegation chain needs.
func startNodeWith(t *testing.T, name string, exts []a2a.AgentExtension, exec a2asrv.AgentExecutor, extra ...a2asrv.CallInterceptor) *node {
	t.Helper()
	return startNodeCustom(t, name, exts, exec, true, extra...)
}

// startNodeCustom additionally chooses whether the node wires up the streaming
// half of the response echo. Only the test that characterizes what the
// interceptor alone can do passes false; every other node passes true, because
// without it a Consign node reports no activated extensions at all on
// message/stream.
func startNodeCustom(t *testing.T, name string, exts []a2a.AgentExtension, exec a2asrv.AgentExecutor, streamingEcho bool, extra ...a2asrv.CallInterceptor) *node {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	url := "http://" + srv.Listener.Addr().String()
	card := consignCard(name, url, exts)

	interceptors := append([]a2asrv.CallInterceptor{&consignExtensions{declared: exts}}, extra...)
	handler := a2asrv.NewHandler(
		exec,
		a2asrv.WithCapabilityChecks(&card.Capabilities),
		a2asrv.WithCallInterceptors(interceptors...),
	)

	var jsonrpc http.Handler = withResponseServiceParams(a2asrv.NewJSONRPCHandler(handler))
	if streamingEcho {
		jsonrpc = withStreamingServiceParams(exts, jsonrpc)
	}
	mux := http.NewServeMux()
	mux.Handle(wellKnownCardPath, a2asrv.NewStaticAgentCardHandler(card))
	mux.Handle("/", jsonrpc)

	srv.Config.Handler = mux
	srv.Start()
	t.Cleanup(srv.Close)

	return &node{Card: card, URL: url}
}

// headerRecorder is an http.RoundTripper that keeps the response headers of the
// most recent exchange. It is the only honest way to observe A2A response
// service parameters on the JSON-RPC binding (see the "SDK API observed" notes:
// a2aclient's JSON-RPC transport discards http.Response.Header).
type headerRecorder struct {
	inner http.RoundTripper

	mu   sync.Mutex
	last http.Header
}

func (h *headerRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := h.inner.RoundTrip(req)
	if resp != nil {
		h.mu.Lock()
		h.last = resp.Header.Clone()
		h.mu.Unlock()
	}
	return resp, err
}

// Last returns the response headers observed on the wire for the most recent call.
func (h *headerRecorder) Last() http.Header {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.last
}

// newClient builds an a2aclient bound to card over the JSON-RPC binding, with a
// header recorder wrapped around its HTTP transport.
func newClient(t *testing.T, card *a2a.AgentCard, interceptors ...a2aclient.CallInterceptor) (*a2aclient.Client, *headerRecorder) {
	t.Helper()
	rec := &headerRecorder{inner: http.DefaultTransport}
	client, err := a2aclient.NewFromCard(
		context.Background(),
		card,
		a2aclient.WithJSONRPCTransport(&http.Client{Transport: rec}),
		a2aclient.WithCallInterceptors(interceptors...),
	)
	if err != nil {
		t.Fatalf("a2aclient.NewFromCard() failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Destroy() })
	return client, rec
}
