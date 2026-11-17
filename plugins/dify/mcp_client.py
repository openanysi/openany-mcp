"""Streamable HTTP client for the fixed OpenAny MCP endpoint.

The only network destination is https://mcp.openany.si. Catalog tools are
unauthenticated. run_api attaches the caller's key on that one tools/call and
can incur a charge. This module never reads an API key from the environment.
"""

from __future__ import annotations

import json
import re
import socket
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any

ENDPOINT = "https://mcp.openany.si"
PROTOCOL_VERSION = "2025-11-25"
CLIENT_NAME = "OpenAny-Dify"
CLIENT_VERSION = "0.1.0"
CATALOG_TIMEOUT = 30.0
RUN_TIMEOUT = 120.0
# initialize, the initialized notification, then one run_api POST. This is the
# longest stretch with no streamed tool message. dify-plugin 0.11.1 applies
# MAX_REQUEST_TIMEOUT only as ServerlessRequestReader's idle gap between
# response chunks (default 300). Local stdio and remote TCP do not use it as a
# tool deadline. The paid HTTP timeout stays RUN_TIMEOUT and is not retried.
HTTP_DEADLINE_SECONDS = CATALOG_TIMEOUT + CATALOG_TIMEOUT + RUN_TIMEOUT
RESULT_DELIVERY_MARGIN_SECONDS = 60
PLUGIN_REQUEST_DEADLINE = int(HTTP_DEADLINE_SECONDS + RESULT_DELIVERY_MARGIN_SECONDS)
MAX_BODY = 4_000_000
MIN_ACCEPTED_SECRET_LENGTH = 8
KEY_PAGE = "https://openany.si/dashboard?page=api-keys"
_VERSION_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
_SESSION_RE = re.compile(r"^[A-Za-z0-9._~-]{1,256}$")
_SESSION_HEADER_RE = re.compile(r"(?i)(mcp-session-id\s*[:=]\s*)(\S+)")
_API_VERSION_RE = re.compile(r"^[a-f0-9]{64}$")
_PLACEHOLDER_LITERALS = {
    "YOUR_API_KEY",
    "YOUR_OPENANY_KEY",
    "YOUR_OPENANY_API_KEY",
    "OPENANY_API_KEY",
}
_SEARCH_LIMITS = {
    "query": 256,
    "kind": 16,
    "provider": 64,
    "category": 128,
    "data_platform": 256,
    "data_product": 128,
    "capability": 256,
    "cursor": 4096,
}
_KINDS = {"api", "model", "all"}


class OpenAnyCallError(Exception):
    """A tool failure. The message is safe to show; it does not contain a key."""

    def __init__(self, message: str, *, code: str, execution_state: str) -> None:
        self.code = code
        self.execution_state = execution_state
        super().__init__(
            f"{message} code={code} execution_state={execution_state}"
        )


class TransportFailure(Exception):
    def __init__(self, kind: str, message: str) -> None:
        self.kind = kind
        super().__init__(message)


@dataclass
class HttpResponse:
    status: int
    headers: dict[str, str]
    body: bytes


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def open_http(
    url: str,
    payload: bytes,
    headers: Mapping[str, str],
    timeout: float,
) -> HttpResponse:
    """POST once. Proxies and redirects are disabled so a key stays on this URL."""

    request = urllib.request.Request(
        url,
        data=payload,
        headers=dict(headers),
        method="POST",
    )
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({}),
        _NoRedirect(),
    )
    try:
        with opener.open(request, timeout=timeout) as response:
            status = getattr(response, "status", 200)
            response_headers = {k.lower(): v for k, v in response.headers.items()}
            body = response.read(MAX_BODY + 1)
    except urllib.error.HTTPError as exc:
        try:
            if 300 <= exc.code < 400:
                raise TransportFailure(
                    "redirect",
                    f"HTTP {exc.code} from the MCP endpoint was not followed",
                ) from None
            response_headers = {
                k.lower(): v for k, v in (exc.headers.items() if exc.headers else [])
            }
            body = exc.read(MAX_BODY + 1)
            return _bounded(exc.code, response_headers, body)
        finally:
            exc.close()
    except TimeoutError:
        raise TransportFailure("timeout", "request timed out") from None
    except urllib.error.URLError as exc:
        reason = exc.reason
        timed_out = isinstance(reason, (TimeoutError, socket.timeout)) or (
            "timed out" in str(reason).lower()
        )
        if timed_out:
            raise TransportFailure("timeout", "request timed out") from None
        raise TransportFailure("network", "network error talking to OpenAny") from None
    return _bounded(status, response_headers, body)


