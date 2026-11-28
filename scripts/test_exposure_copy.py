"""Offline consistency checks for maintained discovery and distribution copy."""
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
ACTIVE_COPY = [
    "README.md", "GEMINI.md", "AGENTS-snippet.md", "server.json", "docs/listing/server.json",
    "docs/mcp.md", "docs/mcp.en.md", "docs/listing/connect.en.md", "docs/listing/submissions.md",
    "docs/listing/tool-annotations.md", "docs/hermes-catalog-draft.yaml", ".antigravity-plugin/README.md",
    "plugins/openany/README.md", "plugins/openany/skills/openany/SKILL.md",
    "plugins/openclaw/openany/README.md", "plugins/pi/README.md", "plugins/pi/skills/openany/SKILL.md",
    "plugins/deepseek-harness/README.md", "plugins/deepseek-harness/DETAIL.md", "package.json", "locale/en.json", "locale/zh.json",
    "providers/README.md", "providers/js/openany-ai-sdk-provider/README.md", "providers/python/langchain-openany/README.md",
    "plugins/claude-code/.codex-plugin/plugin.json", "plugins/claude-code/.claude-plugin/plugin.json",
    "skills/openany/SKILL.md", "plugins/claude-code/skills/openany/SKILL.md", "plugins/openclaw/openany/SKILL.md",
]


class ExposureCopyTests(unittest.TestCase):
    def test_maintained_entry_copy_has_no_fixed_catalog_promises(self):
        for name in ACTIVE_COPY:
            with self.subTest(path=name):
                text = (ROOT / name).read_text()
                self.assertNotRegex(text, r"4,400\+|168\+|3,268|1,247")
                self.assertNotRegex(text, r"(?i)exposes four tools|confirm the four tools|确认四个工具可见")
                self.assertNotIn("`openany_overview`, `search_apis`, `get_api`, and `run_api`.", text)

    def test_default_five_tool_copy_and_image_defaults_agree(self):
        for name in [
            "README.md", "GEMINI.md", "AGENTS-snippet.md", "plugins/pi/README.md",
            "plugins/openany/README.md", "plugins/openclaw/openany/README.md",
            "plugins/deepseek-harness/README.md", "plugins/deepseek-harness/DETAIL.md",
            "skills/openany/SKILL.md", "plugins/claude-code/skills/openany/SKILL.md",
            "plugins/openany/skills/openany/SKILL.md", "plugins/pi/skills/openany/SKILL.md",
            "plugins/openclaw/openany/SKILL.md",
        ]:
            with self.subTest(path=name):
                text = (ROOT / name).read_text()
                self.assertIn("five tools", text)
                for tool in ["openany_overview", "search_apis", "get_api", "run_api", "get_account"]:
                    self.assertIn(tool, text)
        image = (ROOT / "Dockerfile").read_text()
        self.assertIn("MCP_ACCOUNT_ENABLED=true", image)
        self.assertIn("MCP_BENEFITS_ENABLED=true", image)

    def test_registry_and_card_descriptions_are_bounded_and_equal(self):
        registry = json.loads((ROOT / "server.json").read_text())
        self.assertEqual(registry, json.loads((ROOT / "docs/listing/server.json").read_text()))
        source = (ROOT / "internal/mcpserver/servercard.go").read_text()
        base = json.loads(re.search(r'const serverCardDescription = ("[^"\n]*")', source)[1])
        benefits = json.loads(re.search(r'const serverCardBenefits = ("[^"\n]*")', source)[1])
        self.assertEqual(base, registry["description"])
        self.assertEqual(registry["version"], "0.1.5")
        for text in [base, benefits]:
            self.assertLessEqual(len(text), 100)
            for keyword in ["SEO", "social", "crypto", "lead", "AI models"]:
                self.assertIn(keyword, text)
        self.assertIn("Trial and referral", benefits)

    def test_main_routing_keywords_remain_in_shared_client_description(self):
        description = json.loads((ROOT / ".cursor-plugin/plugin.json").read_text())["description"]
        for keyword in ["DataForSEO", "Xiaohongshu", "小红书", "Douyin", "抖音", "Weibo", "微博", "Hunter", "CoinGecko"]:
            self.assertIn(keyword, description)
        self.assertIn("openany_overview", (ROOT / "internal/mcpserver/lite.go").read_text())


if __name__ == "__main__":
    unittest.main()
