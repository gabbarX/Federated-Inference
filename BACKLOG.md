# Bailment — Ticket Backlog

Derived from `Readme.md` (PRD v0.3, 11 Aug 2026) and `spec/bailment-profile-v0.1.md`.

**Status:** filed as GitHub issues on `gabbarX/Federated-Inference`.
The repository transfers to `bailment/bailment` after this rename merges; see ADR-0001 for the sequencing.
**Generated:** 11 Aug 2026. **126 tickets across 16 epics.**

This file is the source of truth. Issues are generated from it by
`tools/backlog_to_issues.py`, which is idempotent and resumable:

```bash
python tools/backlog_to_issues.py --dry-run     # preview
python tools/backlog_to_issues.py               # create missing issues, then sync all bodies
python tools/backlog_to_issues.py --link-only   # sync bodies only, after editing this file
```

Edit a ticket here and re-run with `--link-only` rather than editing the issue on
GitHub, or the backlog and the tracker will diverge. Ticket ID → issue number is
recorded in `tools/.issue-map.json`. Phases map to milestones; epic, priority, and
size map to labels.

---

## How to read this

| Field | Meaning |
| --- | --- |
| **ID** | `CN-###`. Stable. Do not renumber; retire instead. |
| **Traces** | The PRD requirement(s) this ticket discharges — `FR-*`, `NFR-*`, `AC-*`, `D*`, or a `§` section. Every MUST-priority FR is traced by at least one ticket (see §Traceability). |
| **Phase** | The PRD §13.4 phase. `0`–`7`. Tickets marked `—` are cross-phase hygiene. |
| **Priority** | Inherited from the FR's Must/Should/Could where one exists; otherwise assigned from phase exit criteria. |
| **Size** | S ≈ under a day, M ≈ a few days, L ≈ 1–2 weeks, XL ≈ needs splitting before it is worked. |
| **Blocks / Depends** | Hard ordering only. Soft affinity is left to whoever picks up the work. |

Two conventions worth honouring, both taken from the PRD's own delivery rules (§13.1):

- **Borrow, do not build.** Tickets that wrap an existing library say so explicitly, and "evaluate and integrate X" is the deliverable, not "implement X".
- **`spec/` trails the daemon by one release.** Spec tickets for a phase land *after* that phase's implementation tickets, not before — except in Phase 0, where the spec artifact *is* the deliverable.

Capacity is solo with no fixed deadline (D30). Sizes are therefore serial-time estimates, and the phase order in §Suggested order is the real plan; anything labelled Phase 3+ is a placeholder that should be re-scoped when its phase opens rather than worked from this description.

---

## Epic index

| Epic | Title | Phase | Tickets |
| --- | --- | --- | --- |
| **E0** | Open decisions and de-risking spikes | 0 | CN-001 … CN-009 |
| **E1** | Repository, spec scaffolding, governance | 0, — | CN-010 … CN-017 |
| **E2** | Identity, transport, and eligibility | 1 | CN-020 … CN-029 |
| **E3** | Task contracts, submission, and control | 1 | CN-030 … CN-042 |
| **E4** | Execution island and runtime adapters | 1 | CN-050 … CN-059 |
| **E5** | Artifacts and content addressing | 1, 2 | CN-060 … CN-066 |
| **E6** | Verification and side effects | 1 | CN-070 … CN-076 |
| **E7** | Observability, audit, and accounting | 2, 7 | CN-080 … CN-087 |
| **E8** | Conformance suite and spec publication | 2 | CN-090 … CN-094 |
| **E9** | Resilience and failure semantics | 2 | CN-100 … CN-104 |
| **E10** | Task graphs and routing | 3 | CN-110 … CN-120 |
| **E11** | Egress policy and data sovereignty | 4 | CN-130 … CN-138 |
| **E12** | Delegation authority and recursion | 5 | CN-150 … CN-156 |
| **E13** | Participation tiers and reach | 6 | CN-170 … CN-174 |
| **E14** | Evaluation harness and metrics | 1–5 | CN-180 … CN-186 |
| **E15** | Non-functional verification | 1–2 | CN-190 … CN-194 |

---

## E0 — Open decisions and de-risking spikes

Everything in §16 of the PRD, as tickets. CN-001 gates the entire programme; the PRD says so itself ("The one thing to settle before Phase 0").

### CN-001 — Spike: prove the Bailment extension set is expressible as conformant A2A v1.0 extensions

**Traces:** D4, §16, Phase 0 exit criterion, Risk "A2A extensions cannot express the model"
**Phase:** 0 · **Priority:** MUST · **Size:** M · **Blocks:** every other ticket in E2–E13

Build a throwaway spike that declares all seven extensions (`bailment/constraints/v1`, `authority`, `budget`, `contract`, `artifacts`, `verification`, `receipts`) as real A2A v1.0 extensions and round-trips a task through the A2A Go SDK.

**Done when:**
- Each of the seven extension namespaces is declared and negotiated through the SDK's extension mechanism, not smuggled through free-form metadata.
- A task carrying all seven round-trips submit → accept → event stream → terminal result without violating A2A task semantics.
- Any extension that *cannot* be expressed is written up with the specific A2A constraint that blocks it and at least one alternative shape.
- The spike is deleted, not promoted. Its output is a written finding plus test vectors.

**Explicit kill condition:** if the constraints or authority extensions cannot be expressed, stop and escalate before Phase 1 opens. The PRD's whole architecture rests on this answer and it is cheap to get.

### CN-002 — Decide the product name and execute the rename

**Traces:** §16, D28, Readme "Name" note
**Phase:** 0 · **Priority:** MUST · **Size:** S · **Blocks:** CN-093

Resolved: the product is named **Bailment**, recorded in [`docs/adr/0001-product-name.md`](docs/adr/0001-product-name.md). The working name and the earlier `Hermes Federation` name were both retired; see the ADR for the candidates considered and the availability evidence.

**Done when:** name chosen and recorded in an ADR; find-and-replace applied across `spec/`, `node/`, `adapters/`, `README`, and module paths; domain and extension namespace availability checked.

### CN-003 — Decide and integrate the isolation runtime

**Traces:** §16, §13.5, D29, FR-032, NFR-007
**Phase:** 0 · **Priority:** MUST · **Size:** M · **Blocks:** CN-051

Evaluate OCI/runc, gVisor, Firecracker, and Kata against: host reach, startup latency per work package, egress confinement quality, and hostile-adapter containment.

**Done when:** ADR records the choice with measured startup latency and a containment test result for each candidate tried. PRD's recommended starting point is OCI for reach, with gVisor revisited when hostile adapters become a live threat model — deviate only with a stated reason.

### CN-004 — Decide and integrate the capability-token library

**Traces:** §16, D18, FR-036, FR-037, §13.1 "no unreviewed novel cryptography"
**Phase:** 0 · **Priority:** MUST · **Size:** M · **Blocks:** CN-150

Evaluate Biscuit, macaroons, and custom CBOR. PRD recommends Biscuit because attenuation is native rather than bolted on.

**Done when:** ADR records the choice; the library has a published audit history; a spike attenuates a token (narrow constraint, reduce budget, decrement depth) and verifies the chain offline against an originator key with no network call.

### CN-005 — Decide the task-contract registry model

**Traces:** §16, D16, FR-010, FR-014
**Phase:** 0 · **Priority:** SHOULD · **Size:** S

