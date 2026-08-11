# The Consign Profile of A2A v1.0

**Specification version:** 0.1 (draft) · **Date:** 10 August 2026 · **Status:** Draft for implementer review
**Editor:** TBD · **Companion document:** `../Hermes_Federation_PRD_v0.1.md` (product requirements, v0.2)

---

## Editorial notes for this draft

> **N1 — Name.** `CONSIGN` and the extension namespace `https://consign.example/ext/...` are placeholders. Both are a single find-and-replace away from final. Do not publish until settled.
>
> **N2 — A2A method names.** This draft references A2A v1.0 RPC methods as `message/send`, `message/stream`, `tasks/get`, `tasks/cancel`, and `tasks/resubscribe`, and the Agent Card path as `/.well-known/agent-card.json`. **Every one of these MUST be pinned against the normative A2A v1.0 document during the Phase 0 spike** before this draft advances. Where this profile and A2A disagree, A2A wins and this document is wrong.
>
> **N3 — What Phase 0 must prove.** That §6 through §12 can be expressed as conformant A2A `Extension` declarations without altering A2A task semantics. If any extension requires changing how A2A tasks behave, this profile is the wrong shape and must be redesigned before implementation begins.

---

## 1 Scope and conformance

### 1.1 What this specifies

This document specifies **Consign**, a profile of the Agent2Agent protocol (A2A) v1.0 for delegating bounded, autonomous agent work packages across organizational boundaries. It defines:

- how an organization establishes identity without a certificate authority (§4);
- how a node advertises what it can do in a machine-checkable way (§5, §6);
- how authority to act is granted, bounded, and narrowed across delegation hops (§7);
- how tasks are submitted, observed, cancelled, and terminated (§8, §9);
- how content moves without a shared store (§10);
- what "verified" means and who is permitted to say it (§11);
- how work is accounted for without anyone being paid (§12);
- how nodes find each other and how firewalled nodes participate (§13, §14).

### 1.2 What this does not specify

Agent internals, prompting, model selection, tool implementations, sandbox technology, planning strategy, user interface, payment, and settlement. A conformant node makes no assumptions about how a peer produces its results, only about the shape and provenance of what it returns.

### 1.3 Requirement notation

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**, **SHOULD NOT**, **RECOMMENDED**, **MAY**, and **OPTIONAL** are to be interpreted as described in RFC 2119 and RFC 8174, when and only when they appear in all capitals.

Rationale text appears in blockquotes and is **non-normative**.

### 1.4 Conformance profiles

An implementation claims conformance to one or more named profiles. `CORE` is a prerequisite for every other profile.

| Profile | Requires | Adds |
| --- | --- | --- |
| **CORE** | — | §4 identity, §5 card, §6 contracts, §8 lifecycle, §9 events, §10 artifacts, §11 verification, §15 errors |
| **RECURSIVE** | CORE | §7 attenuable authority at depth > 0 |
| **HOSTED** | CORE | §5.4 `hosted_by` disclosure and union eligibility; §16.4 attestation |
| **RELAY** | CORE | §14 end-to-end-encrypted relaying |
| **INDEX** | CORE | §13 index publication and consumption |
| **RECEIPTS** | CORE | §12 co-signed receipts |

A node **MUST** advertise its profiles in its Agent Card (§5.2) and **MUST NOT** advertise a profile whose conformance vectors it does not pass (§17).

> An implementation that supports only `CORE` is fully useful: it can delegate, execute, verify, and be audited. Every other profile is an extension of reach, not of capability.

---

## 2 Relationship to A2A v1.0

### 2.1 Inherited without modification

Consign inherits and **MUST NOT** alter:

- **Transport.** JSON-RPC 2.0 over HTTPS, with Server-Sent Events for streaming.
- **Object model.** `AgentCard`, `AgentSkill`, `Task`, `Message`, `Part`, `Artifact`, `Extension`.
- **Task semantics.** A2A's task state machine, message threading, and artifact association.
- **Extension mechanism.** Extensions are declared in the Agent Card and identified by URI.

### 2.2 Contributed extensions

Every Consign addition is an A2A extension. A node **MUST** declare each extension it supports in its Agent Card and **MUST** reject a task carrying a required extension it has not declared.

| URI | Short name | Section | Required by CORE |
| --- | --- | --- | --- |
| `https://consign.example/ext/contract/v1` | `contract` | §6 | Yes |
| `https://consign.example/ext/constraints/v1` | `constraints` | §7.2 | Yes |
| `https://consign.example/ext/authority/v1` | `authority` | §7 | Yes (depth 0 only in CORE) |
| `https://consign.example/ext/budget/v1` | `budget` | §7.5 | Yes |
| `https://consign.example/ext/artifacts/v1` | `artifacts` | §10 | Yes |
| `https://consign.example/ext/verification/v1` | `verification` | §11 | Yes |
| `https://consign.example/ext/receipts/v1` | `receipts` | §12 | No |

### 2.3 Extension criticality

Each extension declaration carries `required: true|false`. A **required** extension that the receiver does not understand **MUST** cause task rejection with `E_EXTENSION_UNSUPPORTED` (§15). A receiver **MUST NOT** silently ignore a required extension, and **MUST** preserve unknown *optional* fields when relaying or storing envelopes.

> This is the single most important forward-compatibility rule in the document. Silent degradation of a security extension is how a policy control becomes a suggestion.

### 2.4 Terminology map

| A2A term | Consign usage |
| --- | --- |
| Agent | **Node** — one daemon at one organization |
| Agent Card | Node's signed capability declaration, profiled in §5 |
| Skill | Human/model-facing description. **Never** used for eligibility (§6.4) |
| Task | One work package under one capability token |
| Artifact | Content-addressed, holder-hosted object (§10) |

Additional terms defined by this profile:

**Organization** — an entity identified by a DNS domain with a published did:web document (§4).
**Originator** — the node that created a task and minted its root capability token.
**Consumer** — the node awaiting a given task's result. For depth 0 this is the originator.
**Producer** — the node executing a task.
**Task contract** — a versioned input/output schema pair with a verifiability class (§6).
**Work package** — the unit handed to a worker adapter for autonomous execution.

---

## 3 Architectural invariants

These hold across every profile. A conformance failure on any one is a security failure, not a compatibility failure.

