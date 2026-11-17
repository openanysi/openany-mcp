from __future__ import annotations

import json
from collections.abc import Generator
from typing import Any

from dify_plugin.entities.tool import ToolInvokeMessage

import mcp_client

_TEXT_LIMIT = 20000


def invoke(tool: Any, name: str, tool_parameters: dict[str, Any]) -> Generator[ToolInvokeMessage, None, None]:
    secret = None
    paid = name == "run_api"
    if paid:
        credentials = getattr(tool.runtime, "credentials", None) or {}
        secret = credentials.get("openany_api_key")
    payload = mcp_client.call_tool(
        name,
        tool_parameters,
        paid=paid,
        api_key=secret if isinstance(secret, str) else None,
    )
    yield tool.create_json_message(payload)
    text = json.dumps(payload, ensure_ascii=False, separators=(",", ":"))
    if len(text) > _TEXT_LIMIT:
        text = (
            text[:_TEXT_LIMIT]
            + "\n[text view truncated; the JSON message has the full result]"
        )
    yield tool.create_text_message(text)
