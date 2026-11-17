from collections.abc import Generator
from typing import Any

from dify_plugin.entities.tool import ToolInvokeMessage
from dify_plugin.interfaces.tool import Tool

from tools.common import invoke


class GetApiTool(Tool):
    def _invoke(self, tool_parameters: dict[str, Any]) -> Generator[ToolInvokeMessage, None, None]:
        yield from invoke(self, "get_api", tool_parameters)
