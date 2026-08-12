package smoke

import (
	"errors"
	"fmt"
	"slices"
)

// This file holds §11's result envelope and the consumer-side acceptance
// decision of §11.3.
//
// The validators themselves are no-op stubs by controller resolution: a real
// isolated runtime belongs to CN-003. What is modelled is everything that does
// not need one -- the envelope's shape, the separation of claims from evidence,
// and the rule that acceptance rests on evidence the consumer produced.

// ResultType is the result envelope's schema discriminator (§11.2).
const ResultType = "consign.result/v1"

var (
	ErrArtifactUnavailable = errors.New("holder did not serve a referenced artifact")
	ErrClaimUnsupported    = errors.New("claim cites evidence that is not an artifact of this result")
	ErrUnverifiedResult    = errors.New("UNVERIFIED result requires a recorded operator override")
)

// ResultArtifact names one artifact the result produced, by role (§11.2).
type ResultArtifact struct {
	Role  string `json:"role"`
	Ref   string `json:"ref"`
	Label string `json:"label"`
}

// Claim is a statement the producer makes, kept separate from the evidence that
// supports it so a consumer can accept, reject or qualify claims individually
// (§11.2). ProducerValidated is informational only (§11.3).
type Claim struct {
	ID                string   `json:"id"`
	Statement         string   `json:"statement"`
	Evidence          []string `json:"evidence"`
	ProducerValidated bool     `json:"producer_validated"`
}

// Provenance records who and what was in the path (§11.2). Every field is
// *declared* by the producer (§5.5), which is why none of it gates acceptance.
type Provenance struct {
	Chain                 []string `json:"chain"`
	DeclaredModelManifest string   `json:"declared_model_manifest,omitempty"`
	ToolsUsed             []string `json:"tools_used"`
	PolicyVersion         string   `json:"policy_version,omitempty"`
}

// Usage is the producer's declared accounting (§11.2). It is the right home for
// model and tool counts precisely because §12.2 bars them from receipts.
type Usage struct {
	WallSeconds        int `json:"wall_seconds"`
	DeclaredModelCalls int `json:"declared_model_calls"`
	DeclaredToolCalls  int `json:"declared_tool_calls"`
}

// Result is the §11.2 result envelope.
type Result struct {
	Schema        string           `json:"schema"`
	TaskID        string           `json:"task_id"`
	Contract      ContractRef      `json:"contract"`
	Verifiability string           `json:"verifiability"`
	Output        map[string]any   `json:"output"`
	Artifacts     []ResultArtifact `json:"artifacts"`
	Claims        []Claim          `json:"claims"`
	Provenance    Provenance       `json:"provenance"`
	Usage         Usage            `json:"usage"`
}

// AcceptResult is §11.3 from the consumer's side for a DETERMINISTIC contract,
// over artifacts the consumer has already retrieved.
//
// Step 3's "execute every must_pass validator in its own isolated sandbox" is
// the no-op stub: there is no sandbox here and none is faked. Steps 1, 2 and 4
// are real, and step 2 -- retrieve and digest-verify every referenced artifact
// -- is what makes this a decision on the consumer's own evidence. Nothing in
// this function reads Claim.ProducerValidated.
func AcceptResult(res Result, retrieved map[string][]byte) error {
	if res.Verifiability == "UNVERIFIED" {
		return fmt.Errorf("%w: task %s", ErrUnverifiedResult, res.TaskID)
	}
	if len(res.Output) == 0 {
		return fmt.Errorf("result for task %s carries no output", res.TaskID)
	}

	published := make([]string, 0, len(res.Artifacts))
	for _, a := range res.Artifacts {
		data, ok := retrieved[a.Ref]
		if !ok {
			return fmt.Errorf("%w: %s (role %s)", ErrArtifactUnavailable, a.Ref, a.Role)
		}
		if err := VerifyCAS(a.Ref, data); err != nil {
			return fmt.Errorf("artifact %s (role %s): %w", a.Ref, a.Role, err)
		}
		published = append(published, a.Ref)
	}

	for _, c := range res.Claims {
		for _, ref := range c.Evidence {
			if !slices.Contains(published, ref) {
				return fmt.Errorf("%w: claim %s cites %s", ErrClaimUnsupported, c.ID, ref)
			}
		}
	}
	return nil
}
