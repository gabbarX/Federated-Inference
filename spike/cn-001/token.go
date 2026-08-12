package smoke

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// This file holds the whole offline capability-token verification path (§7.1,
// §7.3, §7.4). It deliberately imports no network-capable package -- not "no
// I/O", which would be false, since fmt reaches os and crypto/ed25519 reaches
// crypto/rand. No network-capable package is the claim that matters and the one
// TestChainVerificationCannotReachTheNetwork checks, for this file's direct
// imports; see that test's comment for the limits of its scope.
//
// Out of scope here by controller resolution: did:web resolution, revocation,
// mTLS and WebPKI. Organization keys arrive pre-pinned in a Keyring (§4.5),
// which is what replaces §7.3 step 1's did:web lookup.

// TokenType is the capability token's typ discriminator (§7.1).
const TokenType = "consign.token/v1"

// CodeTokenInvalid is the §15 code §7.3 folds every chain, signature and
// attenuation failure into. The collapse is deliberate: a refusal that said
// which check failed would be an oracle for probing a peer's policy (§8.5).
const CodeTokenInvalid = "E_TOKEN_INVALID"

// Budget holds the budget caveats (§7.1, §7.5).
type Budget struct {
	Deadline         string `json:"deadline"`
	MaxModelCalls    int    `json:"max_model_calls"`
	MaxToolCalls     int    `json:"max_tool_calls"`
	MaxArtifactBytes int64  `json:"max_artifact_bytes"`
}

// Caveats holds the constraint caveats (§7.2) and budget caveats (§7.5).
type Caveats struct {
	Contract             []string `json:"contract"`
	DataClass            string   `json:"data_class"`
	AllowedOrganizations []string `json:"allowed_organizations"`
	AllowedTools         []string `json:"allowed_tools"`
	NetworkAccess        string   `json:"network_access"`
	SideEffects          string   `json:"side_effects"`
	MaxDelegationDepth   int      `json:"max_delegation_depth"`
	AllowedDelegates     []string `json:"allowed_delegates"`
	Budget               Budget   `json:"budget"`
}

// Signature is the token's detached Ed25519 signature (§7.1).
type Signature struct {
	Alg   string `json:"alg"`
	Kid   string `json:"kid"`
	Value string `json:"value"`
}

// Token is the capability token of §7.1. Chain holds at most one element: the
// token this one was attenuated from, which in turn carries its own Chain, so
// each link's signed bytes are exactly the bytes its issuer signed.
type Token struct {
	Typ        string    `json:"typ"`
	TokenID    string    `json:"token_id"`
	Originator string    `json:"originator"`
	RootTaskID string    `json:"root_task_id"`
	IssuedAt   string    `json:"issued_at"`
	ExpiresAt  string    `json:"expires_at"`
	Caveats    Caveats   `json:"caveats"`
	Chain      []Token   `json:"chain"`
	Sig        Signature `json:"sig"`
}

// Keyring maps a key identifier to a pinned organization public key (§4.5).
type Keyring map[string]ed25519.PublicKey

// Verification failures. §7.3 folds all of these into E_TOKEN_INVALID.
var (
	ErrUnknownKey     = errors.New("no pinned key for kid")
	ErrBadSignature   = errors.New("signature does not verify")
	ErrNotOriginator  = errors.New("root token not signed by the originator organization")
	ErrExpired        = errors.New("token has expired")
	ErrDeadlinePassed = errors.New("deadline caveat is not satisfiable")
	ErrNotPermitted   = errors.New("verifying organization is not permitted by the token")
	ErrDepthExhausted = errors.New("remaining delegation depth is negative")
	ErrChainShape     = errors.New("chain link count must not exceed one")
	ErrWidened        = errors.New("child token widens a caveat")
)

// signingBytes is the canonical byte string a token's issuer signs: the token
// with its own signature value cleared. Struct field order is fixed by the type,
// so encoding/json produces the same bytes for the same token every time.
func signingBytes(tok Token) ([]byte, error) {
	tok.Sig.Value = ""
	return json.Marshal(tok)
}