In-repo only for v0.1; define publication rules once a second implementer appears. Record the versioning and deprecation policy for contract IDs now, because `code-review/v1` becomes a compatibility commitment the moment anyone else implements it.

### CN-006 — Decide the audit anchoring posture

**Traces:** §16, FR-062, D24
**Phase:** 0 · **Priority:** SHOULD · **Size:** S · **Blocks:** CN-082

Local hash chain only for v1. The binding constraint: the chain format must not preclude later external anchoring. Record which fields make a chain segment externally anchorable so CN-082 builds them in from the start rather than migrating later.

### CN-007 — Decide the DLP implementation approach

**Traces:** §16, D6, FR-082
**Phase:** 0 · **Priority:** SHOULD · **Size:** S · **Blocks:** CN-132

Rules plus a documented external hook. Do not build a classifier — that is an explicit PRD instruction, and a classifier is a product in its own right.

### CN-008 — Decide spec governance and register the A2A extensions

**Traces:** §16, D28, Risk "Solo maintainer bus factor"
**Phase:** 0 · **Priority:** SHOULD · **Size:** M

Register the extension namespaces early; transfer stewardship at three independent implementations. Write the governance and change process down from day one — the PRD identifies bus factor as the top risk and written governance as its mitigation.

### CN-009 — Decide steering semantics

**Traces:** §16, FR-028, D25
**Phase:** 0 · **Priority:** SHOULD · **Size:** S · **Blocks:** CN-119

Contract-scoped amendments only, so steering cannot smuggle content past the egress gate. Free-text steering would create a second, ungated egress channel — decide against it explicitly and record why.

---

## E1 — Repository, spec scaffolding, governance

### CN-010 — Establish the monorepo layout

**Traces:** §7.4, D24
**Phase:** 0 · **Priority:** MUST · **Size:** S

Create `spec/`, `node/cmd/bailment`, `node/internal/{transport,identity,authority,policy,state,exec,cas,verify,audit,route}`, `adapters/{hermes-python,refworker}`, `sdk/`, `conformance/`. Go module, CI, lint, and test targets included in this ticket rather than split out.

**Done when:** `go build ./...` and `go test ./...` pass on an empty skeleton; CI runs both on push; `spec/` carries its own tag namespace (`spec-vX.Y`) distinct from implementation tags.

### CN-011 — Independent `spec/` versioning and release process

**Traces:** NFR-011, D24, §13.1
**Phase:** 0 · **Priority:** MUST · **Size:** S · **Depends:** CN-010

Semver for `spec/`, tagged independently of the daemon. Document the rule that `spec/` trails the daemon by one release until the protocol stops moving, and what "trails" means concretely for a contributor reading the repo.

### CN-012 — Written governance and change process

**Traces:** D28, Risk "Solo maintainer bus factor" · **Phase:** — · **Priority:** MUST · **Size:** S · **Depends:** CN-008

BDFL with a written change process, plus the stated intent to move `spec/` to a neutral foundation at three independent implementations. This is a credibility deliverable for an open-protocol play, not paperwork.

### CN-013 — JSON Schema 2020-12 for every envelope

**Traces:** §7.4, FR-020, FR-043, NFR-011
**Phase:** 0 · **Priority:** MUST · **Size:** M · **Depends:** CN-001

Schemas for task envelope, result envelope, Agent Card, capability token claims, artifact descriptor, every event in §8.3, and the receipt. Unknown optional fields must be preserved, not stripped (NFR-011).

**Done when:** every schema validates its own examples; a round-trip test proves unknown fields survive; schemas are the single source for `sdk/` generation.

### CN-014 — A2A extension declarations

**Traces:** §8.1, D4 · **Phase:** 0 · **Priority:** MUST · **Size:** M · **Depends:** CN-001

Publish the seven declarations under `spec/extensions/` in whatever form the A2A extension mechanism requires, with explicit version negotiation semantics per NFR-011.

### CN-015 — Signed conformance test vectors

**Traces:** §7.4, AC-014, NFR-014 · **Phase:** 0 · **Priority:** MUST · **Size:** M · **Depends:** CN-013

Signed vectors under `spec/vectors/` covering valid and invalid cases for each envelope and token shape. These are what let an independent implementer self-check before running the full suite (CN-091).

### CN-016 — Regenerate the two stale diagrams

**Traces:** §5 and §7 stale-asset notes · **Phase:** — · **Priority:** SHOULD · **Size:** S

`media/lifecycle.png` still shows the v0.1 two-phase offer/lease handshake removed by D21. `media/architecture.png` still shows a central registry, a shared artifact service, and separate client/gateway components, all removed in v0.2. Both are flagged in the PRD as stale; a reader who trusts the figures over the text gets the wrong architecture.

**Done when:** lifecycle shows single-phase submit with grants released on accept and no `OFFERED` state; architecture shows one symmetric daemon per site, holder-hosted artifacts, and an index as a file format rather than a service.

### CN-017 — Write the Phase 1 normative spec sections

**Traces:** §13.2, §8, AC-014 · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-013, CN-014; the implementation tickets it documents

Cover the constraints, contract, artifacts, and verification extensions in `spec/bailment-profile-v0.1.md`. Per §13.1 this lands *after* the corresponding daemon work, not before.

**Done when:** the four extensions are specified to the level where CN-093's independent implementer needs no access to the reference source.

---

## E2 — Identity, transport, and eligibility

### CN-020 — did:web resolution, caching, and organization key verification

**Traces:** FR-001, D5, §7.2 Identity · **Phase:** 1 · **Priority:** MUST · **Size:** M

Resolve `https://<org-domain>/.well-known/did.json`, verify the org signing key, cache with an explicit TTL and negative-cache policy.

**Done when:** resolution failure fails closed; cache respects TTL; key pinning is available for known counterparties (mitigation for the accepted DNS-compromise residual risk, §10.3).

### CN-021 — mTLS with certificate-to-did:web domain binding

**Traces:** FR-001, §7.3 Transport · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-020

**Done when:** a node presenting a certificate whose domain does not match its claimed organization identity is rejected *before any task content is composed* — the acceptance signal is ordering as much as outcome. Add replay protection (signed timestamps, nonces) at this layer.

### CN-022 — Serve and sign the Agent Card

**Traces:** FR-002, §5.2 · **Phase:** 1 · **Priority:** MUST · **Size:** M

Serve `/.well-known/agent-card.json` with node identity, supported task contracts, tools, data policy, endpoint, capacity limits, participation tier, and `hosted_by` when applicable. Signed by the org key.

### CN-023 — Validate a consumed Agent Card

**Traces:** FR-002, FR-011 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-020, CN-022

Validate signature against the org's did:web key, plus issuer, freshness, and schema, *before* routing. Separate `declared` from `observed` fields at parse time so no declared field can ever reach the ranking path (FR-011) — the type system should make that mistake impossible rather than merely discouraged.

### CN-024 — Short-lived node credentials and published revocation list

**Traces:** FR-003, §10.4 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-020

Node key signed by the org key, default credential lifetime 15 minutes, revocation list published at the org domain.

**Done when:** a revoked node fails verification on its next credential refresh; propagation is bounded by credential lifetime. Note the deliberate change from v0.1's 60 s, which assumed a CA that no longer exists — do not reintroduce the tighter number without reintroducing a CA.

### CN-025 — Bilateral allowlist trust model

**Traces:** FR-005, §3.2 · **Phase:** 1 · **Priority:** MUST · **Size:** S

