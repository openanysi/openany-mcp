"""Static checks for the Hermes portable plugin. No network and no secrets."""

from __future__ import annotations

import json
import re
import tomllib
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PLUGIN = ROOT / "plugins" / "openany"
PLUGIN_SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
MCP_SCHEMA = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
ENDPOINT = "https://mcp.openany.si"
SECRET = "OPENANY_API_KEY"
HEADER = "Bearer ${OPENANY_API_KEY}"


class HermesPluginManifestTests(unittest.TestCase):
    def setUp(self):
        self.plugin = json.loads((PLUGIN / "plugin.json").read_text())
        self.mcp = json.loads((PLUGIN / "mcp.json").read_text())
        self.readme = (PLUGIN / "README.md").read_text()
        self.skill = (PLUGIN / "skills" / "openany" / "SKILL.md").read_text()

    def test_plugin_manifest_matches_the_v1_subset_hermes_loads(self):
        self.assertEqual(self.plugin["$schema"], PLUGIN_SCHEMA)
        self.assertEqual(sorted(self.plugin), [
            "$schema", "author", "description", "extensions", "homepage",
            "keywords", "license", "name", "repository", "version",
        ])
        self.assertEqual(self.plugin["name"], "openany")
        self.assertEqual(self.plugin["version"], "0.2.1")
        self.assertEqual(self.plugin["license"], "MIT")
        self.assertEqual(self.plugin["author"], {"name": "Alephant AI LLC", "url": "https://openany.si"})
        self.assertNotIn("requires_hermes", self.plugin)
        hermes = self.plugin["extensions"]["com.nousresearch.hermes"]
        self.assertEqual(hermes["servers"]["openany"]["trust"], "untrusted")
        secret = hermes["config_schema"]["api_key"]
        self.assertEqual(secret["type"], "secret")
        self.assertEqual(secret["env"], SECRET)
        self.assertTrue(secret["required"])
        self.assertNotIn("costs", hermes["servers"]["openany"])

    def test_pyproject_declares_the_hermes_mcp_pin(self):
        document = tomllib.loads((PLUGIN / "pyproject.toml").read_text())
        project = document["project"]
        self.assertEqual(project["name"], "openany-hermes")
        self.assertEqual(project["version"], "0.2.1")
        self.assertEqual(project["requires-python"], ">=3.11,<3.15")
        self.assertEqual(project["dependencies"], ["mcp==2.3.0"])
        self.assertNotIn("build-system", document)
        self.assertFalse(document["tool"]["uv"]["package"])
        self.assertNotIn("\u2014", (PLUGIN / "pyproject.toml").read_text())

    def test_mcp_server_is_streamable_http_with_a_placeholder_header(self):
        self.assertEqual(self.mcp["$schema"], MCP_SCHEMA)
        self.assertEqual(set(self.mcp), {"$schema", "mcpServers"})
        server = self.mcp["mcpServers"]["openany"]
        self.assertEqual(set(server), {"type", "url", "headers"})
        self.assertEqual(server["type"], "streamable-http")
        self.assertEqual(server["url"], ENDPOINT)
        self.assertEqual(server["headers"], {"Authorization": HEADER})
        self.assertNotIn("sse", json.dumps(self.mcp))

    def test_package_text_does_not_embed_a_credential_or_catalog_count(self):
        blob = "\n".join(path.read_text() for path in PLUGIN.rglob("*") if path.is_file())
        self.assertNotRegex(blob, r"\bsk-[A-Za-z0-9_-]{8,}")
        self.assertNotRegex(blob, r"(?m)^\s*(?:export\s+)?OPENANY_API_KEY\s*=")
        self.assertNotRegex(blob, r"Bearer\s+[A-Za-z0-9._~+/-]{16,}")
        self.assertNotIn(".env", blob.replace("does not read `.env`", "").replace("profile `.env`", ""))
        for banned in ("168+", "3,200", "3,201", "3,220", "3220"):
            self.assertNotIn(banned, blob)
        self.assertNotIn("os.environ", blob)
        self.assertNotRegex(blob, r"(?i)self-updater|check for updates")

    def test_readme_discloses_network_billing_install_and_removal(self):
        for phrase in (
            "marketplace for agents",
            "mcp.openany.si",
            "pay-per-call",
            "OPENANY_API_KEY",
            "trust: untrusted",
            "hermes plugins remove openany",
            "tools/list",
            "not an authenticated",
            "Alephant AI LLC",
            "mcp-lite",
            "50 pages",
            "mcp_servers.openany",
            "coingecko_get_api_v3_simple_price",
            "420decad-3a6b-46f8-acff-53966bbd8b1c",
            "807 microUSD",
        ):
            self.assertIn(phrase, self.readme)
        self.assertIn("../../docs/hermes-e2e.md", self.readme)
        self.assertTrue((ROOT / "docs" / "hermes-e2e.md").exists())

    def test_skill_frontmatter_matches_the_directory_name(self):
        match = re.match(r"^---\n(.*?)\n---\n", self.skill, re.DOTALL)
        self.assertIsNotNone(match)
        frontmatter = match.group(1)
        self.assertIn("name: openany", frontmatter)
        description = re.search(r"description: (.+)", frontmatter).group(1)
        self.assertGreaterEqual(len(description), 1)
        self.assertLessEqual(len(description), 1024)
        self.assertIn("OPENANY_API_KEY", self.skill)
        self.assertIn(ENDPOINT, self.skill)
        self.assertIn("readOnlyHint", self.skill)
        for name in ("search_apis", "get_api", "run_api"):
            self.assertIn(name, self.skill)
        self.assertIn(
            "The packaged server is remote Streamable HTTP at `https://mcp.openany.si`.",
            self.skill,
        )


if __name__ == "__main__":
    unittest.main()
