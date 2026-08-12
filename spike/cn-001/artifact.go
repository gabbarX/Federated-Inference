package smoke

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// This file holds §10: content addressing, the immutable label record, and the
// scope rules a grant has to satisfy.
//
// Deliberately not modelled: §10.2's HTTP range/resumption surface, §10.5
// encryption and §10.6 pinning are transport and storage concerns that say
// nothing about whether the extension is expressible in A2A. Grants carry no
// signature here for the same reason -- the Ed25519 machinery is already
// exercised on tokens and receipts, and repeating it on grants would add no
// evidence about A2A.

// GrantType is the grant's typ discriminator (§10.4).
const GrantType = "consign.grant/v1"

// CodeGrantInvalid is the §15 code a holder returns for an unauthorized
// retrieval, whichever of §10.4's conjuncts failed.
const CodeGrantInvalid = "E_GRANT_INVALID"

var (
	ErrDigestMismatch = errors.New("content did not match its address")
	ErrGrantInvalid   = errors.New("artifact grant missing, expired, or wrong recipient")
	ErrLabelAboveTask = errors.New("artifact label is above the task's data class")
)

const casPrefix = "cas://sha256/"

// CASRef returns the §10.1 content address of data.
func CASRef(data []byte) string {
	sum := sha256.Sum256(data)
	return casPrefix + hex.EncodeToString(sum[:])
}

// VerifyCAS applies §10.1's consumer obligation: verify the digest after
// retrieval and fail closed on mismatch. A mismatch is terminal, so this
// returns a sentinel a caller cannot mistake for a transient failure.
func VerifyCAS(ref string, data []byte) error {
	if !strings.HasPrefix(ref, casPrefix) {
		return fmt.Errorf("artifact reference %q is not a %s address", ref, casPrefix)
	}
	if got := CASRef(data); got != ref {
		return fmt.Errorf("%w: retrieved bytes address to %s, reference says %s", ErrDigestMismatch, got, ref)
	}
	return nil
}

// ArtifactRecord is the immutable metadata record §10.3 requires every artifact
// carry. Label assignment is an operator function: nothing here can change a
// label, because a relabelling is a new artifact with a new digest.
type ArtifactRecord struct {
	Digest         string `json:"digest"`
	MediaType      string `json:"media_type"`
	Size           int64  `json:"size"`
	Label          string `json:"label"`
	Producer       string `json:"producer"`
	AssignedBy     string `json:"assigned_by"`
	AssignedAt     string `json:"assigned_at"`
	RetentionUntil string `json:"retention_until"`
}

// CheckLabel applies §10.3's referencing rule: an artifact must not be
// referenced by a task whose data_class caveat is below its label. The ordering
// is §7.2.1's, shared with the token attenuation table.
func CheckLabel(taskDataClass, label string) error {
	task, ok := dataClassRank[taskDataClass]
	if !ok {
		return fmt.Errorf("unknown task data_class %q", taskDataClass)
	}
	art, ok := dataClassRank[label]
	if !ok {
		return fmt.Errorf("unknown artifact label %q", label)
	}
	if art > task {
		return fmt.Errorf("%w: label %q above task data_class %q", ErrLabelAboveTask, label, taskDataClass)
	}
	return nil
}

// ArtifactPayload is what the `artifacts` namespace carries on an A2A
// Artifact's metadata: the §10.3 label record and the §10.4 grant that lets the
// recipient fetch the bytes from the holder.
type ArtifactPayload struct {
	Record ArtifactRecord `json:"record"`
	Grant  Grant          `json:"grant"`
}

// Grant authorises one organization to retrieve one set of digests for one
// task, until it expires (§10.4).
type Grant struct {
	Typ       string   `json:"typ"`
	TaskID    string   `json:"task_id"`
	Recipient string   `json:"recipient"`
	Digests   []string `json:"digests"`
	ExpiresAt string   `json:"expires_at"`
}

// Check reports whether recipient may retrieve digest under this grant at time
// now. Every conjunct is one of §10.4's MUSTs; unauthorized retrieval is
// E_GRANT_INVALID whichever conjunct failed, so the caller cannot use the error
// to learn the grant's contents.
func (g Grant) Check(recipient, taskID, digest string, now time.Time) error {
	expiry, err := time.Parse(time.RFC3339, g.ExpiresAt)
	if err != nil {
		return fmt.Errorf("parsing grant expires_at %q: %w", g.ExpiresAt, err)
	}
	switch {
	case g.Recipient != recipient,
		g.TaskID != taskID,
		!slices.Contains(g.Digests, digest),
		now.After(expiry):
		return fmt.Errorf("%w: task %s", ErrGrantInvalid, taskID)
	}
	return nil
}
