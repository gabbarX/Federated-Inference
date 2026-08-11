package smoke

import (
	"crypto/ed25519"
	"errors"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
	"time"
)

const (
	orgA = "did:web:lab-a.example"
	orgB = "did:web:lab-b.example"
	orgC = "did:web:company-c.example"

	kidA = orgA + "#org-2026-08"
	kidB = orgB + "#org-2026-08"
)

// testChain mints an originator root token (depth 1, profile RECURSIVE) and a
// child token that narrows every §7.4 caveat, returning both plus the pinned
// keyring needed to verify them.
func testChain(t *testing.T, now time.Time) (root, child Token, keys Keyring, delegateKey ed25519.PrivateKey) {
	t.Helper()

	pubA, privA, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating originator key: %v", err)
	}
	pubB, privB, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating delegate key: %v", err)
	}
	keys = Keyring{kidA: pubA, kidB: pubB}

	rfc := func(d time.Duration) string { return now.Add(d).UTC().Format(time.RFC3339) }

	root, err = Sign(Token{
		Typ:        TokenType,
		TokenID:    "tok_root",
		Originator: orgA,
		RootTaskID: "task_019fe2",
		IssuedAt:   rfc(0),
		ExpiresAt:  rfc(30 * time.Minute),
		Caveats: Caveats{
			Contract:             []string{"code-review/v1", "doc-review/v1"},
			DataClass:            "RESTRICTED",
			AllowedOrganizations: []string{orgA, orgB, orgC},
			AllowedTools:         []string{"git", "pytest", "ruff"},
			// network_access and side_effects are two-valued (§7.2.3), so a
			// single parent/child pair cannot both narrow them and then widen
			// past the parent. The root is minted already at its most
			// restrictive value and the child holds equal, which §7.4 permits;
			// the widening table then has somewhere to widen to.
			NetworkAccess:      "deny",
			SideEffects:        "propose-only",
			MaxDelegationDepth: 1,
			AllowedDelegates:   []string{orgB, orgC},
			Budget: Budget{
				Deadline:         rfc(20 * time.Minute),
				MaxModelCalls:    30,
				MaxToolCalls:     80,
				MaxArtifactBytes: 1073741824,
			},
		},
	}, kidA, privA)
	if err != nil {
		t.Fatalf("signing root token: %v", err)
	}

	child, err = Sign(Token{
		Typ:        TokenType,
		TokenID:    "tok_child",
		Originator: orgA,
		RootTaskID: "task_019fe2",
		IssuedAt:   rfc(0),
		ExpiresAt:  rfc(20 * time.Minute),
		Caveats: Caveats{
			Contract:             []string{"code-review/v1"},
			DataClass:            "CONSORTIUM",
			AllowedOrganizations: []string{orgA, orgC},
			AllowedTools:         []string{"git", "pytest"},
			NetworkAccess:        "deny",
			SideEffects:          "propose-only",
			MaxDelegationDepth:   0,
			AllowedDelegates:     []string{orgC},
			Budget: Budget{
				Deadline:         rfc(15 * time.Minute),
				MaxModelCalls:    10,
				MaxToolCalls:     20,
				MaxArtifactBytes: 1048576,
			},
		},
		Chain: []Token{root},
	}, kidB, privB)
	if err != nil {
		t.Fatalf("signing child token: %v", err)
	}
	return root, child, keys, privB
}

// TestNarrowingChildVerifiesOffline is the first half of verification 4: a child
// token that narrows per §7.4 verifies against the originator's pinned key with
// no I/O of any kind.
func TestNarrowingChildVerifiesOffline(t *testing.T) {
	now := time.Now()
	_, child, keys, _ := testChain(t, now)

	if err := VerifyChain(keys, child, now); err != nil {
		t.Fatalf("VerifyChain() on a correctly narrowed child = %v, want nil", err)
	}
}

// TestChainVerificationFailures covers the remaining §7.3 checks VerifyChain
// performs, so that no rejection path in token.go is left unexercised. In
// particular it pins down "verifies against the originator key": a token whose
// caveats were edited after signing, or whose root was signed by a key that is
// not the originator's, does not verify.
func TestChainVerificationFailures(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		mutate  func(t *testing.T, root, child Token, keys Keyring, delegate ed25519.PrivateKey) (Token, Keyring)
		wantErr error
	}{
		{
			name: "caveat edited after signing",
			mutate: func(_ *testing.T, _, child Token, keys Keyring, _ ed25519.PrivateKey) (Token, Keyring) {
				child.Caveats.DataClass = "PUBLIC"
				return child, keys
			},
			wantErr: ErrBadSignature,
		},
		{
			name: "signing key not pinned",
			mutate: func(_ *testing.T, _, child Token, keys Keyring, _ ed25519.PrivateKey) (Token, Keyring) {
				delete(keys, kidB)
				return child, keys
			},
			wantErr: ErrUnknownKey,
		},
		{
			name: "root not signed by the originator organization",
			mutate: func(t *testing.T, root, _ Token, keys Keyring, delegate ed25519.PrivateKey) (Token, Keyring) {
				// The delegate re-signs the root as if it had minted it.
				forged, err := Sign(root, kidB, delegate)
				if err != nil {
					t.Fatalf("forging root: %v", err)
				}
				return forged, keys
			},
			wantErr: ErrNotOriginator,
		},
		{
			name: "leaf has expired",
			mutate: func(_ *testing.T, _, child Token, keys Keyring, _ ed25519.PrivateKey) (Token, Keyring) {
				return child, keys
			},
			wantErr: ErrExpired,
		},
		{
			// §7.3 step 6 / §7.6: the receiver checks its own remaining depth,
			// which is what makes constraint laundering detectable by the party
			// that would be harmed by it.
			name: "remaining delegation depth is negative",
			mutate: func(t *testing.T, _, child Token, keys Keyring, delegate ed25519.PrivateKey) (Token, Keyring) {
				child.Caveats.MaxDelegationDepth = -1
				resigned, err := Sign(child, kidB, delegate)
				if err != nil {
					t.Fatalf("re-signing exhausted-depth child: %v", err)
				}
				return resigned, keys
			},
			wantErr: ErrDepthExhausted,
		},
		{
			name: "chain carries more than one link",
			mutate: func(t *testing.T, root, child Token, keys Keyring, delegate ed25519.PrivateKey) (Token, Keyring) {
				child.Chain = []Token{root, root}
				resigned, err := Sign(child, kidB, delegate)
				if err != nil {
					t.Fatalf("re-signing over-long chain: %v", err)
				}
				return resigned, keys
			},
			wantErr: ErrChainShape,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, child, keys, delegate := testChain(t, now)
			tok, keys := tc.mutate(t, root, child, keys, delegate)

			// The expiry case is the one that needs a clock past the token's
			// lifetime; every other case must fail at "now".
			at := now
			if tc.wantErr == ErrExpired {
				at = now.Add(2 * time.Hour)
			}

			err := VerifyChain(keys, tok, at)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyChain() = %v, want %v", err, tc.wantErr)
			}
			t.Logf("rejected: %v", err)
		})
	}
}

