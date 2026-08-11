package smoke

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aext"
)

// fetchCard reads a node's published public Agent Card over HTTP.
func fetchCard(t *testing.T, n *node) *a2a.AgentCard {
	t.Helper()
	resp, err := http.Get(n.URL + wellKnownCardPath)
	if err != nil {
		t.Fatalf("GET %s failed: %v", wellKnownCardPath, err)
	}
	defer resp.Body.Close()
	var card a2a.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("decoding agent card failed: %v", err)
	}
	return &card
}

// observedActivatedURIs returns the A2A-Extensions values from the response
// headers of the most recent call, sorted for comparison.
func observedActivatedURIs(t *testing.T, rec *headerRecorder) []string {
	t.Helper()
	got := slices.Clone(rec.Last().Values(a2a.SvcParamExtensions))
	t.Logf("observed %s response header = %v", a2a.SvcParamExtensions, got)
	slices.Sort(got)
	return got
}

func sorted(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}

// TestNegotiationRoundTrip is verification 1: the Consign namespaces are
// declared in the published Agent Card, the client requests them in the
// A2A-Extensions request service parameter, and the activated URIs come back in
// the A2A-Extensions response service parameter.
//
// Who writes that response header matters, and it is not the SDK. a2a-go
// v2.4.0's JSON-RPC binding never turns CallContext.Extensions().ActivatedURIs()
// into a response service parameter -- only its gRPC binding does, and
// a2asrv.Extensions.Activate's own doc comment only promises that a transport
// "might" attach the list. The header asserted below is written by this spike's
// consignExtensions.After in activation_test.go, which is the profile-side glue
// filling that gap. What the test establishes is that A2A permits the echo and
// that the URIs survive the round trip, not that a2a-go performs it.
//
// Every assertion is on bytes observed on the wire: the card is re-fetched over
// HTTP and the response header comes from the client's own RoundTripper, never
// from the request value the test supplied.
func TestNegotiationRoundTrip(t *testing.T) {
	all := ExtensionURIs(AllExtensions())

	for _, name := range []string{"node-a", "node-b"} {
		t.Run(name, func(t *testing.T) {
			n := startNode(t, name, AllExtensions())

			// Declared: the published card carries all seven URIs.
			card := fetchCard(t, n)
			if got := ExtensionURIs(card.Capabilities.Extensions); !slices.Equal(got, all) {
				t.Fatalf("published card extensions = %v, want %v", got, all)
			}

			// Negotiated: the client requests them and the activated set comes back.
			client, rec := newClient(t, n.Card, a2aext.NewActivator(all...))
			if _, err := client.SendMessage(t.Context(), &a2a.SendMessageRequest{
				Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
			}); err != nil {
				t.Fatalf("SendMessage() failed: %v", err)
			}

			if activated := observedActivatedURIs(t, rec); !slices.Equal(activated, sorted(all)) {
				t.Fatalf("observed %s response header = %v, want %v",
					a2a.SvcParamExtensions, activated, sorted(all))
			}
		})
	}

	// The subtests above cannot distinguish "activated" from "echoed": requested,
	// declared and asserted are all the same seven URIs, so glue that replayed the
	// request header verbatim would pass them. This subtest discriminates. The node
	// declares six, the client asks for all seven, and the response must carry the
	// six the node actually activated -- so a verbatim replay fails here.
	t.Run("undeclared extension is not reported as activated", func(t *testing.T) {
		// receipts is the one namespace §2.2 marks optional, so omitting it does
		// not trip the §2.3 guard and the task is served rather than refused.
		declared := Without(ExtReceipts)
		n := startNode(t, "node-without-receipts", declared)

		// Requested explicitly rather than through a2aext.NewActivator, which
		// strips URIs the card does not declare and would collapse requested
		// back down to declared -- destroying the discrimination.
		client, rec := newClient(t, n.Card)
		ctx := a2aclient.AttachServiceParams(t.Context(), a2aclient.ServiceParams{
			a2a.SvcParamExtensions: all,
		})
		if _, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
			Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
		}); err != nil {
			t.Fatalf("SendMessage() failed: %v", err)
		}

		activated := observedActivatedURIs(t, rec)
		want := sorted(ExtensionURIs(declared))
		if len(all) != 7 || len(want) != 6 {
			t.Fatalf("fixture is wrong: requested %d URIs, declared %d, want 7 and 6", len(all), len(want))
		}
		if !slices.Equal(activated, want) {
			t.Fatalf("observed %s response header = %v, want the %d declared URIs %v",
				a2a.SvcParamExtensions, activated, len(want), want)
		}
		if slices.Contains(activated, ExtReceipts) {
			t.Fatalf("%s was requested but not declared, yet came back as activated", ExtReceipts)
		}
	})
}