// Sign returns tok with an EdDSA signature by kid attached.
func Sign(tok Token, kid string, priv ed25519.PrivateKey) (Token, error) {
	tok.Sig = Signature{Alg: "EdDSA", Kid: kid}
	msg, err := signingBytes(tok)
	if err != nil {
		return Token{}, fmt.Errorf("canonicalizing token %s: %w", tok.TokenID, err)
	}
	tok.Sig.Value = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg))
	return tok, nil
}

// verifySignature checks one link's signature against its pinned key.
func verifySignature(keys Keyring, tok Token) error {
	pub, ok := keys[tok.Sig.Kid]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownKey, tok.Sig.Kid)
	}
	value, err := base64.StdEncoding.DecodeString(tok.Sig.Value)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	msg, err := signingBytes(tok)
	if err != nil {
		return fmt.Errorf("canonicalizing token %s: %w", tok.TokenID, err)
	}
	if !ed25519.Verify(pub, msg, value) {
		return ErrBadSignature
	}
	return nil
}

// lineage flattens a token's nested chain into a root-first list ending in tok.
func lineage(tok Token) ([]Token, error) {
	if len(tok.Chain) > 1 {
		return nil, fmt.Errorf("%w: token %s has %d links", ErrChainShape, tok.TokenID, len(tok.Chain))
	}
	if len(tok.Chain) == 0 {
		return []Token{tok}, nil
	}
	ancestors, err := lineage(tok.Chain[0])
	if err != nil {
		return nil, err
	}
	return append(ancestors, tok), nil
}

// VerifyChain runs every check §7.3 requires of a receiving node, from the point
// of view of the organization named by selfOrg. Nothing in the enumeration is
// skipped:
//
//   - step 1, verify the root signature -- done; the did:web *resolution* half is
//     replaced by the pre-pinned Keyring (§4.5), which is the only part the
//     controller placed out of scope.
//   - step 2, every link signed by the organization that produced it.
//   - step 3, every link only narrows, per the §7.4 table in CheckAttenuation.
//   - step 4, expires_at has not passed and the deadline caveat is satisfiable.
//   - step 5, selfOrg appears in allowed_organizations and, when chain is
//     non-empty, in the parent's allowed_delegates.
//   - step 6, remaining depth is >= 0.
//
// It makes no network calls and takes no context, resolver or client, so the
// receiving node cannot contact the originator to resolve a failure even by
// accident -- which is what §7.3's closing sentence demands.
func VerifyChain(keys Keyring, selfOrg string, tok Token, now time.Time) error {
	links, err := lineage(tok)
	if err != nil {
		return err
	}

	for i, link := range links {
		if err := verifySignature(keys, link); err != nil {
			return fmt.Errorf("link %d (%s): %w", i, link.TokenID, err)
		}
	}

	root := links[0]
	if !strings.HasPrefix(root.Sig.Kid, root.Originator+"#") {
		return fmt.Errorf("%w: root kid %s, originator %s", ErrNotOriginator, root.Sig.Kid, root.Originator)
	}

	expiry, err := time.Parse(time.RFC3339, tok.ExpiresAt)
	if err != nil {
		return fmt.Errorf("parsing expires_at of %s: %w", tok.TokenID, err)
	}
	if now.After(expiry) {
		return fmt.Errorf("%w: %s expired at %s", ErrExpired, tok.TokenID, tok.ExpiresAt)
	}

	// §7.3 step 4, second clause. A deadline already in the past cannot be met,
	// so the task is unacceptable however much budget remains.
	deadline, err := time.Parse(time.RFC3339, tok.Caveats.Budget.Deadline)
	if err != nil {
		return fmt.Errorf("parsing budget.deadline of %s: %w", tok.TokenID, err)
	}
	if now.After(deadline) {
		return fmt.Errorf("%w: %s deadline was %s", ErrDeadlinePassed, tok.TokenID, tok.Caveats.Budget.Deadline)
	}

	// §7.3 step 5. Pure offline logic: it needs only the verifier's own identity,
	// so none of the did:web/mTLS/WebPKI exclusions reach it.
	if !slices.Contains(tok.Caveats.AllowedOrganizations, selfOrg) {
		return fmt.Errorf("%w: %s is not in allowed_organizations of %s", ErrNotPermitted, selfOrg, tok.TokenID)
	}
	if len(tok.Chain) > 0 {
		parent := tok.Chain[0]
		if !slices.Contains(parent.Caveats.AllowedDelegates, selfOrg) {
			return fmt.Errorf("%w: %s is not in allowed_delegates of parent %s",
				ErrNotPermitted, selfOrg, parent.TokenID)
		}
	}

	if tok.Caveats.MaxDelegationDepth < 0 {
		return fmt.Errorf("%w: %s", ErrDepthExhausted, tok.TokenID)
	}

	for i := 1; i < len(links); i++ {
		if err := CheckAttenuation(links[i-1], links[i]); err != nil {
			return fmt.Errorf("link %d (%s): %w", i, links[i].TokenID, err)
		}
	}
	return nil
}