// TestWideningChildRejected is the second half of verification 4: every row of
// the §7.4 attenuation table is exercised by a child that widens exactly that
// caveat and is re-signed by the delegate, so the only possible ground for
// rejection is the attenuation rule itself.
func TestWideningChildRejected(t *testing.T) {
	now := time.Now()
	root, child, keys, delegateKey := testChain(t, now)

	parentExpiry, err := time.Parse(time.RFC3339, root.ExpiresAt)
	if err != nil {
		t.Fatalf("parsing root expiry: %v", err)
	}
	parentDeadline, err := time.Parse(time.RFC3339, root.Caveats.Budget.Deadline)
	if err != nil {
		t.Fatalf("parsing root deadline: %v", err)
	}

	tests := []struct {
		caveat string
		widen  func(*Token)
	}{
		{"contract", func(tk *Token) { tk.Caveats.Contract = append(tk.Caveats.Contract, "deploy/v1") }},
		{"data_class", func(tk *Token) { tk.Caveats.DataClass = "LOCAL-ONLY" }},
		{"allowed_organizations", func(tk *Token) {
			tk.Caveats.AllowedOrganizations = append(tk.Caveats.AllowedOrganizations, "did:web:outsider.example")
		}},
		{"allowed_delegates", func(tk *Token) {
			tk.Caveats.AllowedDelegates = append(tk.Caveats.AllowedDelegates, "did:web:outsider.example")
		}},
		{"allowed_tools", func(tk *Token) { tk.Caveats.AllowedTools = append(tk.Caveats.AllowedTools, "curl") }},
		{"network_access", func(tk *Token) { tk.Caveats.NetworkAccess = "allow" }},
		{"side_effects", func(tk *Token) { tk.Caveats.SideEffects = "scoped-execute" }},
		{"max_delegation_depth", func(tk *Token) { tk.Caveats.MaxDelegationDepth = 1 }},
		{"expires_at", func(tk *Token) {
			tk.ExpiresAt = parentExpiry.Add(time.Minute).UTC().Format(time.RFC3339)
		}},
		{"budget.max_model_calls", func(tk *Token) { tk.Caveats.Budget.MaxModelCalls = 31 }},
		{"budget.deadline", func(tk *Token) {
			tk.Caveats.Budget.Deadline = parentDeadline.Add(time.Minute).UTC().Format(time.RFC3339)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.caveat, func(t *testing.T) {
			widened := child
			widened.Caveats.Contract = slices.Clone(child.Caveats.Contract)
			widened.Caveats.AllowedOrganizations = slices.Clone(child.Caveats.AllowedOrganizations)
			widened.Caveats.AllowedDelegates = slices.Clone(child.Caveats.AllowedDelegates)
			widened.Caveats.AllowedTools = slices.Clone(child.Caveats.AllowedTools)
			tc.widen(&widened)

			signed, err := Sign(widened, kidB, delegateKey)
			if err != nil {
				t.Fatalf("re-signing widened child: %v", err)
			}
			if err := VerifyChain(keys, signed, now); err == nil {
				t.Fatalf("VerifyChain() on a child widening %s = nil, want rejection", tc.caveat)
			} else {
				t.Logf("rejected: %v", err)
			}
		})
	}
}

// TestChainVerificationCannotReachTheNetwork is the "zero network calls"
// property of verification 4, asserted structurally rather than behaviourally:
// token.go, which holds the whole verification path, is not allowed to import
// anything capable of I/O. None of the permitted imports transitively reach
// net, net/http or os, so the property holds for the dependency closure and not
// merely for the code paths this test happens to exercise.
func TestChainVerificationCannotReachTheNetwork(t *testing.T) {
	allowed := []string{
		"crypto/ed25519",
		"encoding/base64",
		"encoding/json",
		"errors",
		"fmt",
		"slices",
		"strings",
		"time",
	}

	f, err := parser.ParseFile(token.NewFileSet(), "token.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing token.go: %v", err)
	}
	if len(f.Imports) == 0 {
		t.Fatalf("token.go has no imports; the file is probably not the one under test")
	}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("unquoting import %s: %v", imp.Path.Value, err)
		}
		if !slices.Contains(allowed, path) {
			t.Fatalf("token.go imports %q, which is not on the no-I/O allowlist %v", path, allowed)
		}
	}
	t.Logf("token.go imports %d packages, all on the no-I/O allowlist", len(f.Imports))
}
