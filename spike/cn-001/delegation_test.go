package smoke

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestDelegationHop is demonstration 2: node B really receives a task from A,
// really mints an attenuated child token, and really submits it to node C over
// A2A. Everything asserted below is read either from what C received on its own
// wire or from what A observed on the stream -- never from B's intentions.
func TestDelegationHop(t *testing.T) {
	c := startChain(t)
	obs := c.Submit(t)

	received := c.Delegate.Received(t)

	t.Run("node C really received a task from node B", func(t *testing.T) {
		if received.TaskID == "" {
			t.Fatalf("node C recorded no incoming task, so no delegation happened")
		}
		if received.Token.TokenID == "" {
			t.Fatalf("node C received no capability token")
		}
		// C verified the chain itself before replying; had it failed, C would
		// have returned an error and B's Execute would have aborted. Re-running
		// the same check here makes the evidence visible rather than implicit.
		if err := VerifyChain(c.Keys, orgC, received.Token, time.Now()); err != nil {
			t.Fatalf("the token node C received does not verify at node C: %v", err)
		}
		if len(received.Token.Chain) != 1 {
			t.Fatalf("received token has %d chain links, want 1 (the root A minted)", len(received.Token.Chain))
		}
		if got := received.Token.Chain[0].TokenID; got != c.Root.TokenID {
			t.Fatalf("received token's parent = %q, want A's root %q", got, c.Root.TokenID)
		}
		if received.Token.Sig.Kid != kidB {
			t.Fatalf("received token signed by %q, want node B's key %q", received.Token.Sig.Kid, kidB)
		}
		t.Logf("node C received %s, chained to %s, signed by %s",
			received.Token.TokenID, received.Token.Chain[0].TokenID, received.Token.Sig.Kid)
	})

	t.Run("depth decremented on the hop", func(t *testing.T) {
		if c.Root.Caveats.MaxDelegationDepth != 1 {
			t.Fatalf("fixture is wrong: root depth = %d, want 1", c.Root.Caveats.MaxDelegationDepth)
		}
		if got := received.Token.Caveats.MaxDelegationDepth; got != 0 {
			t.Fatalf("child token depth = %d, want 0 (root's 1 minus one)", got)
		}
	})

	t.Run("node B disclosed the delegation to its consumer", func(t *testing.T) {
		env := obs.envelope(t, EventDelegated)
		if got, _ := env.Payload["child_task_id"].(string); got != string(received.TaskID) {
			t.Fatalf("task.delegated names child task %q, but node C saw %q", got, received.TaskID)
		}
		if got, _ := env.Payload["delegate_org"].(string); got != orgC {
			t.Fatalf("task.delegated delegate_org = %q, want %q", got, orgC)
		}
		remaining, ok := env.Payload["remaining_depth"].(float64)
		if !ok {
			t.Fatalf("task.delegated remaining_depth is %T, want a number", env.Payload["remaining_depth"])
		}
		if int(remaining) != 0 {
			t.Fatalf("task.delegated remaining_depth = %v, want 0", remaining)
		}
		// §7.6 requires the disclosure in real time, which here means before the
		// terminal event rather than bundled into it.
		if env.Seq >= obs.envelope(t, EventCompleted).Seq {
			t.Fatalf("task.delegated at seq %d is not before task.completed", env.Seq)
		}
		t.Logf("task.delegated seq=%d payload=%v", env.Seq, env.Payload)
	})

	// Demonstration 3 on the wire. The offline ledger rules are in
	// TestBudgetSubdivision; what is asserted here is that the token C actually
	// received carries the deducted amount, and that what B reported as
	// remaining is the allowance minus exactly that.
	t.Run("the child's budget was deducted from node B's allowance", func(t *testing.T) {
		child := received.Token.Caveats.Budget
		if child.MaxModelCalls >= c.Allowance.MaxModelCalls {
			t.Fatalf("child budget max_model_calls = %d, allowance = %d: nothing was subdivided",
				child.MaxModelCalls, c.Allowance.MaxModelCalls)
		}

		var reported BudgetReport
		if err := FromMetadata(obs.TerminalMeta[ExtBudget], &reported); err != nil {
			t.Fatalf("decoding the terminal budget namespace: %v", err)
		}
		want := Budget{
			Deadline:         c.Allowance.Deadline,
			MaxModelCalls:    c.Allowance.MaxModelCalls - child.MaxModelCalls,
			MaxToolCalls:     c.Allowance.MaxToolCalls - child.MaxToolCalls,
			MaxArtifactBytes: c.Allowance.MaxArtifactBytes - child.MaxArtifactBytes,
		}
		if reported.Remaining != want {
			t.Fatalf("node B reported remaining %+v, want allowance minus the child's %+v", reported.Remaining, want)
		}
		if len(reported.Delegated) != 1 || reported.Delegated[0] != child {
			t.Fatalf("node B reported delegated budgets %+v, want exactly the child's %+v", reported.Delegated, child)
		}
		t.Logf("allowance %+v, child %+v, remaining %+v", c.Allowance, child, reported.Remaining)
	})
}

// TestReDelegationRefused covers the two ways §7.5 and §7.6 stop a holder
// handing on more than it holds, using the same minting path node B uses on the
// wire. Both are refusals a holder makes about itself, which is what makes
// constraint laundering detectable by the party it would harm.
func TestReDelegationRefused(t *testing.T) {
	c := startChain(t)
	now := time.Now()

	share := childShare(c.Allowance)

	t.Run("a depth-0 holder may not re-delegate", func(t *testing.T) {
		// The same token node C holds on the wire: depth 0, so C is a leaf.
		leaf, err := MintChild(c.Root, NewLedger(c.Allowance), orgC, "tok_child", kidB, share, c.SignKeys[orgB], now)
		if err != nil {
			t.Fatalf("minting the leaf = %v, want nil", err)
		}
		if leaf.Caveats.MaxDelegationDepth != 0 {
			t.Fatalf("fixture is wrong: leaf depth = %d, want 0", leaf.Caveats.MaxDelegationDepth)
		}

		_, err = MintChild(leaf, NewLedger(leaf.Caveats.Budget), "did:web:fourth.example",
			"tok_grandchild", kidC, childShare(leaf.Caveats.Budget), c.SignKeys[orgC], now)
		if !errors.Is(err, ErrDepthExhausted) {
			t.Fatalf("minting a grandchild from a depth-0 token = %v, want %v", err, ErrDepthExhausted)
		}
		t.Logf("refused: %v", err)
	})

	t.Run("a holder may not delegate more budget than it has left", func(t *testing.T) {
		ledger := NewLedger(c.Allowance)
		if _, err := MintChild(c.Root, ledger, orgC, "tok_child_1", kidB, share, c.SignKeys[orgB], now); err != nil {
			t.Fatalf("minting the first child = %v, want nil", err)
		}
		// The second child asks for the same share again, and the allowance was
		// sized so that two of them do not fit.
		_, err := MintChild(c.Root, ledger, orgC, "tok_child_2", kidB, share, c.SignKeys[orgB], now)
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("minting a second equally-sized child = %v, want %v", err, ErrBudgetExceeded)
		}
		if !strings.Contains(err.Error(), "max_model_calls") {
			t.Fatalf("refusal does not name the exhausted unit: %v", err)
		}
		t.Logf("refused: %v", err)
	})
}
