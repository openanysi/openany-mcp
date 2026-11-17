"""Incremental Server-Sent Events parser for Chat Completions streams."""

from __future__ import annotations

import codecs
import json
from typing import Any

_DONE = object()


class SSEDecodeError(ValueError):
    """Raised when an event is not a JSON object or the stream is cut off."""


class SSEParser:
    """Collect SSE events from arbitrary byte chunks, including split UTF-8."""

    def __init__(self) -> None:
        self._decoder = codecs.getincrementaldecoder("utf-8")()
        self._buffer = ""

    def feed(self, chunk: bytes) -> list[dict[str, Any]]:
        self._buffer += self._decoder.decode(chunk)
        return self._drain(final=False)

    def finish(self) -> list[dict[str, Any]]:
        self._buffer += self._decoder.decode(b"", final=True)
        return self._drain(final=True)

    def _drain(self, *, final: bool) -> list[dict[str, Any]]:
        events: list[dict[str, Any]] = []
        while "\n\n" in self._buffer:
            raw, self._buffer = self._buffer.split("\n\n", 1)
            parsed = self._parse_event(raw)
            if parsed is not _DONE and parsed is not None:
                events.append(parsed)
            if parsed is _DONE:
                self._buffer = ""
                return events
        if final and self._buffer.strip():
            parsed = self._parse_event(self._buffer)
            self._buffer = ""
            if parsed is not _DONE and parsed is not None:
                events.append(parsed)
        return events

    def _parse_event(self, raw: str) -> dict[str, Any] | object | None:
        data_lines: list[str] = []
        for line in raw.splitlines():
            if not line or line.startswith(":"):
                continue
            if line.startswith("data:"):
                data_lines.append(line[5:].lstrip())
        if not data_lines:
            return None
        data = "\n".join(data_lines)
        if data == "[DONE]":
            return _DONE
        try:
            payload = json.loads(data)
        except json.JSONDecodeError as error:
            raise SSEDecodeError("Chat Completions stream event was not valid JSON.") from error
        if not isinstance(payload, dict):
            raise SSEDecodeError("Chat Completions stream event was not a JSON object.")
        return payload
