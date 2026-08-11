# Bailment Rename Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the product from Consign to Bailment across all prose, normative wire identifiers, filenames and repository constants, recorded in an ADR, without disturbing the 126 `CN-` ticket IDs or the issue map.

**Architecture:** Test-first. Task 1 writes an executable verification script asserting every invariant the finished rename must satisfy, and proves it fails. Tasks 2-5 each make a subset of those assertions pass — ADR, spec, Readme, backlog/tooling. Task 6 proves the whole suite green. There is no application code here; the "production code" under test is the repository's own content.

**Tech Stack:** Python 3 (stdlib `unittest`, already the language of `tools/`), git, ripgrep/grep.

## Global Constraints

Copied verbatim from `docs/superpowers/specs/2026-08-11-product-name-design.md`. Every task's requirements implicitly include this section.

- **Product name:** `Consign` → `Bailment`, `CONSIGN` → `BAILMENT`, `consign` → `bailment`.
- **Binary name:** `consignd` → `bailment` (subcommand style, e.g. `bailment serve`). Never `bailmentd`.
- **Extension namespace host:** `https://consign.example/…` → `https://bailment.dev/…`. No occurrence of `consign.example` may remain anywhere.
- **Never modify:** the 126 `CN-###` ticket IDs; `tools/.issue-map.json` (must stay byte-identical to `main`); role names `originator`/`participant`; object names `task envelope`/`work package`; any `hermes-agent` / `Hermes adapter` reference naming the real external project.
- **`Hermes Federation`** may appear only as retired-name history, never as a current name.
- **`consign` (case-insensitive) may survive in exactly three places:** `docs/adr/0001-product-name.md`, `docs/superpowers/specs/2026-08-11-product-name-design.md`, and `Readme.md` (the "Name" note and the §0 "Product" row, as deliberate history). `BACKLOG.md` and `spec/` must contain **zero** occurrences.
- **Branch:** all work on `rename/bailment`. Never commit to `main`.
- **Do not** create, regenerate or edit any file under `media/`. Only the Readme's *text* annotation about the banner changes.
- **Do not** run `tools/backlog_to_issues.py`. It performs live GitHub writes.

---

### Task 1: Rename verification suite (RED)

**Files:**
- Create: `tools/test_rename.py`

**Interfaces:**
- Consumes: nothing.
- Produces: `tools/test_rename.py`, a stdlib `unittest` suite runnable as `python3 -m unittest tools.test_rename -v` from the repository root. Tasks 2-5 are complete when their corresponding test methods pass. Test method names later tasks depend on: `test_no_consign_outside_allowlist`, `test_backlog_has_no_consign`, `test_spec_has_no_consign`, `test_no_consign_example_host`, `test_ticket_id_count_unchanged`, `test_issue_map_byte_identical_to_main`, `test_backlog_tool_parses`, `test_spec_file_renamed`, `test_adr_exists`, `test_binary_is_not_bailmentd`, `test_prd_version_bumped`, `test_cover_banner_annotated`.

- [ ] **Step 1: Write the failing test**

Create `tools/test_rename.py`:

