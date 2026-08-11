package smoke

import (
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// TestSDKReportsProtocolVersion1_0 confirms the a2a-go SDK is reachable and
// that it reports A2A protocol version 1.0 (not the older 0.3 line).
func TestSDKReportsProtocolVersion1_0(t *testing.T) {
	t.Logf("a2a.Version = %q", a2a.Version)
	if a2a.Version != "1.0" {
		t.Fatalf("expected a2a.Version to be %q, got %q", "1.0", a2a.Version)
	}
}

// TestExtensionMechanismTypesExist confirms the extension-mechanism types
// that later CN-001 tasks depend on exist and are usable: AgentExtension
// with URI/Description/Required/Params, AgentCapabilities.Extensions, and
// the A2A-Extensions / A2A-Version header-name constants.
func TestExtensionMechanismTypesExist(t *testing.T) {
	ext := a2a.AgentExtension{
		URI:         "https://consign.example/ext/contract/v1",
		Description: "smoke test extension",
		Required:    false,
		Params:      map[string]any{"k": "v"},
	}

	caps := a2a.AgentCapabilities{
		Extensions: []a2a.AgentExtension{ext},
	}
	if len(caps.Extensions) != 1 {
		t.Fatalf("expected 1 extension in AgentCapabilities.Extensions, got %d", len(caps.Extensions))
	}

	t.Logf("SvcParamExtensions header name = %q", a2a.SvcParamExtensions)
	t.Logf("SvcParamVersion header name = %q", a2a.SvcParamVersion)
	if a2a.SvcParamExtensions != "A2A-Extensions" {
		t.Fatalf("expected SvcParamExtensions to be %q, got %q", "A2A-Extensions", a2a.SvcParamExtensions)
	}
	if a2a.SvcParamVersion != "A2A-Version" {
		t.Fatalf("expected SvcParamVersion to be %q, got %q", "A2A-Version", a2a.SvcParamVersion)
	}
}