**Done when:** a test proves A trusts B and B trusts C does not make C eligible for A. There is no code path that reads a peer's trust relations as input to local eligibility.

### CN-026 — Hard eligibility filters, evaluated pre-envelope

**Traces:** FR-004, FR-010, §9.1, D17, D10b · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-023, CN-025

Implement `eligible(node, task)` exactly as §9.1 specifies: verified did:web, bilateral allowlist, policy over the `union(org, hosted_by)` for the data class, contract and tool support, the attestation clause for classes above `PUBLIC`, and capacity freshness.

**Done when:** a node violating any hard constraint never receives task content, and neither does its host; filtering demonstrably runs before envelope composition, not after.

### CN-027 — Versioned, signed local policy bundles

**Traces:** FR-006 · **Phase:** 1 · **Priority:** SHOULD · **Size:** M

**Done when:** every task record references the exact policy version used for routing, so an audit can reconstruct the decision rather than re-derive it from current policy.

### CN-028 — Static peer configuration with no index

**Traces:** FR-016, §13.3 · **Phase:** 1 · **Priority:** MUST for v0.1 (FR priority SHOULD) · **Size:** S

Two operators who exchange domains out of band federate immediately. Indexes are deferred to Phase 4, so this is the *only* discovery mechanism in v0.1 and is on the Phase 1 critical path despite its SHOULD priority in §6.2.

### CN-029 — A2A JSON-RPC + SSE transport

**Traces:** §8.1, D4, NFR-002 · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-001, CN-021

Wire up the A2A Go SDK for JSON-RPC 2.0 over HTTPS with SSE. Borrow the SDK; do not hand-roll transport.

**Done when:** the only well-known HTTP paths served are `/.well-known/did.json`, `/.well-known/agent-card.json`, and the artifact and revocation endpoints. v0.1's REST surface (`POST /v1/tasks`, `POST /v1/task-offers`) is withdrawn and must not reappear — it contradicted the A2A compatibility claim.

---

## E3 — Task contracts, submission, and control

### CN-030 — Task contract as a versioned input/output schema pair

**Traces:** FR-010, FR-050, D16, §8.2 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-013

A capability is a contract ID plus an input schema, an output schema, a named validator set, and a verifiability class — never a free-text skill string.

**Done when:** eligibility matching on contract ID is an exact machine-checkable predicate; there is no code path where a free-text skill affects eligibility.

### CN-031 — Define the `code-review/v1` contract

**Traces:** §13.2, D27, §12.3 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-030

Class `DETERMINISTIC`. Input schema, output schema, named validator set. This one contract done completely is the whole of v0.1 (§13.1).

### CN-032 — Validators for `code-review/v1`: apply-patch and run-tests

**Traces:** §13.2, FR-051, AC-003, §12.3 · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-031, CN-071

Deterministic, reproducible, runnable in the consumer's own sandbox. Static analysis and benchmark thresholds are in the §12.3 evaluation suite; apply-patch and run-tests are the acceptance-critical pair.

### CN-033 — Task envelope: versioned, references-only

**Traces:** FR-020, FR-080, D6 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-013, CN-030

Contract ID, constraints, budget, capability token, verification policy, and artifact *references only*.

**Done when:** envelopes validate against the published schema and a test proves no plaintext above `PUBLIC` class can be placed inline. This property is what made D21's offer/lease pre-flight unnecessary — if inline payloads creep back, the single-phase submit stops being safe.

### CN-034 — Single-phase submit, lease on accept, grants after accept

**Traces:** FR-021, D21, NFR-002, §5.1 · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-029, CN-033

Submit in one call. On acceptance the participant returns a capacity lease and a stream cursor. The originator issues artifact grants scoped to that participant *only after* acceptance.

**Done when:** a refusal transfers no protected bytes and costs one round trip; there is no `OFFERED` state anywhere in the state machine.

### CN-035 — Idempotent submission and result retrieval

**Traces:** FR-022, §14.2 "Replay attempt" · **Phase:** 1 · **Priority:** MUST · **Size:** M

**Done when:** repeating a request with the same idempotency key returns the original task state and does not create duplicate execution.

### CN-036 — Task state machine

**Traces:** §5.3, §9.4 · **Phase:** 1 · **Priority:** MUST · **Size:** M

`CREATED`, `SUBMITTED`, `ACCEPTED`, `RUNNING`, `VERIFYING`, `COMPLETED`, `PARTIAL`, `FAILED`, `CANCELLED`, `UNKNOWN`. No `OFFERED` (D21).

**Done when:** `UNKNOWN` is reachable and terminal, and is used whenever the owner disappeared or side effects cannot be proven — the machine must be unable to launder ambiguity into a clean `FAILED`.

### CN-037 — Sequence-numbered, resumable event stream

**Traces:** FR-023, D7, AC-007, NFR-003, §8.3 · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-029, CN-040

All ten events from §8.3, each carrying `task_id`, monotonic `seq`, and `emitted_at`. Resume from cursor.

**Done when:** a client that disconnects for 10 minutes reconnects and receives every missed event exactly once, in order (AC-007). Events are visible at the consumer p95 under 2 s (NFR-003).

### CN-038 — Cancellation with explicit acknowledgement

**Traces:** FR-024, NFR-005, §14.2 "Cancellation during a model call" · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-036

**Done when:** the originator can distinguish cancelled, completed, and unknown execution; cancellation reaches an online worker p95 under 5 s; a worker cancelled mid-model-call stops at a safe boundary and reports partial artifacts.

### CN-039 — Budgets as attenuating caveats

**Traces:** FR-025, §8.1 `bailment/budget/v1` · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-004

Wall time, model calls, tool calls, and storage, expressed as token caveats that subdivide on re-delegation.

**Done when:** the participant enforces the stricter of received and local limits; budget exhaustion cancels the work package's context. The child-cannot-exceed-parent property is tested in Phase 5 (CN-151) but the caveat structure must be right now.

### CN-040 — Durable local state surviving restart and client detach

**Traces:** D7, §7.2 State, NFR-012 · **Phase:** 1 · **Priority:** MUST · **Size:** L

Durable DAG, lease, budget, and cursor state.

**Done when:** the daemon restarts mid-task and resumes without losing lease, budget, or cursor; a client detaches and reattaches to a still-running task. The daemon runs detached and survives client disconnects (NFR-012).

### CN-041 — Machine-readable refusal

**Traces:** FR-034, §14.2 "Node refusal" · **Phase:** 1 · **Priority:** MUST · **Size:** S

**Done when:** the originator can reroute or explain a refusal without parsing natural language; policy and capacity refusals are distinguishable error codes.

### CN-042 — Heartbeat and failure detection

**Traces:** NFR-004, §9.4, §8.3 `task.heartbeat` · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-037

Heartbeat every 10 s; node considered unavailable after 30 s plus configurable grace.

**Done when:** an expired lease heartbeat stops follow-up routing, marks the task suspect, and waits a bounded grace period before failover — it does not immediately fail the task.

---

## E4 — Execution island and runtime adapters

### CN-050 — Worker-adapter interface over local RPC

**Traces:** FR-070, D8, A.2 · **Phase:** 1 · **Priority:** MUST · **Size:** L

Implement the `WorkerAdapter` / `WorkPackage` / `EventSink` seam sketched in A.2: receive work package, emit progress, emit artifacts, report terminal result, request delegation.

