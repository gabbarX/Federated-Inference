package smoke

import (
	"errors"
	"testing"
)

// resultFixture builds a §11.2 result envelope over two artifacts, one of which
// is cited as evidence for the single claim, plus the bytes a consumer would
// have retrieved for each.
func resultFixture(t *testing.T) (Result, map[string][]byte) {
	t.Helper()
	patch := []byte("--- a/auth.go\n+++ b/auth.go\n@@ rotate session id on login\n")
	report := []byte(`{"tests": 41, "failed": 0}`)
	patchRef, reportRef := CASRef(patch), CASRef(report)

	res := Result{
		Schema:        ResultType,
		TaskID:        "task_019fe2",
		Contract:      ContractRef{ID: "code-review/v1", Digest: "sha256:placeholder"},
		Verifiability: "DETERMINISTIC",
		Output:        map[string]any{"summary": "session fixation fixed"},
		Artifacts: []ResultArtifact{
			{Role: "patch", Ref: patchRef, Label: "CONSORTIUM"},
			{Role: "test_report", Ref: reportRef, Label: "CONSORTIUM"},
		},
		Claims: []Claim{{
			ID:                "c1",
			Statement:         "Session fixation is fixed by rotating the session id on login",
			Evidence:          []string{reportRef},
			ProducerValidated: true,
		}},
		Provenance: Provenance{Chain: []string{orgA, orgB}, ToolsUsed: []string{"git", "pytest"}},
		Usage:      Usage{WallSeconds: 412, DeclaredModelCalls: 23, DeclaredToolCalls: 51},
	}
	return res, map[string][]byte{patchRef: patch, reportRef: report}
}

// TestConsumerSideAcceptance covers §11.3: acceptance is a consumer decision
// made on evidence the consumer produced. The producer's own `producer_validated`
// flag is informational and must not be able to carry a result through.
func TestConsumerSideAcceptance(t *testing.T) {
	t.Run("a result whose evidence checks out is accepted", func(t *testing.T) {
		res, retrieved := resultFixture(t)
		if err := AcceptResult(res, retrieved); err != nil {
			t.Fatalf("AcceptResult() on a sound result = %v, want nil", err)
		}
	})

	// The discriminating case, and the reason this test exists: every claim
	// still says producer_validated:true, so a consumer that trusted the
	// producer's report would accept. The bytes do not address to the ref the
	// result cites, so a consumer that checks for itself must refuse.
	t.Run("producer_validated does not survive a digest mismatch", func(t *testing.T) {
		res, retrieved := resultFixture(t)
		ref := res.Claims[0].Evidence[0]
		retrieved[ref] = append(retrieved[ref], byte(' '))

		err := AcceptResult(res, retrieved)
		if !errors.Is(err, ErrDigestMismatch) {
			t.Fatalf("AcceptResult() with tampered evidence = %v, want %v", err, ErrDigestMismatch)
		}
		if !res.Claims[0].ProducerValidated {
			t.Fatalf("fixture is wrong: the claim must still assert producer_validated")
		}
		t.Logf("rejected: %v", err)
	})

	t.Run("a claim citing evidence outside the result is refused", func(t *testing.T) {
		res, retrieved := resultFixture(t)
		res.Claims[0].Evidence = []string{CASRef([]byte("bytes nobody published"))}

		err := AcceptResult(res, retrieved)
		if !errors.Is(err, ErrClaimUnsupported) {
			t.Fatalf("AcceptResult() with unpublished evidence = %v, want %v", err, ErrClaimUnsupported)
		}
		t.Logf("rejected: %v", err)
	})

	t.Run("an artifact the consumer never retrieved is refused", func(t *testing.T) {
		res, retrieved := resultFixture(t)
		delete(retrieved, res.Artifacts[0].Ref)

		err := AcceptResult(res, retrieved)
		if !errors.Is(err, ErrArtifactUnavailable) {
			t.Fatalf("AcceptResult() with an unretrieved artifact = %v, want %v", err, ErrArtifactUnavailable)
		}
		t.Logf("rejected: %v", err)
	})

	// §6.2 and §11.6: an UNVERIFIED result must not be accepted without an
	// operator override, which this spike does not model, so it is refused.
	t.Run("an UNVERIFIED result is refused", func(t *testing.T) {
		res, retrieved := resultFixture(t)
		res.Verifiability = "UNVERIFIED"

		err := AcceptResult(res, retrieved)
		if !errors.Is(err, ErrUnverifiedResult) {
			t.Fatalf("AcceptResult() on an UNVERIFIED result = %v, want %v", err, ErrUnverifiedResult)
		}
		t.Logf("rejected: %v", err)
	})
}
