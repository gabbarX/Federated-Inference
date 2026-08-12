package smoke

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// This file holds §12's receipt: the units both parties witnessed, and the
// two-signature rule that stops either of them writing the record alone.

// ReceiptType is the receipt's typ discriminator (§12.3).
const ReceiptType = "consign.receipt/v1"

// ErrReceiptUnsigned is §12.3's "valid only with both signatures" seen from the
// verifier's side. §12.3's remedy for disagreement is an unsigned receipt and a
// recorded divergence, never a receipt one party wrote alone.
var ErrReceiptUnsigned = errors.New("receipt is not signed by both parties")

// ErrUnitsDiverge is §12.3's other outcome: the parties do not agree on what
// happened, so there is no receipt to sign and the divergence is recorded
// instead.
var ErrUnitsDiverge = errors.New("proposed units diverge from what this party witnessed")

// CheckUnits is the check a party runs before counter-signing: every unit in
// the proposal must equal what this party witnessed for itself. It is what
// makes §12.1's "accounting without anyone being trusted to self-report" true,
// and it is why §12.2 admits only units both parties can witness -- a unit only
// one side can see could never pass this.
func CheckUnits(witnessed, proposed Units) error {
	fields := []struct {
		name                string
		witnessed, proposed int64
	}{
		{"lease_seconds", int64(witnessed.LeaseSeconds), int64(proposed.LeaseSeconds)},
		{"tasks_accepted", int64(witnessed.TasksAccepted), int64(proposed.TasksAccepted)},
		{"verified_completions", int64(witnessed.VerifiedCompletions), int64(proposed.VerifiedCompletions)},
		{"artifact_bytes_served", witnessed.ArtifactBytesServed, proposed.ArtifactBytesServed},
		{"refusals", int64(witnessed.Refusals), int64(proposed.Refusals)},
		{"failures", int64(witnessed.Failures), int64(proposed.Failures)},
	}
	for _, f := range fields {
		if f.witnessed != f.proposed {
			return fmt.Errorf("%w: %s proposed as %d, witnessed %d", ErrUnitsDiverge, f.name, f.proposed, f.witnessed)
		}
	}
	return nil
}

// Units are the §12.2 observable units. The type is the enforcement: there is
// no field here for model calls, token counts or GPU-hours, because §12.2 bars
// them and §5.5 puts them in declared provenance instead.
type Units struct {
	LeaseSeconds        int   `json:"lease_seconds"`
	TasksAccepted       int   `json:"tasks_accepted"`
	VerifiedCompletions int   `json:"verified_completions"`
	ArtifactBytesServed int64 `json:"artifact_bytes_served"`
	Refusals            int   `json:"refusals"`
	Failures            int   `json:"failures"`
}

// ReceiptSignature is one party's signature over the receipt (§12.3).
//
// §12.3 identifies the signer by organization and not by key, so a verifier has
// no way to tell which of an organization's keys signed. That is fine while an
// organization has one key and becomes a problem at rotation (§16.5); it is
// recorded rather than fixed here, because adding a kid would be inventing
// spec rather than testing it.
type ReceiptSignature struct {
	Org   string `json:"org"`
	Alg   string `json:"alg"`
	Value string `json:"value"`
}

// Receipt records what happened in units both parties independently observed
// (§12.3).
type Receipt struct {
	Typ               string             `json:"typ"`
	TaskID            string             `json:"task_id"`
	Consumer          string             `json:"consumer"`
	Producer          string             `json:"producer"`
	TerminalState     string             `json:"terminal_state"`
	Units             Units              `json:"units"`
	PrevReceiptDigest string             `json:"prev_receipt_digest,omitempty"`
	Signatures        []ReceiptSignature `json:"signatures,omitempty"`
}

// SigningBytes is what both parties sign: the receipt with the signature list
// cleared. Both signatures are therefore over identical bytes and neither
// depends on the other, which is what makes them independently generated and
// independently verifiable, in either order.
func (r Receipt) SigningBytes() ([]byte, error) {
	r.Signatures = nil
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("canonicalizing receipt for task %s: %w", r.TaskID, err)
	}
	return raw, nil
}

// SignedBy returns r with org's signature appended.
func (r Receipt) SignedBy(org string, priv ed25519.PrivateKey) (Receipt, error) {
	msg, err := r.SigningBytes()
	if err != nil {
		return Receipt{}, err
	}
	r.Signatures = append(slices.Clone(r.Signatures), ReceiptSignature{
		Org:   org,
		Alg:   "EdDSA",
		Value: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg)),
	})
	return r, nil
}

// VerifyReceipt applies §12.3: a receipt is valid only with both parties'
// signatures, and each must verify against that party's own organization key.
// orgKeys is keyed by organization DID rather than by kid, because that is all
// §12.3's signature object names.
func VerifyReceipt(orgKeys Keyring, r Receipt) error {
	msg, err := r.SigningBytes()
	if err != nil {
		return err
	}

	for _, party := range []string{r.Producer, r.Consumer} {
		i := slices.IndexFunc(r.Signatures, func(s ReceiptSignature) bool { return s.Org == party })
		if i < 0 {
			return fmt.Errorf("%w: no signature by %s on task %s", ErrReceiptUnsigned, party, r.TaskID)
		}
		pub, ok := orgKeys[party]
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownKey, party)
		}
		value, err := base64.StdEncoding.DecodeString(r.Signatures[i].Value)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrBadSignature, party, err)
		}
		if !ed25519.Verify(pub, msg, value) {
			return fmt.Errorf("%w: signature attributed to %s does not verify against %s's key", ErrBadSignature, party, party)
		}
	}
	return nil
}