| # | Invariant |
| --- | --- |
| **I1** | Trust is bilateral. A node's trust in a peer **MUST NOT** be derived from any third party's trust in that peer. |
| **I2** | Authority only narrows. A capability token **MUST NOT** be transformed into one permitting more than it permitted (§7.4). |
| **I3** | Eligibility precedes composition. A node **MUST** determine a peer's eligibility before composing any envelope addressed to it (§8.2). |
| **I4** | Context is by reference. Task envelopes **MUST NOT** carry plaintext content above class `PUBLIC` (§7.2.2, §10). |
| **I5** | Verification is consumer-side. A result **MUST NOT** be accepted on the producer's assertion that its validators passed (§11.3). |
| **I6** | Declared is not measured. Any field a peer asserts about its own internals **MUST** be labelled `declared` and **MUST NOT** influence ranking (§5.5). |
| **I7** | Policy lives in the daemon. Isolation and egress control **MUST NOT** depend on worker-adapter cooperation (§16.2). |
| **I8** | Terminal ambiguity is reported, not resolved. A node that cannot establish whether external side effects occurred **MUST** report `UNKNOWN` (§8.6). |

---

## 4 Identity

### 4.1 Organization identity

An organization is identified by a DNS domain. Its identifier is the did:web DID for that domain, e.g. `did:web:lab-a.example`.

An organization **MUST** publish a DID document at `https://<domain>/.well-known/did.json`, served over TLS with a WebPKI-valid certificate for `<domain>`. The document **MUST** contain at least one `assertionMethod` verification key (the **organization key**) and **MUST** declare a `consignRevocation` service endpoint.

```json
{
  "@context": ["https://www.w3.org/ns/did/v1"],
  "id": "did:web:lab-a.example",
  "verificationMethod": [{
    "id": "did:web:lab-a.example#org-2026-08",
    "type": "JsonWebKey2020",
    "controller": "did:web:lab-a.example",
    "publicKeyJwk": { "kty": "OKP", "crv": "Ed25519", "x": "..." }
  }],
  "assertionMethod": ["did:web:lab-a.example#org-2026-08"],
  "service": [{
    "id": "did:web:lab-a.example#consign-revocation",
    "type": "ConsignRevocation",
    "serviceEndpoint": "https://lab-a.example/.well-known/consign-revocation.json"
  }]
}
```

Organization keys **SHOULD** be held offline and used only to sign Agent Cards (§5) and node credentials (§4.2). Key rotation is performed by publishing a new verification method and retaining the old one for the lifetime of the longest-lived credential it signed.

### 4.2 Node identity and credentials

A node has a **node key** and a **node credential**: a short-lived assertion signed by the organization key binding the node key to a node identifier, an endpoint, and an expiry.

- A node credential **MUST** have a lifetime of no more than 24 hours; the RECOMMENDED default is 15 minutes.
- The node **MUST** refresh before expiry and **MUST** stop serving on expiry.
- The `endpoint` host **MUST** be within the organization's domain or a subdomain of it.

```json
{
  "typ": "consign.node-credential/v1",
  "org": "did:web:lab-a.example",
  "node_id": "lab-a-coding-1",
  "endpoint": "https://coding-1.lab-a.example/a2a",
  "node_key": { "kty": "OKP", "crv": "Ed25519", "x": "..." },
  "issued_at": "2026-08-10T09:00:00Z",
  "expires_at": "2026-08-10T09:15:00Z",
  "sig": { "alg": "EdDSA", "kid": "did:web:lab-a.example#org-2026-08", "value": "..." }
}
```

### 4.3 Transport binding

All Consign traffic **MUST** use mutually authenticated TLS 1.3.

A verifier **MUST** reject a connection unless **all** of the following hold:

1. The peer's TLS certificate is WebPKI-valid for a host within the claimed organization's domain.
2. The peer presents a node credential (§4.2) that is unexpired and signed by a current organization key resolved from that organization's did:web document.
3. The node credential's `endpoint` host matches the TLS certificate's host.
4. The node identifier is not listed in the organization's revocation list (§4.4).

> Conditions 1 and 3 together are what make a stolen node key useless without also controlling DNS and obtaining a certificate. The DNS anchor is the whole trust root; §16.1 states the residual risk plainly.

### 4.4 Revocation

An organization **MUST** serve a signed revocation list at its `ConsignRevocation` endpoint.

```json
{
  "typ": "consign.revocation/v1",
  "org": "did:web:lab-a.example",
  "issued_at": "2026-08-10T09:00:00Z",
  "next_update": "2026-08-10T09:30:00Z",
  "revoked_nodes": ["lab-a-coding-3"],
  "revoked_keys": ["did:web:lab-a.example#org-2026-02"],
  "sig": { "alg": "EdDSA", "kid": "did:web:lab-a.example#org-2026-08", "value": "..." }
}
```

Verifiers **MUST** fetch the list at least once per `next_update` interval and **MUST** treat a list older than `next_update + 2×interval` as stale, refusing new tasks to that organization while continuing existing leases.

Revocation propagation is bounded by **node credential lifetime**, not by list freshness: a revoked node stops being able to authenticate once its current credential expires. Implementations **MUST** document their credential lifetime, since it is the true revocation bound.

> This is a deliberate downgrade from the product's earlier 60-second target, which assumed a central CA that this profile does not have. Short credentials are the honest mechanism; a shorter default trades revocation latency against refresh traffic.

### 4.5 Key pinning

A consumer **MAY** pin an organization key for a known counterparty. A pinned key that changes **MUST** cause tasks to that organization to be refused until an operator re-approves. Implementations **SHOULD** log every observed change to a counterparty's did:web document.

---

## 5 The Agent Card

### 5.1 Location and signature

A node **MUST** serve its Agent Card at `/.well-known/agent-card.json`. The card **MUST** be signed by an organization key and **MUST** carry `issued_at` and `expires_at`. A consumer **MUST** reject a card whose signature, issuer, freshness, or schema fails validation, and **MUST NOT** compose an envelope for a node whose card it has not validated (I3).

### 5.2 Consign profile block

The card carries a Consign block alongside standard A2A fields.

```json
{
  "protocolVersion": "1.0",
  "name": "Lab A Coding Node",
  "url": "https://coding-1.lab-a.example/a2a",
  "skills": [
    { "id": "python-debugging", "name": "Python debugging",
      "description": "Discovery only; never used for eligibility.", "tags": ["code"] }
  ],
  "extensions": [
    { "uri": "https://consign.example/ext/contract/v1",      "required": true },
    { "uri": "https://consign.example/ext/constraints/v1",   "required": true },
    { "uri": "https://consign.example/ext/authority/v1",     "required": true },
    { "uri": "https://consign.example/ext/budget/v1",        "required": true },
    { "uri": "https://consign.example/ext/artifacts/v1",     "required": true },
    { "uri": "https://consign.example/ext/verification/v1",  "required": true }
  ],

  "consign": {
    "spec_version": "0.1",
    "profiles": ["CORE"],
    "org": "did:web:lab-a.example",
    "node_id": "lab-a-coding-1",
    "tier": "standard",

    "contracts": [
      { "id": "code-review/v1",
        "verifiability": "DETERMINISTIC",
        "max_input_bytes": 268435456,
        "validators": ["apply_patch", "run_tests", "static_analysis"] }
    ],

    "policy": {
      "accepted_data_classes": ["PUBLIC", "CONSORTIUM"],
      "tools": ["git", "pytest", "ruff"],
      "outbound_network": "deny",
      "side_effects": "propose-only",
      "max_delegation_depth_accepted": 0
    },

    "capacity": {
      "max_concurrent_tasks": 4,
      "queue_depth": 1,
      "expected_start_delay_seconds": 12,
      "heartbeat_seconds": 10,
      "state": "ready",
      "observed_at": "2026-08-10T09:14:02Z"
    },

    "declared": {
      "models": [
        { "id": "code-specialist-70b",
          "manifest_digest": "sha256:2b71...",
          "context_tokens": 131072 }
      ]
    },

    "issued_at": "2026-08-10T09:00:00Z",
    "expires_at": "2026-08-11T09:00:00Z"
  },
  "sig": { "alg": "EdDSA", "kid": "did:web:lab-a.example#org-2026-08", "value": "..." }
}
```