def _bounded(status: int, headers: dict[str, str], body: bytes) -> HttpResponse:
    if len(body) > MAX_BODY:
        raise TransportFailure(
            "too_large",
            "OpenAny response exceeded 4000000 bytes and was discarded",
        )
    return HttpResponse(status=status, headers=headers, body=body)


def exchange(payload: dict[str, Any], headers: Mapping[str, str], timeout: float) -> HttpResponse:
    _assert_fixed_endpoint(ENDPOINT)
    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    return open_http(ENDPOINT, body, headers, timeout)


def _assert_fixed_endpoint(url: str) -> None:
    parts = urllib.parse.urlsplit(url)
    if (
        parts.scheme != "https"
        or parts.hostname != "mcp.openany.si"
        or parts.port not in (None, 443)
        or parts.path not in ("", "/")
        or parts.query
        or parts.fragment
        or parts.username
        or parts.password
    ):
        raise TransportFailure(
            "network",
            "refusing an endpoint that is not https://mcp.openany.si",
        )


def call_tool(
    name: str,
    tool_parameters: Mapping[str, Any] | None,
    *,
    paid: bool = False,
    api_key: str | None = None,
) -> dict[str, Any]:
    """Call one lite MCP tool. paid=True is only for an explicit run_api invocation."""

    arguments = build_arguments(name, tool_parameters or {})
    secret: str | None = None
    if paid:
        if name != "run_api":
            raise OpenAnyCallError(
                "Only run_api can send an OpenAny API key.",
                code="openany_dify_invalid_arguments",
                execution_state="not_started",
            )
        secret = require_key(api_key)
    protocol, session = _handshake()
    headers = _headers(protocol, session, secret if paid else None)
    payload = {
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/call",
        "params": {"name": name, "arguments": arguments},
    }
    timeout = RUN_TIMEOUT if paid else CATALOG_TIMEOUT
    try:
        response = exchange(payload, headers, timeout)
    except TransportFailure as exc:
        raise _transport_error(exc, paid=paid, phase="tools/call") from None
    message = _decode(response, expected_id=2, secret=secret, session=session)
    if "error" in message:
        raise _rpc_error(
            message["error"], secret, session=session, paid=paid, phase="tools/call"
        )
    result = message.get("result")
    if not isinstance(result, dict):
        raise OpenAnyCallError(
            "OpenAny returned a tools/call response without a result object.",
            code="openany_dify_invalid_response",
            execution_state="unknown" if paid else "not_started",
        )
    if result.get("isError"):
        raise _tool_error(result, secret, session)
    return _scrub(interpret_success(result), secret, session)


def require_key(api_key: str | None) -> str:
    material = key_material(api_key)
    if material is None:
        raise OpenAnyCallError(
            "run_api needs an OpenAny API key in the Dify provider secret. "
            f"Create one at {KEY_PAGE} and save it there. "
            "Catalog tools work with the secret left empty. "
            "Do not paste the key into the chat.",
            code="openany_mcp_credentials_missing",
            execution_state="not_started",
        )
    if is_placeholder(material) or _unsafe_key(material):
        raise OpenAnyCallError(
            "The Dify provider secret is not a usable OpenAny API key. "
            "Replace it in the provider settings. "
            f"Create a key at {KEY_PAGE}. Do not paste the key into the chat.",
            code="openany_mcp_credentials_placeholder"
            if is_placeholder(material)
            else "openany_dify_invalid_credentials",
            execution_state="not_started",
        )
    return material


def credential_status(api_key: str | None) -> tuple[str, str | None]:
    """Local credential check. Returns (status, reason_code). Never calls the network."""

    if key_material(api_key) is None:
        return "valid", None
    try:
        require_key(api_key)
    except OpenAnyCallError:
        return "invalid", "credential_invalid"
    return "valid", None


