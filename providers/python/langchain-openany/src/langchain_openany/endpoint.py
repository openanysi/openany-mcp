"""Chat Completions endpoint checks."""

from __future__ import annotations

from urllib.parse import urlsplit

DEFAULT_BASE_URL = "https://api.openany.si/v1"
CHAT_COMPLETIONS_PATH = "/chat/completions"
_BLOCKED_HOSTS = frozenset({"mcp.openany.si"})
_LOOPBACK = frozenset({"localhost", "127.0.0.1", "::1"})


class OpenAnyEndpointError(ValueError):
    """Raised before a request when the endpoint is not allowed."""

    is_retryable = False


def canonical_host(hostname: str) -> str:
    """Lower-case a host and drop a trailing DNS root dot.

    ``mcp.openany.si.`` is the same name as ``mcp.openany.si``. Comparison is
    done only after this normalization.
    """
    host = hostname.strip().lower().rstrip(".")
    if not host or ".." in host or any(label == "" for label in host.split(".")):
        raise OpenAnyEndpointError("OpenAny base_url host is not valid.")
    return host


def resolve_endpoint(base_url: str | None, allow_custom_endpoint: bool, api_key: str | None = None) -> str:
    """Return the normalized API root. The path is exactly /v1."""
    raw = DEFAULT_BASE_URL if base_url is None else base_url
    if api_key and api_key in raw:
        raise OpenAnyEndpointError(
            "OpenAny base_url must not contain the API key. Pass the key as api_key or OPENANY_API_KEY."
        )
    parts = urlsplit(raw)
    if parts.username or parts.password:
        raise OpenAnyEndpointError("OpenAny base_url must not include userinfo.")
    if parts.query or parts.fragment:
        raise OpenAnyEndpointError("OpenAny base_url must not include a query or fragment.")
    if not parts.hostname:
        raise OpenAnyEndpointError("OpenAny base_url is not a valid URL.")
    host = canonical_host(parts.hostname)
    if host in _BLOCKED_HOSTS:
        raise OpenAnyEndpointError(
            "OpenAny chat requests are not sent to the MCP host. Use https://api.openany.si/v1."
        )
    path = parts.path.rstrip("/") or "/"
    if path != "/v1":
        raise OpenAnyEndpointError("OpenAny base_url path must be /v1. The adapter appends /chat/completions.")
    if parts.scheme == "http" and host not in _LOOPBACK:
        raise OpenAnyEndpointError("OpenAny base_url must use https, except for loopback tests.")
    if parts.scheme not in {"https", "http"}:
        raise OpenAnyEndpointError("OpenAny base_url must use https.")
    default_port = 443 if parts.scheme == "https" else 80
    port = "" if parts.port in (None, default_port) else f":{parts.port}"
    normalized = f"{parts.scheme}://{host}{port}/v1"
    if normalized != DEFAULT_BASE_URL and not allow_custom_endpoint:
        raise OpenAnyEndpointError(
            "A non-default OpenAny base_url requires allow_custom_endpoint=True. "
            "The API key is not sent to another host unless you set that option."
        )
    return normalized


def chat_completions_url(base_url: str) -> str:
    return f"{base_url}{CHAT_COMPLETIONS_PATH}"