### 5.3 Capacity freshness

`capacity.observed_at` **MUST** reflect when the values were last measured. A consumer **MUST** treat capacity as stale, and the node as ineligible for new work, when `now - observed_at > 3 × heartbeat_seconds`. Stale capacity **MUST NOT** cause an existing lease to be abandoned.

### 5.4 Tiers and hosting (profile HOSTED)

`tier` **MUST** be one of `standard`, `anchor`, or `hosted`.

A node whose models execute on hardware operated by another organization **MUST** set `tier: "hosted"` and include:

```json
"hosting": {
  "hosted_by": "did:web:anchor-h.example",
  "host_attestation": { "type": "nvidia-cc", "evidence_ref": "cas://sha256/...", "expires_at": "..." },
  "cosign": { "alg": "EdDSA", "kid": "did:web:anchor-h.example#org-2026-07", "value": "..." }
}
```

- The `hosting` block **MUST** be co-signed by the host organization. An uncosigned hosting claim **MUST** be rejected.
- A consumer **MUST** evaluate eligibility against the **union** `{node org, hosted_by org}`. Excluding an organization therefore automatically excludes every node it hosts, without the consumer needing to know the hosting map in advance.
- A consumer **MUST NOT** route a task of class `RESTRICTED` or `LOCAL-ONLY` to a hosted node unless `host_attestation` is present, unexpired, and verifies (§16.4).
- A node that omits `hosting` while in fact being hosted is in violation of this specification. This is detectable only out of band; §16.1 states the limitation.

An `anchor` node **MAY** declare infrastructure roles it operates:

```json
"roles": ["index", "relay", "mirror"]
```

Declaring a role creates no obligation on any consumer to use it, and confers no trust (I1).

### 5.5 Declared versus observed

Every field under `consign.declared` is an unverifiable assertion by the node about its own internals. A consumer:

- **MUST NOT** use any `declared` field as an input to ranking (§13.4);
- **MAY** use `declared` fields for eligibility filtering, since a false claim there produces a task the node cannot fulfil, which the consumer then observes;
- **MUST** record `declared` fields in provenance labelled as declared;
- **MUST NOT** present a `declared` field to an operator as a verified property.

> Model identity lives here permanently. A node signs claims about its own private serving stack; nothing on the wire can contradict them. The profile records the claim and rests acceptance on §11 instead of pretending to detect substitution.

---

## 6 Task contracts

### 6.1 Definition

A **task contract** is the unit of capability matching. It is a versioned identifier `<name>/v<major>` bound to exactly four things:

1. an input JSON Schema,
2. an output JSON Schema,
3. a verifiability class,
4. a named validator set (empty only for class `UNVERIFIED`).

Two nodes that both declare `code-review/v1` agree on all four. This is the only capability assertion in the profile that is machine-checkable, and therefore the only one permitted to gate eligibility.

### 6.2 Verifiability classes

| Class | Meaning | Acceptance basis |
| --- | --- | --- |
| `DETERMINISTIC` | Output can be checked by running the validator set on the consumer's own machine, reproducibly | Consumer-run validators (§11.3) |
| `ATTESTED_LOCAL` | Validators can only run where the data lives; the producer runs them and signs the transcript | Producer-signed validator transcript, plus schema and aggregate checks |
| `UNVERIFIED` | No mechanical check exists | Nothing. Results are labelled and **MUST** be marked as unverified wherever surfaced |

A CORE implementation **MUST** support `DETERMINISTIC`. It **MAY** support the others. A consumer **MUST** refuse to accept an `UNVERIFIED` result without an explicit operator override recorded in the audit chain.

### 6.3 Contract document

```json
{
  "typ": "consign.contract/v1",
  "id": "code-review/v1",
  "verifiability": "DETERMINISTIC",
  "input_schema": "https://consign.example/contracts/code-review/v1/input.json",
  "output_schema": "https://consign.example/contracts/code-review/v1/output.json",
  "validators": [
    { "name": "apply_patch",      "kind": "process", "must_pass": true },
    { "name": "run_tests",        "kind": "process", "must_pass": true },
    { "name": "static_analysis",  "kind": "process", "must_pass": false }
  ],
  "required_artifacts": ["patch", "test_report"],
  "digest": "sha256:9c4e..."
}
```

The contract `digest` covers the canonical serialization of the whole document. An envelope **MUST** carry the digest of the contract it was composed against, and a producer **MUST** reject a task whose contract digest does not match its own copy (`E_CONTRACT_MISMATCH`).

> Contract digests are what make "we both support `code-review/v1`" mean something. Without them, a version string is a promise; with them, it is a checkable fact.

### 6.4 Skills are not capabilities

A2A `AgentSkill` entries are free text for humans and models. An implementation **MUST NOT** use skill text, tags, or descriptions in any eligibility predicate, and **MUST NOT** route a task on a semantic match alone. Skills **MAY** be surfaced to an operator or planner to *suggest* a contract; the contract is what is matched.

---

## 7 Authority

### 7.1 The capability token

Every task carries exactly one **capability token**: an attenuable, offline-verifiable credential minted by the originator, stating everything the holder is permitted to do with the work.

Tokens **MUST** be attenuable in the narrowing direction only, **MUST** be verifiable offline against the originator's organization key, and **MUST** require no communication with the originator to validate. Implementations **SHOULD** use an existing attenuable-token library (Biscuit is RECOMMENDED) rather than defining new cryptography.