def key_material(api_key: str | None) -> str | None:
    if api_key is None:
        return None
    if not isinstance(api_key, str):
        return ""
    value = api_key.strip()
    if not value:
        return None
    if value.lower().startswith("bearer ") and " " in value:
        value = value.split(" ", 1)[1].strip()
    return value


def is_placeholder(material: str) -> bool:
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
        and not any(ch in material[1:-1] for ch in "<> \t")
    ):
        return True
    return material.upper() in _PLACEHOLDER_LITERALS


def build_arguments(name: str, parameters: Mapping[str, Any]) -> dict[str, Any]:
    if name == "openany_overview":
        return {}
    if name == "search_apis":
        return _search_arguments(parameters)
    if name == "get_api":
        return {"api": _api_name(parameters.get("api"))}
    if name == "run_api":
        return _run_arguments(parameters)
    raise OpenAnyCallError(
        f"Unknown OpenAny tool {name}.",
        code="openany_dify_invalid_arguments",
        execution_state="not_started",
    )


def interpret_success(result: Mapping[str, Any]) -> dict[str, Any]:
    structured = result.get("structuredContent")
    value: Any
    if isinstance(structured, (dict, list)):
        value = structured
    else:
        texts = _text_blocks(result)
        if len(texts) == 1:
            parsed = _try_json(texts[0])
            if isinstance(parsed, (dict, list)):
                value = parsed
            elif texts[0] == "":
                raise OpenAnyCallError(
                    "OpenAny returned an empty tool result.",
                    code="openany_dify_invalid_response",
                    execution_state="unknown",
                )
            else:
                return {"result_kind": "text", "text": texts[0]}
        elif texts:
            return {"result_kind": "text", "text": "\n".join(texts)}
        else:
            value = None
    if isinstance(value, dict):
        return value
    if isinstance(value, list):
        return {"items": value}
    raise OpenAnyCallError(
        "OpenAny returned a tool result with no content.",
        code="openany_dify_invalid_response",
        execution_state="unknown",
    )


def redact(value: Any, secret: str | None) -> Any:
    # Empty and one-character needles would corrupt the text. Accepted secrets
    # are at least MIN_ACCEPTED_SECRET_LENGTH, so those are always replaced.
    if not secret or len(secret) < MIN_ACCEPTED_SECRET_LENGTH:
        return value
    if isinstance(value, str):
        return value.replace(secret, "[REDACTED]")
    if isinstance(value, list):
        return [redact(item, secret) for item in value]
    if isinstance(value, dict):
        return {key: redact(item, secret) for key, item in value.items()}
    return value


def _scrub(value: Any, secret: str | None, session: str | None) -> Any:
    value = redact(value, secret)
    if isinstance(value, str):
        value = _SESSION_HEADER_RE.sub(r"\1[REDACTED]", value)
        if session and len(session) >= MIN_ACCEPTED_SECRET_LENGTH:
            value = value.replace(session, "[REDACTED]")
        return value
    if isinstance(value, list):
        return [_scrub(item, secret, session) for item in value]
    if isinstance(value, dict):
        return {key: _scrub(item, secret, session) for key, item in value.items()}
    return value


def _handshake() -> tuple[str, str | None]:
    headers = _headers(PROTOCOL_VERSION, None, None)
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": PROTOCOL_VERSION,
            "capabilities": {},
            "clientInfo": {"name": CLIENT_NAME, "version": CLIENT_VERSION},
        },
    }
    try:
        response = exchange(payload, headers, CATALOG_TIMEOUT)
    except TransportFailure as exc:
        raise _transport_error(exc, paid=False, phase="initialize") from None
    message = _decode(response, expected_id=1, secret=None, session=None)
    if "error" in message:
        raise _rpc_error(message["error"], None, paid=False, phase="initialize")
    result = message.get("result")
    if not isinstance(result, dict):
        raise OpenAnyCallError(
            "OpenAny initialize returned no result object.",
            code="openany_dify_invalid_response",
            execution_state="not_started",
        )
    protocol = result.get("protocolVersion") or PROTOCOL_VERSION
    if not isinstance(protocol, str) or not _VERSION_RE.fullmatch(protocol):
        protocol = PROTOCOL_VERSION
    session = _session_id(response.headers)
    notice_headers = _headers(protocol, session, None)
    notice = {"jsonrpc": "2.0", "method": "notifications/initialized"}
    try:
        notice_response = exchange(notice, notice_headers, CATALOG_TIMEOUT)
    except TransportFailure as exc:
        raise _transport_error(exc, paid=False, phase="initialized") from None
    if notice_response.body:
        notice_message = _decode(
            notice_response, expected_id=None, secret=None, session=session
        )
        if "error" in notice_message:
            raise _rpc_error(
                notice_message["error"], None, paid=False, phase="initialized"
            )
    return protocol, session


