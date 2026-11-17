"""LangChain standard chat-model unit tests.

Live ChatModelIntegrationTests are not collected. They would call the
provider, and no paid OpenAny request is part of this package.
"""

from __future__ import annotations

from typing import Any

from langchain_tests.unit_tests import ChatModelUnitTests

from langchain_openany import ChatOpenAny


class TestChatOpenAnyStandard(ChatModelUnitTests):
    @property
    def chat_model_class(self) -> type[ChatOpenAny]:
        return ChatOpenAny

    @property
    def chat_model_params(self) -> dict[str, Any]:
        return {
            "model": "model-from-listing",
            "api_key": "sk-test-openany-not-a-real-key",
        }

    @property
    def init_from_env_params(self) -> tuple[dict[str, str], dict[str, Any], dict[str, Any]]:
        return (
            {"OPENANY_API_KEY": "sk-test-openany-not-a-real-key"},
            {"model": "model-from-listing"},
            {"openany_api_key": "sk-test-openany-not-a-real-key"},
        )


def test_live_integration_suite_is_not_run() -> None:
    """Document the skipped official integration suite. Do not remove the skip."""
    reason = (
        "ChatModelIntegrationTests perform live model calls. "
        "No OpenAny API key is available and this package was not validated against the paid API."
    )
    assert "live model calls" in reason