```json
{
  "typ": "consign.token/v1",
  "token_id": "tok_019fe2",
  "originator": "did:web:lab-a.example",
  "root_task_id": "task_019fe2",
  "issued_at": "2026-08-10T09:15:00Z",
  "expires_at": "2026-08-10T09:45:00Z",
  "caveats": {
    "contract": ["code-review/v1"],
    "data_class": "CONSORTIUM",
    "allowed_organizations": ["did:web:lab-a.example", "did:web:company-c.example"],
    "allowed_tools": ["git", "pytest"],
    "network_access": "deny",
    "side_effects": "propose-only",
    "max_delegation_depth": 0,
    "allowed_delegates": [],
    "budget": {
      "deadline": "2026-08-10T09:30:00Z",
      "max_model_calls": 30,
      "max_tool_calls": 80,
      "max_artifact_bytes": 1073741824
    }
  },
  "chain": [],
  "sig": { "alg": "EdDSA", "kid": "did:web:lab-a.example#org-2026-08", "value": "..." }
}
```

### 7.2 Constraint caveats

#### 7.2.1 Data classes

| Class | Meaning |
| --- | --- |
| `PUBLIC` | No disclosure restriction beyond integrity |
| `CONSORTIUM` | Restricted to bilaterally allowlisted organizations |
| `RESTRICTED` | Restricted to an explicit allowlist; hosted recipients require attestation |
| `LOCAL-ONLY` | Raw data **MUST NOT** leave the owning organization under any circumstance |

Classes are totally ordered `PUBLIC < CONSORTIUM < RESTRICTED < LOCAL-ONLY`.

#### 7.2.2 Content confinement (invariant I4)

- A task envelope **MUST NOT** contain plaintext application content above class `PUBLIC`. All such content **MUST** be referenced as artifacts (§10).
- Free-text fields (`notes`, `goal`, steering messages) **MUST** be size-capped. The RECOMMENDED cap is 4 KiB per field and 16 KiB per envelope. Exceeding a cap **MUST** fail submission rather than truncate.
- An artifact **MUST NOT** be referenced by a task whose `data_class` caveat is lower than the artifact's label. Labels are assigned at ingest by the data owner and are immutable (§10.3).
- Implementations **SHOULD** run a deterministic egress scan over all free-text fields before transmission and **MUST** record the verdict in the audit chain.

> An agent composes the envelope. If free text could carry payload, the data-class boundary would be enforced by a language model's judgement and one prompt injection away from failure. Confining content to labelled references turns the boundary into a checkable invariant.

#### 7.2.3 Tools, network, side effects

`allowed_tools` and `network_access` are **upper bounds requested by the originator**, never grants. The producer's own policy applies independently and the effective permission set is the intersection (§16.2). `side_effects` **MUST** be one of `propose-only` (default) or `scoped-execute`; `scoped-execute` **MUST** enumerate the permitted effects.

### 7.3 Chain verification

A receiving node **MUST**, before accepting a task:

1. Resolve the originator's did:web document and verify the root signature.
2. Verify every link in `chain`, each signed by the organization that produced it.
3. Verify that each link **only narrows** (§7.4).
4. Verify that `expires_at` has not passed and that the deadline caveat is satisfiable.
5. Verify that its own organization appears in `allowed_organizations` and, if `chain` is non-empty, in the parent's `allowed_delegates`.
6. Verify that the remaining depth is ≥ 0.

Failure of any step **MUST** produce `E_TOKEN_INVALID` and **MUST** be recorded in the audit chain with the offending link identified. The receiving node **MUST NOT** contact the originator to resolve the failure.

> Step 6 executed by the *grandchild* is the entire point. Constraint laundering by an intermediary is detected by the party that would be harmed by it, at accept time, with no round trip.

### 7.4 Attenuation rules

A holder creating a child token **MUST** produce one that satisfies all of:

| Caveat | Permitted transformation |
| --- | --- |
| `contract` | Subset of parent's |
| `data_class` | Equal to or lower than parent's |
| `allowed_organizations` | Subset of parent's |
| `allowed_delegates` | Subset of parent's `allowed_delegates` |
| `allowed_tools` | Subset of parent's |
| `network_access` | Equal or more restrictive |
| `side_effects` | Equal or more restrictive |
| `max_delegation_depth` | ≤ parent's − 1 |
| `expires_at` | ≤ parent's |
| Budget caveats | Each ≤ parent's **remaining** value (§7.5) |

Any other transformation is invalid. A node **MUST** reject a child token that violates any row, regardless of who signed it.

### 7.5 Budget subdivision

Budgets are caveats, subdivided rather than shared. A holder delegating a child task **MUST** deduct the child's budget from its own remaining allowance at delegation time, **MUST NOT** issue children whose combined budgets exceed its remaining allowance, and **MUST** treat budget exhaustion as a terminal `PARTIAL` rather than continuing unfunded.

Budget accounting is producer-local and unverifiable by the consumer. Deadlines, by contrast, are wall-clock and externally observable, so consumers **SHOULD** treat `deadline` as the enforceable bound and other budget caveats as cooperative.

### 7.6 Depth and recursion (profile RECURSIVE)

`max_delegation_depth` defaults to `0`, meaning the receiving node **MUST NOT** re-delegate. A node that receives a depth-0 task and attempts re-delegation **MUST** fail with `E_DEPTH_EXCEEDED`.

A node performing re-delegation **MUST** emit `task.delegated` to its consumer (§9.3) naming the child task, the delegate organization, and the remaining depth, so the originator learns of every hop in real time rather than at completion.

A CORE-only implementation **MUST** set `max_delegation_depth: 0` on every token it mints and **MUST** reject any received token with depth > 0 rather than silently flattening it.

---

## 8 Task lifecycle

### 8.1 States

```
CREATED ──► SUBMITTED ──► ACCEPTED ──► RUNNING ──► VERIFYING ──► COMPLETED
                │              │           │            │
                │              │           │            └──────► PARTIAL
                ├──────────────┴───────────┴───────────────────► FAILED
                ├──────────────────────────────────────────────► CANCELLED
                └──────────────────────────────────────────────► UNKNOWN
```

`COMPLETED`, `PARTIAL`, `FAILED`, `CANCELLED`, and `UNKNOWN` are terminal. `VERIFYING` is a **consumer-side** state: the producer's stream ends at `task.completed`, and the consumer then runs validators before deciding acceptance.

There is no `OFFERED` state. Submission is single-phase (§8.3).

### 8.2 Eligibility (invariant I3)

Before composing any envelope for a candidate node, a consumer **MUST** evaluate:

```
eligible(node, task) =
      did_web_verified(node.org)
  AND bilaterally_allowlisted(node.org)
  AND NOT revoked(node)
  AND card_fresh(node) AND capacity_fresh(node)
  AND task.contract_id ∈ node.contracts
  AND contract_digest_matches(node, task.contract_id)
  AND task.data_class ∈ node.policy.accepted_data_classes
  AND task.required_tools ⊆ node.policy.tools
  AND policy_allows_all(union(node.org, node.hosting.hosted_by), task.data_class)
  AND (task.data_class ≤ PUBLIC
       OR node.tier ≠ "hosted"
       OR valid_attestation(node.hosting.host_attestation))
```

