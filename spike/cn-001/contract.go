package smoke

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// This file holds the §6 task contract: the document, its digest, and the
// producer-side check §6.3 requires before a task is accepted.

// ContractType is the contract document's typ discriminator (§6.3).
const ContractType = "consign.contract/v1"

// CodeContractMismatch and CodeContractUnsupported are the §15 machine-readable
// codes for the two ways a contract reference can fail. They are Consign codes,
// not A2A ones; see contract_test.go for the A2A surface they travel on.
const (
	CodeContractMismatch    = "E_CONTRACT_MISMATCH"
	CodeContractUnsupported = "E_CONTRACT_UNSUPPORTED"
	CodeSchemaInvalid       = "E_SCHEMA_INVALID"
)

var (
	ErrContractMismatch    = errors.New("contract digest differs from this node's copy")
	ErrContractUnsupported = errors.New("contract identifier not offered by this node")
)

// Validator names one member of a contract's validator set (§6.1, §6.3).
type Validator struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	MustPass bool   `json:"must_pass"`
}

// Contract is the §6.3 contract document. It binds an identifier to the four
// things §6.1 makes it mean: input schema, output schema, verifiability class
// and validator set.
type Contract struct {
	Typ               string      `json:"typ"`
	ID                string      `json:"id"`
	Verifiability     string      `json:"verifiability"`
	InputSchema       string      `json:"input_schema"`
	OutputSchema      string      `json:"output_schema"`
	Validators        []Validator `json:"validators"`
	RequiredArtifacts []string    `json:"required_artifacts"`
	Digest            string      `json:"digest"`
}

// ContractRef is the {id, digest} pair a task envelope carries (§8.3).
type ContractRef struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// Sealed returns c with its §6.3 digest attached. The digest covers the
// serialization of the whole document with the digest field itself cleared,
// which is the only reading under which a document can carry its own digest.
// Struct field order fixes encoding/json's output, so the same document always
// hashes to the same value; a production profile needs a real canonical JSON
// form (RFC 8785) because Consign documents cross language boundaries.
func (c Contract) Sealed() (Contract, error) {
	c.Digest = ""
	raw, err := json.Marshal(c)
	if err != nil {
		return Contract{}, fmt.Errorf("canonicalizing contract %s: %w", c.ID, err)
	}
	sum := sha256.Sum256(raw)
	c.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return c, nil
}

// Ref returns the reference an envelope carries for this contract.
func (c Contract) Ref() ContractRef {
	return ContractRef{ID: c.ID, Digest: c.Digest}
}

// CheckContractRef applies §6.3's producer-side rule: a producer must reject a
// task whose contract digest does not match its own copy. The two failure modes
// stay distinguishable here because §15 gives them separate codes; §8.5's
// non-disclosure rule applies to what is told to the *peer*, not to what the
// node knows locally.
func CheckContractRef(local []Contract, ref ContractRef) error {
	i := slices.IndexFunc(local, func(c Contract) bool { return c.ID == ref.ID })
	if i < 0 {
		return fmt.Errorf("%w: %s", ErrContractUnsupported, ref.ID)
	}
	if local[i].Digest != ref.Digest {
		return fmt.Errorf("%w: %s", ErrContractMismatch, ref.ID)
	}
	return nil
}