```python
"""Invariants the CN-002 Consign -> Bailment rename must satisfy.

Run from the repository root:
    python3 -m unittest tools.test_rename -v
"""

import ast
import pathlib
import re
import subprocess
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent

# The only files permitted to still contain "consign" (case-insensitive)
# after the rename. Everything else must be clean.
CONSIGN_ALLOWLIST = {
    "docs/adr/0001-product-name.md",
    "docs/superpowers/specs/2026-08-11-product-name-design.md",
    "docs/superpowers/plans/2026-08-11-bailment-rename.md",
    "Readme.md",
    "tools/test_rename.py",
}

TEXT_SUFFIXES = {".md", ".py", ".json", ".txt", ".yml", ".yaml", ".go"}


def tracked_files():
    """Every git-tracked file, as repo-relative POSIX paths."""
    out = subprocess.run(
        ["git", "ls-files"], cwd=ROOT, capture_output=True, text=True, check=True
    )
    return [p for p in out.stdout.splitlines() if p]


def text_files():
    for rel in tracked_files():
        if pathlib.Path(rel).suffix in TEXT_SUFFIXES:
            yield rel


def read(rel):
    return (ROOT / rel).read_text(encoding="utf-8", errors="replace")


class TestNoConsignRemains(unittest.TestCase):
    def test_no_consign_outside_allowlist(self):
        offenders = []
        for rel in text_files():
            if rel in CONSIGN_ALLOWLIST:
                continue
            if "consign" in read(rel).lower():
                offenders.append(rel)
        self.assertEqual(
            [], sorted(offenders), f"'consign' still present in: {sorted(offenders)}"
        )

    def test_backlog_has_no_consign(self):
        self.assertNotIn("consign", read("BACKLOG.md").lower())

    def test_spec_has_no_consign(self):
        spec = ROOT / "spec" / "bailment-profile-v0.1.md"
        self.assertTrue(spec.exists(), "spec/bailment-profile-v0.1.md missing")
        self.assertNotIn("consign", spec.read_text(encoding="utf-8").lower())

    def test_no_consign_example_host(self):
        offenders = [rel for rel in text_files()
                     if rel not in CONSIGN_ALLOWLIST
                     and "consign.example" in read(rel).lower()]
        self.assertEqual([], sorted(offenders))


class TestPreservedInvariants(unittest.TestCase):
    def test_ticket_id_count_unchanged(self):
        ids = set()
        for rel in text_files():
            ids.update(re.findall(r"CN-\d{3}", read(rel)))
        self.assertEqual(126, len(ids), f"expected 126 CN- ids, found {len(ids)}")

    def test_issue_map_byte_identical_to_main(self):
        current = (ROOT / "tools" / ".issue-map.json").read_bytes()
        on_main = subprocess.run(
            ["git", "show", "main:tools/.issue-map.json"],
            cwd=ROOT, capture_output=True, check=True,
        ).stdout
        self.assertEqual(on_main, current, "tools/.issue-map.json was modified")

    def test_backlog_tool_parses(self):
        src = read("tools/backlog_to_issues.py")
        ast.parse(src)  # raises SyntaxError on failure

    def test_roles_and_objects_untouched(self):
        readme = read("Readme.md")
        for term in ("originator", "participant", "work package"):
            self.assertIn(term, readme.lower(), f"'{term}' disappeared from Readme")


class TestRenameArtifacts(unittest.TestCase):
    def test_spec_file_renamed(self):
        self.assertTrue((ROOT / "spec" / "bailment-profile-v0.1.md").exists())
        self.assertFalse((ROOT / "spec" / "consign-profile-v0.1.md").exists())

    def test_adr_exists(self):
        adr = ROOT / "docs" / "adr" / "0001-product-name.md"
        self.assertTrue(adr.exists(), "docs/adr/0001-product-name.md missing")
        body = adr.read_text(encoding="utf-8")
        for heading in ("## Status", "## Context", "## Decision", "## Consequences"):
            self.assertIn(heading, body, f"ADR missing {heading}")
        self.assertIn("Bailment", body)

    def test_binary_is_not_bailmentd(self):
        for rel in text_files():
            if rel in CONSIGN_ALLOWLIST:
                continue
            self.assertNotIn("bailmentd", read(rel).lower(), f"'bailmentd' in {rel}")

    def test_extension_uris_use_bailment_dev(self):
        # Guard first: without it this raises FileNotFoundError, and an
        # erroring test is not a valid RED state.
        spec_path = ROOT / "spec" / "bailment-profile-v0.1.md"
        self.assertTrue(spec_path.exists(), "spec/bailment-profile-v0.1.md missing")
        spec = spec_path.read_text(encoding="utf-8")
        for ext in ("contract", "constraints", "authority", "budget",
                    "artifacts", "verification", "receipts"):
            self.assertIn(f"https://bailment.dev/ext/{ext}/v1", spec)

    def test_prd_version_bumped(self):
        # A bare `assertIn("0.3", readme)` is worthless here: it matches the
        # existing heading "### 10.3 Threats and mitigations" and is green
        # before the rename. Assert the two places the version actually
        # lives, without tripping on the revision-history "**0.2**" rows.
        readme = read("Readme.md")
        self.assertIn("Version 0.3", readme, "PRD header line still not v0.3")
        self.assertNotIn("Version 0.2", readme, "PRD header line still claims v0.2")
        self.assertRegex(
            readme, r"\|\s*\*\*Version\*\*\s*\|\s*0\.3\s*\|",
            "section 0 document-control Version row not bumped to 0.3",
        )

    def test_cover_banner_annotated(self):
        readme = read("Readme.md")
        idx = readme.find("cover_banner.png")
        self.assertNotEqual(-1, idx, "cover_banner.png reference missing")
        window = readme[idx:idx + 400]
        self.assertIn("Stale asset", window,
                      "cover banner lacks the 'Stale asset - regenerate.' annotation")


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the suite to verify it fails**

Run: `cd "$(git rev-parse --show-toplevel)" && python3 -m unittest tools.test_rename -v`

Expected: **FAIL**. Most tests fail because the rename has not happened. Confirm at minimum these failures appear, and that they fail for the stated reason and not on an import error or typo:
- `test_no_consign_outside_allowlist` — lists `BACKLOG.md`, `spec/consign-profile-v0.1.md`, `tools/backlog_to_issues.py`
- `test_spec_file_renamed` — `spec/bailment-profile-v0.1.md` does not exist
- `test_adr_exists` — ADR missing
- `test_prd_version_bumped` — Readme still says 0.2
- `test_cover_banner_annotated` — no annotation

`test_ticket_id_count_unchanged`, `test_issue_map_byte_identical_to_main`, `test_backlog_tool_parses` and `test_roles_and_objects_untouched` should **PASS** already — they guard things that must not change.

If any test errors rather than fails, fix the error and re-run until the failures are clean.

- [ ] **Step 3: Commit**

```bash
git add tools/test_rename.py
git commit -m "test: add CN-002 rename verification suite (red)"
```

---

### Task 2: ADR-0001

**Files:**
- Create: `docs/adr/0001-product-name.md`

**Interfaces:**
- Consumes: `tools/test_rename.py::test_adr_exists` from Task 1 — requires the literal headings `## Status`, `## Context`, `## Decision`, `## Consequences` and the word `Bailment`.
- Produces: `docs/adr/0001-product-name.md`, establishing the numbered-ADR convention that CN-003 and CN-004 reuse.

