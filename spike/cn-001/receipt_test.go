package smoke

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// receiptFixture builds a receipt over units both parties witnessed, signed by
// the producer and then counter-signed by the consumer, plus the keys needed to
// verify each signature against its own organization.
func receiptFixture(t *testing.T) (Receipt, Keyring) {
	t.Helper()

	pubProducer, privProducer, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating producer key: %v", err)
	}
	pubConsumer, privConsumer, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating consumer key: %v", err)
	}
	keys := Keyring{orgB: pubProducer, orgA: pubConsumer}

	receipt := Receipt{
		Typ:           ReceiptType,
		TaskID:        "task_019fe2",
		Consumer:      orgA,
		Producer:      orgB,
		TerminalState: "COMPLETED",
		Units: Units{
			LeaseSeconds:        412,
			TasksAccepted:       1,
			VerifiedCompletions: 1,
			ArtifactBytesServed: 41890,
		},
	}

	// Producer first, then consumer: §12.3's order, and the only order that can
	// occur, since a consumer has nothing to counter-sign until the producer
	// has reported a terminal state.
	signed, err := receipt.SignedBy(orgB, privProducer)
	if err != nil {
		t.Fatalf("producer signing: %v", err)
	}
	cosigned, err := signed.SignedBy(orgA, privConsumer)
	if err != nil {
		t.Fatalf("consumer counter-signing: %v", err)
	}
	return cosigned, keys
}

// TestReceiptCoSignature is demonstration 5: the terminal receipt carries two
// independently generated, independently valid Ed25519 signatures, each
// verified against its own organization's key. A receipt valid with only one
// would violate §12.3.
func TestReceiptCoSignature(t *testing.T) {
	receipt, keys := receiptFixture(t)

	t.Run("both signatures verify, each against its own key", func(t *testing.T) {
		if err := VerifyReceipt(keys, receipt); err != nil {
			t.Fatalf("VerifyReceipt() on a co-signed receipt = %v, want nil", err)
		}
		if len(receipt.Signatures) != 2 {
			t.Fatalf("receipt carries %d signatures, want 2", len(receipt.Signatures))
		}

		// Verified here with raw crypto/ed25519 rather than through
		// VerifyReceipt, so the evidence for "independently valid" does not
		// depend on the function under test.
		signed, err := receipt.SigningBytes()
		if err != nil {
			t.Fatalf("canonicalizing receipt: %v", err)
		}
		orgs := []string{}
		for _, sig := range receipt.Signatures {
			orgs = append(orgs, sig.Org)
			value, err := base64.StdEncoding.DecodeString(sig.Value)
			if err != nil {
				t.Fatalf("decoding %s signature: %v", sig.Org, err)
			}
			if !ed25519.Verify(keys[sig.Org], signed, value) {
				t.Fatalf("%s signature does not verify against %s's own key", sig.Org, sig.Org)
			}
			t.Logf("%s signature verifies against %s's key", sig.Org, sig.Org)
		}
		if !slices.Contains(orgs, receipt.Consumer) || !slices.Contains(orgs, receipt.Producer) {
			t.Fatalf("signature organizations %v, want both %s and %s", orgs, receipt.Consumer, receipt.Producer)
		}
		if receipt.Signatures[0].Value == receipt.Signatures[1].Value {
			t.Fatalf("the two signatures are byte-identical, so they were not independently generated")
		}
	})

	t.Run("the producer's signature alone is not a receipt", func(t *testing.T) {
		half := receipt
		half.Signatures = receipt.Signatures[:1]
		err := VerifyReceipt(keys, half)
		if !errors.Is(err, ErrReceiptUnsigned) {
			t.Fatalf("VerifyReceipt() on a singly-signed receipt = %v, want %v", err, ErrReceiptUnsigned)
		}
		t.Logf("rejected: %v", err)
	})

	// The discriminating case for "independently valid". A verifier that only
	// counted signatures, or that checked every signature against one key,
	// would accept this: the producer has signed twice and the second signature
	// has simply been relabelled as the consumer's.
	t.Run("a signature relabelled as the other party is refused", func(t *testing.T) {
		forged := receipt
		forged.Signatures = slices.Clone(receipt.Signatures)
		producerSig := forged.Signatures[slices.IndexFunc(forged.Signatures,
			func(s ReceiptSignature) bool { return s.Org == orgB })]
		for i := range forged.Signatures {
			if forged.Signatures[i].Org == orgA {
				forged.Signatures[i].Value = producerSig.Value
			}
		}

		err := VerifyReceipt(keys, forged)
		if !errors.Is(err, ErrBadSignature) {
			t.Fatalf("VerifyReceipt() on a relabelled signature = %v, want %v", err, ErrBadSignature)
		}
		if !strings.Contains(err.Error(), orgA) {
			t.Fatalf("rejection does not name the organization whose signature failed: %v", err)
		}
		t.Logf("rejected: %v", err)
	})

	t.Run("units edited after co-signing invalidate both signatures", func(t *testing.T) {
		inflated := receipt
		inflated.Units.ArtifactBytesServed *= 10

		err := VerifyReceipt(keys, inflated)
		if !errors.Is(err, ErrBadSignature) {
			t.Fatalf("VerifyReceipt() on an inflated receipt = %v, want %v", err, ErrBadSignature)
		}
		t.Logf("rejected: %v", err)
	})

	t.Run("a third party's signature does not stand in for a party's", func(t *testing.T) {
		_, privOutsider, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("generating outsider key: %v", err)
		}
		outsider := receipt
		outsider.Signatures = receipt.Signatures[:1]
		outsider, err = outsider.SignedBy("did:web:outsider.example", privOutsider)
		if err != nil {
			t.Fatalf("outsider signing: %v", err)
		}

		err = VerifyReceipt(keys, outsider)
		if !errors.Is(err, ErrReceiptUnsigned) {
			t.Fatalf("VerifyReceipt() with an outsider's second signature = %v, want %v", err, ErrReceiptUnsigned)
		}
		t.Logf("rejected: %v", err)
	})

}