**Done when:** an adapter can be written in any language without linking the daemon; the daemon fetches referenced artifacts and presents them as local paths; the adapter cannot open a socket, read outside the sandbox, mint a token, widen a constraint, or publish an artifact without the daemon labelling and hashing it.

### CN-051 — Sandbox creation and adapter lifecycle

**Traces:** FR-032, NFR-007, D29, §7.2 Execution · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-003, CN-050

The daemon creates the sandbox and launches the adapter inside it. Tool allowlists, network policy, filesystem policy, and resource limits are enforced in the daemon, outside the adapter's reach.

**Done when:** a deliberately hostile adapter cannot exceed node policy, and the isolation guarantee provably does not depend on adapter quality (NFR-007). Covers AC-004: a policy-violating tool request is denied by the participant daemon *even when the originator requests it*.

### CN-052 — Egress confinement in the daemon

**Traces:** FR-032, §10.3 "Prompt injection", §14.2 "Hostile worker adapter" · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-051

**Done when:** an adapter attempting direct network egress or filesystem escape is contained; injected instructions in an artifact do not expand tools, network, or data permissions.

### CN-053 — Hermes Python worker adapter

**Traces:** FR-071, D8, §13.2, A.4 · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-050

Expose delegation, status, and cancellation to a standard Hermes session.

**Done when:** a standard Hermes session runs a federated task with no edits to the Hermes core loop. Adapter #1, not the substrate — the daemon must not acquire any Hermes-specific dependency.

### CN-054 — Minimal reference worker

**Traces:** FR-072, D8 · **Phase:** 3 · **Priority:** SHOULD · **Size:** M · **Depends:** CN-050

A plain OpenAI-compatible loop that proves the adapter seam is real rather than notional.

**Done when:** both adapters pass the same adapter conformance profile (CN-092) running the same contract unmodified.

### CN-055 — Keep the participant boundary tight

**Traces:** FR-031, §3.2 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-051

Local model API calls, tool calls, memory, secrets, and private data access stay inside the participant boundary.

**Done when:** an inspection of federation traces finds no raw local secrets and no unapproved data. No originator-to-worker secret forwarding except explicitly scoped task credentials (§10.4).

### CN-056 — Result shape: rationale, claims, artifacts, evidence — no hidden reasoning

**Traces:** FR-033, §3.4 · **Phase:** 1 · **Priority:** MUST · **Size:** S · **Depends:** CN-030

**Done when:** no protocol schema has a mandatory chain-of-thought field. Participants are never required to reveal hidden reasoning.

### CN-057 — Propagate session and task identity without conversation history

**Traces:** FR-073, FR-060 · **Phase:** 1 · **Priority:** SHOULD · **Size:** S

Correlation preserved while context stays minimized.

### CN-058 — Task affinity to a local worker/model replica

**Traces:** FR-035 · **Phase:** 3 · **Priority:** SHOULD · **Size:** M

Preserve affinity while the lease remains valid so follow-up context can reuse the local session and cache.

### CN-059 — Model-requested contracts via the delegation tool schema

**Traces:** FR-074 · **Phase:** 3 · **Priority:** COULD · **Size:** M

**Done when:** the model may request a contract but demonstrably cannot bypass eligibility filters.

---

## E5 — Artifacts and content addressing

### CN-060 — Content-addressed store, holder-served, pulled by digest

**Traces:** FR-040, D22, §7.1 · **Phase:** 1 · **Priority:** MUST · **Size:** L

Each node serves its own artifacts. There is no shared artifact service to trust or operate — v0.1's was deleted in v0.2.

**Done when:** digest verification fails closed on mismatch (AC-005: a tampered artifact or mismatched digest is rejected and recorded without entering synthesis).

### CN-061 — Artifact descriptor metadata

**Traces:** FR-041 · **Phase:** 1 · **Priority:** MUST · **Size:** S · **Depends:** CN-013, CN-060

Type, size, digest, producer, policy label, retention, and access scope on every artifact.

### CN-062 — Resumable transfer and bounded retention

**Traces:** FR-042 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-060

**Done when:** an interrupted transfer resumes without duplicating completed chunks; retention bounds are enforced rather than advisory.

### CN-063 — Validate result envelopes against the contract output schema

**Traces:** FR-043, §5.1 step 8 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-030

**Done when:** a malformed result cannot enter verification or synthesis as an accepted output.

### CN-064 — Encrypt artifacts to the recipient key; scope grants by capability token

**Traces:** FR-044, §14.2 "Unauthorized artifact retrieval" · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-004, CN-060

**Done when:** a relay or mirror in the path never sees plaintext; an unauthorized retrieval is denied *and audited*, with the audit record showing the grant did not cover the requester.

### CN-065 — Consumer-side pinning of every dependency before completion

**Traces:** FR-046, AC-006, §9.4, §14.2 "Producer offline after completion" · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-060

Fetch and retain every artifact a task depends on *before* marking that task complete.

**Done when:** a producer going offline after completion does not orphan the originator's dependencies; pinned dependencies still resolve and unpinned references fail loudly rather than silently.

### CN-066 — Holder-hosted artifacts for local-only workflows

**Traces:** FR-045, D15 · **Phase:** 4 · **Priority:** SHOULD · **Size:** M

The originator passes an opaque reference to an approved verifier without downloading raw data.

---

## E6 — Verification and side effects

### CN-070 — Verifiability class on every contract and every result

**Traces:** FR-050, D14 · **Phase:** 1 · **Priority:** MUST · **Size:** S · **Depends:** CN-030

`DETERMINISTIC`, `ATTESTED_LOCAL`, or `UNVERIFIED`, travelling in the result envelope so a consumer always knows what it is trusting.

### CN-071 — Run deterministic validators in the consumer's own sandbox

**Traces:** FR-051, AC-003, D14, §7.2 Verification · **Phase:** 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-051

**Done when:** a remote patch is never merged on the producer's report; acceptance is reproducible by the consumer alone. The `VERIFYING` state runs in the *originator's* sandbox, not the producer's.

### CN-072 — Separate claims from evidence

**Traces:** FR-052 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-063

**Done when:** verification can accept, reject, or qualify individual claims, and the final result identifies unsupported or disputed ones.

### CN-073 — Propose-only side effects with explicit approval

**Traces:** FR-053, §10.4, §3.2 · **Phase:** 1 · **Priority:** MUST · **Size:** M

Remote workers return proposals or signed plans unless delegated a scoped execution capability.

**Done when:** every external write, deployment, message, merge, or infrastructure change requires human approval; unattended cross-organization side effects are impossible, not merely discouraged.

### CN-074 — Restrict v1 federation to verifiable classes

**Traces:** FR-056, D14, D27 · **Phase:** 1 · **Priority:** MUST · **Size:** S · **Depends:** CN-070

`DETERMINISTIC`, or `ATTESTED_LOCAL` for local-only work. `UNVERIFIED` contracts require an explicit operator override and are marked in the result.

**Done when:** the word "verified" in every product surface corresponds to a reproducible check.

### CN-075 — Expose provenance to the synthesizer without flooding context

**Traces:** FR-055 · **Phase:** 1 · **Priority:** SHOULD · **Size:** M

Summaries, artifacts, evidence, and key events by default — not full remote transcripts.

### CN-076 — Multi-party proposal, critique, and adjudication

**Traces:** FR-054 · **Phase:** post-v1 · **Priority:** COULD · **Size:** L

