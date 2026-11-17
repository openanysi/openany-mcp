"""Regression tests for request-boundary safety, timeouts, and SSE framing."""

from __future__ import annotations

import json
import threading
import time
from collections.abc import Iterator
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

import httpx
import pytest
from pydantic import SecretStr

from langchain_openany import ChatOpenAny
from langchain_openany.chat_models import OpenAnyResponseError, OpenAnyTimeoutError
from langchain_openany.endpoint import resolve_endpoint
from langchain_openany.sse import SSEParser

FAKE_KEY = "sk-test-openany-not-a-real-key"
MODEL_ID = "listing/model:route-1"


class _Server(ThreadingHTTPServer):
    requests: list[dict[str, Any]]


@contextmanager
def serve(handler: type[BaseHTTPRequestHandler]) -> Iterator[_Server]:
    server = _Server(("127.0.0.1", 0), handler)
    server.requests = []
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server
    finally:
        server.shutdown()
        thread.join(timeout=2)
        server.server_close()


def _record(handler: BaseHTTPRequestHandler, body: bytes | None = None) -> None:
    handler.server.requests.append(  # type: ignore[attr-defined]
        {"path": handler.path, "authorization": handler.headers.get("Authorization"), "body": body}
    )


def test_endpoint_rejects_trailing_dot_and_case_variants() -> None:
    for base_url in (
        "https://mcp.openany.si./v1",
        "https://MCP.OpenAny.SI./v1",
        "https://mcp.openany.si.../v1",
    ):
        with pytest.raises(ValueError, match="MCP host"):
            resolve_endpoint(base_url, allow_custom_endpoint=True, api_key=FAKE_KEY)
    assert resolve_endpoint("https://API.OpenAny.SI./v1", allow_custom_endpoint=False) == "https://api.openany.si/v1"


