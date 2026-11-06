"""Offline checks for the public repository descriptor and client examples."""
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
ENDPOINT = "https://mcp.openany.si/mcp"


class PublicDocsTests(unittest.TestCase):
    def test_lightweight_endpoint_keeps_execution_boundaries(self):
        readme = (ROOT / "README.md").read_text()
        for value in ["https://mcp.openany.si/mcp-lite", "search_apis", "get_api", "run_api", "MCP_LITE_ENABLED=false", "may incur charges", "continue to use `/mcp`"]:
            self.assertIn(value, readme)
        self.assertIn("When enabled and routed by the deployment", readme)
        self.assertIn("No new authenticated live", readme)

    def test_registry_descriptor(self):
        data = json.loads((ROOT / "server.json").read_text())
        self.assertEqual(data["$schema"], "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json")
        self.assertEqual(data["name"], "si.openany/openany")
        self.assertEqual(data["title"], "OpenAny")
        self.assertEqual(data["version"], "0.1.0")
        self.assertLessEqual(len(data["description"]), 100)
        self.assertEqual(data["repository"], {"url": "https://github.com/openanysi/openany-mcp", "source": "github"})
        remote, = data["remotes"]
        self.assertEqual(remote["type"], "streamable-http")
        self.assertEqual(remote["url"], ENDPOINT)
        header, = remote["headers"]
        self.assertEqual(header["name"], "Authorization")
        self.assertTrue(header["isRequired"])
        self.assertTrue(header["isSecret"])

    def test_json_examples_preserve_environment_syntax(self):
        for filename, variable in [("cursor.mcp.json", "${env:OPENANY_API_KEY}"), ("pi.mcp.json", "${OPENANY_API_KEY}"), ("openclaw.json", "${OPENANY_API_KEY}")]:
            with self.subTest(filename=filename):
                data = json.loads((ROOT / "examples" / filename).read_text())
                self.assertIn("CONFIRMED authentication header", data["$comment"])
                server = data["mcp"]["servers"]["openany"] if filename == "openclaw.json" else data["mcpServers"]["openany"]
                self.assertEqual(server["url"], ENDPOINT)
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
        for text in ["Alephant AI LLC", "No OAuth", "Confirmed authentication header", "3,201", "3,220", "130", "no subscription", "https://api.openany.si/v1"]:
            self.assertIn(text, readme)
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