- [ ] **Step 1: Create the ADR**

Create `docs/adr/0001-product-name.md` in Nygard format. Use these exact contents:

```markdown
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
  jargon on every reader, against NFR-014.
- The repository moves to `github.com/bailment/bailment`, giving a Go
  module path of `github.com/bailment/bailment` for D24's monorepo.
- CN-093 (#76) is unblocked; CN-003 and CN-004 inherit this ADR convention.
- The three `media/` diagrams still carry retired names and need a
  follow-up ticket to regenerate.
```

- [ ] **Step 2: Run the ADR test to verify it passes**

Run: `cd "$(git rev-parse --show-toplevel)" && python3 -m unittest tools.test_rename.TestRenameArtifacts.test_adr_exists -v`

Expected: **PASS**.

- [ ] **Step 3: Commit**

```bash
git add docs/adr/0001-product-name.md
git commit -m "docs: add ADR-0001 recording the Bailment name (CN-002)"
```

---

### Task 3: Rename the normative spec

**Files:**
- Rename: `spec/consign-profile-v0.1.md` → `spec/bailment-profile-v0.1.md`
- Modify: the renamed file (45 occurrences)

**Interfaces:**
- Consumes: Task 1's `test_spec_has_no_consign`, `test_spec_file_renamed`, `test_extension_uris_use_bailment_dev`.
- Produces: `spec/bailment-profile-v0.1.md`. Task 5 updates `BACKLOG.md` references to this new path.

- [ ] **Step 1: Rename the file with git**

```bash
cd "$(git rev-parse --show-toplevel)"
git mv spec/consign-profile-v0.1.md spec/bailment-profile-v0.1.md
```

- [ ] **Step 2: Apply the identifier substitutions**

Apply these substitutions to `spec/bailment-profile-v0.1.md`, longest-first so no substitution corrupts another. Order matters.