def _headers(
    protocol: str,
    session: str | None,
    secret: str | None,
) -> dict[str, str]:
    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": protocol,
        "User-Agent": f"{CLIENT_NAME}/{CLIENT_VERSION}",
    }
    if session:
        headers["Mcp-Session-Id"] = session
    if secret:
        headers["Authorization"] = "Bearer " + secret
    return headers


def _session_id(headers: Mapping[str, str]) -> str | None:
    for key, value in headers.items():
        if key.lower() == "mcp-session-id" and _SESSION_RE.fullmatch(value or ""):
            return value
    return None


def _decode(
    response: HttpResponse,
    *,
    expected_id: int | None,
    secret: str | None,
    session: str | None,
) -> dict[str, Any]:
    if response.status not in (200, 202):
        detail = _body_excerpt(response.body, secret, session)
        state = "unknown" if expected_id == 2 else "not_started"
        raise OpenAnyCallError(
            f"OpenAny returned HTTP {response.status}. {detail}".strip(),
            code="openany_dify_http_error",
            execution_state=state,
        )
    if not response.body:
        if expected_id is None and response.status in (200, 202):
            return {}
        raise OpenAnyCallError(
            "OpenAny returned an empty MCP response.",
            code="openany_dify_invalid_response",
            execution_state="unknown" if expected_id == 2 else "not_started",
        )
    content_type = response.headers.get("content-type", "")
    message = _parse_body(response.body, content_type, expected_id)
    if message is None:
        excerpt = _body_excerpt(response.body, secret, session)
        raise OpenAnyCallError(
            f"OpenAny returned a non-JSON MCP response. {excerpt}".strip(),
            code="openany_dify_invalid_response",
            execution_state="unknown" if expected_id == 2 else "not_started",
        )
    return message


def _parse_body(
    body: bytes,
    content_type: str,
    expected_id: int | None,
) -> dict[str, Any] | None:
    text = body.decode("utf-8", errors="replace").strip()
    if "text/event-stream" in content_type.lower() or text.startswith("event:") or text.startswith("data:"):
        messages = []
        for blob in _sse_data(text):
            parsed = _try_json(blob)
            if isinstance(parsed, dict):
                messages.append(parsed)
        if expected_id is None:
            for message in messages:
                if "error" in message:
                    return message
            return messages[-1] if messages else None
        for message in messages:
            if message.get("id") == expected_id:
                return message
        return None
    parsed = _try_json(text)
    if not isinstance(parsed, dict):
        return None
    if expected_id is not None and parsed.get("id") != expected_id:
        return None
    return parsed


def _sse_data(text: str) -> list[str]:
    events: list[str] = []
    data_lines: list[str] = []
    for line in text.splitlines():
        if line == "":
            if data_lines:
                events.append("\n".join(data_lines))
                data_lines = []
            continue
        if line.startswith(":"):
            continue
        if line.startswith("data:"):
            data_lines.append(line[5:].lstrip())
    if data_lines:
        events.append("\n".join(data_lines))
    return events


def _tool_error(
    result: Mapping[str, Any],
    secret: str | None,
    session: str | None = None,
) -> OpenAnyCallError:
    parsed = None
    texts = _text_blocks(result)
    if len(texts) == 1:
        parsed = _try_json(texts[0])
    error = parsed.get("error") if isinstance(parsed, dict) else None
    if isinstance(error, dict):
        code = str(error.get("code") or "openany_tool_error")
        state = str(error.get("execution_state") or "unknown")
        message = str(error.get("message") or "OpenAny returned a tool error.")
        next_action = error.get("next_action")
        if isinstance(next_action, str) and next_action and next_action not in message:
            message = f"{message} {next_action}"
        if state == "unknown" and "retry" not in message.lower():
            message = f"{message} Do not retry automatically."
        return OpenAnyCallError(
            _scrub(message, secret, session),
            code=_scrub(code, secret, session),
            execution_state=_scrub(state, secret, session),
        )
    text = _scrub("\n".join(texts), secret, session).strip()
    return OpenAnyCallError(
        text or "OpenAny returned a tool error.",
        code="openany_tool_error",
        execution_state="unknown",
    )


