package smoke

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// TestUndeclaredRequiredExtensionRejected is verification 3: a node whose Agent
// Card omits a CORE-required extension must refuse a task that requests it,
// rather than serve the task with that extension silently ignored (§2.3).
//
// The client states its required set explicitly instead of using
// a2aext.NewActivator, because the activator only forwards extensions the
// server's card already declares -- exactly the silent degradation §2.3 forbids.
func TestUndeclaredRequiredExtensionRejected(t *testing.T) {
	n := startNode(t, "node-without-authority", Without(ExtAuthority))
	client, _ := newClient(t, n.Card)

	ctx := a2aclient.AttachServiceParams(t.Context(), a2aclient.ServiceParams{
		a2a.SvcParamExtensions: ExtensionURIs(AllExtensions()),
	})
	_, err := client.SendMessage(ctx, &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
	})
	if err == nil {
		t.Fatalf("SendMessage() to a node not declaring %s = nil error, want rejection", ExtAuthority)
	}
	if !errors.Is(err, a2a.ErrExtensionSupportRequired) {
		t.Fatalf("SendMessage() error = %v, want %v", err, a2a.ErrExtensionSupportRequired)
	}
	t.Logf("client-visible error = %v", err)

	// Record the A2A error surface a Consign E_EXTENSION_UNSUPPORTED has to map
	// onto, read off the JSON-RPC wire rather than inferred from the typed error.
	code, message := rawSendMessageError(t, n, ExtensionURIs(AllExtensions()), nil)
	t.Logf("JSON-RPC error on the wire: code=%d message=%q", code, message)
	if code != -32008 {
		t.Fatalf("JSON-RPC error code = %d, want -32008 (ErrExtensionSupportRequired)", code)
	}
}

// TestRequiredExtensionUndeclaredByClientRejectedBySDK records the mirror-image
// case, which a2a-go enforces on its own via WithCapabilityChecks: a card
// declaring required:true extensions refuses a client that does not name them in
// A2A-Extensions. No spike code participates in this rejection; the test exists
// to document which of the two directions A2A v1.0 gives for free.
func TestRequiredExtensionUndeclaredByClientRejectedBySDK(t *testing.T) {
	n := startNode(t, "node-all-seven", AllExtensions())
	client, _ := newClient(t, n.Card)

	_, err := client.SendMessage(t.Context(), &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("submit")),
	})
	if !errors.Is(err, a2a.ErrExtensionSupportRequired) {
		t.Fatalf("SendMessage() without an %s header: error = %v, want %v",
			a2a.SvcParamExtensions, err, a2a.ErrExtensionSupportRequired)
	}
	t.Logf("client-visible error = %v", err)
}

// rawSendMessageError posts a JSON-RPC SendMessage by hand so the numeric error
// code and message can be read directly off the wire.
func rawSendMessageError(t *testing.T, n *node, extensions []string, metadata map[string]any) (int, string) {
	t.Helper()

	params := map[string]any{
		"message": map[string]any{
			"messageId": "msg-cn-001",
			"role":      "user",
			"parts":     []any{map[string]any{"kind": "text", "text": "submit"}},
		},
	}
	if metadata != nil {
		params["metadata"] = metadata
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "cn-001",
		"method":  "SendMessage",
		"params":  params,
	})
	if err != nil {
		t.Fatalf("marshalling raw request: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, n.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("building raw request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, uri := range extensions {
		req.Header.Add(a2a.SvcParamExtensions, uri)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("raw POST failed: %v", err)
	}
	defer resp.Body.Close()

	var envelope struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decoding raw response: %v", err)
	}
	if envelope.Error == nil {
		t.Fatalf("raw JSON-RPC response carried no error object, HTTP status %s", resp.Status)
	}
	return envelope.Error.Code, envelope.Error.Message
}