```bash
cd "$(git rev-parse --show-toplevel)"
F=spec/bailment-profile-v0.1.md
python3 - "$F" <<'PY'
import sys, pathlib
p = pathlib.Path(sys.argv[1])
s = p.read_text(encoding="utf-8")
pairs = [
    ("https://consign.example", "https://bailment.dev"),
    ("consign.example",         "bailment.dev"),
    ("ConsignRevocation",       "BailmentRevocation"),
    ("consignRevocation",       "bailmentRevocation"),
    ("consign-revocation",      "bailment-revocation"),
    ("agent-card-consign",      "agent-card-bailment"),
    ("vnd.consign.",            "vnd.bailment."),
    ("/consign/artifacts",      "/bailment/artifacts"),
    ("consign.",                "bailment."),
    ("consignd",                "bailment"),
    ("CONSIGN",                 "BAILMENT"),
    ("Consign",                 "Bailment"),
    ("consign",                 "bailment"),
]
for old, new in pairs:
    s = s.replace(old, new)
p.write_text(s, encoding="utf-8")
PY
```

- [ ] **Step 3: Delete resolved note N1**

Open `spec/bailment-profile-v0.1.md` and find the note block near line 10 that begins `> **N1 — Name.**`. This ticket resolves it. Delete the entire `N1` blockquote line. If notes are numbered sequentially (N2, N3, …) leave the others' numbering untouched — do not renumber.

- [ ] **Step 4: Verify no placeholder host or stray name remains**

```bash
cd "$(git rev-parse --show-toplevel)"
grep -in "consign\|\.example/ext\|N1 — Name" spec/bailment-profile-v0.1.md || echo "CLEAN"
```

Expected: `CLEAN`. Any hit is a miss — fix it before continuing.

- [ ] **Step 5: Run the spec tests to verify they pass**

Run: `cd "$(git rev-parse --show-toplevel)" && python3 -m unittest tools.test_rename.TestNoConsignRemains.test_spec_has_no_consign tools.test_rename.TestRenameArtifacts.test_spec_file_renamed tools.test_rename.TestRenameArtifacts.test_extension_uris_use_bailment_dev -v`

Expected: **3 tests PASS**.

- [ ] **Step 6: Commit**

```bash
git add spec/
git commit -m "spec: rename Consign profile to Bailment, resolve note N1 (CN-002)"
```

---

### Task 4: Rename the PRD

**Files:**
- Modify: `Readme.md` (32 occurrences, plus version bump, Name note, banner annotation)

**Interfaces:**
- Consumes: Task 1's `test_prd_version_bumped`, `test_cover_banner_annotated`, `test_roles_and_objects_untouched`; Task 3's new spec filename.
- Produces: the final `Readme.md`. Nothing depends on it.

- [ ] **Step 1: Apply the identifier substitutions**

```bash
cd "$(git rev-parse --show-toplevel)"
python3 - <<'PY'
import pathlib
p = pathlib.Path("Readme.md")
s = p.read_text(encoding="utf-8")
pairs = [
    ("https://consign.example", "https://bailment.dev"),
    ("consign.example",         "bailment.dev"),
    ("consign-profile-v0.1.md", "bailment-profile-v0.1.md"),
    ("consignd",                "bailment"),
    ("consign/",                "bailment/"),
    ("CONSIGN",                 "BAILMENT"),
    ("Consign",                 "Bailment"),
    ("consign",                 "bailment"),
]
for old, new in pairs:
    s = s.replace(old, new)
p.write_text(s, encoding="utf-8")
PY
```

- [ ] **Step 2: Rewrite the naming block**

Near line 16 there is a blockquote beginning `> **Naming placeholder**`. The substitution in Step 1 will have mangled its prose (it explains the *consignment* metaphor, which no longer applies). Replace the **entire** blockquote — both lines — with exactly:

```markdown
> **Name**
> **BAILMENT** is the product name, settled in ADR-0001. A bailment is the delivery of goods for a specific purpose, without transfer of ownership, under a duty of care, with an obligation to return them per instructions — and the holder's authority is bounded by that purpose. That is this protocol's object model: possession crosses an organizational boundary while ownership does not, authority attenuates and never widens, and the result must come back conforming to a declared schema. Two earlier names were retired: *Hermes Federation* in v0.2, because the node runtime became agent-runtime-agnostic (D8) and Hermes is one adapter rather than the product; and the working name *Consign* in v0.3, because a consignment is goods sent for sale, which implies the settlement semantics D13 removed from scope.
```