A node failing any conjunct **MUST NOT** receive an envelope, a token, or an artifact grant.

### 8.3 Submission

Submission is a single A2A `message/send` (or `message/stream`) carrying the task envelope.

```json
{
  "schema": "consign.task/v1",
  "task_id": "task_019fe2",
  "parent_task_id": null,
  "idempotency_key": "b0d1...",
  "contract": {
    "id": "code-review/v1",
    "digest": "sha256:9c4e..."
  },
  "input": {
    "goal": "Review the authentication service and produce a patch",
    "notes": "Focus on JWT validation and session fixation",
    "artifact_refs": ["cas://sha256/870d..."]
  },
  "token": { "...": "consign.token/v1 as in §7.1" },
  "verification": {
    "validators": ["apply_patch", "run_tests"],
    "run_by": "consumer"
  },
  "reply_to": {
    "org": "did:web:lab-a.example",
    "endpoint": "https://root-1.lab-a.example/a2a"
  },
  "sig": { "alg": "EdDSA", "kid": "did:web:lab-a.example#node-1", "value": "..." }
}
```

- `input` **MUST** validate against the contract's input schema.
- `input.artifact_refs` **MUST** be references only; the envelope carries no artifact bytes.
- Two submissions with the same `idempotency_key` **MUST** return the same task state, never a second execution.
- The envelope **MUST** be signed by the submitting node key.

### 8.4 Acceptance

A producer accepting a task **MUST** respond with a lease and stream cursor, and **MUST** emit `task.accepted` as sequence 1 of the event stream.

```json
{
  "task_id": "task_019fe2",
  "state": "ACCEPTED",
  "lease": { "lease_id": "lease_77c", "expires_at": "2026-08-10T09:30:00Z",
             "heartbeat_seconds": 10 },
  "stream": { "cursor": 1 },
  "grant_endpoint": "https://coding-1.lab-a.example/consign/artifacts"
}
```

The consumer **MUST NOT** issue artifact grants (§10.4) before receiving acceptance. This is what makes single-phase submission safe: a refusal transfers no protected bytes because none were ever grantable.

A lease **MUST** be renewed by heartbeat. A lease that expires without renewal moves the task to suspect status at the consumer (§8.6).

### 8.5 Refusal

A producer refusing a task **MUST** return a machine-readable reason from §15 and **MUST NOT** return free-text-only refusals. Refusals **MUST NOT** disclose which specific allowlist, quota, or policy rule triggered, beyond the coarse error code.

> A refusal that explains itself precisely is an oracle for probing a peer's policy configuration.

### 8.6 Termination and ambiguity

- `COMPLETED` — result envelope validated against the output schema and delivered.
- `PARTIAL` — acceptance criteria unmet; any useful artifacts **MUST** still be published and their grants remain valid for the retention period.
- `FAILED` — execution failed and the producer can assert no ambiguous external side effect occurred. `side_effect_state` **MUST** be `none`.
- `CANCELLED` — cancellation acknowledged; partial artifacts listed.
- `UNKNOWN` — the producer disappeared, or side effects cannot be established. A consumer **MUST NOT** automatically retry an `UNKNOWN` task whose contract permits side effects (I8).

On lease expiry without heartbeat, a consumer **MUST** wait a configurable grace period (RECOMMENDED: 3 × `heartbeat_seconds`) before declaring `UNKNOWN`, and **MUST NOT** route follow-up messages to the suspect task during that window.

A node whose child task ends `UNKNOWN` **MUST** propagate `UNKNOWN` upward rather than absorbing the ambiguity into a clean `FAILED`.

### 8.7 Cancellation

Cancellation uses A2A `tasks/cancel`. A producer **MUST** acknowledge, stop at the next safe boundary, publish partial artifacts, and emit `task.cancelled` as its terminal event. A producer with live children **MUST** cancel them before reporting its own terminal state, and **MUST** report `UNKNOWN` if any child's terminal state cannot be established.

### 8.8 Steering

Steering uses A2A messaging within the task thread. A steering message **MUST** validate against the contract's steering schema where one exists, **MUST** obey the same size caps and egress rules as the original envelope (§7.2.2), and **MUST NOT** alter the capability token. A producer **MAY** ignore steering that arrives after its final iteration boundary and **MUST** report whether it was applied.

---

## 9 Events

### 9.1 Envelope

Every event carries:

| Field | Type | Notes |
| --- | --- | --- |
| `task_id` | string | |
| `seq` | integer | Monotonic from 1, no gaps, per task |
| `emitted_at` | RFC 3339 | Producer clock |
| `type` | string | §9.3 |
| `payload` | object | Type-specific |

### 9.2 Resumability

A producer **MUST** retain the event stream for at least the lease duration plus one hour, and **MUST** support resumption from a cursor via A2A `tasks/resubscribe`. On resume, the producer **MUST** deliver every event with `seq >` cursor, in order, exactly once. A consumer that observes a gap in `seq` **MUST** treat the stream as broken and re-resubscribe rather than proceeding.

> Without this, any SSE reconnect silently loses progress, and a durable consumer that survives client disconnects is impossible.

### 9.3 Event types

| Type | Payload | Notes |
| --- | --- | --- |
| `task.accepted` | `lease`, `node_id`, `grant_endpoint` | Always `seq: 1` |
| `task.started` | `worker_id`, `declared_model_manifest` | Manifest is declared (§5.5) |
| `task.progress` | `phase`, `message`, `counters` | No hidden reasoning; message is size-capped |
| `task.artifact` | `artifact_ref`, `media_type`, `digest`, `label`, `size` | Reference only |
| `task.delegated` | `child_task_id`, `delegate_org`, `remaining_depth` | REQUIRED on every re-delegation |
| `task.heartbeat` | `lease_id`, `last_activity`, `phase` | Distinguishes slow from dead |
| `task.completed` | `result_ref`, `usage`, `provenance_chain`, `verifiability` | Terminal producer event |
| `task.failed` | `error_code`, `retryable`, `side_effect_state` | |
| `task.cancelled` | `acknowledged_at`, `partial_artifacts` | |
| `task.receipt` | `receipt_digest`, `signatures` | Profile RECEIPTS only |

Producers **MUST NOT** emit model reasoning, raw tool output, or secrets in `task.progress`. Consumers **MUST** treat all event text as untrusted data and **MUST NOT** interpret it as instructions.

---

## 10 Artifacts

### 10.1 Content addressing

Artifacts are identified by `cas://sha256/<hex>` over the ciphertext-independent plaintext. A consumer **MUST** verify the digest after retrieval and decryption, and **MUST** fail closed on mismatch (`E_DIGEST_MISMATCH`). A digest mismatch is terminal and auditable; it **MUST NOT** be retried against the same holder without operator action.

