from __future__ import annotations

from typing import Any

from dify_plugin.entities.credential import CredentialCheckResult
from dify_plugin.interfaces.tool import ToolProvider

import mcp_client


class OpenAnyProvider(ToolProvider):
    def _validate_credentials(self, credentials: dict[str, Any]) -> CredentialCheckResult:
        """Accept an empty secret. A present secret is checked locally and is not spent."""

        raw = None
        if isinstance(credentials, dict):
            raw = credentials.get("openany_api_key")
        status, _reason = mcp_client.credential_status(
            raw if isinstance(raw, str) or raw is None else ""
        )
        return CredentialCheckResult(
            status="valid" if status == "valid" else "invalid",
            reason_code=None if status == "valid" else "credential_invalid",
            retryable=False,
        )