// MintChild is the delegating side of §7.4 to §7.6: it attenuates parent into a
// child token for delegate, deducting the child's budget from ledger as §7.5
// requires be done at delegation time.
//
// The order of the checks is the order in which a holder loses the right to
// delegate at all. Depth comes first, because §7.6 makes a depth-0 holder
// unable to re-delegate however much budget or permission it otherwise has;
// then the parent's allowed_delegates; then the budget. The child it builds is
// run back through CheckAttenuation before signing, so a mistake here is caught
// by the same rule a receiving node would apply.
func MintChild(parent Token, ledger *Ledger, delegate, tokenID, kid string, budget Budget, priv ed25519.PrivateKey, now time.Time) (Token, error) {
	if parent.Caveats.MaxDelegationDepth <= 0 {
		return Token{}, fmt.Errorf("%w: %s permits no further delegation", ErrDepthExhausted, parent.TokenID)
	}
	if !slices.Contains(parent.Caveats.AllowedDelegates, delegate) {
		return Token{}, fmt.Errorf("%w: %s is not in allowed_delegates of %s", ErrNotPermitted, delegate, parent.TokenID)
	}
	if err := ledger.Subdivide(budget); err != nil {
		return Token{}, err
	}

	child := Token{
		Typ:        TokenType,
		TokenID:    tokenID,
		Originator: parent.Originator,
		RootTaskID: parent.RootTaskID,
		IssuedAt:   now.UTC().Format(time.RFC3339),
		ExpiresAt:  parent.ExpiresAt,
		Caveats: Caveats{
			Contract:             slices.Clone(parent.Caveats.Contract),
			DataClass:            parent.Caveats.DataClass,
			AllowedOrganizations: []string{delegate},
			AllowedTools:         slices.Clone(parent.Caveats.AllowedTools),
			NetworkAccess:        parent.Caveats.NetworkAccess,
			SideEffects:          parent.Caveats.SideEffects,
			MaxDelegationDepth:   parent.Caveats.MaxDelegationDepth - 1,
			// A child minted at the bottom of the permitted depth names no
			// delegates of its own, which is the narrowest legal choice.
			AllowedDelegates: nil,
			Budget:           budget,
		},
		Chain: []Token{parent},
	}
	if err := CheckAttenuation(parent, child); err != nil {
		return Token{}, fmt.Errorf("minting %s: %w", tokenID, err)
	}
	return Sign(child, kid, priv)
}

// dataClassRank orders the data classes of §7.2.1:
// PUBLIC < CONSORTIUM < RESTRICTED < LOCAL-ONLY.
var dataClassRank = map[string]int{"PUBLIC": 0, "CONSORTIUM": 1, "RESTRICTED": 2, "LOCAL-ONLY": 3}

// networkRank and sideEffectRank order by restrictiveness, most permissive first,
// so "equal or more restrictive" is child >= parent.
var (
	networkRank    = map[string]int{"allow": 0, "deny": 1}
	sideEffectRank = map[string]int{"scoped-execute": 0, "propose-only": 1}
)