Demoted in v0.2: an independent model reviewer produces an opinion, not evidence. Do not schedule this ahead of anything deterministic.

---

## E7 — Observability, audit, and accounting

### CN-080 — Correlation ID across the whole path

**Traces:** FR-060, NFR-010 · **Phase:** 2 · **Priority:** MUST · **Size:** M

Session, task graph, remote task, contract, tools, artifacts, verification.

**Done when:** an operator reconstructs the complete path from one ID, with no prompt content in shared telemetry.

### CN-081 — Metrics collection

**Traces:** FR-061, §12.1, §12.2 · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-080

Task latency, queue time, execution time, usage, failures, retries, verification outcome, artifact volume.

**Done when:** declared metrics (model calls, tokens, tool calls, GPU-hours) are labelled non-normative per D19 and are structurally separated from observed metrics, so nothing downstream can mistake one for the other.

### CN-082 — Hash-chained append-only audit log

**Traces:** FR-062, AC-013, §14.2 "Audit export" · **Phase:** 2 · **Priority:** MUST · **Size:** L · **Depends:** CN-006

**Done when:** any gap or reordering is detectable on export; the chain format does not preclude later external anchoring; a reviewer reconstructs the path without access to hidden reasoning or unrelated private data.

### CN-083 — Quotas and admission control

**Traces:** FR-063, §10.3 "Denial of service" · **Phase:** 2 · **Priority:** MUST · **Size:** M

By organization, node, contract, concurrent tasks, and usage units.

**Done when:** a node rejects or queues excess work predictably, with a machine-readable reason (CN-041).

### CN-084 — Health, maintenance, draining, and incident states

**Traces:** FR-064, FR-012 · **Phase:** 2 · **Priority:** SHOULD · **Size:** M

Schedulers stop assigning new work while allowing safe completion or migration.

### CN-085 — Co-signed receipts over observable units

**Traces:** FR-066, D11b, D13, §8.2 · **Phase:** 7 · **Priority:** MUST (Phase 7) · **Size:** L · **Depends:** CN-082

Lease duration, task count, verified completions, artifact bytes, refusals. Signed by both parties.

**Done when:** neither party can unilaterally inflate a receipt; receipts reconcile across both parties' audit chains. Nothing prices or settles anything — money is out of v1 scope entirely (D13), and this ticket must not grow a pricing field.

### CN-086 — Contribution reports

**Traces:** FR-065, D13 · **Phase:** 7 · **Priority:** COULD · **Size:** M · **Depends:** CN-085

Informational only. Contribution confers no priority and no governance weight in v1.

### CN-087 — Audit export tool

**Traces:** AC-013, §14.2 "Audit export" · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-082

**Done when:** every accepted result traces to node identity, contract, policy version, events, artifacts, and validator outcome, with declared and observed facts distinguishable in the export.

---

## E8 — Conformance suite and spec publication

### CN-090 — Conformance runner

**Traces:** §7.4, §13.2, Risk "Nobody adopts the profile" · **Phase:** 2 · **Priority:** MUST · **Size:** L

Point it at any endpoint. A first-class Phase 2 deliverable, not an afterthought — it is the PRD's primary mitigation for adoption risk.

### CN-091 — Core profile conformance tests

**Traces:** AC-014, NFR-014, §13.2 · **Phase:** 2 · **Priority:** MUST · **Size:** L · **Depends:** CN-015, CN-090

Cover identity, card validation, submit/accept, events and resume, cancellation, artifacts and digests, verification classes, refusals.

**Done when:** the suite runs green against a second, independently built endpoint.

### CN-092 — Adapter conformance profile

**Traces:** FR-070, FR-072 · **Phase:** 3 · **Priority:** SHOULD · **Size:** M · **Depends:** CN-053, CN-054, CN-090

Both the Hermes adapter and the reference worker pass the same profile.

### CN-093 — Publish the spec and validate with an independent implementer

**Traces:** AC-014, NFR-014, §12.1 "Independent implementations" · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-002, CN-017, CN-091

**Done when:** an independent implementer passes the core conformance profile using `spec/` alone, without reading the reference source.

### CN-094 — Onboarding path: one binary, one config file, one DNS record

**Traces:** NFR-013, G8, §5.2, §12.1 "Onboarding time" · **Phase:** 2 · **Priority:** MUST · **Size:** M

**Done when:** a new organization goes from download to first accepted task in under 30 minutes, unassisted and with no approval from anyone. Measure it on someone who has not seen the repo; self-timing does not count.

---

## E9 — Resilience and failure semantics

### CN-100 — `UNKNOWN` semantics and the side-effect state machine

**Traces:** §9.4, §5.3, AC-006, §10.3 "Ambiguous side effects" · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-036

Represent uncertain execution as `UNKNOWN` rather than pretending the work failed cleanly.

**Done when:** automatic retry is impossible from `UNKNOWN`; a parent that cannot confirm a child's terminal state reports `UNKNOWN` upward rather than absorbing the ambiguity.

### CN-101 — Suspect leases, grace period, and failover

**Traces:** §9.4, NFR-004, AC-006 · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-042, CN-100

**Done when:** a participant becoming unavailable mid-task is detected, pinned dependencies remain retrievable, and the workflow either completes or reports `UNKNOWN` honestly (AC-006).

### CN-102 — Retry only when retryable and side effects are unambiguous

**Traces:** §9.4, FR-022 · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-035, CN-100

**Done when:** heartbeat loss triggers policy-driven retry with no duplicate unsafe side effects.

### CN-103 — Injected-failure test harness

**Traces:** §12.1 "Recovery rate", §12.3 Resilience suite, AC-006 · **Phase:** 2 · **Priority:** MUST · **Size:** L · **Depends:** CN-101

**Done when:** ≥95% of retryable node failures are rerouted without user restart, measured by the harness rather than asserted.

### CN-104 — Replay and duplication protection

**Traces:** §10.3 "Replay or duplication", §14.2 "Replay attempt" · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-021, CN-035

Idempotency keys, signed timestamps and nonces, side-effect state machine.

---

## E10 — Task graphs and routing

Phase 3. Re-scope these against reality when the phase opens; §13.3 defers scheduler ranking, hedging, and fan-out until now.

### CN-110 — Dependency-aware DAG execution

**Traces:** FR-026, AC-008 · **Phase:** 3 · **Priority:** MUST · **Size:** L · **Depends:** CN-040

A task starts only after its declared dependencies satisfy completion rules.

### CN-111 — Parallel fan-out and reduce

**Traces:** FR-026, §9.3 · **Phase:** 3 · **Priority:** MUST · **Size:** M · **Depends:** CN-110

**Done when:** a graph across three organizations completes with deterministic ordering and full provenance (AC-008).

### CN-112 — Locally observed reputation ranking

**Traces:** FR-011, D26, §9.1 · **Phase:** 3 · **Priority:** MUST · **Size:** L · **Depends:** CN-081

Implement `rank()` from §9.1 using only locally observed signals: verification pass rate, acceptance rate, lease-honoured rate, minus time-to-first-event, straggler rate, and recent failure penalty.

**Done when:** routing logs show every declared claim as declared, and no declared field participates in rank. v0.1's `score()` — capability quality, cost, predicted queue time — is not reintroduced; nothing trustworthy was left in that formula.

### CN-113 — Trial routing for cold-start nodes

**Traces:** §9.1 · **Phase:** 3 · **Priority:** MUST · **Size:** M · **Depends:** CN-112