### 10.2 Holder-hosted retrieval

There is no shared artifact service. Every artifact is served by its **holder** — the node that produced or ingested it — over the authenticated channel:

```
GET  {grant_endpoint}/{digest}      Range and resumption supported
PUT  {grant_endpoint}/{digest}      Chunked, resumable, idempotent by digest
HEAD {grant_endpoint}/{digest}      Size, media type, label
```

Transfers **MUST** support byte-range resumption. A re-uploaded chunk **MUST NOT** duplicate storage or re-trigger side effects.

### 10.3 Labels

Every artifact carries an immutable metadata record:

```json
{
  "digest": "cas://sha256/870d...",
  "media_type": "application/vnd.consign.patch",
  "size": 20481,
  "label": "CONSORTIUM",
  "producer": "did:web:company-c.example",
  "assigned_by": "did:web:lab-a.example",
  "assigned_at": "2026-08-10T09:02:00Z",
  "retention_until": "2026-08-17T09:02:00Z"
}
```

- `label` **MUST** be assigned at ingest by the owning organization and **MUST NOT** be changed thereafter. A relabelling requires a new artifact with a new digest.
- A model or agent **MUST NOT** be able to assign or lower a label. Label assignment is a daemon and operator function.
- An artifact **MUST NOT** be referenced by a task whose `data_class` caveat is below its label.

### 10.4 Grants

A grant authorizes one organization to retrieve one set of digests for a bounded period. Grants **MUST** be scoped to a specific recipient and task, **MUST** expire, and **MUST** be revocable. A grant **MUST NOT** be issued before task acceptance (§8.4).

```json
{
  "typ": "consign.grant/v1",
  "task_id": "task_019fe2",
  "recipient": "did:web:company-c.example",
  "digests": ["cas://sha256/870d..."],
  "expires_at": "2026-08-10T09:45:00Z",
  "sig": { "alg": "EdDSA", "kid": "did:web:lab-a.example#node-1", "value": "..." }
}
```

Unauthorized retrieval **MUST** be denied with `E_GRANT_INVALID` and audited.

### 10.5 Encryption

Artifact payloads above class `PUBLIC` **MUST** be encrypted to the recipient node's key. Any relay or mirror in the path therefore observes ciphertext, sizes, and timing only (§14). Content addressing is over plaintext so that digests remain stable across recipients.

### 10.6 Pinning

Before marking a task `COMPLETED`, a consumer **MUST** fetch and retain (**pin**) every artifact that any downstream task or audit obligation depends on.

> The failure this closes: artifacts live only at their holder, so a producer that goes offline after completion leaves the consumer holding references to bytes it can no longer fetch — precisely the recovery case the acceptance plan tests. Pinning makes the dependency local before the dependency's owner can vanish.

Implementations **SHOULD** expose retention policy per class and **MUST** honour `retention_until` when serving artifacts to others.

---

## 11 Verification

### 11.1 Principle

Acceptance is a consumer decision made on evidence the consumer produced. A producer's report that its own validators passed is provenance, not proof (I5).

### 11.2 Result envelope

```json
{
  "schema": "consign.result/v1",
  "task_id": "task_019fe2",
  "contract": { "id": "code-review/v1", "digest": "sha256:9c4e..." },
  "verifiability": "DETERMINISTIC",
  "output": { "...": "validates against the contract output schema" },
  "artifacts": [
    { "role": "patch",       "ref": "cas://sha256/aa10...", "label": "CONSORTIUM" },
    { "role": "test_report", "ref": "cas://sha256/bb21...", "label": "CONSORTIUM" }
  ],
  "claims": [
    { "id": "c1", "statement": "Session fixation is fixed by rotating the session id on login",
      "evidence": ["cas://sha256/bb21..."], "producer_validated": true }
  ],
  "provenance": {
    "chain": ["did:web:lab-a.example", "did:web:company-c.example"],
    "declared_model_manifest": "sha256:2b71...",
    "tools_used": ["git", "pytest"],
    "policy_version": "lab-a/2026-08-01"
  },
  "usage": { "wall_seconds": 412, "declared_model_calls": 23, "declared_tool_calls": 51 },
  "sig": { "alg": "EdDSA", "kid": "did:web:company-c.example#node-2", "value": "..." }
}
```

Claims and evidence are separate so that verification can accept, reject, or qualify individual claims rather than the result as a whole.

### 11.3 Consumer-side validation

For a `DETERMINISTIC` contract, the consumer **MUST**:

1. Validate `output` against the contract output schema. Failure ⇒ reject (`E_SCHEMA_INVALID`); the result **MUST NOT** enter synthesis.
2. Retrieve and digest-verify every referenced artifact.
3. Execute every `must_pass` validator **in its own isolated sandbox**, with no network access unless the validator declares it.
4. Accept only if every `must_pass` validator passes.
5. Record every validator's outcome in the audit chain.

A consumer **MUST NOT** execute a received artifact outside a sandbox, and **MUST NOT** apply a patch, merge, or deploy on the basis of the producer's report (`producer_validated` is informational only).

### 11.4 Attested-local validation

For `ATTESTED_LOCAL`, the producer runs the validators where the data lives and returns a signed transcript naming validator identities, their digests, and their outcomes. The consumer **MUST** verify the signature and validator identity, **MUST** validate the output schema and any declared aggregate or range checks, and **MUST** label the result as attested rather than reproduced wherever it is surfaced.

### 11.5 Ship code, not goals

For work over `LOCAL-ONLY` data, the recommended pattern is that the consumer supplies a **program and an output schema** rather than a prose goal. The remote agent **MAY** author that program, but the program becomes the artifact under review: its digest enters provenance, its output is schema-validated, and declared aggregates, ranges, and null-rates are checked on return.

> This is what keeps data-sovereign work inside a deterministic-only stance. What is trusted is a reviewable program, not a model's assurance about data nobody else may see.

### 11.6 Unverified results

An `UNVERIFIED` result **MUST** carry `verifiability: "UNVERIFIED"`, **MUST NOT** be accepted without an operator override recorded in the audit chain, and **MUST** be labelled as unverified in every surface that displays it, including any synthesis derived from it.

---

## 12 Receipts (profile RECEIPTS)

### 12.1 Purpose

Receipts record what happened in units both parties independently observed. They exist so that accounting is possible without anyone being trusted to self-report, and so that a settlement layer can be added later without a breaking change. **This specification defines no pricing, billing, or transfer of value.**

### 12.2 Observable units only

A receipt **MUST** contain only units both parties can independently witness:

