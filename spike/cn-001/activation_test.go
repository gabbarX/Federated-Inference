package smoke

import (
	"context"
	"net/http"
	"slices"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// respHeaderKey carries the outgoing HTTP header map down to the server call
// interceptor. It is needed because a2asrv's JSON-RPC binding never converts
// CallContext.Extensions().ActivatedURIs() into a response service parameter --
// only the gRPC binding does (a2agrpc/v1/handler.go toTrailer). See the report's
// "SDK API observed" section.
type respHeaderKey struct{}

// withResponseServiceParams exposes the outgoing header map to the a2asrv call
// interceptor stack. Interceptor After hooks run before the JSON-RPC handler
// writes the response body, so headers set there still reach the client.
func withResponseServiceParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), respHeaderKey{}, rw.Header())
		next.ServeHTTP(rw, req.WithContext(ctx))
	})
}

// withStreamingServiceParams is the second half of the response echo, and it
// exists because the first half provably cannot work on message/stream.
//
// jsonrpcHandler.handleStreamingRequest calls sseWriter.WriteHeaders() -- which
// ends in w.writer.WriteHeader(http.StatusOK) -- *before* it dispatches to the
// request handler. The interceptor stack, and therefore Activate() and
// consignExtensions.After, all run after that point, so anything they add to
// rw.Header() is added to an already-committed header map and never reaches the
// wire. Outer http.Handler middleware is the only place that holds both the
// request's A2A-Extensions values and an un-committed response header.
//
// The cost of moving out here is real and is the reason both halves exist: this
// middleware cannot see a2asrv's CallContext (jsonrpcHandler.ServeHTTP calls
// NewCallContext unconditionally and shadows any outer one), so it recomputes
// requested ∩ declared instead of reporting the SDK's own ActivatedURIs().
// It is gated on the Accept header the SDK's own streaming client sets
// (a2aclient/jsonrpc.go sendStreamingRequest: Accept: text/event-stream) so
// that the non-streaming path keeps using the interceptor and no URI is echoed
// twice.
func withStreamingServiceParams(declared []a2a.AgentExtension, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Accept") == "text/event-stream" {
			for _, uri := range req.Header.Values(a2a.SvcParamExtensions) {
				if declares(declared, uri) {
					rw.Header().Add(a2a.SvcParamExtensions, uri)
				}
			}
		}
		next.ServeHTTP(rw, req)
	})
}

// consignExtensions is the a2asrv.CallInterceptor that implements the Consign
// half of A2A extension negotiation: it rejects tasks requesting a CORE-required
// extension this node has not declared (§2.3), activates the declared
// extensions the client asked for, and reports the activated set back to the
// client in the A2A-Extensions response service parameter.
type consignExtensions struct {
	declared []a2a.AgentExtension
}

var _ a2asrv.CallInterceptor = (*consignExtensions)(nil)

func (c *consignExtensions) Before(ctx context.Context, callCtx *a2asrv.CallContext, _ *a2asrv.Request) (context.Context, any, error) {
	requested := callCtx.Extensions().RequestedURIs()

	// §2.3: a required extension the receiver does not support MUST cause
	// rejection. a2a-go enforces only the mirror-image rule -- that a client
	// must name the required extensions this node's card declares -- so without
	// this loop the node serves the task with the extension silently ignored.
	for _, uri := range requested {
		if RequiredByCore(uri) && !declares(c.declared, uri) {
			return ctx, nil, a2a.ErrExtensionSupportRequired
		}
	}

	for i := range c.declared {
		if slices.Contains(requested, c.declared[i].URI) {
			callCtx.Extensions().Activate(&c.declared[i])
		}
	}
	return ctx, nil, nil
}

func (c *consignExtensions) After(ctx context.Context, callCtx *a2asrv.CallContext, _ *a2asrv.Response) error {
	activated := callCtx.Extensions().ActivatedURIs()
	if len(activated) == 0 {
		return nil
	}
	header, ok := ctx.Value(respHeaderKey{}).(http.Header)
	if !ok {
		return nil
	}
	// One header value per URI, mirroring how a2aclient's JSON-RPC transport
	// serialises the request-side service parameter (http.Header.Add per value).
	for _, uri := range activated {
		header.Add(a2a.SvcParamExtensions, uri)
	}
	return nil
}

func declares(exts []a2a.AgentExtension, uri string) bool {
	return slices.ContainsFunc(exts, func(e a2a.AgentExtension) bool { return e.URI == uri })
}
