package smoke

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aext"
)

// constraintsPayload is the `constraints` namespace payload (§7.2). The two
// x_-prefixed members are fields this spike does not define anywhere: they exist
// only to prove that unknown optional fields survive the round trip, which §2.3
// and NFR-011 require.
const constraintsPayload = `{
  "schema": "consign.constraints/v1",
  "data_class": "CONSORTIUM",
  "allowed_organizations": ["did:web:lab-a.example", "did:web:company-c.example"],
  "allowed_tools": ["git", "pytest"],
  "network_access": "deny",
  "side_effects": "propose-only",
  "egress_scan": {
    "verdict": "clean",
    "x_unmodelled_scanner_detail": {"engine": "stub", "rules": 3}
  },
  "x_future_constraint_caveat": ["not-a-field-this-spike-defines", 42, null]
}`

// canonicalJSON renders a decoded JSON value back to bytes. encoding/json sorts
// map keys, so two values that decode to the same structure always render to the
// same bytes, which makes byte comparison a meaningful fidelity test.
func canonicalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling %T: %v", v, err)
	}
	return b
}

// decodeJSON decodes a JSON document into the generic representation A2A
// metadata uses.
func decodeJSON(t *testing.T, doc []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		t.Fatalf("decoding payload: %v", err)
	}
	return m
}

// authorityPayload renders a signed capability token as an `authority` namespace
// payload, then adds two fields the Token type does not model, one at the top
// level and one nested inside caveats.
func authorityPayload(t *testing.T, tok Token) map[string]any {
	t.Helper()
	m := decodeJSON(t, canonicalJSON(t, tok))
	m["x_future_authority_field"] = map[string]any{"minted_by": "cn-001"}
	caveats, ok := m["caveats"].(map[string]any)
	if !ok {
		t.Fatalf("token caveats decoded as %T, want map", m["caveats"])
	}
	caveats["x_unmodelled_caveat"] = "survives-or-the-gate-fails"
	return m
}

// TestMetadataFidelity is verification 2: the `constraints` and `authority`
// payloads survive submit and reply byte-identically, unknown optional fields
// included. The payload crosses two JSON encode/decode boundaries (client to
// server, server to client), so equality here is a property of the wire format
// and not of a shared in-memory value.
func TestMetadataFidelity(t *testing.T) {
	now := time.Now()
	root, child, _, _ := testChain(t, now)

	cases := []struct {
		node  string
		token Token
	}{
		{"node-a-root-token", root},
		{"node-b-attenuated-child", child},
	}

	for _, tc := range cases {
		t.Run(tc.node, func(t *testing.T) {
			sent := map[string]any{
				ExtConstraints: decodeJSON(t, []byte(constraintsPayload)),
				ExtAuthority:   authorityPayload(t, tc.token),
			}

			n := startNode(t, tc.node, AllExtensions())
			client, _ := newClient(t, n.Card, a2aext.NewActivator(ExtensionURIs(AllExtensions())...))

			result, err := client.SendMessage(t.Context(), &a2a.SendMessageRequest{
				Message:  a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
				Metadata: sent,
			})
			if err != nil {
				t.Fatalf("SendMessage() failed: %v", err)
			}
			reply, ok := result.(*a2a.Message)
			if !ok {
				t.Fatalf("SendMessage() returned %T, want *a2a.Message", result)
			}

			for _, uri := range []string{ExtConstraints, ExtAuthority} {
				got, ok := reply.Metadata[uri]
				if !ok {
					t.Fatalf("reply metadata has no %s entry; keys present: %v", uri, keysOf(reply.Metadata))
				}
				want := canonicalJSON(t, sent[uri])
				if have := canonicalJSON(t, got); !bytes.Equal(want, have) {
					t.Fatalf("%s payload changed in transit\n sent: %s\n back: %s", uri, want, have)
				}
			}

			// Byte equality already covers this, but name the requirement
			// explicitly so a regression reports the right thing.
			assertUnknownFieldPresent(t, reply.Metadata, ExtConstraints, "x_future_constraint_caveat")
			assertUnknownFieldPresent(t, reply.Metadata, ExtAuthority, "x_future_authority_field")
		})
	}
}

func assertUnknownFieldPresent(t *testing.T, metadata map[string]any, uri, field string) {
	t.Helper()
	payload, ok := metadata[uri].(map[string]any)
	if !ok {
		t.Fatalf("%s payload decoded as %T, want map", uri, metadata[uri])
	}
	if _, ok := payload[field]; !ok {
		t.Fatalf("unknown optional field %q was stripped from %s (§2.3, NFR-011)", field, uri)
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