| Unit | Witnessed by |
| --- | --- |
| `lease_seconds` | Both, from lease issue to terminal event |
| `tasks_accepted` | Both |
| `verified_completions` | Consumer, from its own validator runs |
| `artifact_bytes_served` | Both, from transfer |
| `refusals`, `failures` | Both |

Model calls, token counts, and GPU-hours **MUST NOT** appear in a receipt. They are producer-internal, unverifiable, and belong in `declared` provenance (§5.5).

### 12.3 Co-signature

```json
{
  "typ": "consign.receipt/v1",
  "task_id": "task_019fe2",
  "consumer": "did:web:lab-a.example",
  "producer": "did:web:company-c.example",
  "terminal_state": "COMPLETED",
  "units": { "lease_seconds": 412, "tasks_accepted": 1,
             "verified_completions": 1, "artifact_bytes_served": 41890 },
  "prev_receipt_digest": "sha256:...",
  "signatures": [
    { "org": "did:web:company-c.example", "alg": "EdDSA", "value": "..." },
    { "org": "did:web:lab-a.example",     "alg": "EdDSA", "value": "..." }
  ]
}
```

A receipt is valid only with **both** signatures. Neither party can inflate a receipt unilaterally; a disagreement produces an unsigned receipt and a recorded divergence rather than a false record.

### 12.4 Audit chain

Each node maintains a hash-chained append-only log of identity events, policy decisions, task transitions, egress verdicts, approvals, artifact digests, validator outcomes, and receipts. Each entry **MUST** include the digest of its predecessor. Export **MUST** allow a third party to detect gaps or reordering without access to task content. External anchoring is **OPTIONAL** and out of scope; the chain format **MUST NOT** preclude it.

---

## 13 Discovery (profile INDEX)

### 13.1 Indexes are hints

Discovery conveys no trust (I1). A consumer **MUST** re-resolve and re-verify every index entry against the organization's did:web document and live Agent Card before that node becomes eligible. A compromised or hostile index can waste a consumer's time; it **MUST NOT** be able to make an untrusted organization eligible.

### 13.2 Index format

```json
{
  "typ": "consign.index/v1",
  "publisher": "did:web:anchor-h.example",
  "issued_at": "2026-08-10T09:00:00Z",
  "expires_at": "2026-08-10T10:00:00Z",
  "entries": [
    { "org": "did:web:lab-a.example",
      "card_url": "https://coding-1.lab-a.example/.well-known/agent-card.json",
      "contracts": ["code-review/v1"],
      "tier": "standard",
      "last_seen": "2026-08-10T08:59:41Z" }
  ],
  "sig": { "alg": "EdDSA", "kid": "did:web:anchor-h.example#org-2026-07", "value": "..." }
}
```

Entries carry contract identifiers for filtering only. Capacity **MUST NOT** be taken from an index; it comes from the node's live card and heartbeat (§5.3).

### 13.3 No index required

A conformant node **MUST** be usable with statically configured peers and no index at all. Two operators who exchange domains out of band can federate immediately.

### 13.4 Ranking inputs

Where a consumer ranks eligible nodes, it **MUST** use only locally observed signals:

```
rank(node) = w1·verification_pass_rate(node)
           + w2·acceptance_rate(node)
           + w3·lease_honoured_rate(node)
           − w4·time_to_first_event(node)
           − w5·straggler_rate(node)
           − w6·recent_failure_penalty(node)
```

A consumer **MUST NOT** use any `declared` field, third-party reputation, or index-supplied score as a ranking input. Unknown nodes begin at a neutral prior; implementations **SHOULD** build history through trial routing of low-value `PUBLIC` tasks.

---

## 14 Relays (profile RELAY)

A firewalled node **MAY** register an outbound persistent connection to a relay operated by an anchor. The relay forwards A2A traffic to the registered node.

- Payloads **MUST** be end-to-end encrypted to the destination node's key. The relay operator observes routing metadata, sizes, and timing only.
- A relayed node's Agent Card **MUST** declare `"relay": {"via": "did:web:anchor-h.example"}` so consumers can see the topology.
- A relay **MUST NOT** appear in the eligibility union (§5.4) — it is not a recipient of plaintext and does not join the trust boundary. This is the operative difference between relaying and hosting, and it holds only because of the encryption requirement above.
- A relay **MAY** apply rate limits and admission control and **MUST NOT** modify, reorder, or drop events silently; a relay that cannot forward **MUST** signal failure so the consumer can observe a broken stream (§9.2).

---

## 15 Errors

All errors are machine-readable. A producer **MUST NOT** return a free-text-only failure.

| Code | Retryable | Meaning |
| --- | --- | --- |
| `E_AUTH_FAILED` | No | mTLS, node credential, or did:web verification failed |
| `E_REVOKED` | No | Node or key is revoked |
| `E_TOKEN_INVALID` | No | Capability token chain, signature, or attenuation invalid |
| `E_DEPTH_EXCEEDED` | No | Re-delegation attempted beyond permitted depth |
| `E_CONTRACT_UNSUPPORTED` | No | Contract identifier not offered by this node |
| `E_CONTRACT_MISMATCH` | No | Contract digest differs from this node's copy |
| `E_SCHEMA_INVALID` | No | Envelope or result failed schema validation |
| `E_EXTENSION_UNSUPPORTED` | No | A required extension is not supported |
| `E_POLICY_REFUSED` | No | Local policy refuses; no further detail disclosed |
| `E_DATA_CLASS_REFUSED` | No | Node does not accept this data class |
| `E_CAPACITY` | Yes | No capacity now; `retry_after` MAY be included |
| `E_LEASE_EXPIRED` | Yes | Lease lapsed before completion |
| `E_BUDGET_EXHAUSTED` | No | Budget caveat exhausted; result MAY be `PARTIAL` |
| `E_DEADLINE_EXCEEDED` | No | Deadline caveat passed |
| `E_GRANT_INVALID` | No | Artifact grant missing, expired, or wrong recipient |
| `E_DIGEST_MISMATCH` | No | Content did not match its address — terminal, auditable |
| `E_ARTIFACT_UNAVAILABLE` | Yes | Holder cannot serve; pinning may have failed |
| `E_EGRESS_BLOCKED` | No | Local egress control blocked the outbound envelope |
| `E_APPROVAL_REQUIRED` | Yes | Human approval pending |
| `E_INTERNAL` | Yes | Unspecified producer-side failure |

Errors **MUST NOT** disclose which specific rule, allowlist entry, or quota triggered a refusal (§8.5).

---

## 16 Security considerations

### 16.1 What this profile does not protect against

Stated plainly, because a specification that hides its limits is worse than one that has them.

