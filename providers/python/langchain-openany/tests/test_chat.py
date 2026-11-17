"""Mocked HTTP tests for ChatOpenAny. No OpenAny host is contacted."""

from __future__ import annotations

import json
import threading
from collections.abc import Iterator
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

import pytest
from langchain_core.messages import HumanMessage

from langchain_openany import ChatOpenAny
from langchain_openany.chat_models import OpenAnyServerError
from langchain_openany.sse import SSEParser

FAKE_KEY = "sk-test-openany-not-a-real-key"
MODEL_ID = "listing/model:route-1"


class _Server(ThreadingHTTPServer):
    def __init__(self, handler: type[BaseHTTPRequestHandler]) -> None:
        super().__init__(("127.0.0.1", 0), handler)
        self.requests: list[dict[str, Any]] = []


@contextmanager
def serve(handler: type[BaseHTTPRequestHandler]) -> Iterator[_Server]:
    server = _Server(handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server
    finally:
        server.shutdown()
        thread.join(timeout=5)
        server.server_close()


def _model(server: _Server, **kwargs: Any) -> ChatOpenAny:
    return ChatOpenAny(
        model=MODEL_ID,
        api_key=FAKE_KEY,
        base_url=f"http://127.0.0.1:{server.server_address[1]}/v1",
        allow_custom_endpoint=True,
        **kwargs,
    )


def test_sse_parser_reassembles_split_utf8_and_events() -> None:
    payload = (
        'data: {"choices":[{"delta":{"content":"He"}}]}\n\n'
        'data: {"choices":[{"delta":{"content":"llo 你"}}]}\n\n'
        "data: [DONE]\n\n"
    ).encode()
    parser = SSEParser()
    events: list[dict[str, Any]] = []
    for index in range(len(payload)):
        events.extend(parser.feed(payload[index : index + 1]))
    events.extend(parser.finish())
    assert [event["choices"][0]["delta"]["content"] for event in events] == ["He", "llo 你"]


def test_missing_and_placeholder_keys_do_not_connect(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("OPENANY_API_KEY", raising=False)
    with pytest.raises(ValueError, match="missing"):
        ChatOpenAny(model=MODEL_ID)
    for value in ("${OPENANY_API_KEY}", "$OPENANY_API_KEY", "<key>", "YOUR_API_KEY", f"Bearer {FAKE_KEY}"):
        with pytest.raises(ValueError, match="placeholder|raw key"):
            ChatOpenAny(model=MODEL_ID, api_key=value)


def test_custom_endpoint_requires_opt_in_and_rejects_mcp_and_userinfo() -> None:
    with pytest.raises(ValueError, match="allow_custom_endpoint"):
        ChatOpenAny(model=MODEL_ID, api_key=FAKE_KEY, base_url="http://127.0.0.1:9/v1")
    with pytest.raises(ValueError, match="MCP host"):
        ChatOpenAny(
            model=MODEL_ID,
            api_key=FAKE_KEY,
            base_url="https://mcp.openany.si/v1",
            allow_custom_endpoint=True,
        )
    with pytest.raises(ValueError, match="userinfo"):
        ChatOpenAny(
            model=MODEL_ID,
            api_key=FAKE_KEY,
            base_url="http://user:password@127.0.0.1:9/v1",
            allow_custom_endpoint=True,
        )
    with pytest.raises(ValueError, match="must not contain the API key"):
        ChatOpenAny(
            model=MODEL_ID,
            api_key=FAKE_KEY,
            base_url=f"http://127.0.0.1:9/v1?access={FAKE_KEY}",
            allow_custom_endpoint=True,
        )


def test_text_generation_passes_model_id_and_usage() -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length)
            self.server.requests.append(  # type: ignore[attr-defined]
                {
                    "path": self.path,
                    "authorization": self.headers.get("Authorization"),
                    "body": json.loads(raw),
                }
            )
            payload = {
                "id": "resp-1",
                "model": MODEL_ID,
                "choices": [{"message": {"role": "assistant", "content": "OpenAny connected"}, "finish_reason": "stop"}],
                "usage": {"prompt_tokens": 11, "completion_tokens": 3, "total_tokens": 14},
            }
            encoded = json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        model = _model(server, temperature=0, max_tokens=32, max_retries=2)
        result = model.invoke("Reply with exactly: OpenAny connected")
        assert result.content == "OpenAny connected"
        assert result.response_metadata["finish_reason"] == "stop"
        assert result.usage_metadata is not None
        assert result.usage_metadata["input_tokens"] == 11
        assert result.usage_metadata["output_tokens"] == 3
        assert result.usage_metadata["total_tokens"] == 14
        assert len(server.requests) == 1
        request = server.requests[0]
        assert request["path"] == "/v1/chat/completions"
        assert request["authorization"] == f"Bearer {FAKE_KEY}"
        assert request["body"]["model"] == MODEL_ID
        assert "stream" not in request["body"]
        assert request["body"]["max_tokens"] == 32
        assert "tools" not in request["body"]


def test_http_error_is_not_retried_and_key_is_redacted() -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            self.rfile.read(length)
            self.server.requests.append({"path": self.path})  # type: ignore[attr-defined]
            body = json.dumps({"error": {"message": f"upstream failed {FAKE_KEY}"}}).encode()
            self.send_response(500)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        model = _model(server, max_retries=6)
        with pytest.raises(OpenAnyServerError, match="redacted") as caught:
            model.invoke("Hello")
        assert caught.value.is_retryable is False
        assert FAKE_KEY not in str(caught.value)
        assert len(server.requests) == 1


def test_redirect_is_not_followed() -> None:
    stolen: list[str] = []

    class Thief(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            stolen.append(self.headers.get("Authorization", ""))
            self.send_response(200)
            self.end_headers()

        def log_message(self, format: str, *args: object) -> None:
            return

    thief = _Server(Thief)
    thief_thread = threading.Thread(target=thief.serve_forever, daemon=True)
    thief_thread.start()

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            self.rfile.read(length)
            self.server.requests.append({"path": self.path})  # type: ignore[attr-defined]
            self.send_response(302)
            self.send_header("Location", f"http://127.0.0.1:{thief.server_address[1]}/steal")
            self.end_headers()

        def log_message(self, format: str, *args: object) -> None:
            return

    try:
        with serve(Handler) as server:
            with pytest.raises(Exception, match="redirect"):
                _model(server).invoke("Hello")
            assert len(server.requests) == 1
            assert stolen == []
    finally:
        thief.shutdown()
        thief_thread.join(timeout=5)
        thief.server_close()


def test_tools_are_rejected_before_the_network() -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            raise AssertionError("tool request must not be sent")

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        model = _model(server)
        with pytest.raises(Exception, match="text Chat Completions"):
            model.invoke(
                [HumanMessage(content=[{"type": "text", "text": "hi"}, {"type": "image_url", "image_url": {}}])]
            )
        with pytest.raises(Exception, match="not sent"):
            model.invoke("Hello", tools=[{"type": "function", "function": {"name": "lookup"}}])


def test_streaming_fragmented_chunks_report_usage_and_finish_reason() -> None:
    events = "".join(
        [
            'data: {"choices":[{"delta":{"content":"He"}}]}\n\n',
            'data: {"choices":[{"delta":{"content":"llo 你"}}]}\n\n',
            'data: {"model":"listing/model:route-1","choices":[{"delta":{},"finish_reason":"length"}],'
            '"usage":{"prompt_tokens":5,"completion_tokens":2}}\n\n',
            "data: [DONE]\n\n",
        ]
    ).encode()

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length)
            self.server.requests.append(json.loads(raw))  # type: ignore[attr-defined]
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            split = events.find("你".encode()) + 1
            self.wfile.write(events[:split])
            self.wfile.flush()
            self.wfile.write(events[split:])

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        chunks = list(_model(server).stream("Hello"))
        assert "".join(str(chunk.content) for chunk in chunks) == "Hello 你"
        assert any(chunk.response_metadata.get("finish_reason") == "length" for chunk in chunks)
        usage = next(chunk.usage_metadata for chunk in chunks if chunk.usage_metadata)
        assert usage["input_tokens"] == 5
        assert usage["total_tokens"] == 7
        assert any(
            chunk.response_metadata.get("usage_total_source") == "sum_of_prompt_and_completion" for chunk in chunks
        )
        assert server.requests[0]["stream"] is True
        assert server.requests[0]["model"] == MODEL_ID
        assert "stream_options" not in server.requests[0]


def test_closing_a_stream_does_not_send_another_request() -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            self.rfile.read(length)
            self.server.requests.append({"path": self.path})  # type: ignore[attr-defined]
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self.wfile.write(b'data: {"choices":[{"delta":{"content":"partial"}}]}\n\n')
            self.wfile.flush()
            self.close_connection = True

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        stream = _model(server).stream("Hello")
        first = next(stream)
        assert first.content == "partial"
        stream.close()
        assert len(server.requests) == 1


def test_env_key_is_used_when_the_constructor_omits_it(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("OPENANY_API_KEY", FAKE_KEY)

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length)
            self.server.requests.append(  # type: ignore[attr-defined]
                {"authorization": self.headers.get("Authorization"), "body": json.loads(raw)}
            )
            payload = {"choices": [{"message": {"content": "from env"}, "finish_reason": "stop"}]}
            encoded = json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def log_message(self, format: str, *args: object) -> None:
            return

    with serve(Handler) as server:
        model = ChatOpenAny(
            model="env-model",
            base_url=f"http://127.0.0.1:{server.server_address[1]}/v1",
            allow_custom_endpoint=True,
        )
        assert model.invoke("Hello").content == "from env"
        assert server.requests[0]["authorization"] == f"Bearer {FAKE_KEY}"
        assert server.requests[0]["body"]["model"] == "env-model"
        assert model.invoke("Hello").usage_metadata is None
