package smoke

import "github.com/a2aproject/a2a-go/v2/a2a"

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
