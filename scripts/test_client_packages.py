"""Static checks for the Gemini CLI, Cursor, Claude Code, and OpenClaw install files."""

from __future__ import annotations

import json
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LITE = "https://mcp.openany.si"
SECRET = "OPENANY_API_KEY"
HEADER = "Bearer ${OPENANY_API_KEY}"
CLAUDE_HEADER = "Bearer ${OPENANY_API_KEY:-}"
DESCRIPTION = (
    "The B2A (business to agent) marketplace. One MCP endpoint for 3,200+ APIs "
    "(social, SEO, crypto, company, finance, legal) and 168+ AI models "
    "(GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo, Kling). Browse free; pay per call."
)
SHORT_DESCRIPTION = (
    "The B2A (business to agent) marketplace: 3,200+ APIs and 168+ AI models. Browse free; pay per call."
)


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
        self.assertNotIn(SECRET, plugin["variables"]["required"])
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
        self.assertNotIn("requires:", frontmatter)
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
        self.assertEqual(plugin["description"], DESCRIPTION)

    def test_new_install_files_have_no_secret_or_em_dash(self):
        paths = [
            ROOT / "gemini-extension.json",
            ROOT / "GEMINI.md",
            ROOT / ".mcp.json",
            ROOT / ".cursor-plugin" / "plugin.json",
            ROOT / "plugins" / "openclaw" / "openany" / "SKILL.md",
            ROOT / "plugins" / "openclaw" / "openany" / "README.md",
            ROOT / ".claude-plugin" / "marketplace.json",
            ROOT / "plugins" / "claude-code" / ".claude-plugin" / "plugin.json",
            ROOT / "plugins" / "claude-code" / ".mcp.json",
            ROOT / "plugins" / "claude-code" / "skills" / "openany" / "SKILL.md",
        ]
        for path in paths:
            text = path.read_text()
            with self.subTest(path=path.name):
                self.assertNotIn("\u2014", text)
                self.assertNotIn("\u2013", text)
                self.assertNotRegex(text, r"\bsk-[A-Za-z0-9_-]{8,}")
                self.assertNotRegex(text, r"(?m)^\s*(?:export\s+)?OPENANY_API_KEY\s*=")
                self.assertNotRegex(text, r"Bearer\s+[A-Za-z0-9._~+/-]{16,}")
                for banned in ("3,201", "3,220", "3220"):
                    self.assertNotIn(banned, text)


    def test_claude_code_plugin_marketplace_and_optional_header(self):
        hermes = load_json(ROOT / "plugins" / "openany" / "plugin.json")
        plugin = load_json(ROOT / "plugins" / "claude-code" / ".claude-plugin" / "plugin.json")
        market = load_json(ROOT / ".claude-plugin" / "marketplace.json")
        mcp = load_json(ROOT / "plugins" / "claude-code" / ".mcp.json")
        self.assertLessEqual(len(SHORT_DESCRIPTION), 200)
        self.assertGreater(len(DESCRIPTION), 200)
        self.assertEqual(plugin["description"], DESCRIPTION)
        self.assertEqual(plugin["description"], hermes["description"])
        self.assertEqual(plugin["name"], "openany")
        self.assertEqual(plugin["displayName"], "OpenAny")
        self.assertEqual(plugin["version"], hermes["version"])
        self.assertEqual(plugin["author"]["name"], "Alephant AI LLC")
        self.assertEqual(plugin["author"]["email"], "team@openany.si")
        self.assertEqual(plugin["homepage"], "https://openany.si")
        self.assertEqual(plugin["repository"], "https://github.com/openanysi/openany-mcp")
        self.assertEqual(plugin["license"], "MIT")
        self.assertEqual(plugin["privacyPolicyUrl"], "https://openany.si/privacy/")
        self.assertEqual(market["name"], "openanysi")
        self.assertEqual(market["description"], DESCRIPTION)
        self.assertEqual(market["owner"]["name"], "Alephant AI LLC")
        self.assertEqual(market["owner"]["email"], "team@openany.si")
        entry, = market["plugins"]
        self.assertEqual(entry["name"], plugin["name"])
        self.assertEqual(entry["source"], "./plugins/claude-code")
        self.assertNotIn("version", entry)
        self.assertEqual(entry["description"], DESCRIPTION)
        self.assertEqual(entry["license"], "MIT")
        self.assertNotIn("..", entry["source"])
        server = mcp["mcpServers"]["openany"]
        self.assertEqual(set(server), {"type", "url", "headers"})
        self.assertEqual(server["type"], "streamable-http")
        self.assertEqual(server["url"], LITE)
        self.assertEqual(server["headers"], {"Authorization": CLAUDE_HEADER})
        self.assertNotIn("/mcp/", server["url"])
        skill = (ROOT / "plugins" / "claude-code" / "skills" / "openany" / "SKILL.md").read_text()
        match = re.match(r"^---\n(.*?)\n---\n", skill, re.DOTALL)
        self.assertIsNotNone(match)
        frontmatter = match.group(1)
        self.assertIn("name: openany", frontmatter)
        description = re.search(r"description: (.+)", frontmatter).group(1)
        self.assertEqual(description, DESCRIPTION)
        self.assertLessEqual(len(description), 1024)
        self.assertIn(LITE, skill)
        self.assertIn("Streamable HTTP", skill)
        self.assertIn(SECRET, skill)
        self.assertIn("readOnlyHint", skill)
        for name in ("openany_overview", "search_apis", "get_api", "run_api"):
            self.assertIn(name, skill)
        readme = (ROOT / "README.md").read_text()
        self.assertIn("## Claude Code", readme)
        self.assertIn("/plugin marketplace add openanysi/openany-mcp", readme)
        self.assertIn("/plugin install openany@openanysi", readme)
        self.assertIn(CLAUDE_HEADER, readme)
        self.assertIn("plugins/claude-code/.mcp.json", readme)


if __name__ == "__main__":
    unittest.main()
