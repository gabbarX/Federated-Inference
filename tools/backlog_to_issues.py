#!/usr/bin/env python3
"""Create GitHub issues from BACKLOG.md.

Parses the ticket sections of BACKLOG.md and files one issue per ticket, with
milestones per PRD delivery phase and labels for epic, priority, and size.

Idempotent and resumable: tickets whose ID already appears as an issue title
prefix are skipped, so a partial run can simply be re-run.

Usage:
    python tools/backlog_to_issues.py --dry-run          # show what would happen
    python tools/backlog_to_issues.py                    # create everything
    python tools/backlog_to_issues.py --only CN-001,CN-002
    python tools/backlog_to_issues.py --phase 0 --phase 1
    python tools/backlog_to_issues.py --link-only        # redo cross-ref pass only
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
import time
from pathlib import Path

# Correct only after the repository transfer completes (see ADR-0001).
# Do not run this script until then.
REPO = "bailment/bailment"
ROOT = Path(__file__).resolve().parent.parent
BACKLOG = ROOT / "BACKLOG.md"
MAP_FILE = Path(__file__).resolve().parent / ".issue-map.json"

# Throttle content-creating calls to stay clear of GitHub's secondary rate limit.
SLEEP_SECONDS = 1.0

# PRD section 13.4. Keys are the **Phase:** field values used in BACKLOG.md.
# Titles use ASCII hyphens so they survive argv on every platform.
MILESTONES: dict[str, tuple[str, str]] = {
    "0": ("Phase 0 - Extension spike",
          "Prove the profile is expressible. Exit: the extension set round-trips "
          "through the A2A Go SDK without protocol violations."),
    "1": ("Phase 1 - Two-node core",
          "One verified task crosses an org boundary. Exit: AC-001 through AC-005 pass."),
    "2": ("Phase 2 - Resilience and conformance",
          "Failures are boring; strangers can implement. Exit: AC-006, AC-007 pass; "
          "suite runs green against a second endpoint."),
    "3": ("Phase 3 - Graphs",
          "Parallel specialists. Exit: AC-008 passes; three-node graph completes."),
    "4": ("Phase 4 - Policy and sovereignty",
          "Data-sovereign work becomes real. Exit: AC-009, AC-010 pass."),
    "5": ("Phase 5 - Authority and recursion",
          "Safe depth > 0. Exit: AC-011 passes; widening attempts are rejected by "
          "the grandchild."),
    "6": ("Phase 6 - Tiers and reach",
          "Anchors, hosted orgs, firewalled peers. Exit: AC-012 passes."),
    "7": ("Phase 7 - Accounting",
          "Settlement-ready without settlement. Exit: receipts reconcile across "
          "both parties' audit chains."),
    "cross": ("Cross-phase",
              "Hygiene and governance work not tied to a single delivery phase."),
    "post-v1": ("Post-v1",
                "Deferred beyond v1. Demoted in PRD v0.2; do not schedule ahead of "
                "deterministic work."),
}

EPIC_COLORS = [
    "5319e7", "0e8a16", "1d76db", "b60205", "fbca04", "006b75",
    "d93f0b", "0052cc", "5319e7", "0e8a16", "1d76db", "b60205",
    "fbca04", "006b75", "d93f0b", "0052cc",
]

PRIORITY_LABELS = {
    "MUST": ("priority:must", "b60205", "Defines the acceptance boundary (PRD section 6)"),
    "SHOULD": ("priority:should", "fbca04", "Expected but not acceptance-blocking"),
    "COULD": ("priority:could", "c2e0c6", "Optional; often demoted in PRD v0.2"),
}

SIZE_LABELS = {
    "S": ("size:S", "ededed", "Under a day"),
    "M": ("size:M", "d4c5f9", "A few days"),
    "L": ("size:L", "f9d0c4", "One to two weeks"),
    "XL": ("size:XL", "e99695", "Needs splitting before it is worked"),
}

GATE_LABEL = ("gate", "000000", "Blocks other work; read the Blocks field before starting")

BACKLOG_URL = f"https://github.com/{REPO}/blob/main/BACKLOG.md"

FOOTER = (
    "\n\n---\n"
    "Source: [`BACKLOG.md`](" + BACKLOG_URL + ") &middot; ticket `{tid}` "
    "&middot; epic **{epic_id} - {epic_title}**\n\n"
    "Derived from `Readme.md` (Bailment PRD v0.2). Edit the ticket in `BACKLOG.md` "
    "if scope changes, so the backlog and the issue tracker do not diverge.\n"
)


# --------------------------------------------------------------------------- #
# Parsing
# --------------------------------------------------------------------------- #

def parse_backlog(text: str) -> list[dict]:
    lines = text.split("\n")
    tickets: list[dict] = []
    epic: tuple[str, str] | None = None
    i = 0

    while i < len(lines):
        line = lines[i]

        epic_match = re.match(r"^## (E\d+) — (.+)$", line)
        if epic_match:
            epic = (epic_match.group(1), epic_match.group(2).strip())
            i += 1
            continue

        # Any other H2 ends the current epic (Epic index, Traceability, ...).
        if line.startswith("## "):
            epic = None
            i += 1
            continue

        ticket_match = re.match(r"^### (CN-\d+) — (.+)$", line)
        if ticket_match and epic is not None:
            tid = ticket_match.group(1)
            title = ticket_match.group(2).strip()
            j = i + 1
            body_lines: list[str] = []
            while j < len(lines):
                nxt = lines[j]
                if re.match(r"^#{2,3} ", nxt) or nxt.strip() == "---":
                    break
                body_lines.append(nxt)
                j += 1
            body = "\n".join(body_lines).strip("\n")
            tickets.append(
                {
                    "id": tid,
                    "title": title,
                    "epic_id": epic[0],
                    "epic_title": epic[1],
                    "body": body,
                    **extract_meta(body),
                }
            )
            i = j
            continue

        i += 1

    return tickets


def extract_meta(body: str) -> dict:
    """Pull phase / priority / size / gate / depends out of a ticket body.

    The metadata block is the first few lines of the body. Tickets in E14 carry
    their phase inside the Traces line instead of a dedicated Phase field, so
    fall back to scanning for 'Phase <n>'.
    """
    head = "\n".join(body.split("\n")[:8])

    phase_match = re.search(r"\*\*Phase:\*\*\s*([^\s·]+)", head)
    if phase_match:
        raw_phase = phase_match.group(1).strip()
    else:
        fallback = re.search(r"Phase (\d)", head)
        raw_phase = fallback.group(1) if fallback else "cross"

    if raw_phase == "—":          # em dash: cross-phase hygiene
        phase = "cross"
    elif raw_phase in MILESTONES:
        phase = raw_phase
    else:
        phase = "cross"

    priority_match = re.search(r"\*\*Priority:\*\*\s*(MUST|SHOULD|COULD)", head)
    size_match = re.search(r"\*\*Size:\*\*\s*(XL|S|M|L)\b", head)
    depends_match = re.search(r"\*\*Depends:\*\*\s*([^\n]+)", head)

    return {
        "phase": phase,
        "priority": priority_match.group(1) if priority_match else None,
        "size": size_match.group(1) if size_match else None,
        "gate": "**Blocks:**" in head,
        "depends": depends_match.group(1).strip() if depends_match else "",
    }


# --------------------------------------------------------------------------- #
# gh helpers
# --------------------------------------------------------------------------- #

def gh(args: list[str], check: bool = True) -> str:
    proc = subprocess.run(
        ["gh", *args],
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    if check and proc.returncode != 0:
        raise RuntimeError(
            f"gh {' '.join(args)}\nexit={proc.returncode}\n"
            f"stdout={proc.stdout}\nstderr={proc.stderr}"
        )
    return proc.stdout.strip()


def write_temp(body: str) -> Path:
    handle = tempfile.NamedTemporaryFile(
        mode="w", suffix=".md", delete=False, encoding="utf-8", newline="\n"
    )
    handle.write(body)
    handle.close()
    return Path(handle.name)


def ensure_labels(tickets: list[dict], dry_run: bool) -> None:
    wanted: list[tuple[str, str, str]] = []

    epics = {(t["epic_id"], t["epic_title"]) for t in tickets}
    for epic_id, epic_title in sorted(epics, key=lambda e: int(e[0][1:])):
        idx = int(epic_id[1:]) % len(EPIC_COLORS)
        wanted.append((f"epic:{epic_id}", EPIC_COLORS[idx], ascii_only(epic_title)))

    for priority in {t["priority"] for t in tickets if t["priority"]}:
        wanted.append(PRIORITY_LABELS[priority])
    for size in {t["size"] for t in tickets if t["size"]}:
        wanted.append(SIZE_LABELS[size])
    if any(t["gate"] for t in tickets):
        wanted.append(GATE_LABEL)

    for name, color, description in wanted:
        if dry_run:
            print(f"  label   {name:<16} #{color}  {description}")
            continue
        gh([
            "label", "create", name,
            "--repo", REPO,
            "--color", color,
            "--description", description,
            "--force",
        ])
        time.sleep(SLEEP_SECONDS)


def ensure_milestones(tickets: list[dict], dry_run: bool) -> dict[str, str]:
    existing_raw = gh([
        "api", f"repos/{REPO}/milestones",
        "--paginate", "-X", "GET", "-f", "state=all",
        "--jq", ".[].title",
    ], check=False)
    existing = {line.strip() for line in existing_raw.split("\n") if line.strip()}

    needed = {t["phase"] for t in tickets}
    resolved: dict[str, str] = {}

    for phase in sorted(needed, key=lambda p: (p not in "01234567", p)):
        title, description = MILESTONES[phase]
        resolved[phase] = title
        if title in existing:
            continue
        if dry_run:
            print(f"  milestone  {title}")
            continue
        gh([
            "api", f"repos/{REPO}/milestones",
            "-f", f"title={title}",
            "-f", f"description={description}",
        ])
        time.sleep(SLEEP_SECONDS)

    return resolved


def existing_issue_map() -> dict[str, int]:
    raw = gh([
        "issue", "list",
        "--repo", REPO,
        "--state", "all",
        "--limit", "1000",
        "--json", "number,title",
    ], check=False)
    if not raw:
        return {}
    found: dict[str, int] = {}
    for item in json.loads(raw):
        match = re.match(r"^(CN-\d+)\b", item["title"])
        if match:
            found[match.group(1)] = item["number"]
    return found


def ascii_only(text: str) -> str:
    replacements = {"—": "-", "–": "-", "·": "-", "≥": ">=", "§": "sec "}
    for src, dst in replacements.items():
        text = text.replace(src, dst)
    return text.encode("ascii", "ignore").decode("ascii")[:100]


# --------------------------------------------------------------------------- #
# Main passes
# --------------------------------------------------------------------------- #

def create_issues(tickets, milestones, mapping, dry_run) -> dict[str, int]:
    for ticket in tickets:
        tid = ticket["id"]
        if tid in mapping:
            print(f"  skip    {tid}  already filed as #{mapping[tid]}")
            continue

        labels = [f"epic:{ticket['epic_id']}"]
        if ticket["priority"]:
            labels.append(PRIORITY_LABELS[ticket["priority"]][0])
        if ticket["size"]:
            labels.append(SIZE_LABELS[ticket["size"]][0])
        if ticket["gate"]:
            labels.append(GATE_LABEL[0])

        title = f"{tid} — {ticket['title']}"
        body = ticket["body"] + FOOTER.format(
            tid=tid, epic_id=ticket["epic_id"], epic_title=ticket["epic_title"]
        )

        if dry_run:
            print(f"  create  {title[:78]}")
            print(f"            milestone={milestones[ticket['phase']]}  labels={','.join(labels)}")
            continue

        body_file = write_temp(body)
        try:
            args = [
                "issue", "create",
                "--repo", REPO,
                "--title", title,
                "--body-file", str(body_file),
                "--milestone", milestones[ticket["phase"]],
            ]
            for label in labels:
                args += ["--label", label]
            url = gh(args)
        finally:
            body_file.unlink(missing_ok=True)

        number = int(url.rstrip("/").rsplit("/", 1)[-1])
        mapping[tid] = number
        print(f"  created #{number:<4} {title[:70]}")
        MAP_FILE.write_text(json.dumps(mapping, indent=2, sort_keys=True), encoding="utf-8")
        time.sleep(SLEEP_SECONDS)

    return mapping


def link_cross_references(tickets, mapping, dry_run) -> None:
    """Second pass: sync every body from BACKLOG.md, annotating CN-### mentions.

    This writes unconditionally rather than only when cross-references change, so
    the pass doubles as a 'BACKLOG.md is the source of truth' sync: edit a ticket
    in the backlog, re-run with --link-only, and the issue body catches up.
    """

    def annotate(text: str, self_id: str) -> str:
        def repl(match: re.Match) -> str:
            tid = match.group(0)
            if tid == self_id or tid not in mapping:
                return tid
            tail = text[match.end():match.end() + 2]
            if tail.startswith(" (#"):
                return tid
            return f"{tid} (#{mapping[tid]})"

        return re.sub(r"\bCN-\d{3}\b", repl, text)

    for ticket in tickets:
        tid = ticket["id"]
        if tid not in mapping:
            continue
        original = ticket["body"] + FOOTER.format(
            tid=tid, epic_id=ticket["epic_id"], epic_title=ticket["epic_title"]
        )
        updated = annotate(original, tid)
        if dry_run:
            refs = sorted(set(re.findall(r"CN-\d{3} \(#\d+\)", updated)))
            print(f"  link    {tid} -> {len(refs)} refs")
            continue
        body_file = write_temp(updated)
        try:
            gh([
                "issue", "edit", str(mapping[tid]),
                "--repo", REPO,
                "--body-file", str(body_file),
            ])
        finally:
            body_file.unlink(missing_ok=True)
        print(f"  linked  #{mapping[tid]:<4} {tid}")
        time.sleep(SLEEP_SECONDS)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--only", default="", help="comma-separated ticket IDs")
    parser.add_argument("--phase", action="append", default=[], help="repeatable phase filter")
    parser.add_argument("--no-link", action="store_true", help="skip the cross-reference pass")
    parser.add_argument("--link-only", action="store_true", help="run only the cross-reference pass")
    args = parser.parse_args()

    tickets = parse_backlog(BACKLOG.read_text(encoding="utf-8"))
    if not tickets:
        print("No tickets parsed from BACKLOG.md", file=sys.stderr)
        return 1

    all_tickets = list(tickets)

    if args.only:
        wanted = {t.strip() for t in args.only.split(",") if t.strip()}
        tickets = [t for t in tickets if t["id"] in wanted]
    if args.phase:
        tickets = [t for t in tickets if t["phase"] in set(args.phase)]

    print(f"Parsed {len(all_tickets)} tickets; {len(tickets)} selected.")
    missing = [t["id"] for t in all_tickets if not t["priority"] or not t["size"]]
    if missing:
        print(f"  note: no priority/size parsed for {', '.join(missing)}")

    mapping = json.loads(MAP_FILE.read_text(encoding="utf-8")) if MAP_FILE.exists() else {}
    mapping.update({k: v for k, v in existing_issue_map().items()})

    if args.link_only:
        link_cross_references(all_tickets, mapping, args.dry_run)
        return 0

    print("Labels:")
    ensure_labels(all_tickets, args.dry_run)
    print("Milestones:")
    milestones = ensure_milestones(all_tickets, args.dry_run)
    if args.dry_run:
        milestones = {p: MILESTONES[p][0] for p in {t["phase"] for t in all_tickets}}

    print("Issues:")
    mapping = create_issues(tickets, milestones, mapping, args.dry_run)

    if not args.no_link:
        print("Cross-references:")
        link_cross_references(all_tickets, mapping, args.dry_run)

    print(f"\nDone. {len(mapping)} tickets mapped to issues.")
    if not args.dry_run:
        print(f"Map written to {MAP_FILE}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
