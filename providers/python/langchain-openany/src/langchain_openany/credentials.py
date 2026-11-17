"""Credential checks. Values are never copied into errors."""

from __future__ import annotations

KEY_PAGE_URL = "https://openany.si/dashboard?page=api-keys"

_PLACEHOLDERS = frozenset(
    {
        "YOUR_API_KEY",
        "YOUR_OPENANY_KEY",
        "YOUR_OPENANY_API_KEY",
        "OPENANY_API_KEY",
    }
)


class OpenAnyCredentialsError(ValueError):
    """Raised before a request when the API key is missing or unusable."""

    is_retryable = False


def _ident(value: str) -> bool:
    if not value:
        return False
    for index, character in enumerate(value):
        if index == 0:
            if not (character == "_" or character.isalpha()):
                return False
        elif not (character == "_" or character.isalnum()):
            return False
    return True


def is_credential_placeholder(material: str) -> bool:
    """Return whether a raw key is an unexpanded template."""
    if not material:
        return False
    if "${" in material:
        return True
    if material.startswith("$") and _ident(material[1:]):
        return True
    if (
        len(material) > 2
        and material.startswith("<")
        and material.endswith(">")
        and not any(character.isspace() or character in "<>" for character in material[1:-1])
    ):
        return True
    return material.upper() in _PLACEHOLDERS


def assert_api_key(value: str | None) -> str:
    """Return a raw API key or raise before any network call."""
    if value is None or value.strip() == "":
        raise OpenAnyCredentialsError(
            "OpenAny API key is missing. Set OPENANY_API_KEY or pass api_key. "
            f"Create a key at {KEY_PAGE_URL}. Do not paste the key into chat."
        )
    if any(character.isspace() for character in value) or value.lower().startswith("bearer"):
        raise OpenAnyCredentialsError(
            "OpenAny api_key must be the raw key, without a Bearer prefix or whitespace. "
            "The adapter sets Authorization itself."
        )
    if is_credential_placeholder(value.strip()):
        raise OpenAnyCredentialsError(
            "OpenAny API key is an unexpanded placeholder. Set OPENANY_API_KEY in the "
            "environment or secret settings. Do not paste the key into chat. "
            f"Create a key at {KEY_PAGE_URL}."
        )
    return value