def _rpc_error(
    error: Any,
    secret: str | None,
    *,
    session: str | None = None,
    paid: bool,
    phase: str,
) -> OpenAnyCallError:
    message = "OpenAny rejected the MCP request."
    if isinstance(error, dict) and isinstance(error.get("message"), str):
        message = error["message"]
    state = "unknown" if paid and phase == "tools/call" else "not_started"
    if state == "unknown":
        message = f"{message} Do not retry automatically."
    return OpenAnyCallError(
        _scrub(message, secret, session),
        code="openany_dify_rpc_error",
        execution_state=state,
    )


def _transport_error(exc: TransportFailure, *, paid: bool, phase: str) -> OpenAnyCallError:
    sent = paid and phase == "tools/call"
    state = "unknown" if sent else "not_started"
    if exc.kind == "timeout" and sent:
        message = (
            "run_api timed out after the request was sent. "
            "execution_state is unknown: the call may already have run or been charged. "
            "Do not retry automatically."
        )
        code = "openany_dify_timeout"
    elif exc.kind == "timeout":
        message = "OpenAny did not respond before the timeout. No run_api request was sent."
        code = "openany_dify_timeout"
    elif exc.kind == "redirect":
        message = (
            "OpenAny returned a redirect. The plugin did not follow it and did not "
            "forward the API key to another host."
        )
        if sent:
            message += " Do not retry automatically."
        code = "openany_dify_redirect"
    elif exc.kind == "too_large":
        message = str(exc)
        if sent:
            message += " Do not retry automatically."
        code = "openany_dify_invalid_response"
    else:
        message = str(exc)
        code = "openany_dify_transport"
    return OpenAnyCallError(message, code=code, execution_state=state)


def _search_arguments(parameters: Mapping[str, Any]) -> dict[str, Any]:
    arguments: dict[str, Any] = {}
    for name, limit in _SEARCH_LIMITS.items():
        if name not in parameters or parameters[name] is None:
            continue
        value = parameters[name]
        if not isinstance(value, str):
            _bad(f"{name} must be a string.")
        value = value.strip()
        if not value:
            continue
        if len(value) > limit:
            _bad(f"{name} is longer than {limit} characters.")
        if name == "kind" and value not in _KINDS:
            _bad("kind must be api, model, or all.")
        arguments[name] = value
    if "limit" in parameters and parameters["limit"] not in (None, ""):
        arguments["limit"] = _limit(parameters["limit"])
    filters = [key for key in arguments if key != "limit"]
    if not filters:
        _bad(
            "search_apis needs a query or one filter. "
            "kind=model can browse models without a query."
        )
    return arguments


def _run_arguments(parameters: Mapping[str, Any]) -> dict[str, Any]:
    api_version = parameters.get("api_version")
    if not isinstance(api_version, str) or not _API_VERSION_RE.fullmatch(api_version.strip()):
        _bad("api_version must be the 64-character lowercase hex value from get_api.")
    payload = _object(parameters.get("input"), "input")
    arguments: dict[str, Any] = {
        "api": _api_name(parameters.get("api")),
        "api_version": api_version.strip(),
        "input": payload,
    }
    if "output" in parameters and parameters["output"] not in (None, ""):
        arguments["output"] = _output(parameters["output"])
    return arguments


def _api_name(value: Any) -> str:
    if not isinstance(value, str):
        _bad("api must be a string.")
    value = value.strip()
    if not value or len(value) > 64:
        _bad("api must be 1 to 64 characters.")
    return value


def _limit(value: Any) -> int:
    number = _whole_number(value, "limit")
    if number < 1 or number > 10:
        _bad("limit must be an integer from 1 to 10.")
    return number


def _object(value: Any, name: str) -> dict[str, Any]:
    if isinstance(value, str):
        parsed = _try_json(value.strip())
        if parsed is None:
            _bad(f"{name} must be a JSON object.")
        value = parsed
    if not isinstance(value, dict):
        _bad(f"{name} must be a JSON object.")
    return value


