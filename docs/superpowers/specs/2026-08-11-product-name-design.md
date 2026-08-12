# Product name and rename — design

**Ticket:** CN-002 (issue #2) · **Phase:** 0 · **Traces:** §16, D28, Readme "Naming placeholder", spec note N1
**Date:** 2026-08-11 · **Status:** Approved
**Blocks:** CN-093 (#76)

## Decision

**The product is named Bailment.**

A bailment is the delivery of goods by one party to another for a specific
purpose, without transfer of ownership, under a duty of care, with an
obligation to return or dispose of them according to instructions. The
bailee's authority is bounded by the purpose of the bailment; exceeding it
is conversion.

That definition is the protocol's object model:

| Bailment law | Consign protocol |
| --- | --- |
| Possession transfers, title does not | Work package crosses a boundary; data ownership stays with the originator (§3.2) |
| Authority bounded by the stated purpose | Attenuable capability tokens (FR-036, FR-037, D18) |
| Duty of care while held | Participant-side policy and daemon-owned sandboxing (FR-032, D29) |
| Obligation to return per instructions | Result envelope validated against the contract's output schema (FR-043) |

The name describes Phase 1 — a work package crossing an organizational
boundary under bounded authority — which is what v0.1 actually ships.

## Constraints this decision had to satisfy

1. Exact-match `.dev` or `.org` unregistered, plus a free GitHub org and
   clean package namespaces.
2. Custody/logistics register — the metaphor should teach the object model
   on first contact.
3. No collision with an existing project or software trademark. `Hermes
   Federation` was retired in v0.2 for wearing another project's identity
   (D28); repeating that is the primary failure mode.
4. Must work as a wire identifier and a binary name typed by implementers
   who have never met the maintainer.

## Availability evidence

Measured 2026-08-11 via RDAP (404 = unregistered), the GitHub API, and the
PyPI, npm and crates.io registries.

| Asset | Status |
| --- | --- |
| `bailment.dev` | free |
| `github.com/bailment` | free (HTML and API both 404) |
| PyPI `bailment` | free |
| npm `bailment` | free |
| crates.io `bailment` | free |
| `bailment.org` | registered — accepted, the bar was `.dev` **or** `.org` |

No open-source project or software trademark named Bailment was found.

## Candidates rejected

| Candidate | Why rejected |
| --- | --- |
| **CONSIGN** (incumbent) | `consign.dev` registered 2026-01-08, `consign.org` held since 2001. "Consignment" means goods sent *for sale*, leaking the money semantics D13 removed from scope. Crowded commercial category (consignment retail software). |
| **Lading** | `lading.dev` free and "bill of lading" is the best-known custody document, but [DataDog/lading](https://github.com/DataDog/lading) is an active load-testing tool in the infrastructure space — the Hermes mistake exactly. |
| **Chirograph** | Names only the co-signed receipt (FR-066), which §13.3 defers and §13.4 schedules in Phase 7 — the last thing built. Live modern referent is a papal instrument, against D28's "neutral name". `github.com/chirograph` is a dormant org (created 2024-11-10, zero repos); `.org` taken and locked. |
| **Bailiwick** | `.dev` free, but GitHub org, PyPI and npm all occupied. Covers bounded authority only — nothing about custody, packages, or return. |
| **Cocket** | Only candidate with both `.dev` and `.org` free, and historically the sealed customs document certifying goods cleared for export. Unshippable phonetics for a public project. |
| **Consignor / Consignee** | `.dev` free, but naming a deliberately symmetric protocol after one role contradicts §1: "any node can originate; there is no permanent root." |

## Considered and rejected objections to Bailment

**The "bail" root.** English speakers hear *bail out*, *bail bonds*,
*bailing water*. This is structurally the weakness that dogs CONSIGN
(*consign to the dustbin*) — but the parallel does not hold. "Consign to
the dustbin" uses the exact verb the product would be named after;
"bail out" is a phrasal verb in a different register, and no one says
"bailment out". The `-ment` suffix does the disambiguating work, and
bailment is a term of art the security-and-compliance-reviewer persona
(§4.1) met in first-year property law. Accepted as a residual cost.

## Scope of the rename

### Product name and prose

- `Consign` → `Bailment`, `CONSIGN` → `BAILMENT`, `consign` → `bailment`
- Binary `consignd` → `bailment`, invoked with subcommands
  (`bailment serve`), following the vault/nomad/consul pattern. §7.1
  collapsed five deployables into one symmetric binary, so the daemon `d`
  suffix describes a shape the architecture no longer has — and it removes
  the unpronounceable `-ntd` cluster.
- §7.4 repository tree root `consign/` → `bailment/`

### Normative wire identifiers

These are strings a conformance implementer would hardcode. Renaming now is
free; renaming after publication is a breaking change. Spec note **N1**
flags exactly this and is resolved by this ticket.

| From | To |
| --- | --- |
| `https://consign.example/ext/{contract,constraints,authority,budget,artifacts,verification,receipts}/v1` | `https://bailment.dev/ext/…/v1` |
| `https://consign.example/contracts/…` | `https://bailment.dev/contracts/…` |
| `consign.{task,token,contract,result,receipt,grant,index,revocation,node-credential}/v1` | `bailment.…/v1` |
| `ConsignRevocation` / `consignRevocation` | `BailmentRevocation` / `bailmentRevocation` |
| `consign-revocation.json`, `#consign-revocation` | `bailment-revocation.json`, `#bailment-revocation` |
| `"consign": { … }` Agent Card block, `consign.declared` | `"bailment": { … }`, `bailment.declared` |
| `application/vnd.consign.patch` | `application/vnd.bailment.patch` |
| `/consign/artifacts` | `/bailment/artifacts` |
| `agent-card-consign.json` | `agent-card-bailment.json` |

The extension namespace moves off the reserved `.example` TLD onto
`bailment.dev`, which the maintainer will own. D5 anchors all identity in
DNS, and A2A extension URIs conventionally resolve to their own
documentation.

### Files and paths

- `git mv spec/consign-profile-v0.1.md spec/bailment-profile-v0.1.md`
- Repository `gabbarX/Federated-Inference` → `bailment/bailment`, matching
  the `istio/istio` pattern and giving a Go module path of
  `github.com/bailment/bailment` for D24's monorepo. GitHub preserves issue
  numbers on transfer and redirects the old URL.
- `tools/backlog_to_issues.py:29` `REPO` constant updated; `BACKLOG_URL`
  derives from it.

  **Ordering caveat.** `REPO` becomes `bailment/bailment`, which is correct
  only *after* the repository transfer. Between merging this PR and
  completing the transfer, `tools/backlog_to_issues.py` will point at a
  repository that does not yet exist and must not be run. This is
  acceptable because the script is a one-shot backlog-to-issues generator
  that has already been run — `tools/.issue-map.json` holds all 126
  mappings — and is not part of any routine workflow. Do not re-run it
  until the transfer completes.

### Document control

- PRD version `0.2` → `0.3`, with a revision-history row recording the
  rename and citing ADR-0001.
- §0 "Product" row becomes `Bailment (formerly Consign; earlier Hermes
  Federation)`.
- The "Naming placeholder" block (Readme.md:16-17) is rewritten as a short
  "Name" note recording Bailment and both retired names — not deleted, so
  the history survives.
- Spec note **N1** is deleted; this ticket resolves it.

## Explicitly out of scope

| Not changed | Why |
| --- | --- |
| The 126 `CN-` ticket IDs | Opaque stable identifiers bound to live GitHub issues by title. Renaming means retitling 126 issues and rewriting the issue map for zero functional gain. |
| `tools/.issue-map.json` | Must stay byte-identical — it is the binding between `CN-` IDs and issue numbers. |
| Role names (originator, participant) | Adopting bailor/bailee forces legal jargon on every reader, and expands a naming ticket into a spec-wide vocabulary change. |
| Object names (task envelope, work package) | Same reason. Renaming the delegated unit to "a bailment" is defensible but is its own ticket. |
| `hermes-agent` references | Names a real external project — worker adapter #1 (§A.4), not the product. |
| The three PNGs in `media/` | Cannot be find-and-replaced. See below. |

## Media assets

`media/cover_banner.png` reads **ROOT HERMES** — the v0.1 name, retired two
versions ago — and draws the hub-and-spoke "root" topology that D8 and §7.1
removed. Unlike `lifecycle.png` and `architecture.png` it carries no
staleness warning, and it is the first image in the README.

This ticket adds the **"Stale asset — regenerate."** annotation to match the
two existing ones. Regenerating the three diagrams is a separate ticket:
inventing new diagrams is design work, not a rename.

## Verification

The rename is verified by executable invariants, written before the rename
is performed:

1. `grep -ri consign` over tracked files returns hits only in the five
   permitted locations, and nowhere else:
   - `docs/adr/0001-product-name.md` — the decision record
   - `docs/superpowers/specs/2026-08-11-product-name-design.md` — this document
   - `docs/superpowers/plans/2026-08-11-bailment-rename.md` — the implementation plan
   - `Readme.md`, in the rewritten "Name" note and the §0 "Product" row,
     where "formerly Consign" is deliberate history
   - `tools/test_rename.py` — it necessarily contains the string it scans for

   The documents *about* the rename cannot avoid naming the old product.

   In particular `BACKLOG.md` must contain **zero** occurrences: the CN-002
   ticket body is rewritten to record the decision in the past tense
   ("Named Bailment; see ADR-0001") rather than preserving "CONSIGN is a
   working placeholder", and its other seven references (lines 1, 3, 71,
   76, 157, 207, 353) are renamed like any other prose.
2. The `CN-` ticket ID count is still exactly 126.
3. `tools/.issue-map.json` is byte-identical to its state on `main`.
4. `tools/backlog_to_issues.py` still parses as valid Python.
5. `spec/bailment-profile-v0.1.md` exists and `spec/consign-profile-v0.1.md`
   does not.
6. No occurrence of `consign.example` remains anywhere.

## Sequencing

Work happens on branch `rename/bailment`; `main` is never modified. The
maintainer registers `bailment.dev` and claims `github.com/bailment` before
the PR merges — publishing a name you do not hold is how you lose it, and
this repository is public. The repository transfer happens after the merge,
so the 126-entry issue map is never in flight.

## Consequences

- The spec can go public: N1 is resolved and no placeholder identifiers
  remain.
- CN-093 (#76) is unblocked.
- CN-003 (isolation runtime) and CN-004 (token library) inherit the
  numbered-ADR convention this ticket establishes at `docs/adr/`.
- A follow-up ticket is needed to regenerate the three `media/` diagrams.
