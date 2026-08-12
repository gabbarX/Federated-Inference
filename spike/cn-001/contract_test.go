package smoke

import (
	"errors"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// codeReviewContract is the §6.3 contract document both nodes hold a copy of.
func codeReviewContract(t *testing.T) Contract {
	t.Helper()
	c := Contract{
		Typ:           ContractType,
		ID:            "code-review/v1",
		Verifiability: "DETERMINISTIC",
		InputSchema:   "https://consign.example/contracts/code-review/v1/input.json",
		OutputSchema:  "https://consign.example/contracts/code-review/v1/output.json",
		Validators: []Validator{
			{Name: "apply_patch", Kind: "process", MustPass: true},
			{Name: "run_tests", Kind: "process", MustPass: true},
			{Name: "static_analysis", Kind: "process", MustPass: false},
		},
		RequiredArtifacts: []string{"patch", "test_report"},
	}
	sealed, err := c.Sealed()
	if err != nil {
		t.Fatalf("sealing contract: %v", err)
	}
	return sealed
}

// TestContractDigestIdentifiesTheDocument covers §6.3's digest rule: the digest
// covers the whole document, so two nodes that agree on `code-review/v1` but
// disagree on any of the four things §6.1 binds produce different digests.
func TestContractDigestIdentifiesTheDocument(t *testing.T) {
	base := codeReviewContract(t)

	if !strings.HasPrefix(base.Digest, "sha256:") {
		t.Fatalf("contract digest = %q, want a sha256: prefix", base.Digest)
	}

	// Sealing is idempotent: a document that already carries its digest still
	// digests to the same value, so `digest` cannot be part of what is hashed.
	resealed, err := base.Sealed()
	if err != nil {
		t.Fatalf("re-sealing: %v", err)
	}
	if resealed.Digest != base.Digest {
		t.Fatalf("re-sealing changed the digest: %s then %s", base.Digest, resealed.Digest)
	}

	// One case per thing §6.1 binds to the identifier, plus the identifier itself.
	tests := []struct {
		name  string
		edit  func(*Contract)
		fails bool
	}{
		{"identifier", func(c *Contract) { c.ID = "code-review/v2" }, true},
		{"input schema", func(c *Contract) { c.InputSchema = "https://consign.example/other.json" }, true},
		{"output schema", func(c *Contract) { c.OutputSchema = "https://consign.example/other.json" }, true},
		{"verifiability class", func(c *Contract) { c.Verifiability = "UNVERIFIED" }, true},
		{"validator set", func(c *Contract) { c.Validators = c.Validators[:2] }, true},
		{"a validator's must_pass", func(c *Contract) { c.Validators[2].MustPass = true }, true},
		// The discriminating case: a change the digest is *not* meant to notice
		// would make every row above vacuous if the digest simply changed on
		// every call. There is no such field in §6.3, so this row edits nothing
		// and asserts the digest is stable.
		{"nothing at all", func(c *Contract) {}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			edited := base
			edited.Validators = append([]Validator(nil), base.Validators...)
			tc.edit(&edited)

			resealed, err := edited.Sealed()
			if err != nil {
				t.Fatalf("sealing edited contract: %v", err)
			}
			if changed := resealed.Digest != base.Digest; changed != tc.fails {
				t.Fatalf("editing %s changed the digest = %v, want %v (%s vs %s)",
					tc.name, changed, tc.fails, base.Digest, resealed.Digest)
			}
		})
	}
}

// TestContractRefCheck covers the producer-side half of §6.3 offline: a
// reference is accepted only when this node holds a contract with that
// identifier *and* the same digest, and the two failure modes are
// distinguishable (§15 gives them separate codes).
func TestContractRefCheck(t *testing.T) {
	local := codeReviewContract(t)
	other := local
	other.Validators = append([]Validator(nil), local.Validators...)
	other.Validators[2].MustPass = true
	other, err := other.Sealed()
	if err != nil {
		t.Fatalf("sealing the divergent copy: %v", err)
	}

	tests := []struct {
		name    string
		ref     ContractRef
		wantErr error
	}{
		{"same id and digest", ContractRef{ID: local.ID, Digest: local.Digest}, nil},
		{"same id, other node's digest", ContractRef{ID: local.ID, Digest: other.Digest}, ErrContractMismatch},
		{"unknown id", ContractRef{ID: "deploy/v1", Digest: local.Digest}, ErrContractUnsupported},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckContractRef([]Contract{local}, tc.ref)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CheckContractRef(%+v) = %v, want %v", tc.ref, err, tc.wantErr)
			}
		})
	}
}