- [ ] **Step 3: Update document control and revision history**

In §0, change the **Product** row value to exactly:

```
Bailment (formerly Consign; earlier Hermes Federation)
```

Change the **Version** row from `0.2` to `0.3`, and the header line under the title from `Version 0.2` to `Version 0.3`.

Append this row to the end of the revision-history table:

```markdown
| **0.3** | 11 Aug 2026 | Product renamed from Consign to Bailment (CN-002, ADR-0001). Extension namespace moved from the reserved `consign.example` placeholder to `bailment.dev`. Daemon renamed `consignd` → `bailment` with subcommands. No requirement, decision, or scope change. | TBD |
```

- [ ] **Step 4: Annotate the stale cover banner**

Find the line `![cover banner](media/cover_banner.png)`. The image reads "ROOT HERMES" — the v0.1 name — and draws a hub-and-spoke root topology that D8 and §7.1 removed. Immediately after the existing italic caption line that follows it, add exactly:

```markdown
**Stale asset — regenerate.** This banner still shows the v0.1 `ROOT HERMES` name and a central-root topology; both were removed in v0.2 (D8, §7.1).
```

- [ ] **Step 5: Verify the residue is only deliberate history**

```bash
cd "$(git rev-parse --show-toplevel)"
grep -in "consign" Readme.md
```

Expected: exactly two hits — the "Name" blockquote and the §0 Product row, both reading "Consign" as retired history. Any other hit is a miss.

- [ ] **Step 6: Run the Readme tests to verify they pass**

Run: `cd "$(git rev-parse --show-toplevel)" && python3 -m unittest tools.test_rename.TestRenameArtifacts.test_prd_version_bumped tools.test_rename.TestRenameArtifacts.test_cover_banner_annotated tools.test_rename.TestPreservedInvariants.test_roles_and_objects_untouched -v`

Expected: **3 tests PASS**.

- [ ] **Step 7: Commit**

```bash
git add Readme.md
git commit -m "docs: rename PRD to Bailment, bump to v0.3, flag stale banner (CN-002)"
```

---

### Task 5: Rename the backlog and tooling

**Files:**
- Modify: `BACKLOG.md` (8 occurrences, at lines 1, 3, 71, 76, 91, 157, 207, 353)
- Modify: `tools/backlog_to_issues.py:29` and its docstring at line 94

**Interfaces:**
- Consumes: Task 1's `test_backlog_has_no_consign`, `test_ticket_id_count_unchanged`, `test_issue_map_byte_identical_to_main`, `test_backlog_tool_parses`; Task 3's new spec filename.
- Produces: nothing downstream.

- [ ] **Step 1: Apply the substitutions**

```bash
cd "$(git rev-parse --show-toplevel)"
python3 - <<'PY'
import pathlib
pairs = [
    ("consign-profile-v0.1.md", "bailment-profile-v0.1.md"),
    ("node/cmd/consignd",       "node/cmd/bailment"),
    ("consignd",                "bailment"),
    ("consign/",                "bailment/"),
    ("CONSIGN",                 "BAILMENT"),
    ("Consign",                 "Bailment"),
    ("consign",                 "bailment"),
]
for name in ("BACKLOG.md", "tools/backlog_to_issues.py"):
    p = pathlib.Path(name)
    s = p.read_text(encoding="utf-8")
    for old, new in pairs:
        s = s.replace(old, new)
    p.write_text(s, encoding="utf-8")
PY
```

- [ ] **Step 2: Rewrite the CN-002 ticket body**

In `BACKLOG.md`, find the `### CN-002 — Decide the product name and execute the rename` section. Step 1 will have turned its sentence about the placeholder into nonsense ("BAILMENT is a working placeholder"). Replace the paragraph that begins `Choose a final name before the spec goes public` with exactly:

```markdown
Resolved: the product is named **Bailment**, recorded in [`docs/adr/0001-product-name.md`](docs/adr/0001-product-name.md). The working name and the earlier `Hermes Federation` name were both retired; see the ADR for the candidates considered and the availability evidence.
```