// CheckAttenuation applies the §7.4 attenuation table. Every row is checked and
// the first violation is reported by name; any transformation not permitted by
// the table is invalid regardless of who signed the child.
func CheckAttenuation(parent, child Token) error {
	if err := subset("contract", child.Caveats.Contract, parent.Caveats.Contract); err != nil {
		return err
	}
	if err := notAbove("data_class", dataClassRank, child.Caveats.DataClass, parent.Caveats.DataClass); err != nil {
		return err
	}
	if err := subset("allowed_organizations", child.Caveats.AllowedOrganizations, parent.Caveats.AllowedOrganizations); err != nil {
		return err
	}
	if err := subset("allowed_delegates", child.Caveats.AllowedDelegates, parent.Caveats.AllowedDelegates); err != nil {
		return err
	}
	if err := subset("allowed_tools", child.Caveats.AllowedTools, parent.Caveats.AllowedTools); err != nil {
		return err
	}
	if err := notBelow("network_access", networkRank, child.Caveats.NetworkAccess, parent.Caveats.NetworkAccess); err != nil {
		return err
	}
	if err := notBelow("side_effects", sideEffectRank, child.Caveats.SideEffects, parent.Caveats.SideEffects); err != nil {
		return err
	}
	if child.Caveats.MaxDelegationDepth > parent.Caveats.MaxDelegationDepth-1 {
		return fmt.Errorf("%w: max_delegation_depth %d exceeds parent's %d minus one",
			ErrWidened, child.Caveats.MaxDelegationDepth, parent.Caveats.MaxDelegationDepth)
	}
	if err := notLater("expires_at", child.ExpiresAt, parent.ExpiresAt); err != nil {
		return err
	}
	if err := notLater("budget.deadline", child.Caveats.Budget.Deadline, parent.Caveats.Budget.Deadline); err != nil {
		return err
	}
	// §7.5 makes each budget caveat "<= parent's remaining value". This spike
	// keeps no spend ledger, so it compares against the parent's full value.
	if err := atMost("budget.max_model_calls", int64(child.Caveats.Budget.MaxModelCalls), int64(parent.Caveats.Budget.MaxModelCalls)); err != nil {
		return err
	}
	if err := atMost("budget.max_tool_calls", int64(child.Caveats.Budget.MaxToolCalls), int64(parent.Caveats.Budget.MaxToolCalls)); err != nil {
		return err
	}
	return atMost("budget.max_artifact_bytes", child.Caveats.Budget.MaxArtifactBytes, parent.Caveats.Budget.MaxArtifactBytes)
}

func subset(caveat string, child, parent []string) error {
	for _, v := range child {
		if !slices.Contains(parent, v) {
			return fmt.Errorf("%w: %s gained %q", ErrWidened, caveat, v)
		}
	}
	return nil
}

func notAbove(caveat string, ranks map[string]int, child, parent string) error {
	c, ok := ranks[child]
	if !ok {
		return fmt.Errorf("%w: %s has unknown value %q", ErrWidened, caveat, child)
	}
	p, ok := ranks[parent]
	if !ok {
		return fmt.Errorf("%w: parent %s has unknown value %q", ErrWidened, caveat, parent)
	}
	if c > p {
		return fmt.Errorf("%w: %s %q is above parent's %q", ErrWidened, caveat, child, parent)
	}
	return nil
}

func notBelow(caveat string, ranks map[string]int, child, parent string) error {
	c, ok := ranks[child]
	if !ok {
		return fmt.Errorf("%w: %s has unknown value %q", ErrWidened, caveat, child)
	}
	p, ok := ranks[parent]
	if !ok {
		return fmt.Errorf("%w: parent %s has unknown value %q", ErrWidened, caveat, parent)
	}
	if c < p {
		return fmt.Errorf("%w: %s %q is less restrictive than parent's %q", ErrWidened, caveat, child, parent)
	}
	return nil
}

func notLater(caveat, child, parent string) error {
	c, err := time.Parse(time.RFC3339, child)
	if err != nil {
		return fmt.Errorf("parsing %s %q: %w", caveat, child, err)
	}
	p, err := time.Parse(time.RFC3339, parent)
	if err != nil {
		return fmt.Errorf("parsing parent %s %q: %w", caveat, parent, err)
	}
	if c.After(p) {
		return fmt.Errorf("%w: %s %s is later than parent's %s", ErrWidened, caveat, child, parent)
	}
	return nil
}

func atMost(caveat string, child, parent int64) error {
	if child > parent {
		return fmt.Errorf("%w: %s %d exceeds parent's %d", ErrWidened, caveat, child, parent)
	}
	return nil
}