// TestContractMismatchRejectedOnTheWire is demonstration 4: a task whose
// contract digest differs from the producer's copy is refused, before
// acceptance, over a real A2A call.
//
// Per resolution R2 no A2A error code is invented. The assertions below record
// the A2A surface that actually appears: the JSON-RPC code, the a2a-go
// sentinel it maps back to, and the Consign §15 code carried in the error's
// structured ErrorInfo metadata -- which is the only place a machine-readable
// Consign code can live, because A2A's error message field is A2A's own.
func TestContractMismatchRejectedOnTheWire(t *testing.T) {
	local := codeReviewContract(t)
	divergent := local
	divergent.Validators = append([]Validator(nil), local.Validators...)
	divergent.Validators[2].MustPass = true
	divergent, err := divergent.Sealed()
	if err != nil {
		t.Fatalf("sealing the consumer's divergent copy: %v", err)
	}

	n := startNodeWith(t, "producer", AllExtensions(), echoExecutor{}, &consignAcceptance{contracts: []Contract{local}})

	submit := func(t *testing.T, ref ContractRef) error {
		t.Helper()
		client, _ := newClient(t, n.Card)
		ctx := a2aclient.AttachServiceParams(t.Context(), a2aclient.ServiceParams{
			a2a.SvcParamExtensions: ExtensionURIs(AllExtensions()),
		})
		_, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
			Message:  a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
			Metadata: map[string]any{ExtContract: jsonValue(t, ref)},
		})
		return err
	}

	// Discriminating case: the same node, the same code path, accepts the
	// matching digest. Without it, a node that refused everything would pass.
	t.Run("matching digest is accepted", func(t *testing.T) {
		if err := submit(t, ContractRef{ID: local.ID, Digest: local.Digest}); err != nil {
			t.Fatalf("SendMessage() with a matching contract digest = %v, want nil", err)
		}
	})

	t.Run("differing digest is refused", func(t *testing.T) {
		err := submit(t, ContractRef{ID: local.ID, Digest: divergent.Digest})
		if err == nil {
			t.Fatalf("SendMessage() with a differing contract digest = nil, want refusal")
		}
		if !errors.Is(err, a2a.ErrInvalidParams) {
			t.Fatalf("client-visible error = %v, want it to wrap %v", err, a2a.ErrInvalidParams)
		}
		t.Logf("client-visible error = %v (%T)", err, err)

		if got := consignCodeOf(t, err); got != CodeContractMismatch {
			t.Fatalf("Consign code carried in ErrorInfo metadata = %q, want %q", got, CodeContractMismatch)
		}

		// §8.5: the refusal must not say which digest, contract or copy differed.
		if strings.Contains(err.Error(), local.Digest) || strings.Contains(err.Error(), divergent.Digest) {
			t.Fatalf("refusal message discloses a digest, which §8.5 forbids: %q", err.Error())
		}

		code, message := rawSendMessageError(t, n, ExtensionURIs(AllExtensions()),
			map[string]any{ExtContract: jsonValue(t, ContractRef{ID: local.ID, Digest: divergent.Digest})})
		t.Logf("JSON-RPC error on the wire: code=%d message=%q", code, message)
		if code != -32602 {
			t.Fatalf("JSON-RPC error code = %d, want -32602 (ErrInvalidParams)", code)
		}
	})

	// The third acceptance-time outcome, kept distinct from the digest rule so
	// that "refused" never stands in for "refused for the right reason".
	t.Run("malformed contract reference is refused as E_SCHEMA_INVALID", func(t *testing.T) {
		client, _ := newClient(t, n.Card)
		ctx := a2aclient.AttachServiceParams(t.Context(), a2aclient.ServiceParams{
			a2a.SvcParamExtensions: ExtensionURIs(AllExtensions()),
		})
		_, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
			Message:  a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
			Metadata: map[string]any{ExtContract: "code-review/v1"},
		})
		if err == nil {
			t.Fatalf("SendMessage() with a non-object contract namespace = nil, want refusal")
		}
		if got := consignCodeOf(t, err); got != CodeSchemaInvalid {
			t.Fatalf("Consign code = %q, want %q", got, CodeSchemaInvalid)
		}
	})
}

// consignCodeOf digs the Consign §15 code out of the A2A error's ErrorInfo
// typed detail, which is where the producer put it and where a2aclient's
// JSON-RPC transport puts it back.
func consignCodeOf(t *testing.T, err error) string {
	t.Helper()
	var a2aErr *a2a.Error
	if !errors.As(err, &a2aErr) {
		t.Fatalf("error %v (%T) is not an *a2a.Error, so it carries no structured details", err, err)
	}
	meta, ok := a2aErr.ErrorInfo().Value["metadata"].(map[string]string)
	if !ok {
		t.Fatalf("ErrorInfo metadata is %T, want map[string]string; whole detail = %v",
			a2aErr.ErrorInfo().Value["metadata"], a2aErr.ErrorInfo().Value)
	}
	t.Logf("ErrorInfo detail = %v", a2aErr.ErrorInfo().Value)
	return meta["consign_code"]
}