def test_assignment_and_model_copy_do_not_dispatch() -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            _record(self, self.rfile.read(length))
            payload = json.dumps(
                {"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}
            ).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        local = f"http://127.0.0.1:{server.server_address[1]}/v1"
        model = ChatOpenAny(model=MODEL_ID, api_key=FAKE_KEY)
        with pytest.raises(ValueError, match="allow_custom_endpoint"):
            model.base_url = local
        object.__setattr__(model, "base_url", local)
        with pytest.raises(ValueError, match="allow_custom_endpoint"):
            model.invoke("Hello")
        allowed = ChatOpenAny(model=MODEL_ID, api_key=FAKE_KEY, base_url=local, allow_custom_endpoint=True)
        with pytest.raises(ValueError, match="MCP host"):
            allowed.model_copy(update={"base_url": "https://mcp.openany.si./v1"})
        object.__setattr__(allowed, "base_url", "https://MCP.openany.si./v1")
        with pytest.raises(ValueError, match="MCP host"):
            allowed.invoke("Hello")
        object.__setattr__(allowed, "base_url", local)
        object.__setattr__(allowed, "openany_api_key", SecretStr("${OPENANY_API_KEY}"))
        with pytest.raises(ValueError, match="placeholder"):
            allowed.invoke("Hello")
        assert server.requests == []
        fresh = ChatOpenAny(model=MODEL_ID, api_key=FAKE_KEY, base_url=local, allow_custom_endpoint=True)
        assert fresh.model_copy(update={"temperature": 0}).invoke("Hello").content == "ok"
        assert len(server.requests) == 1


def test_sse_parser_accepts_crlf_lf_and_split_utf8() -> None:
    payload = (
        'data: {"choices":[{"delta":{"content":"He"}}]}\r\n\r\n'
        'data: {"choices":[{"delta":{"content":"llo 你"}}]}\n\n'
        "data: [DONE]\r\n\r\n"
    ).encode()
    parser = SSEParser()
    events: list[dict[str, Any]] = []
    for index in range(len(payload)):
        events.extend(parser.feed(payload[index : index + 1]))
    events.extend(parser.finish())
    assert [event["choices"][0]["delta"]["content"] for event in events] == ["He", "llo 你"]


def _crlf_body() -> bytes:
    return (
        'data: {"choices":[{"delta":{"content":"He"}}]}\r\n\r\n'
        'data: {"choices":[{"delta":{"content":"llo 你"}}]}\r\n\r\n'
        'data: {"choices":[{"delta":{},"finish_reason":"stop"}]}\r\n\r\n'
        "data: [DONE]\r\n\r\n"
    ).encode()


def test_http_stream_reads_split_crlf_events() -> None:
    body = _crlf_body()
    split = body.find("你".encode()) + 1

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            _record(self, self.rfile.read(length))
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self.wfile.write(body[:split])
            self.wfile.flush()
            self.wfile.write(body[split:])

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        model = ChatOpenAny(
            model=MODEL_ID,
            api_key=FAKE_KEY,
            base_url=f"http://127.0.0.1:{server.server_address[1]}/v1",
            allow_custom_endpoint=True,
        )
        chunks = list(model.stream("Hello"))
        assert "".join(str(chunk.content) for chunk in chunks) == "Hello 你"
        assert len(server.requests) == 1


@pytest.mark.asyncio
async def test_async_http_stream_reads_split_crlf_events() -> None:
    body = _crlf_body()
    split = body.find(b"\r\n\r\n") + 1

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            _record(self, self.rfile.read(length))
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self.wfile.write(body[:split])
            self.wfile.flush()
            self.wfile.write(body[split:])

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        model = ChatOpenAny(
            model=MODEL_ID,
            api_key=FAKE_KEY,
            base_url=f"http://127.0.0.1:{server.server_address[1]}/v1",
            allow_custom_endpoint=True,
        )
        pieces: list[str] = []
        async for chunk in model.astream("Hello"):
            pieces.append(str(chunk.content))
        assert "".join(pieces) == "Hello 你"
        assert len(server.requests) == 1


def test_stream_error_object_fails_and_redacts_before_later_text() -> None:
    events = (
        f'data: {{"error":{{"message":"upstream failed {FAKE_KEY}"}},"model":"m"}}\n\n'
        'data: {"choices":[{"delta":{"content":"should not succeed"}}]}\n\n'
    ).encode()

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            _record(self)
            self.rfile.read(length)
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self.wfile.write(events)

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        model = ChatOpenAny(
            model=MODEL_ID,
            api_key=FAKE_KEY,
            base_url=f"http://127.0.0.1:{server.server_address[1]}/v1",
            allow_custom_endpoint=True,
            max_retries=5,
        )
        seen: list[str] = []
        with pytest.raises(OpenAnyResponseError, match="redacted") as caught:
            for chunk in model.stream("Hello"):
                seen.append(str(chunk.content))
        assert FAKE_KEY not in str(caught.value)
        assert seen == []
        assert "should not succeed" not in str(caught.value)
        assert len(server.requests) == 1


@pytest.mark.parametrize("phase", ["headers", "body"])
@pytest.mark.parametrize("method", ["invoke", "stream"])
def test_sync_stall_times_out_once(phase: str, method: str) -> None:
    _assert_stall(phase, method)


@pytest.mark.asyncio
@pytest.mark.parametrize("phase", ["headers", "body"])
@pytest.mark.parametrize("method", ["ainvoke", "astream"])
async def test_async_stall_times_out_once(phase: str, method: str) -> None:
    await _assert_stall_async(phase, method)


def _stall_handler(phase: str) -> type[BaseHTTPRequestHandler]:
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            self.rfile.read(length)
            _record(self)
            if phase == "body":
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                self.wfile.write(b'data: {"choices":[{"delta":{"content":"x"}}]}\n\n')
                self.wfile.flush()
            time.sleep(2)

        def log_message(self, format: str, *args: object) -> None:
            return

    return Handler


def _stall_model(server: _Server) -> ChatOpenAny:
    return ChatOpenAny(
        model=MODEL_ID,
        api_key=FAKE_KEY,
        base_url=f"http://127.0.0.1:{server.server_address[1]}/v1",
        allow_custom_endpoint=True,
        timeout=0.2,
        max_retries=5,
    )


def _assert_stall(phase: str, method: str) -> None:
    with serve(_stall_handler(phase)) as server:
        model = _stall_model(server)
        with pytest.raises(OpenAnyTimeoutError, match="not retried"):
            if method == "invoke":
                model.invoke("Hello")
            else:
                list(model.stream("Hello"))
        assert model.max_retries == 5
        assert len(server.requests) == 1


async def _assert_stall_async(phase: str, method: str) -> None:
    with serve(_stall_handler(phase)) as server:
        model = _stall_model(server)
        with pytest.raises(OpenAnyTimeoutError, match="not retried"):
            if method == "ainvoke":
                await model.ainvoke("Hello")
            else:
                async for _chunk in model.astream("Hello"):
                    pass
        assert len(server.requests) == 1


def test_bind_tools_fails_immediately() -> None:
    model = ChatOpenAny(model=MODEL_ID, api_key=FAKE_KEY)
    with pytest.raises(NotImplementedError):
        model.bind_tools([{"name": "lookup"}])


def test_httpx_timeout_type_is_converted() -> None:
    assert issubclass(OpenAnyTimeoutError, Exception)
    assert OpenAnyTimeoutError.is_retryable is False
    assert httpx.TimeoutException
