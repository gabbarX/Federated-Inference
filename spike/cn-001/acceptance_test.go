package smoke

import (
	"context"
	"errors"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// consignAcceptance is the §8.3/§8.4 pre-acceptance guard: it applies §6.3's
// rule that a producer must reject a task whose contract digest does not match
// its own copy. It runs as an a2asrv.CallInterceptor rather than inside the
// agent because a refusal has to happen *before* acceptance -- once the agent
// is executing, the task exists and the SDK's only remaining vocabulary is a
// FAILED terminal state, which is a different thing from a refusal.
//
// Its single responsibility is the digest rule. A CORE node also has to refuse
// a submission carrying no envelope at all (§8.3); that check is not modelled
// here, so a message with no contract namespace passes through untouched.
type consignAcceptance struct {
	contracts []Contract
}

var _ a2asrv.CallInterceptor = (*consignAcceptance)(nil)

func (a *consignAcceptance) Before(ctx context.Context, _ *a2asrv.CallContext, req *a2asrv.Request) (context.Context, any, error) {
	send, ok := req.Payload.(*a2a.SendMessageRequest)
	if !ok {
		return ctx, nil, nil
	}
	payload, ok := send.Metadata[ExtContract]
	if !ok {
		return ctx, nil, nil
	}

	var ref ContractRef
	if err := FromMetadata(payload, &ref); err != nil {
		return ctx, nil, consignRefusal(a2a.ErrInvalidParams, "malformed contract reference", CodeSchemaInvalid)
	}
	if err := CheckContractRef(a.contracts, ref); err != nil {
		code := CodeContractUnsupported
		if errors.Is(err, ErrContractMismatch) {
			code = CodeContractMismatch
		}
		// §8.5: the peer learns the coarse code and nothing else. Neither digest
		// appears in the message, so the refusal is not an oracle for what this
		// node holds.
		return ctx, nil, consignRefusal(a2a.ErrInvalidParams, "contract reference not accepted", code)
	}
	return ctx, nil, nil
}

func (a *consignAcceptance) After(context.Context, *a2asrv.CallContext, *a2asrv.Response) error {
	return nil
}

// consignRefusal wraps an A2A error so it keeps A2A's own transport code while
// carrying the Consign §15 code §15 requires be machine-readable. The Consign
// code rides the ErrorInfo typed detail's metadata map, which a2a-go serialises
// into the JSON-RPC error object's `data` array and reconstructs client-side.
func consignRefusal(a2aErr error, message, consignCode string) error {
	return a2a.NewError(a2aErr, message).
		WithErrorInfoMeta(map[string]string{"consign_code": consignCode})
}