An unknown node begins with a neutral prior and earns history through low-value `PUBLIC`-class tasks. Cold start is the normal case in an open network, so this is not an edge case.

### CN-114 — Live capacity advertisement and freshness

**Traces:** FR-012, §9.1 · **Phase:** 3 · **Priority:** MUST · **Size:** M · **Depends:** CN-022

Queue depth, concurrency, maintenance state, expected start delay.

**Done when:** stale capacity is excluded after the configured heartbeat threshold.

### CN-115 — Manual target selection and pinning

**Traces:** FR-013 · **Phase:** 3 · **Priority:** SHOULD · **Size:** S

**Done when:** the user can force an eligible node and see the specific reason when a node is ineligible.

### CN-116 — Multiple contracts and models per organization

**Traces:** FR-014 · **Phase:** 3 · **Priority:** SHOULD · **Size:** M

One node group exposes coding, science, verifier, and domain contracts independently.

### CN-117 — Second task contract

**Traces:** §13.4 Phase 3, §12.3 Security suite · **Phase:** 3 · **Priority:** MUST · **Size:** L · **Depends:** CN-030

Candidates from A.1: `code-impl/v1` or `test-plan/v1`. Must be `DETERMINISTIC` with real validators, per FR-056.

### CN-118 — Granularity advisory warning

**Traces:** FR-027, §9.2 · **Phase:** 3 · **Priority:** SHOULD · **Size:** S

Warn on delegation too fine to amortize WAN overhead. Warn, do not block — demoted to advisory in v0.2.

### CN-119 — Contract-scoped steering

**Traces:** FR-028, §16 · **Phase:** 3 · **Priority:** SHOULD · **Size:** M · **Depends:** CN-009

Redirect a running remote task at a safe iteration boundary without exposing the full parent transcript, and without a free-text channel that bypasses the egress gate.

### CN-120 — Hedged or replicated execution

**Traces:** FR-029 · **Phase:** post-v1 · **Priority:** COULD · **Size:** L

Demoted: depends on capacity signals v0.1 does not trust.

---

## E11 — Egress policy and data sovereignty

Phase 4.

### CN-130 — Structural labelling at ingest, immutable thereafter

**Traces:** FR-080, D6 · **Phase:** 4 · **Priority:** MUST · **Size:** L

Labels assigned at ingest by the data owner; context permitted by reference to labelled artifacts only.

**Done when:** no model-authored free text can carry a payload above the task's declared class.

### CN-131 — Size-cap and deterministically scan free-text envelope fields

**Traces:** FR-081 · **Phase:** 4 · **Priority:** MUST · **Size:** M · **Depends:** CN-130

**Done when:** oversized or flagged text blocks submission rather than truncating silently — silent truncation would be an exfiltration channel with a plausible-looking success path.

### CN-132 — DLP check over every outbound envelope

**Traces:** FR-082, D6 · **Phase:** 4 · **Priority:** SHOULD · **Size:** M · **Depends:** CN-007

Rules plus a documented external hook. Escalate flags to a human regardless of data class.

**Done when:** a flagged envelope never leaves unattended.

### CN-133 — Egress gate by data class and side effect

**Traces:** FR-083, D25, §10.2 · **Phase:** 4 · **Priority:** MUST · **Size:** L · **Depends:** CN-130

`PUBLIC`/`CONSORTIUM` flow automatically. `RESTRICTED`/`LOCAL-ONLY` require approval once per counterparty per contract. Every external side effect always requires approval.

**Done when:** a 20-way fan-out to an approved org and contract prompts once, not twenty times — the per-counterparty-per-contract granularity is the requirement, not an optimization.

### CN-134 — Record every approval, denial, and DLP verdict in the audit chain

**Traces:** FR-084, AC-010 · **Phase:** 4 · **Priority:** MUST · **Size:** M · **Depends:** CN-082, CN-133

**Done when:** a reviewer can reconstruct who allowed what and when; an envelope carrying content above its declared class is blocked before transmission *and* audited (AC-010).

### CN-135 — Approval surface

**Traces:** §13.3 "DLP and approval UI", FR-083 · **Phase:** 4 · **Priority:** MUST · **Size:** L · **Depends:** CN-133

Whatever an operator actually uses to grant a per-counterparty-per-contract approval and to review escalated DLP flags.

### CN-136 — Ship-code-not-goals contract for `LOCAL-ONLY` work

**Traces:** D15, AC-009, §4.3, §10.2 · **Phase:** 4 · **Priority:** MUST · **Size:** XL · **Depends:** CN-030, CN-066

The originator sends a program and a strict output schema rather than a prose goal. The remote agent may write the program, but the program is the artifact under review, its digest is in the provenance, and returned values are schema-validated, range-checked, and aggregate-checked.

**Done when:** a `LOCAL-ONLY` dataset is analyzed at its owning organization via shipped code and schema; raw records never leave; returned values pass schema, aggregate, and range checks (AC-009). **Split before working** — this is a contract, a verification mode, and a trust argument in one ticket.

### CN-137 — Signed index format, treated as untrusted hints

**Traces:** FR-015, D17, §14.2 "Index poisoning" · **Phase:** 4 · **Priority:** MUST · **Size:** L · **Depends:** CN-023

A replicable file format anyone may host. Not a service — v0.1's capability registry was deleted.

**Done when:** a malicious index can waste time but cannot make an untrusted org eligible; a forged entry fails did:web verification and never becomes eligible; every entry is re-verified against did:web before use.

### CN-138 — Enforce the four data classes end to end

**Traces:** §10.2, FR-004 · **Phase:** 4 · **Priority:** MUST · **Size:** M · **Depends:** CN-026, CN-133

`PUBLIC`, `CONSORTIUM`, `RESTRICTED`, `LOCAL-ONLY` — each with its delegation policy, egress gate, and artifact policy per the §10.2 table.

---

## E12 — Delegation authority and recursion

Phase 5. §13.5 makes a security review of daemon sandboxing, token attenuation, and egress confinement a hard gate *before* this phase enables recursion (CN-156). §13.3 adds: never before attenuable tokens ship.

### CN-150 — Mint attenuable capability tokens

**Traces:** FR-036, D18, §8.2 · **Phase:** 5 · **Priority:** MUST · **Size:** L · **Depends:** CN-004

Every task carries a token stating constraints, budget caveats, delegation depth, and permitted sub-delegate set.

**Done when:** the token is verifiable to the originator's did:web key by any node in the chain, offline.

### CN-151 — Attenuation-only enforcement

**Traces:** FR-037, AC-011, §10.3 "Constraint laundering at depth" · **Phase:** 5 · **Priority:** MUST · **Size:** L · **Depends:** CN-150

A holder may narrow constraints, reduce budget, and decrement depth — never widen.

**Done when:** a grandchild presented with a widened token rejects it at accept time, by verifying the chain to the originator's key, with **no round trip to the originator** (AC-011); the attempt is recorded in both audit chains. Also proves FR-025's child-cannot-exceed-parent budget property.

### CN-152 — `max_delegation_depth` defaults to 0

**Traces:** FR-038, D3, §10.4 · **Phase:** 5 · **Priority:** MUST · **Size:** S · **Depends:** CN-150

Recursion is opt-in per task.

**Done when:** a task at depth 0 that attempts re-delegation fails with a machine-readable error.

### CN-153 — Signed provenance chain

**Traces:** FR-039, §8.3 `task.completed` · **Phase:** 5 · **Priority:** MUST · **Size:** M · **Depends:** CN-150

