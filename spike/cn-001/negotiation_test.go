package smoke

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
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

// TestNegotiationRoundTrip is verification 1: all seven Consign namespaces are
// declared in the published Agent Card, the client requests them in the
// A2A-Extensions request service parameter, and the server reports the
// activated URIs back in the A2A-Extensions response service parameter.
//
// Every assertion is on bytes observed on the wire: the card is re-fetched over
// HTTP and the response header comes from the client's own RoundTripper, never
// from the request value the test supplied.
func TestNegotiationRoundTrip(t *testing.T) {
	want := ExtensionURIs(AllExtensions())

	for _, name := range []string{"node-a", "node-b"} {
		t.Run(name, func(t *testing.T) {
			n := startNode(t, name, AllExtensions())

			// Declared: the published card carries all seven URIs.
			card := fetchCard(t, n)
			got := ExtensionURIs(card.Capabilities.Extensions)
			if !slices.Equal(got, want) {
				t.Fatalf("published card extensions = %v, want %v", got, want)
			}

			// Negotiated: the client requests them and the server echoes back
			// the ones it activated.
			client, rec := newClient(t, n.Card, a2aext.NewActivator(want...))
			if _, err := client.SendMessage(t.Context(), &a2a.SendMessageRequest{
				Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
			}); err != nil {
				t.Fatalf("SendMessage() failed: %v", err)
			}

			activated := rec.Last().Values(a2a.SvcParamExtensions)
			t.Logf("observed %s response header = %v", a2a.SvcParamExtensions, activated)
			slices.Sort(activated)
			wantSorted := slices.Clone(want)
			slices.Sort(wantSorted)
			if !slices.Equal(activated, wantSorted) {
				t.Fatalf("observed %s response header = %v, want %v",
					a2a.SvcParamExtensions, activated, wantSorted)
			}
		})
	}
}