- **DNS or domain compromise.** Identity is anchored in DNS. An adversary controlling a domain and able to obtain a certificate for it becomes that organization. Mitigations are pinning (§4.5) and monitoring did:web changes — neither is prevention.
- **A dishonest host that does not disclose hosting.** §5.4 requires the `hosted_by` declaration and host co-signature, but a host and node conspiring to omit it are not detectable on the wire. Attestation (§16.4) is the only technical remedy.
- **Producer-internal claims.** Model identity, token counts, and GPU-hours are unverifiable by construction. The profile records them as declared and rests acceptance on §11.
- **`UNVERIFIED` results.** Nothing in this profile makes them trustworthy. It only makes them clearly labelled.
- **Traffic analysis.** Relays and mirrors see sizes and timing. Padding and cover traffic are out of scope.
- **A malicious operator inside a participant.** The operator of a node can see everything that node receives. Only §16.4 attestation narrows this, and only on supporting hardware.

### 16.2 Enforcement point (invariant I7)

Isolation and egress control **MUST** be enforced by the node daemon, outside the worker adapter's reach. Specifically, the daemon **MUST**:

- create the execution environment and launch the adapter inside it;
- apply filesystem, network, and resource confinement the adapter cannot reconfigure;
- mediate all artifact publication, so every artifact is hashed, labelled, and stored by the daemon;
- mediate all delegation, so tokens are minted and attenuated only by the daemon;
- apply the intersection of the token's requested tool set and local policy — never the union.

An implementation that relies on adapter cooperation for any of these **MUST NOT** claim CORE conformance.

> This is what makes "runtime-agnostic" real rather than notional. If enforcement lived in the adapter, every adapter would re-implement the security model, and one sloppy adapter would break the guarantee for everyone who trusted the node.

### 16.3 Untrusted content

All remote content — task input, artifacts, events, results, index entries, card fields — **MUST** be treated as untrusted data and **MUST NOT** be interpreted as instructions to the receiving agent. Implementations **MUST NOT** expand tool, network, or data permissions in response to content received over the wire, regardless of what that content claims.

### 16.4 Attestation (profile HOSTED)

Where present, `host_attestation` conveys hardware-backed evidence that the serving environment is confidential.

- Verifiers **MUST** check evidence freshness and the attestation chain to a recognized root, and **MUST** treat an unverifiable attestation as absent.
- Attestation is **OPTIONAL** in every profile. It unlocks higher data classes for hosted nodes; it is never a prerequisite for participation.
- **Hardware note (normative for expectations, not for implementations):** GPU confidential computing exists on Hopper and Blackwell class hardware and does **not** exist on Ampere. A large share of participants will have no attestation available. Any deployment policy that requires attestation universally excludes them; §5.4's disclosed-host union policy is therefore the mandatory baseline and attestation is the tier above it.

### 16.5 Cryptographic agility

Signature algorithm and key type are carried in every signature object. Ed25519 is the mandatory-to-implement baseline. SHA-256 is the mandatory-to-implement digest for content addressing. New algorithms are added by extension version, never by silent substitution.

---

## 17 Conformance and test vectors

### 17.1 Vectors

`spec/vectors/` contains signed test vectors for each profile: valid and invalid Agent Cards, valid and invalid token chains (including every attenuation violation in §7.4), envelopes that breach the egress rules of §7.2.2, event streams with gaps and resumption points, digest mismatches, and grant misuse.

An implementation claiming a profile **MUST** pass every vector for it, **MUST** reject every negative vector for the stated reason, and **MUST NOT** pass a negative vector by accident of a different check.

### 17.2 Interoperability requirement

`CORE` conformance requires a live interop run against a second, independently operated implementation: submit, accept, stream with a forced disconnect and resume, publish and retrieve an artifact, run consumer-side validation, and reach a terminal state — in both directions.

### 17.3 Claiming conformance

Conformance is self-asserted and published in the Agent Card. There is no certification authority, consistent with there being no membership authority. Consumers observe behaviour and rank on it (§13.4); a false conformance claim degrades a node's observed reputation with every peer independently.

---

## 18 Versioning

- `spec_version` follows semver. Breaking changes to any envelope, caveat, or invariant require a major bump and a new extension URI.
- Extension URIs are versioned in their path; a node **MAY** support several versions simultaneously and **MUST** negotiate via card declaration.
- Unknown **optional** fields **MUST** be preserved through storage and relaying. Unknown **required** extensions **MUST** cause rejection (§2.3).
- A node **MUST NOT** downgrade a security semantic to reach agreement. If the only common version weakens an invariant in §3, the correct outcome is refusal.

---

## Appendix A — Schema index

Extracted to `spec/schemas/` as a build step; the definitions inline above are normative until extraction lands.

| Schema | Section |
| --- | --- |
| `did-document.json` | §4.1 |
| `node-credential.json` | §4.2 |
| `revocation-list.json` | §4.4 |
| `agent-card-consign.json` | §5.2 |
| `contract.json` | §6.3 |
| `token.json` | §7.1 |
| `task-envelope.json` | §8.3 |
| `event.json` | §9.1 |
| `artifact-meta.json` | §10.3 |
| `grant.json` | §10.4 |
| `result.json` | §11.2 |
| `receipt.json` | §12.3 |
| `index.json` | §13.2 |
| `error.json` | §15 |

## Appendix B — Deferred to later spec versions

| Item | Reason |
| --- | --- |
| Settlement, pricing, credits | Out of product scope; receipts (§12) are the forward-compatible substrate |
| Hedged and replicated execution | Depends on capacity signals this version does not trust |
| Independent model review as an acceptance basis | Produces an opinion, not evidence; `UNVERIFIED` covers it |
| Cross-federation trust bundles | Bilateral allowlists are sufficient at current scale |
| Push notifications for detached consumers | A2A supports it; not yet profiled |
| Padding and cover traffic | Traffic analysis explicitly out of scope (§16.1) |

## Appendix C — Provenance of design decisions

Each normative choice traces to a decision recorded in the PRD's Appendix A.5.

| Spec section | Decision |
| --- | --- |
| §2 Profile of A2A | D4 |
| §4 did:web identity | D5 |
| §5.4 Hosted tier, union eligibility, attestation | D9, D10, D10b |
| §5.5 Declared vs observed | D19 |
| §6 Contract-first capabilities | D16 |
| §7 Attenuable authority | D3, D18 |
| §7.2.2 Content confinement | D6 |
| §8.3 Single-phase submission | D21 |
| §9.2 Resumable event streams | D7 |
| §10 Holder-hosted artifacts and pinning | D22 |
| §11 Deterministic verification | D14 |
| §11.5 Ship code, not goals | D15 |
| §12 Receipts without settlement | D11b, D12, D13 |
| §13 Indexes as hints | D17 |
| §13.4 Locally observed ranking | D26 |
| §14 Relays | D20 |
| §16.2 Daemon-owned enforcement | D8, D29 |
| §17 Self-asserted conformance | D2, D28 |
