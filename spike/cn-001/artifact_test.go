package smoke

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestContentAddressing covers §10.1: an artifact is identified by the digest
// of its bytes, the consumer verifies that digest after retrieval, and a
// mismatch fails closed.
func TestContentAddressing(t *testing.T) {
	patch := []byte("--- a/auth.go\n+++ b/auth.go\n@@ rotate session id on login\n")

	ref := CASRef(patch)
	if !strings.HasPrefix(ref, "cas://sha256/") {
		t.Fatalf("CASRef() = %q, want a cas://sha256/ address", ref)
	}
	if got := CASRef(patch); got != ref {
		t.Fatalf("CASRef() is not deterministic: %q then %q", ref, got)
	}

	if err := VerifyCAS(ref, patch); err != nil {
		t.Fatalf("VerifyCAS() on the bytes the address was taken over = %v, want nil", err)
	}

	// One flipped byte, same length: the check must be over content, not size.
	tampered := append([]byte(nil), patch...)
	tampered[10] ^= 0x01
	err := VerifyCAS(ref, tampered)
	if !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("VerifyCAS() on tampered bytes = %v, want %v", err, ErrDigestMismatch)
	}
	t.Logf("rejected: %v", err)

	if err := VerifyCAS("sha256:"+ref, patch); err == nil {
		t.Fatalf("VerifyCAS() on a malformed address = nil, want an error")
	}
}

// TestArtifactLabelBoundsReference covers §10.3's last rule: an artifact must
// not be referenced by a task whose data_class caveat is below the artifact's
// label. The ordering is §7.2.1's, so the same rank table governs both.
func TestArtifactLabelBoundsReference(t *testing.T) {
	tests := []struct {
		taskClass string
		label     string
		wantErr   error
	}{
		{"RESTRICTED", "CONSORTIUM", nil},
		{"CONSORTIUM", "CONSORTIUM", nil},
		{"CONSORTIUM", "RESTRICTED", ErrLabelAboveTask},
		{"PUBLIC", "CONSORTIUM", ErrLabelAboveTask},
	}
	for _, tc := range tests {
		t.Run(tc.taskClass+"_task_"+tc.label+"_artifact", func(t *testing.T) {
			err := CheckLabel(tc.taskClass, tc.label)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CheckLabel(%q, %q) = %v, want %v", tc.taskClass, tc.label, err, tc.wantErr)
			}
		})
	}
}

// TestGrantScope covers §10.4: a grant authorises one recipient to retrieve one
// set of digests for one task, for a bounded period.
func TestGrantScope(t *testing.T) {
	now := time.Now()
	patch := CASRef([]byte("patch bytes"))
	grant := Grant{
		Typ:       GrantType,
		TaskID:    "task_019fe2",
		Recipient: orgC,
		Digests:   []string{patch},
		ExpiresAt: now.Add(30 * time.Minute).UTC().Format(time.RFC3339),
	}

	tests := []struct {
		name      string
		recipient string
		taskID    string
		digest    string
		at        time.Time
		wantErr   error
	}{
		{"the named recipient, task and digest", orgC, "task_019fe2", patch, now, nil},
		{"another organization", orgB, "task_019fe2", patch, now, ErrGrantInvalid},
		{"another task", orgC, "task_other", patch, now, ErrGrantInvalid},
		{"a digest not in the grant", orgC, "task_019fe2", CASRef([]byte("other bytes")), now, ErrGrantInvalid},
		{"after expiry", orgC, "task_019fe2", patch, now.Add(31 * time.Minute), ErrGrantInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := grant.Check(tc.recipient, tc.taskID, tc.digest, tc.at)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Grant.Check(%s, %s, %s) = %v, want %v",
					tc.recipient, tc.taskID, tc.digest, err, tc.wantErr)
			}
		})
	}
}
