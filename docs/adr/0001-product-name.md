# 1. Product name: Bailment

## Status

Accepted — 2026-08-11. Resolves CN-002 (issue #2) and spec note N1.
Supersedes the working name Consign (v0.2) and the retired name
Hermes Federation (v0.1).

## Context

The product needed a final name before the spec could be published.
CONSIGN was a working placeholder. `Hermes Federation` had already been
retired once in v0.2 for using another project's name for something this
project does not govern (D28), so repeating that mistake was the primary
risk.

Constraints applied:

1. Exact-match `.dev` or `.org` unregistered, plus a free GitHub
   organization and clean PyPI/npm/crates namespaces.
2. A custody or logistics metaphor, so the name teaches the object model
   on first contact.
3. No collision with an existing project or software trademark.
4. Usable as a wire identifier and a binary name.

34 candidates were swept against RDAP, the GitHub API, PyPI, npm and
crates.io on 2026-08-11.

## Decision

The product is named **Bailment**.

A bailment is the delivery of goods by one party to another for a specific
purpose, without transfer of ownership, under a duty of care, with an
obligation to return or dispose of them per instructions. The bailee's
authority is bounded by the purpose of the bailment.

That is this protocol's object model: possession crosses an organizational
boundary while ownership does not (§3.2), authority is bounded and
attenuable (FR-036, FR-037, D18), the holder owes a duty of care enforced
by daemon-owned sandboxing (FR-032, D29), and the result must return
conforming to a declared schema (FR-043).

The name describes Phase 1 — the work package crossing a boundary under
bounded authority — which is what v0.1 ships.

Availability, measured 2026-08-11: `bailment.dev` free; `github.com/bailment`
free; PyPI, npm and crates.io all free; `bailment.org` registered and
accepted, since the bar was `.dev` **or** `.org`. No open-source project or
software trademark named Bailment was found.

### Rejected candidates

| Candidate | Reason |
| --- | --- |
| CONSIGN | `consign.dev` registered 2026-01-08; `consign.org` held since 2001. "Consignment" means goods sent *for sale*, leaking the money semantics D13 removed from scope. |
| Lading | `lading.dev` free, but `DataDog/lading` is an active load-testing tool in the infrastructure space — the Hermes mistake repeated. |
| Chirograph | Names only the co-signed receipt (FR-066), deferred to Phase 7. Live modern referent is a papal instrument, against D28's "neutral name". GitHub org is a dormant squat; `.org` taken. |
| Bailiwick | `.dev` free but GitHub org, PyPI and npm occupied. Covers bounded authority only. |
| Cocket | Both domains free; unshippable phonetics. |
| Consignor / Consignee | Naming a symmetric protocol after one role contradicts §1's "no permanent root". |

### Accepted residual risk

The `bail` root evokes *bail out* and *bail bonds*. This is not the same
weakness as CONSIGN's: "consign to the dustbin" uses the exact verb, while
"bail out" is a phrasal verb in a different register and nobody says
"bailment out". The `-ment` suffix disambiguates, and bailment is a term of
art familiar to the security-and-compliance-reviewer persona (§4.1).

## Consequences

- The binary is `bailment` with subcommands (`bailment serve`), not
  `bailmentd`. §7.1 collapsed five deployables into one symmetric binary,
  so a daemon suffix describes a shape the architecture no longer has.
- The A2A extension namespace moves off the reserved `.example` TLD to
  `https://bailment.dev/ext/…`, resolving spec note N1.
- The 126 `CN-` ticket IDs and `tools/.issue-map.json` are deliberately
  untouched: they are opaque stable identifiers bound to live GitHub
  issues by title.
- Role names (originator, participant) and object names (task envelope,
  work package) are unchanged. Adopting bailor/bailee would force legal
  jargon on every reader.
- The repository moves to `github.com/bailment/bailment`, giving a Go
  module path of `github.com/bailment/bailment` for D24's monorepo.
- CN-093 (#76) is unblocked; CN-003 and CN-004 inherit this ADR convention.
- The three `media/` diagrams still carry retired names and need a
  follow-up ticket to regenerate.
