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
        spec_path = ROOT / "spec" / "bailment-profile-v0.1.md"
        self.assertTrue(spec_path.exists(), "spec/bailment-profile-v0.1.md missing")
        spec = spec_path.read_text(encoding="utf-8")
        for ext in ("contract", "constraints", "authority", "budget",
                    "artifacts", "verification", "receipts"):
            self.assertIn(f"https://bailment.dev/ext/{ext}/v1", spec)

    def test_prd_version_bumped(self):
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