def _output(value: Any) -> dict[str, Any]:
    raw = _object(value, "output")
    unknown = set(raw) - {"mode", "items_path", "fields", "max_items"}
    if unknown:
        _output_bad("unknown output option")
    mode = "raw"
    if "mode" in raw:
        if not isinstance(raw["mode"], str):
            _output_bad("output.mode must be a string")
        mode = raw["mode"]
    if mode not in {"raw", "compact", "summary"}:
        _output_bad("output.mode must be raw, compact or summary")
    items_path = None
    if "items_path" in raw:
        path = raw["items_path"]
        if not isinstance(path, str) or len(path) > 512 or not _valid_pointer(path):
            _output_bad("output.items_path must be a JSON pointer relative to data")
        items_path = path
    fields = None
    if "fields" in raw:
        candidate = raw["fields"]
        if (
            not isinstance(candidate, list)
            or not 1 <= len(candidate) <= 20
            or len(set(candidate)) != len(candidate)
        ):
            _output_bad("output.fields must contain 1 to 20 object keys")
        for item in candidate:
            if not isinstance(item, str) or not item or len(item) > 128 or _has_control(item):
                _output_bad("invalid output.fields key")
        fields = candidate
    max_items = None
    if "max_items" in raw:
        max_items = _whole_number(raw["max_items"], "output.max_items")
        if max_items < 1 or max_items > 100:
            _output_bad("output.max_items must be an integer from 1 to 100")
    if mode != "compact" and (items_path is not None or fields is not None or max_items is not None):
        _output_bad("items_path, fields and max_items require compact mode")
    output: dict[str, Any] = {}
    if "mode" in raw:
        output["mode"] = mode
    if items_path is not None:
        output["items_path"] = items_path
    if fields is not None:
        output["fields"] = fields
    if max_items is not None:
        output["max_items"] = max_items
    return output


def _valid_pointer(path: str) -> bool:
    if path == "":
        return True
    if not path.startswith("/") or _has_control(path):
        return False
    unescaped = path.replace("~0", "").replace("~1", "")
    return "~" not in unescaped


def _whole_number(value: Any, name: str) -> int:
    if isinstance(value, bool) or isinstance(value, str):
        if isinstance(value, str) and value.strip().isdigit():
            return int(value.strip())
        _bad(f"{name} must be an integer.")
    if isinstance(value, int):
        return value
    if isinstance(value, float) and value.is_integer():
        return int(value)
    _bad(f"{name} must be an integer.")
    return 0


def _unsafe_key(material: str) -> bool:
    if (
        not material
        or len(material) < MIN_ACCEPTED_SECRET_LENGTH
        or len(material) > 512
    ):
        return True
    return any(ch.isspace() or ord(ch) < 32 for ch in material)


def _ident(value: str) -> bool:
    if not value:
        return False
    for index, char in enumerate(value):
        if index == 0 and not (char == "_" or char.isalpha()):
            return False
        if index and not (char == "_" or char.isalnum()):
            return False
    return True


def _has_control(value: str) -> bool:
    return any(ord(char) < 32 for char in value)


def _text_blocks(result: Mapping[str, Any]) -> list[str]:
    texts: list[str] = []
    content = result.get("content")
    if not isinstance(content, list):
        return texts
    for block in content:
        if isinstance(block, dict) and block.get("type") == "text" and isinstance(block.get("text"), str):
            texts.append(block["text"])
    return texts


def _try_json(text: str) -> Any:
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return None


def _body_excerpt(body: bytes, secret: str | None, session: str | None = None) -> str:
    text = body.decode("utf-8", errors="replace").strip()
    text = _scrub(text, secret, session)
    if len(text) > 180:
        text = text[:180] + "..."
    return text


def _bad(message: str) -> None:
    raise OpenAnyCallError(
        f"{message} No request was sent.",
        code="openany_mcp_invalid_input",
        execution_state="not_started",
    )


def _output_bad(message: str) -> None:
    raise OpenAnyCallError(
        f"{message} No request was sent.",
        code="openany_mcp_invalid_output",
        execution_state="not_started",
    )