// TestReceiptCarriesObservableUnitsOnly is demonstration 6: §12.2 forbids
// model calls, token counts and GPU-hours in a receipt, because they are
// producer-internal and unverifiable.
//
// The scan is over the receipt's serialized form rather than its Go type, so a
// field smuggled in through an untyped map would be caught too.
func TestReceiptCarriesObservableUnitsOnly(t *testing.T) {
	receipt, _ := receiptFixture(t)

	forbidden := []string{"model", "token", "gpu"}
	found := forbiddenKeys(t, receipt, forbidden)
	if len(found) != 0 {
		t.Fatalf("receipt carries producer-internal keys %v, which §12.2 forbids", found)
	}

	// Every unit that *is* present must be one §12.2's table says both parties
	// can witness. Listing them positively catches an addition the forbidden
	// list does not happen to name.
	witnessable := []string{
		"lease_seconds", "tasks_accepted", "verified_completions",
		"artifact_bytes_served", "refusals", "failures",
	}
	var units map[string]any
	if err := FromMetadata(receipt.Units, &units); err != nil {
		t.Fatalf("decoding units: %v", err)
	}
	for k := range units {
		if !slices.Contains(witnessable, k) {
			t.Fatalf("receipt unit %q is not in §12.2's table of independently witnessable units", k)
		}
	}
	t.Logf("receipt units = %v", slices.Sorted(mapKeys(units)))

	// Discriminating case: the scan is only evidence if it can find something.
	// §12.2's closing sentence says model and token counts belong in declared
	// provenance instead, and the §11.2 result envelope is where they are -- so
	// the same scan over a result must find them.
	res, _ := resultFixture(t)
	if got := forbiddenKeys(t, res, forbidden); len(got) == 0 {
		t.Fatalf("the scan found nothing in a result envelope that declares model calls, so it proves nothing about the receipt")
	} else {
		t.Logf("same scan over a §11.2 result envelope finds %v, which is where §12.2 says they belong", got)
	}
}