**Done when:** the originator can enumerate every organization that handled the task, not merely the direct producer.

### CN-154 — Offline chain verification at accept time

**Traces:** AC-011, D18 · **Phase:** 5 · **Priority:** MUST · **Size:** M · **Depends:** CN-151

Verification happens locally at the receiving node, at accept time. This property is what makes recursion safe; a design that needs to ask the originator has failed.

### CN-155 — `task.delegated` disclosure and cancellation propagation

**Traces:** §8.3, §9.4 · **Phase:** 5 · **Priority:** MUST · **Size:** M · **Depends:** CN-038, CN-153

`task.delegated` carries `child_task_id`, `delegate_org`, `remaining_depth` and discloses re-delegation to the originator in real time.

**Done when:** cancellation propagates down the chain; a parent that cannot confirm its child's terminal state reports `UNKNOWN` upward.

### CN-156 — Security review gate before recursion is enabled

**Traces:** §13.5, Risk "Security-critical code with no reviewer" · **Phase:** 5 · **Priority:** MUST · **Size:** M · **Blocks:** shipping any of CN-150 … CN-155

External review of daemon sandboxing, token attenuation, and egress confinement. The PRD makes this a precondition, not a follow-up, and the solo-maintainer risk register names "security-critical code with no reviewer" explicitly.

---

## E13 — Participation tiers and reach

Phase 6.

### CN-170 — Four participation tiers

**Traces:** FR-007, D9 · **Phase:** 6 · **Priority:** MUST · **Size:** M

Standard, anchor (SLA/capacity class), infrastructure-role holder, hosted. Declared in the card, verifiable, consumable by eligibility filters.

### CN-171 — `hosted_by` union eligibility

**Traces:** FR-008, D10b, AC-012, §7.3 · **Phase:** 6 · **Priority:** MUST · **Size:** M · **Depends:** CN-026, CN-170

A hosted node's effective recipient set is `union(node org, host org)`.

**Done when:** excluding org H from a task automatically excludes every node H hosts, without the originator knowing the hosting map in advance.

### CN-172 — TEE attestation evidence and class-gated policy

**Traces:** FR-009, D10, D10b, AC-012 · **Phase:** 6 · **Priority:** SHOULD · **Size:** L · **Depends:** CN-171

Optional attestation in the card; policy may require it for classes above `PUBLIC`.

**Done when:** an attested node is eligible for `RESTRICTED`; a non-attested hosted node is not; a `RESTRICTED` task is refused routing to a hosted node without valid attestation purely on the disclosed union (AC-012). Ampere-class hardware remains a first-class citizen at `PUBLIC`/`CONSORTIUM` — D10b corrected the original TEE assumption, and the union policy, not attestation, is the mandatory baseline.

### CN-173 — End-to-end-encrypted relay for firewalled nodes

**Traces:** D20, §7.3 Relay operator · **Phase:** 6 · **Priority:** SHOULD · **Size:** L · **Depends:** CN-064

**Done when:** the relay sees routing metadata, sizes, and timing but never payloads; payloads are encrypted to the destination node's key.

### CN-174 — Anchor SLA class and infrastructure roles

**Traces:** FR-007, D9, §4.1 "Anchor operator" · **Phase:** 6 · **Priority:** SHOULD · **Size:** M · **Depends:** CN-170

Serve high concurrency, publish an index, relay for firewalled peers under a declared SLA.

---

## E14 — Evaluation harness and metrics

### CN-180 — Three-way evaluation harness

**Traces:** §12.3 · **Phase:** 1 · **Priority:** MUST · **Size:** L

Compare local-only, best single remote specialist, and a federated graph, per task.

**Done when:** every task in the suite has a validator, not a rubric — that is what deterministic-only means. A task without a validator cannot enter the suite.

### CN-181 — Software-engineering suite area

**Traces:** §12.3 Phase 1 · **Priority:** MUST · **Size:** L · **Depends:** CN-032, CN-180

Implement, review, and benchmark a multi-file change. Validators: apply patch, run tests, static analysis, benchmark thresholds.

### CN-182 — Resilience suite area

**Traces:** §12.3 Phase 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-103

Complete a task while one participant fails mid-run. Validators: state-machine and recovery assertions.

### CN-183 — Security suite area

**Traces:** §12.3 Phase 3 · **Priority:** SHOULD · **Size:** L · **Depends:** CN-117

Find and remediate a vulnerability in a service. Validators: independent scanner, exploit test, regression suite.

### CN-184 — Data-sovereign suite area

**Traces:** §12.3 Phase 4, AC-009 · **Priority:** MUST (Phase 4) · **Size:** L · **Depends:** CN-136

Compute findings over a local-only dataset via shipped code. Validators: output schema, declared aggregates, range and null checks.

### CN-185 — Authority suite area

**Traces:** §12.3 Phase 5, AC-011 · **Priority:** MUST (Phase 5) · **Size:** M · **Depends:** CN-151

Attempt constraint widening at depth 2. Validator: grandchild rejects, audit records the attempt.

### CN-186 — North-star and product metric instrumentation

**Traces:** §12 north star, §12.1 · **Phase:** 2 · **Priority:** MUST · **Size:** M · **Depends:** CN-081

Federated completion rate (≥85% target), quality lift, time-to-verified-result, coordination overhead ratio (<10% median), verification cost ratio, recovery rate (≥95%), evidence coverage (100% for `DETERMINISTIC`), policy violation rate (zero), onboarding time (<30 min).

**Done when:** coordination overhead **excludes verification and reports it separately** — deterministic validation is product value, not overhead, and folding it in would make the headline metric lie about the product's core claim.

---

## E15 — Non-functional verification

### CN-190 — No central service whose failure stops the network

**Traces:** NFR-001, G4, §7.1 · **Phase:** 1 · **Priority:** MUST · **Size:** M

**Done when:** an architecture test asserts there is no shared service dependency in any task path, and a participant failure does not terminate unrelated tasks. v0.1's registry, scheduler, artifact service, and verification service are internal packages or file formats — a test should fail if any of them regains a network dependency.

### CN-191 — Submission latency budget

**Traces:** NFR-002, D21 · **Phase:** 1 · **Priority:** MUST · **Size:** S · **Depends:** CN-034

Single-phase submit-and-accept p95 under 1 s on a healthy WAN, excluding participant queueing and artifact transfer. One round trip, not two.

### CN-192 — Integrity failures are terminal and auditable

**Traces:** NFR-008, AC-005 · **Phase:** 1 · **Priority:** MUST · **Size:** S · **Depends:** CN-060, CN-082

All task-critical artifacts, results, tokens, and receipts digest-verified.

### CN-193 — Platform and compatibility targets

**Traces:** NFR-009, D23, §7.5 · **Phase:** 1 · **Priority:** MUST · **Size:** M · **Depends:** CN-010

Linux nodes; a single static Go binary; adapters in any language over local RPC; OpenAI-compatible local model servers.

**Done when:** CI produces a static binary with no runtime to install; the Python adapter is not a build dependency of the daemon.

### CN-194 — Scale design validation

**Traces:** NFR-006 · **Phase:** 2 · **Priority:** SHOULD · **Size:** M · **Depends:** CN-190

v0.1 ships 2 nodes. The v1 *design* target is 100 organizations and 10,000 concurrent tasks with no central bottleneck by construction.

