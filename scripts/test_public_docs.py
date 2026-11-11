"""Offline checks for the public repository descriptor and client examples."""
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
ENDPOINT = "https://mcp.openany.si"
RECOMMENDED = "https://mcp.openany.si"
FULL_CATALOG = "https://mcp.openany.si/mcp"
REGISTRY_DESCRIPTION = "The marketplace for agents: 3,200+ APIs and 100+ AI models, one key. Browse free; pay per call."


class PublicDocsTests(unittest.TestCase):
    def test_lightweight_endpoint_keeps_execution_boundaries(self):
        readme = (ROOT / "README.md").read_text()
        for value in ["https://mcp.openany.si/mcp-lite", "openany_overview", "search_apis", "get_api", "run_api", "MCP_LITE_ENABLED=false", "may incur charges"]:
            self.assertIn(value, readme)
        self.assertIn("When enabled and routed by the deployment", readme)
        self.assertIn("No new authenticated live", readme)
        self.assertNotIn("continue to use `/mcp`", readme)
        self.assertIn("advanced full-catalog endpoint", readme)
        self.assertIn("3,268 per-API tools", readme)

    def test_registry_descriptor(self):
        data = json.loads((ROOT / "server.json").read_text())
        self.assertEqual(data["$schema"], "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json")
        self.assertEqual(data["name"], "si.openany/openany")
        self.assertEqual(data["title"], "OpenAny")
        self.assertEqual(data["version"], "0.1.1")
        self.assertEqual(data["description"], REGISTRY_DESCRIPTION)
        self.assertLessEqual(len(data["description"]), 100)
        self.assertEqual(data["websiteUrl"], "https://openany.si")
        self.assertEqual(data["repository"], {"url": "https://github.com/openanysi/openany-mcp", "source": "github"})
        remote, = data["remotes"]
        self.assertEqual(remote["type"], "streamable-http")
        self.assertEqual(remote["url"], RECOMMENDED)
        header, = remote["headers"]
        self.assertEqual(header["name"], "Authorization")
        self.assertFalse(header["isRequired"])
        self.assertTrue(header["isSecret"])
        self.assertEqual(header["value"], "Bearer {OPENANY_API_KEY}")
        self.assertEqual(header["description"], "OpenAny API key as Bearer token. Optional for browsing; required for run_api.")
        variable = header["variables"]["OPENANY_API_KEY"]
        self.assertFalse(variable["isRequired"])
        self.assertTrue(variable["isSecret"])
        listing = json.loads((ROOT / "docs" / "listing" / "server.json").read_text())
        self.assertEqual(listing, data)
        glama = json.loads((ROOT / "glama.json").read_text())
        self.assertEqual(glama["$schema"], "https://glama.ai/mcp/schemas/server.json")
        self.assertEqual(glama["maintainers"], ["ComunionIO"])

    def test_json_examples_preserve_environment_syntax(self):
        for filename, variable in [("cursor.mcp.json", "${env:OPENANY_API_KEY}"), ("pi.mcp.json", "${OPENANY_API_KEY}"), ("openclaw.json", "${OPENANY_API_KEY}")]:
            with self.subTest(filename=filename):
                data = json.loads((ROOT / "examples" / filename).read_text())
                self.assertIn("CONFIRMED authentication header", data["$comment"])
                server = data["mcp"]["servers"]["openany"] if filename == "openclaw.json" else data["mcpServers"]["openany"]
                self.assertEqual(server["url"], ENDPOINT)
                self.assertNotIn(FULL_CATALOG, json.dumps(data))
                self.assertEqual(server["headers"]["Authorization"], "Bearer " + variable)
                if filename == "openclaw.json":
                    self.assertEqual(server["transport"], "streamable-http")
                    self.assertEqual(server["connectionTimeoutMs"], 10000)
                    self.assertEqual(server["requestTimeoutMs"], 30000)

    def test_all_examples_use_confirmed_header_and_never_contain_a_real_key(self):
        files = sorted((ROOT / "examples").iterdir())
        self.assertEqual(len(files), 7)
        for path in files:
            with self.subTest(path=path.name):
                text = path.read_text()
                self.assertIn("CONFIRMED authentication header", text)
                self.assertNotRegex(text, r"(?i)unconfirmed|assumed")
                self.assertIn("OPENANY_API_KEY", text)
                self.assertIn(ENDPOINT, text)
                self.assertNotIn(FULL_CATALOG, text)
                self.assertNotRegex(text, r"\bsk-[A-Za-z0-9_-]{8,}")
                self.assertNotRegex(text, r"(?m)^\s*(?:export\s+)?OPENANY_API_KEY\s*=")
        codex = (ROOT / "examples/codex.config.toml").read_text()
        self.assertIn('[mcp_servers.openany]', codex)
        self.assertIn('bearer_token_env_var = "OPENANY_API_KEY"', codex)
        hermes = (ROOT / "examples/hermes.config.yaml").read_text()
        self.assertNotRegex(hermes, r"(?m)^\s*transport:\s*sse\s*$")

    def test_readme_links_and_evidence_boundaries(self):
        readme = (ROOT / "README.md").read_text()
        for name in [path.name for path in (ROOT / "examples").iterdir()]:
            self.assertIn("examples/" + name, readme)
        for target in re.findall(r"\]\(([^)]+)\)", readme):
            if not target.startswith(("https://", "#")):
                self.assertTrue((ROOT / target).exists(), target)
        for text in ["Alephant AI LLC", "No OAuth", "Confirmed authentication header", "One MCP for 3,200+ APIs and 100+ AI models", "Browsing is free; pay per call.", "no subscription", "https://api.openany.si/v1", "Authorization: Bearer <OpenAny API key>` is optional"]:
            self.assertIn(text, readme)
        for stale in ["168+", "3,201", "3,220", "3,220 MCP tools"]:
            self.assertNotIn(stale, readme)
        self.assertIn("No new authenticated live `tools/call`", readme)
        self.assertNotRegex(readme, r"(?i)unconfirmed|assumed")
        snippets = (ROOT / "AGENTS-snippet.md").read_text()
        self.assertIn("confirmed authentication header", snippets)
        self.assertIn("已确认认证头", snippets)
        self.assertNotRegex(snippets, r"(?i)unconfirmed|assumed|待确认")
        self.assertIn("MIT License", (ROOT / "LICENSE").read_text())
        self.assertIn("## Runtime behavior", (ROOT / "docs/development.md").read_text())


if __name__ == "__main__":
    unittest.main()
