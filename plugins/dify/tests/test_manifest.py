"""Validate the Dify manifest and provider schema with dify-plugin 0.11.1."""

from __future__ import annotations

import os
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from dify_plugin.config.config import DifyPluginEnv
from dify_plugin.core.plugin_registration import PluginRegistration
from dify_plugin.entities.provider_config import ProviderConfig
from dify_plugin.entities.tool import ToolParameter


class ManifestTests(unittest.TestCase):
    def test_sdk_loads_manifest_provider_tools_and_icon(self):
        previous = Path.cwd()
        os.chdir(ROOT)
        try:
            registration = PluginRegistration(DifyPluginEnv())
        finally:
            os.chdir(previous)

        configuration = registration.configuration
        self.assertEqual(configuration.version, "0.1.0")
        self.assertEqual(configuration.author, "openanysi")
        self.assertEqual(configuration.name, "openany")
        self.assertEqual(configuration.meta.version, "0.0.1")
        self.assertIsNone(configuration.meta.minimum_dify_version)
        self.assertEqual(configuration.meta.runner.language.value, "python")
        self.assertEqual(configuration.meta.runner.version, "3.12")
        self.assertEqual(configuration.meta.runner.entrypoint, "main")
        self.assertEqual(configuration.repo, "https://github.com/openanysi/openany-mcp")
        self.assertIn("3,200+", configuration.description.en_us)
        self.assertIn("168+", configuration.description.en_us)
        self.assertIn("The B2A (business to agent) marketplace", configuration.description.en_us)

        raw = (ROOT / "manifest.yaml").read_text()
        self.assertIn("privacy: ./PRIVACY.md", raw)
        self.assertIn("contact: team@openany.si", raw)
        self.assertIn("mcp.openany.si", raw)
        self.assertNotIn("minimum_dify_version", raw)
        self.assertNotIn("verified:", raw)

        provider = registration.tools_configuration[0]
        self.assertEqual(provider.identity.name, "openany")
        credential = provider.credentials_schema[0]
        self.assertIsInstance(credential, ProviderConfig)
        self.assertEqual(credential.name, "openany_api_key")
        self.assertEqual(credential.type.value, "secret-input")
        self.assertFalse(credential.required)
        names = [tool.identity.name for tool in provider.tools]
        self.assertEqual(
            names,
            ["openany_overview", "search_apis", "get_api", "run_api"],
        )
        run = next(tool for tool in provider.tools if tool.identity.name == "run_api")
        by_name = {parameter.name: parameter for parameter in run.parameters}
        self.assertEqual(by_name["input"].type, ToolParameter.ToolParameterType.OBJECT)
        self.assertEqual(by_name["input"].input_schema["type"], "object")
        self.assertEqual(by_name["output"].type, ToolParameter.ToolParameterType.OBJECT)
        self.assertEqual(
            by_name["output"].input_schema["properties"]["mode"]["enum"],
            ["raw", "compact", "summary"],
        )
        self.assertTrue(by_name["api_version"].required)
        self.assertFalse(by_name["output"].required)
        search = next(tool for tool in provider.tools if tool.identity.name == "search_apis")
        limit = next(parameter for parameter in search.parameters if parameter.name == "limit")
        self.assertEqual(limit.min, 1)
        self.assertEqual(limit.max, 10)
        self.assertEqual(limit.default, 5)
        icons = [asset.filename for asset in registration.files]
        self.assertIn("logo.png", icons)
        self.assertGreater(len(registration.tools_mapping["openany"][2]), 0)


if __name__ == "__main__":
    unittest.main()
