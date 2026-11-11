"""Static checks for the Gemini CLI, Cursor, and OpenClaw install files."""

from __future__ import annotations

import json
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LITE = "https://mcp.openany.si"
SECRET = "OPENANY_API_KEY"
HEADER = "Bearer ${OPENANY_API_KEY}"


def load_json(path: Path) -> dict:
    return json.loads(path.read_text())


class ClientPackageTests(unittest.TestCase):
    def test_gemini_extension_uses_lite_http_and_a_sensitive_setting(self):
        manifest = load_json(ROOT / "gemini-extension.json")
        self.assertEqual(manifest["name"], "openany")
        self.assertEqual(manifest["version"], "0.1.0")
        self.assertRegex(manifest["name"], r"^[a-z0-9-]+$")
        server = manifest["mcpServers"]["openany"]
        self.assertEqual(set(server), {"httpUrl", "headers"})
        self.assertEqual(server["httpUrl"], LITE)
        self.assertEqual(server["headers"], {"Authorization": HEADER})
        self.assertNotIn("url", server)
        self.assertEqual(manifest["contextFileName"], "GEMINI.md")
        self.assertTrue((ROOT / "GEMINI.md").is_file())
        setting, = manifest["settings"]
        self.assertEqual(setting["envVar"], SECRET)
        self.assertTrue(setting["sensitive"])
        self.assertIn("search_apis", (ROOT / "GEMINI.md").read_text())
        self.assertIn("get_api", (ROOT / "GEMINI.md").read_text())
        self.assertIn("run_api", (ROOT / "GEMINI.md").read_text())

    def test_cursor_plugin_points_at_the_directory_file(self):
        plugin = load_json(ROOT / ".cursor-plugin" / "plugin.json")
        mcp = load_json(ROOT / ".mcp.json")
        self.assertEqual(plugin["name"], "openany")
        self.assertEqual(plugin["mcpServers"], ".mcp.json")
        self.assertEqual(plugin["license"], "MIT")
        variable = plugin["variables"]["properties"][SECRET]
        self.assertEqual(variable["type"], "string")
        self.assertIn(SECRET, plugin["variables"]["required"])
        server = mcp["mcpServers"]["openany"]
        self.assertEqual(server["url"], LITE)
        self.assertEqual(server["headers"], {"Authorization": HEADER})
        self.assertFalse((ROOT / "mcp.json").exists())

    def test_openclaw_skill_front_matter_declares_the_key(self):
        text = (ROOT / "plugins" / "openclaw" / "openany" / "SKILL.md").read_text()
        match = re.match(r"^---\n(.*?)\n---\n", text, re.DOTALL)
        self.assertIsNotNone(match)
        frontmatter = match.group(1)
        self.assertIn("name: openany", frontmatter)
        self.assertIn("homepage: https://openany.si", frontmatter)
        self.assertIn("primaryEnv: OPENANY_API_KEY", frontmatter)
        self.assertRegex(frontmatter, r"requires:\n\s+env:\n\s+- OPENANY_API_KEY")
        self.assertIn(LITE, text)
        self.assertIn("streamable-http", text)
        self.assertIn(HEADER, text)
        for name in ("search_apis", "get_api", "run_api"):
            self.assertIn(name, text)
        readme = (ROOT / "plugins" / "openclaw" / "openany" / "README.md").read_text()
        self.assertIn(LITE, readme)
        hermes_skill = (ROOT / "plugins" / "openany" / "skills" / "openany" / "SKILL.md").read_text()
        self.assertNotEqual(text, hermes_skill)

    def test_hermes_manifest_description_is_not_hermes_specific(self):
        plugin = load_json(ROOT / "plugins" / "openany" / "plugin.json")
        self.assertNotIn("Hermes", plugin["description"])
        self.assertIn("com.nousresearch.hermes", plugin["extensions"])
        self.assertIn("lite MCP server", plugin["description"])

    def test_new_install_files_have_no_secret_or_em_dash(self):
        paths = [
            ROOT / "gemini-extension.json",
            ROOT / "GEMINI.md",
            ROOT / ".mcp.json",
            ROOT / ".cursor-plugin" / "plugin.json",
            ROOT / "plugins" / "openclaw" / "openany" / "SKILL.md",
            ROOT / "plugins" / "openclaw" / "openany" / "README.md",
        ]
        for path in paths:
            text = path.read_text()
            with self.subTest(path=path.name):
                self.assertNotIn("\u2014", text)
                self.assertNotIn("\u2013", text)
                self.assertNotRegex(text, r"\bsk-[A-Za-z0-9_-]{8,}")
                self.assertNotRegex(text, r"(?m)^\s*(?:export\s+)?OPENANY_API_KEY\s*=")
                self.assertNotRegex(text, r"Bearer\s+[A-Za-z0-9._~+/-]{16,}")
                for banned in ("168+", "3,200", "3,201", "3,220", "3220"):
                    self.assertNotIn(banned, text)


if __name__ == "__main__":
    unittest.main()
