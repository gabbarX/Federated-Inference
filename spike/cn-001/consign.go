package smoke

import (
	"encoding/json"
	"fmt"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// The seven Consign extension namespaces (Consign profile §2.2). URIs are used
// verbatim; they are the identity of each extension on the wire.
const (
	ExtContract     = "https://consign.example/ext/contract/v1"
	ExtConstraints  = "https://consign.example/ext/constraints/v1"
	ExtAuthority    = "https://consign.example/ext/authority/v1"
	ExtBudget       = "https://consign.example/ext/budget/v1"
	ExtArtifacts    = "https://consign.example/ext/artifacts/v1"
	ExtVerification = "https://consign.example/ext/verification/v1"
	ExtReceipts     = "https://consign.example/ext/receipts/v1"
)

// AllExtensions declares all seven Consign namespaces as A2A AgentExtensions.
// Criticality follows §2.2's "Required by CORE" column: everything except
// receipts is required.
func AllExtensions() []a2a.AgentExtension {
	return []a2a.AgentExtension{
		{URI: ExtContract, Description: "Consign task contract (§6)", Required: true},
		{URI: ExtConstraints, Description: "Consign constraint caveats (§7.2)", Required: true},
		{URI: ExtAuthority, Description: "Consign capability token (§7)", Required: true},
		{URI: ExtBudget, Description: "Consign budget subdivision (§7.5)", Required: true},
		{URI: ExtArtifacts, Description: "Consign content-addressed artifacts (§10)", Required: true},
		{URI: ExtVerification, Description: "Consign verification results (§11)", Required: true},
		{URI: ExtReceipts, Description: "Consign receipts (§12)", Required: false},
	}
}

// RequiredByCore reports whether uri is one of the extensions §2.2 marks
// "Required by CORE". Criticality is a property of the profile, not of any one
// node's card: a node that does not declare one of these is not a conformant
// CORE node and must refuse CORE tasks rather than degrade silently (§2.3).
func RequiredByCore(uri string) bool {
	for _, e := range AllExtensions() {
		if e.URI == uri {
			return e.Required
		}
	}
	return false
}

// ExtensionURIs returns the URIs of the given declarations, in declaration order.
func ExtensionURIs(exts []a2a.AgentExtension) []string {
	uris := make([]string, len(exts))
	for i, e := range exts {
		uris[i] = e.URI
	}
	return uris
}

// ToMetadata converts a Consign payload into the shape A2A metadata accepts.
// This is not cosmetic: a2asrv's task store validates every metadata value and
// rejects anything outside {nil, bool, int, float, string, []any,
// map[string]any} (a2asrv/taskstore/validator.go validateMetaRecursive), so a
// Go struct -- or even a []string -- put straight into a Task, Artifact or
// status metadata map fails the task save and moves the task to FAILED. A JSON
// round trip is what makes a typed Consign payload carriable.
func ToMetadata(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encoding %T as metadata: %w", v, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding %T as metadata: %w", v, err)
	}
	return out, nil
}

// FromMetadata decodes a namespace's metadata payload into a Consign type.
func FromMetadata(payload any, dst any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("re-encoding metadata payload: %w", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decoding metadata payload into %T: %w", dst, err)
	}
	return nil
}

// Without returns AllExtensions minus the named URI, for building the
// deliberately deficient node used by the §2.3 rejection test.
func Without(uri string) []a2a.AgentExtension {
	all := AllExtensions()
	kept := make([]a2a.AgentExtension, 0, len(all)-1)
	for _, e := range all {
		if e.URI != uri {
			kept = append(kept, e)
		}
	}
	return kept
}