Leave the `**Done when:**` line and the ticket heading untouched.

- [ ] **Step 3: Update the repository constant**

In `tools/backlog_to_issues.py`, change line 29 from:

```python
REPO = "gabbarX/Federated-Inference"
```

to:

```python
# Correct only after the repository transfer completes (see ADR-0001).
# Do not run this script until then.
REPO = "bailment/bailment"
```

- [ ] **Step 4: Verify the backlog is clean and the tool still parses**

```bash
cd "$(git rev-parse --show-toplevel)"
grep -in "consign" BACKLOG.md tools/backlog_to_issues.py || echo "CLEAN"
python3 -c "import ast;ast.parse(open('tools/backlog_to_issues.py').read());print('PARSES')"
git diff --stat tools/.issue-map.json
```

Expected: `CLEAN`, then `PARSES`, then **empty output** from the last command — `.issue-map.json` must be unmodified.

- [ ] **Step 5: Run the backlog tests to verify they pass**

Run: `cd "$(git rev-parse --show-toplevel)" && python3 -m unittest tools.test_rename.TestNoConsignRemains.test_backlog_has_no_consign tools.test_rename.TestPreservedInvariants -v`

Expected: **all PASS**.

- [ ] **Step 6: Commit**

```bash
git add BACKLOG.md tools/backlog_to_issues.py
git commit -m "chore: rename backlog and tooling to Bailment (CN-002)"
```

---

### Task 6: Full suite green

**Files:**
- Modify: none expected. If a test fails, fix the offending content file — never weaken the test.

**Interfaces:**
- Consumes: every test from Task 1 and all content changes from Tasks 2-5.
- Produces: a verified-green branch ready for PR.

- [ ] **Step 1: Run the complete suite**

Run: `cd "$(git rev-parse --show-toplevel)" && python3 -m unittest tools.test_rename -v`

Expected: **all tests PASS**, zero failures, zero errors.

- [ ] **Step 2: Confirm main is untouched**

```bash
cd "$(git rev-parse --show-toplevel)"
git branch --show-current   # must print: rename/bailment
git diff --stat main..HEAD -- tools/.issue-map.json   # must print nothing
```

Expected: `rename/bailment`, then empty output.

- [ ] **Step 3: Confirm no media file was altered**

```bash
cd "$(git rev-parse --show-toplevel)"
git diff --stat main..HEAD -- media/
```

Expected: empty output. The banner annotation lives in `Readme.md`, not in `media/`.

- [ ] **Step 4: Commit any fixes**

If Steps 1-3 required content fixes, commit them:

```bash
git add -A ':!.gitignore' ':!CLAUDE.md' ':!.claude'
git commit -m "fix: correct rename misses found by verification suite (CN-002)"
```

If nothing needed fixing, skip this step — do not create an empty commit.

---

## Self-Review

**Spec coverage.** Every section of `2026-08-11-product-name-design.md` maps to a task: the decision and rejected candidates → Task 2 (ADR); normative wire identifiers and the `.example` → `bailment.dev` move → Task 3; document control, the Name note, the version bump and the media annotation → Task 4; the backlog, the `REPO` constant and its ordering caveat → Task 5; all six verification invariants → Tasks 1 and 6. Out-of-scope items (ticket IDs, issue map, roles, objects, `hermes-agent`, `media/` contents) are enforced as passing-from-the-start guard tests in Task 1 and as Global Constraints.

**Placeholder scan.** No TBD/TODO, no "add appropriate error handling", no "similar to Task N". Every code step carries its literal content, including the full ADR body and the full Name blockquote.

**Type consistency.** Test method names are defined once in Task 1's Interfaces block and referenced identically in Tasks 2-6. Substitution pair lists are repeated in full per task rather than cross-referenced, because implementers may read tasks out of order — and they differ per file by design (the spec has `ConsignRevocation`; the backlog has `node/cmd/consignd`).

**Known ordering hazard.** Tasks 3, 4 and 5 each run a naive `replace` before hand-fixing the prose it mangles. This is deliberate: the mechanical pass is verifiable by grep, and the two blocks whose *meaning* changes (the Name note, the CN-002 body) are rewritten explicitly in a following step rather than left to a substitution that cannot know intent.