// forbiddenKeys walks the JSON form of v and returns every object key
// containing one of the given substrings, at any depth.
func forbiddenKeys(t *testing.T, v any, substrings []string) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling %T: %v", v, err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding %T: %v", v, err)
	}

	var found []string
	var walk func(any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			for k, child := range n {
				for _, s := range substrings {
					if strings.Contains(strings.ToLower(k), s) {
						found = append(found, k)
						break
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(decoded)
	slices.Sort(found)
	return found
}

func mapKeys(m map[string]any) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// TestReceiptCoSignedOnTheWire is demonstration 5 as it actually happens: the
// producer proposes a receipt with its own signature on the terminal event, the
// consumer checks every unit against what it independently witnessed, and only
// then counter-signs and returns the co-signed receipt as a final A2A Message.
//
// The receipt asserted on below is the copy node B read off its own wire, so it
// has crossed two JSON boundaries and two signatures were applied by two
// separate parties on either side of a network call.
func TestReceiptCoSignedOnTheWire(t *testing.T) {
	c := startChain(t)
	obs := c.Submit(t)

	proposed := obs.Receipt
	if len(proposed.Signatures) != 1 || proposed.Signatures[0].Org != orgB {
		t.Fatalf("producer proposed %d signatures %v, want exactly node B's",
			len(proposed.Signatures), proposed.Signatures)
	}

	// The consumer runs its own §11.3 validation pass first. Until it has, it
	// has retrieved no bytes and reached no acceptance decision, so it has
	// nothing of its own to weigh the proposal against -- which is exactly why
	// §12.2 admits verified_completions only from the consumer's own run.
	c.Validate(t, obs)

	// The consumer's own accounting, from what it saw rather than what it was
	// told: bytes it actually retrieved, and its own validator outcome.
	witnessed := obs.WitnessedUnits(t)
	if err := CheckUnits(witnessed, proposed.Units); err != nil {
		t.Fatalf("consumer's own units diverge from the producer's proposal: %v", err)
	}
	if witnessed.ArtifactBytesServed == 0 {
		t.Fatalf("consumer witnessed 0 artifact bytes, so the units agree only vacuously")
	}

	cosigned := c.CoSignAndReturn(t, obs, proposed)

	if len(cosigned.Signatures) != 2 {
		t.Fatalf("receipt node B read back carries %d signatures, want 2", len(cosigned.Signatures))
	}
	if err := VerifyReceipt(c.OrgKeys, cosigned); err != nil {
		t.Fatalf("VerifyReceipt() on the receipt from the wire = %v, want nil", err)
	}
	if err := VerifyReceipt(c.OrgKeys, proposed); !errors.Is(err, ErrReceiptUnsigned) {
		t.Fatalf("the producer's own proposal verified as a receipt (%v); one signature must not be enough", err)
	}
	t.Logf("co-signed by %s and %s over %d artifact bytes",
		cosigned.Signatures[0].Org, cosigned.Signatures[1].Org, cosigned.Units.ArtifactBytesServed)

	// §12.3's divergence rule: a proposal the consumer cannot corroborate is
	// never counter-signed. Inflating one witnessed unit is enough.
	inflated := proposed
	inflated.Units.ArtifactBytesServed *= 2
	err := CheckUnits(witnessed, inflated.Units)
	if !errors.Is(err, ErrUnitsDiverge) {
		t.Fatalf("CheckUnits() against an inflated proposal = %v, want %v", err, ErrUnitsDiverge)
	}
	if !strings.Contains(err.Error(), "artifact_bytes_served") {
		t.Fatalf("divergence does not name the unit: %v", err)
	}
	t.Logf("refused to counter-sign: %v", err)
}
