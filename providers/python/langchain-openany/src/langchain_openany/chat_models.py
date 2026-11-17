"""ChatOpenAny, a LangChain BaseChatModel for OpenAny Chat Completions."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator, Iterator, Mapping
from typing import Any

import httpx
from langchain_core.exceptions import (
    ModelAPIError,
    ModelAuthenticationError,
    ModelError,
    ModelInvalidRequestError,
    ModelNotFoundError,
    ModelPermissionDeniedError,
    ModelRateLimitError,
    ModelTimeoutError,
)
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import (
    AIMessage,
    AIMessageChunk,
    BaseMessage,
    ChatMessage,
    HumanMessage,
    SystemMessage,
)
from langchain_core.messages.ai import UsageMetadata
from langchain_core.outputs import ChatGeneration, ChatGenerationChunk, ChatResult
from langchain_core.utils import from_env
from pydantic import ConfigDict, Field, SecretStr, model_validator
from typing_extensions import Self

from langchain_openany.credentials import assert_api_key
from langchain_openany.endpoint import chat_completions_url, resolve_endpoint
from langchain_openany.sse import SSEDecodeError, SSEParser

_ALLOWED_CALL_KEYS = frozenset(
    {
        "temperature",
        "max_tokens",
        "top_p",
        "frequency_penalty",
        "presence_penalty",
        "seed",
        "model",
        "stream",
    }
)
_FORBIDDEN_HEADERS = frozenset(
    {"authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "xi-api-key"}
)
_BODY_LIMIT = 65_536
DEFAULT_TIMEOUT_SECONDS = 60.0


class OpenAnyUnsupportedError(ModelInvalidRequestError):
    """The call uses a feature this adapter does not send."""


class OpenAnyServerError(ModelAPIError):
    """HTTP 5xx. Not retried: a timeout or 5xx may already have been billed."""

    is_retryable = False


class OpenAnyRateLimitError(ModelRateLimitError):
    """HTTP 429. The caller can honor Retry-After. This class does not retry."""

    is_retryable = False


class OpenAnyResponseError(ModelError):
    """A response that is not a usable Chat Completions payload."""

    is_retryable = False


class OpenAnyRedirectError(ModelError):
    """A redirect response was refused. This class does not follow it."""

    is_retryable = False


class OpenAnyTimeoutError(ModelTimeoutError):
    """The chat request exceeded its timeout. It is not retried."""

    is_retryable = False


def _redact(value: str, secret: str) -> str:
    if secret and secret in value:
        return value.replace(secret, "[redacted]")
    return value


def _finite_int(value: object) -> int | None:
    if isinstance(value, bool) or not isinstance(value, int):
        return None
    return value


def _usage(payload: Mapping[str, Any]) -> UsageMetadata | None:
    raw = payload.get("usage")
    if not isinstance(raw, Mapping):
        return None
    prompt = _finite_int(raw.get("prompt_tokens"))
    completion = _finite_int(raw.get("completion_tokens"))
    if prompt is None or completion is None:
        return None
    total = _finite_int(raw.get("total_tokens"))
    usage: dict[str, Any] = {
        "input_tokens": prompt,
        "output_tokens": completion,
        "total_tokens": total if total is not None else prompt + completion,
    }
    details = raw.get("prompt_tokens_details")
    if isinstance(details, Mapping):
        cached = _finite_int(details.get("cached_tokens"))
        if cached is not None:
            usage["input_token_details"] = {"cache_read": cached}
    output_details = raw.get("completion_tokens_details")
    if isinstance(output_details, Mapping):
        reasoning = _finite_int(output_details.get("reasoning_tokens"))
        if reasoning is not None:
            usage["output_token_details"] = {"reasoning": reasoning}
    return UsageMetadata(**usage)


def _text_from_content(content: object) -> tuple[str, list[str]]:
    if content is None:
        return "", []
    if isinstance(content, str):
        return content, []
    if isinstance(content, list):
        pieces: list[str] = []
        skipped: list[str] = []
        for part in content:
            if isinstance(part, str):
                pieces.append(part)
                continue
            if isinstance(part, Mapping) and part.get("type") == "text" and isinstance(part.get("text"), str):
                pieces.append(part["text"])
                continue
            kind = part.get("type") if isinstance(part, Mapping) else type(part).__name__
            skipped.append(str(kind))
        return "".join(pieces), skipped
    raise OpenAnyUnsupportedError("Message content must be text.")


def _message_text(message: BaseMessage) -> str:
    if isinstance(message, AIMessage) and message.tool_calls:
        raise OpenAnyUnsupportedError(
            "Tool calls are not supported. This adapter does not send or continue tool rounds."
        )
    text, skipped = _text_from_content(message.content)
    if skipped:
        raise OpenAnyUnsupportedError(
            "This adapter sends text Chat Completions content only. "
            "Image, audio, video, and file parts are not supported."
        )
    return text


def _message_role(message: BaseMessage) -> str:
    if isinstance(message, SystemMessage) or message.type == "system":
        return "system"
    if isinstance(message, HumanMessage) or message.type in {"human", "user"}:
        return "user"
    if isinstance(message, AIMessage) or message.type == "ai":
        return "assistant"
    if isinstance(message, ChatMessage) and message.role in {"system", "user", "assistant"}:
        return message.role
    raise OpenAnyUnsupportedError(
        "Only system, user, and assistant text messages are supported. Tool messages are not sent."
    )


def _messages(messages: list[BaseMessage]) -> list[dict[str, str]]:
    if not messages:
        raise OpenAnyUnsupportedError("Chat Completions requires at least one message.")
    return [{"role": _message_role(message), "content": _message_text(message)} for message in messages]


def _choice_text(choice: Mapping[str, Any], *, stream: bool) -> tuple[str, list[str], bool]:
    container = choice.get("delta") if stream else choice.get("message")
    if not isinstance(container, Mapping):
        return "", [], False
    text, skipped = _text_from_content(container.get("content"))
    tool_calls = container.get("tool_calls") or container.get("function_call")
    return text, skipped, bool(tool_calls)


class ChatOpenAny(BaseChatModel):
    """OpenAny text chat model.

    The model id is whatever string you pass. Copy it from the current listing.
    This class does not retry Chat Completions calls, including when
    ``max_retries`` is set. ``max_retries`` is accepted so the standard LangChain
    constructor shape works.

    A non-default ``base_url`` requires ``allow_custom_endpoint=True``.
    The endpoint and API key are checked again immediately before every
    HTTP call, including after attribute assignment or ``model_copy``.

    ``timeout`` is a positive number of seconds for connect, write, and
    idle read. The default is 60. A stalled response is not retried.
    """

    model_config = ConfigDict(populate_by_name=True, validate_assignment=True)

    model_name: str = Field(alias="model")
    """Model id copied from the current OpenAny listing."""

    openany_api_key: SecretStr | None = Field(
        default_factory=from_env("OPENANY_API_KEY", default=None),
        alias="api_key",
    )
    base_url: str = Field(default="https://api.openany.si/v1")
    allow_custom_endpoint: bool = False
    temperature: float | None = None
    max_tokens: int | None = None
    top_p: float | None = None
    frequency_penalty: float | None = None
    presence_penalty: float | None = None
    seed: int | None = None
    timeout: float = DEFAULT_TIMEOUT_SECONDS
    stop: list[str] | None = None
    max_retries: int = 0
    """Accepted for LangChain compatibility. Chat requests are not retried."""

    streaming: bool = False
    default_headers: dict[str, str] = Field(default_factory=dict)

    @model_validator(mode="after")
    def _check_configuration(self) -> Self:
        self._normalize_configuration()
        return self

    def model_copy(self, *, update: Mapping[str, Any] | None = None, deep: bool = False) -> Self:
        copied = super().model_copy(update=update, deep=deep)
        copied._normalize_configuration()
        return copied

    def _normalize_configuration(self) -> str:
        """Validate the key and endpoint. Return the canonical chat URL.

        Uses ``object.__setattr__`` so assignment validation cannot recurse.
        Called from construction, ``model_copy``, and every request.
        """
        key_field = self.openany_api_key
        if isinstance(key_field, str):
            object.__setattr__(self, "openany_api_key", SecretStr(key_field))
        key = self._key()
        normalized = resolve_endpoint(self.base_url, self.allow_custom_endpoint, key)
        if self.base_url != normalized:
            object.__setattr__(self, "base_url", normalized)
        self._check_timeout()
        for name, value in self.default_headers.items():
            if name.lower() in _FORBIDDEN_HEADERS:
                raise OpenAnyUnsupportedError(f"{name} is reserved. The adapter sets Authorization.")
            if key in value:
                raise OpenAnyCredentialsProxyError()
        return chat_completions_url(normalized)

    def _check_timeout(self) -> float:
        value = self.timeout
        if isinstance(value, bool) or not isinstance(value, (int, float)) or value <= 0 or value != value:
            raise OpenAnyUnsupportedError("timeout must be a positive number of seconds. Unlimited waits are not available.")
        return float(value)

    @property
    def _llm_type(self) -> str:
        return "openany-chat"

    @property
    def _identifying_params(self) -> dict[str, Any]:
        return {"model": self.model_name, "base_url": self.base_url}

    @property
    def lc_secrets(self) -> dict[str, str]:
        return {"openany_api_key": "OPENANY_API_KEY"}

    def _key(self) -> str:
        raw = None if self.openany_api_key is None else self.openany_api_key.get_secret_value()
        return assert_api_key(raw)

    def _headers(self, *, stream: bool) -> dict[str, str]:
        key = self._key()
        headers = {
            "Authorization": f"Bearer {key}",
            "Content-Type": "application/json",
            "Accept": "text/event-stream" if stream else "application/json",
        }
        for name, value in self.default_headers.items():
            if name.lower() in _FORBIDDEN_HEADERS or key in value:
                continue
            headers[name] = value
        return headers

    def _body(self, messages: list[BaseMessage], stop: list[str] | None, kwargs: dict[str, Any], *, stream: bool) -> dict[str, Any]:
        unsupported = sorted(set(kwargs) - _ALLOWED_CALL_KEYS)
        if unsupported or kwargs.get("tools") or kwargs.get("tool_choice"):
            raise OpenAnyUnsupportedError(
                "Tool calling, response formats, and undocumented parameters are not sent. "
                f"Rejected: {', '.join(unsupported) or 'tools'}."
            )
        model = kwargs.get("model", self.model_name)
        if not isinstance(model, str) or not model.strip() or any(character in model for character in "\r\n\x00"):
            raise OpenAnyUnsupportedError(
                "Pass the model id copied from the current OpenAny listing. This package does not enumerate models."
            )
        body: dict[str, Any] = {"model": model, "messages": _messages(messages)}
        if stream:
            body["stream"] = True
        sampling = {
            "temperature": kwargs.get("temperature", self.temperature),
            "max_tokens": kwargs.get("max_tokens", self.max_tokens),
            "top_p": kwargs.get("top_p", self.top_p),
            "frequency_penalty": kwargs.get("frequency_penalty", self.frequency_penalty),
            "presence_penalty": kwargs.get("presence_penalty", self.presence_penalty),
            "seed": kwargs.get("seed", self.seed),
        }
        for name, value in sampling.items():
            if value is not None:
                body[name] = value
        stop_sequences = stop if stop is not None else self.stop
        if stop_sequences:
            body["stop"] = stop_sequences
        return body

    def _client_kwargs(self) -> dict[str, Any]:
        return {
            "timeout": httpx.Timeout(self._check_timeout()),
            "follow_redirects": False,
            "trust_env": False,
        }

    def _timeout_error(self) -> OpenAnyTimeoutError:
        return OpenAnyTimeoutError(
            f"OpenAny chat request exceeded the {self._check_timeout():g}s timeout. It was not retried."
        )

    def _chat_url(self) -> str:
        """Re-check the key and endpoint immediately before a socket is opened."""
        return self._normalize_configuration()

    def _raise_for_stream_error(self, payload: Mapping[str, Any]) -> None:
        if "error" not in payload or payload.get("error") in (None, False):
            return
        error = payload.get("error")
        secret = self._key()
        detail = "Chat Completions stream returned an error object."
        if isinstance(error, str) and error:
            detail = error
        elif isinstance(error, Mapping):
            message = error.get("message")
            if isinstance(message, str) and message:
                detail = message
        raise OpenAnyResponseError(_redact(detail, secret))

    def _raise_for_status(self, status: int, raw: bytes, headers: Mapping[str, str]) -> None:
        secret = self._key()
        text = _redact(raw[:_BODY_LIMIT].decode("utf-8", errors="replace"), secret)
        message = f"OpenAny chat request failed with HTTP {status}."
        try:
            parsed = json.loads(text) if text else None
        except json.JSONDecodeError:
            parsed = None
        if isinstance(parsed, dict) and isinstance(parsed.get("error"), dict):
            detail = parsed["error"].get("message")
            if isinstance(detail, str) and detail:
                message = f"{message} {_redact(detail, secret)}"
        if status in {401, 403} and "Retry-After" in headers:
            message = f"{message} Retry-After was provided; this client does not retry."
        if status == 401:
            raise ModelAuthenticationError(message)
        if status == 403:
            raise ModelPermissionDeniedError(message)
        if status == 404:
            raise ModelNotFoundError(message)
        if status == 429:
            retry_after = headers.get("retry-after") or headers.get("Retry-After")
            if retry_after:
                message = f"{message} Retry-After: {_redact(retry_after, secret)}."
            raise OpenAnyRateLimitError(message)
        if 500 <= status <= 599:
            raise OpenAnyServerError(message)
        raise ModelInvalidRequestError(message)

    def _read_error(self, response: httpx.Response) -> None:
        if 300 <= response.status_code < 400:
            raise OpenAnyRedirectError(
                "OpenAny refused to follow a redirect. The API key was not sent to another URL."
            )
        if response.status_code >= 400:
            self._raise_for_status(response.status_code, response.read(), response.headers)

    def _metadata(self, payload: Mapping[str, Any], choice: Mapping[str, Any], skipped: list[str], had_tools: bool) -> dict[str, Any]:
        metadata: dict[str, Any] = {}
        finish = choice.get("finish_reason")
        if isinstance(finish, str):
            metadata["finish_reason"] = finish
        model = payload.get("model")
        if isinstance(model, str):
            metadata["model_name"] = model
        if skipped:
            metadata["unsupported_content_parts"] = skipped
        if had_tools:
            metadata["unsupported_tool_calls"] = True
        usage = payload.get("usage")
        if isinstance(usage, Mapping) and _finite_int(usage.get("total_tokens")) is None and _usage(payload) is not None:
            metadata["usage_total_source"] = "sum_of_prompt_and_completion"
        return metadata

    def _generation_from_payload(self, payload: Mapping[str, Any]) -> ChatResult:
        choices = payload.get("choices")
        if not isinstance(choices, list) or not choices or not isinstance(choices[0], Mapping):
            raise OpenAnyResponseError("Chat Completions response did not include choices.")
        choice = choices[0]
        text, skipped, had_tools = _choice_text(choice, stream=False)
        message = AIMessage(
            content=text,
            usage_metadata=_usage(payload),
            response_metadata=self._metadata(payload, choice, skipped, had_tools),
        )
        return ChatResult(generations=[ChatGeneration(message=message)])

    def _chunks_from_event(self, payload: Mapping[str, Any]) -> list[ChatGenerationChunk]:
        self._raise_for_stream_error(payload)
        choices = payload.get("choices")
        if isinstance(choices, list) and choices and isinstance(choices[0], Mapping):
            choice = choices[0]
            text, skipped, had_tools = _choice_text(choice, stream=True)
        else:
            choice = {}
            text, skipped, had_tools = "", [], False
        metadata = self._metadata(payload, choice, skipped, had_tools)
        usage = _usage(payload)
        if not text and not metadata and usage is None:
            return []
        return [
            ChatGenerationChunk(
                message=AIMessageChunk(
                    content=text,
                    usage_metadata=usage,
                    response_metadata=metadata,
                )
            )
        ]

    def _generate(
        self,
        messages: list[BaseMessage],
        stop: list[str] | None = None,
        run_manager: Any = None,
        **kwargs: Any,
    ) -> ChatResult:
        del run_manager
        body = self._body(messages, stop, kwargs, stream=False)
        url = self._chat_url()
        try:
            with httpx.Client(**self._client_kwargs()) as client:
                response = client.post(url, headers=self._headers(stream=False), json=body)
                self._read_error(response)
                try:
                    payload = response.json()
                except json.JSONDecodeError as error:
                    raise OpenAnyResponseError("Chat Completions response was not JSON.") from error
        except httpx.TimeoutException as error:
            raise self._timeout_error() from error
        if not isinstance(payload, dict):
            raise OpenAnyResponseError("Chat Completions response was not a JSON object.")
        return self._generation_from_payload(payload)

    def _stream(
        self,
        messages: list[BaseMessage],
        stop: list[str] | None = None,
        run_manager: Any = None,
        **kwargs: Any,
    ) -> Iterator[ChatGenerationChunk]:
        del run_manager
        body = self._body(messages, stop, kwargs, stream=True)
        url = self._chat_url()
        parser = SSEParser()
        try:
            with httpx.Client(**self._client_kwargs()) as client, client.stream(
                "POST",
                url,
                headers=self._headers(stream=True),
                json=body,
            ) as response:
                if 300 <= response.status_code < 400:
                    raise OpenAnyRedirectError(
                        "OpenAny refused a redirect response and did not read the redirected body."
                    )
                if response.status_code >= 400:
                    self._raise_for_status(response.status_code, response.read(), response.headers)
                for chunk in response.iter_bytes():
                    for event in parser.feed(chunk):
                        yield from self._chunks_from_event(event)
                for event in parser.finish():
                    yield from self._chunks_from_event(event)
        except httpx.TimeoutException as error:
            raise self._timeout_error() from error
        except SSEDecodeError as error:
            raise OpenAnyResponseError(str(error)) from error

    async def _agenerate(
        self,
        messages: list[BaseMessage],
        stop: list[str] | None = None,
        run_manager: Any = None,
        **kwargs: Any,
    ) -> ChatResult:
        del run_manager
        body = self._body(messages, stop, kwargs, stream=False)
        url = self._chat_url()
        try:
            async with httpx.AsyncClient(**self._client_kwargs()) as client:
                response = await client.post(url, headers=self._headers(stream=False), json=body)
                if 300 <= response.status_code < 400:
                    raise OpenAnyRedirectError(
                        "OpenAny refused a redirect response and did not read the redirected body."
                    )
                if response.status_code >= 400:
                    self._raise_for_status(response.status_code, response.content, response.headers)
                try:
                    payload = response.json()
                except json.JSONDecodeError as error:
                    raise OpenAnyResponseError("Chat Completions response was not JSON.") from error
        except httpx.TimeoutException as error:
            raise self._timeout_error() from error
        if not isinstance(payload, dict):
            raise OpenAnyResponseError("Chat Completions response was not a JSON object.")
        return self._generation_from_payload(payload)

    async def _astream(
        self,
        messages: list[BaseMessage],
        stop: list[str] | None = None,
        run_manager: Any = None,
        **kwargs: Any,
    ) -> AsyncIterator[ChatGenerationChunk]:
        del run_manager
        body = self._body(messages, stop, kwargs, stream=True)
        url = self._chat_url()
        parser = SSEParser()
        try:
            async with httpx.AsyncClient(**self._client_kwargs()) as client, client.stream(
                "POST",
                url,
                headers=self._headers(stream=True),
                json=body,
            ) as response:
                if 300 <= response.status_code < 400:
                    raise OpenAnyRedirectError(
                        "OpenAny refused a redirect response and did not read the redirected body."
                    )
                if response.status_code >= 400:
                    self._raise_for_status(response.status_code, await response.aread(), response.headers)
                async for chunk in response.aiter_bytes():
                    for event in parser.feed(chunk):
                        for item in self._chunks_from_event(event):
                            yield item
                for event in parser.finish():
                    for item in self._chunks_from_event(event):
                        yield item
        except httpx.TimeoutException as error:
            raise self._timeout_error() from error
        except SSEDecodeError as error:
            raise OpenAnyResponseError(str(error)) from error


class OpenAnyCredentialsProxyError(OpenAnyUnsupportedError):
    def __init__(self) -> None:
        super().__init__("Custom headers must not contain the OpenAny API key.")