**Done when:** the no-central-bottleneck claim is argued from architecture and spot-checked by load-testing a single daemon's task ceiling — not asserted from a two-node run.

---

## Traceability

### Functional requirements → tickets

| FR | Ticket(s) | FR | Ticket(s) |
| --- | --- | --- | --- |
| FR-001 | CN-020, CN-021 | FR-043 | CN-063 |
| FR-002 | CN-022, CN-023 | FR-044 | CN-064 |
| FR-003 | CN-024 | FR-045 | CN-066 |
| FR-004 | CN-026, CN-138 | FR-046 | CN-065 |
| FR-005 | CN-025 | FR-050 | CN-070 |
| FR-006 | CN-027 | FR-051 | CN-071 |
| FR-007 | CN-170, CN-174 | FR-052 | CN-072 |
| FR-008 | CN-171 | FR-053 | CN-073 |
| FR-009 | CN-172 | FR-054 | CN-076 |
| FR-010 | CN-026, CN-030 | FR-055 | CN-075 |
| FR-011 | CN-023, CN-112 | FR-056 | CN-074 |
| FR-012 | CN-114, CN-084 | FR-060 | CN-080, CN-057 |
| FR-013 | CN-115 | FR-061 | CN-081 |
| FR-014 | CN-116, CN-005 | FR-062 | CN-082 |
| FR-015 | CN-137 | FR-063 | CN-083 |
| FR-016 | CN-028 | FR-064 | CN-084 |
| FR-020 | CN-033, CN-013 | FR-065 | CN-086 |
| FR-021 | CN-034 | FR-066 | CN-085 |
| FR-022 | CN-035, CN-102 | FR-070 | CN-050, CN-092 |
| FR-023 | CN-037 | FR-071 | CN-053 |
| FR-024 | CN-038 | FR-072 | CN-054, CN-092 |
| FR-025 | CN-039, CN-151 | FR-073 | CN-057 |
| FR-026 | CN-110, CN-111 | FR-074 | CN-059 |
| FR-027 | CN-118 | FR-080 | CN-130, CN-033 |
| FR-028 | CN-119, CN-009 | FR-081 | CN-131 |
| FR-029 | CN-120 | FR-082 | CN-132, CN-007 |
| FR-030 | CN-050 | FR-083 | CN-133, CN-135 |
| FR-031 | CN-055 | FR-084 | CN-134 |
| FR-032 | CN-051, CN-052 | FR-036 | CN-150 |
| FR-033 | CN-056 | FR-037 | CN-151 |
| FR-034 | CN-041 | FR-038 | CN-152 |
| FR-035 | CN-058 | FR-039 | CN-153 |
| FR-040 | CN-060 | | |
| FR-041 | CN-061 | | |
| FR-042 | CN-062 | | |

### Acceptance criteria → tickets

| AC | Phase | Ticket(s) |
| --- | --- | --- |
| AC-001 | 1 | CN-034, CN-053, CN-063 |
| AC-002 | 1 | CN-050, CN-033, CN-181 |
| AC-003 | 1 | CN-071, CN-032 |
| AC-004 | 1 | CN-051, CN-041 |
| AC-005 | 1 | CN-060, CN-192 |
| AC-006 | 2 | CN-101, CN-065, CN-100 |
| AC-007 | 2 | CN-037 |
| AC-008 | 3 | CN-110, CN-111 |
| AC-009 | 4 | CN-136, CN-184 |
| AC-010 | 4 | CN-131, CN-134 |
| AC-011 | 5 | CN-151, CN-154, CN-185 |
| AC-012 | 6 | CN-171, CN-172 |
| AC-013 | 2 | CN-087, CN-082 |
| AC-014 | 2 | CN-091, CN-093, CN-015 |

### Non-functional requirements → tickets

| NFR | Ticket(s) | NFR | Ticket(s) |
| --- | --- | --- | --- |
| NFR-001 | CN-190 | NFR-008 | CN-192 |
| NFR-002 | CN-191, CN-034 | NFR-009 | CN-193 |
| NFR-003 | CN-037 | NFR-010 | CN-080 |
| NFR-004 | CN-042, CN-101 | NFR-011 | CN-011, CN-013, CN-014 |
| NFR-005 | CN-038 | NFR-012 | CN-040 |
| NFR-006 | CN-194 | NFR-013 | CN-094 |
| NFR-007 | CN-051, CN-003 | NFR-014 | CN-091, CN-093 |

### §14.2 mandatory test scenarios → owning ticket

| Scenario | Ticket |
| --- | --- |
| Happy path | CN-181 |
| Node refusal | CN-041 |
| Heartbeat loss | CN-101, CN-102 |
| Cancellation during a model call | CN-038 |
| Replay attempt | CN-104 |
| Unauthorized artifact retrieval | CN-064 |
| Producer offline after completion | CN-065 |
| Prompt-injected artifact | CN-052 |
| Hostile worker adapter | CN-051, CN-052 |
| Manifest change | CN-081 (declared provenance; **not** drift detection — D19) |
| Token widening at depth | CN-151 |
| Index poisoning | CN-137 |
| Audit export | CN-087 |

### Residual open decisions (§16) → tickets

| Decision | Ticket |
| --- | --- |
| Product name | CN-002 |
| Isolation runtime | CN-003 |
| Token library | CN-004 |
| Task contract registry | CN-005 |
| Audit anchoring | CN-006 |
| DLP implementation | CN-007 |
| Spec governance transfer | CN-008 |
| Steering semantics | CN-009 |

---

## Suggested order

**Gate:** CN-001 before anything else. If the extension set is not expressible as conformant A2A v1.0 extensions, the architecture changes and most of this backlog is rewritten.

1. **Phase 0** — CN-001, then CN-010, CN-003, CN-004, CN-013, CN-014, CN-015, CN-011. Remaining E0 decisions (CN-002, CN-005 … CN-009) in parallel; they are cheap and each unblocks something later.
2. **Phase 1 spine** — CN-020 → CN-021 → CN-029 → CN-022/CN-023 → CN-026 → CN-030 → CN-031 → CN-033 → CN-034. Then CN-040, CN-036, CN-037.
3. **Phase 1 execution** — CN-050 → CN-051 → CN-052 → CN-053. Then CN-060 → CN-061/CN-062/CN-064.
4. **Phase 1 verification** — CN-070 → CN-071 → CN-032 → CN-063 → CN-072 → CN-074. AC-001 … AC-005 close here; that is the Phase 1 exit.
5. **Phase 2** — CN-065, CN-037 resume tests, CN-100 … CN-104, CN-080 … CN-083, CN-087, CN-090, CN-091, CN-093, CN-094, CN-017.
6. **Phases 3–7** — re-scope each epic when its phase opens. Do not work E10–E13 descriptions as written; they are placeholders carrying the requirement text, not designs.

Two things the PRD flags that this ordering respects:

- **CN-156 gates all of E12.** Security review before recursion is enabled, and recursion never before attenuable tokens ship.
- **CN-017 and CN-093 come after their implementation tickets.** `spec/` trails the daemon by one release until the protocol stops moving (§13.1).

Two things worth deciding before Phase 1 opens, both cheap now and expensive later:

- **CN-016** (stale diagrams). The PRD flags both figures as showing a deleted architecture. Anyone who reads the pictures instead of the tables gets v0.1.
- **CN-002** (the name). It is one find-and-replace today and a compatibility problem once extension namespaces are registered (CN-008) and a spec is published (CN-093).
