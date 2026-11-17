# langchain-openany

Named LangChain Python chat model for OpenAny text Chat Completions.

This package is version `0.1.0` and is **not published**. The PyPI name `langchain-openany` was not in the registry when this package was added. Publishing needs a PyPI account, 2FA, and an API token. See `providers/PUBLISHING.md`.

It is **not** listed in the LangChain integration docs. LangChain does not accept new integrations as pull requests to `langchain-ai` repositories. A listing, after publication, is an issue in the LangChain docs repository. This monorepo does not by itself meet LangChain's request for a standalone package repository.

## What it calls

- `POST https://api.openany.si/v1/chat/completions`
- `Authorization: Bearer <OPENANY_API_KEY>`
- The `model` argument is sent unchanged. Copy the id from the current listing. This package does not embed a model catalog.

The root MCP URL `https://mcp.openany.si` is a different product surface. This class never posts chat requests there.

OpenAny is the B2A (business to agent) marketplace. The marketplace also lists 3,200+ APIs and 168+ AI models. This chat class does not grant access to those APIs.

## Install

Requires Python 3.11+ and `langchain-core` 1.8 or newer, below 2.

```bash
pip install langchain-openany
```

Verified in development against `langchain-core` 1.8.1 and `langchain-tests` 1.1.9. Those are stable releases, not prereleases.

## Use

```python
import os
from langchain_openany import ChatOpenAny

model = ChatOpenAny(
    model=os.environ["OPENANY_MODEL"],  # id from the current listing
    max_tokens=32,
)
message = model.invoke("Hello")
```

`api_key` overrides `OPENANY_API_KEY`. The value is the raw key, without a `Bearer` prefix. A missing key, a blank key, or an unexpanded placeholder such as `${OPENANY_API_KEY}` raises before a request is sent.

Create a key at https://openany.si/dashboard?page=api-keys. Do not paste it into chat, a URL, or a log.

## Supported

- Text `invoke`, `stream`, `ainvoke`, and `astream`
- Model id pass-through, including a per-call `model=` override
- Sampling fields only when set: `temperature`, `max_tokens`, `top_p`, `frequency_penalty`, `presence_penalty`, `seed`, and `stop`
- `usage_metadata` when the response includes `prompt_tokens` and `completion_tokens`
- `response_metadata["finish_reason"]` when the response includes `finish_reason`
- Closing a stream stops reading. The request is not repeated
- `max_retries` is accepted and ignored. This class makes one HTTP attempt

If `total_tokens` is omitted, `usage_metadata["total_tokens"]` is the sum of the supplied prompt and completion counts, because LangChain's field is that sum. `response_metadata["usage_total_source"]` is `sum_of_prompt_and_completion` in that case. When the response includes `total_tokens`, that value is kept even if it differs from the sum.

## Not supported

These raise before the request:

- Tool calling, tool messages, and `bind_tools` results (the inherited `bind_tools` is not a working OpenAny tool API; a later invoke rejects `tools`)
- Structured output
- Image, audio, video, and file parts
- Embeddings and other modalities
- Responses API and Anthropic Messages
- Automatic retries, including HTTP 429 and 5xx
- Redirects. `follow_redirects` is false. Environment proxies are not used (`trust_env=False`)
- A non-default `base_url`, unless `allow_custom_endpoint=True`
- The MCP host, even with that opt-in

`stream_options` is not sent. A stream reports usage only when an event includes `usage`.

A response that contains tool calls does not become LangChain tool calls. `response_metadata["unsupported_tool_calls"]` is true. Non-text response parts are not decoded; their type names are listed in `unsupported_content_parts`.

No live paid call was made to verify this package. Tests use a local mock HTTP server. The LangChain `ChatModelUnitTests` suite is included. Its serialization case is skipped: `ChatOpenAny` is not marked LangChain-serializable, because `langchain-core` 1.8.1 refuses to load a class that is outside its built-in allowlist. `ChatModelIntegrationTests` are not run.

## Endpoint override

```python
ChatOpenAny(
    model="model-from-listing",
    base_url="https://example.invalid/v1",
    allow_custom_endpoint=True,
)
```

`base_url` must use path `/v1`. The client appends `/chat/completions`. `http` is accepted only for loopback. The URL must not contain the key, a user, a query, or a fragment.

## License

MIT. Copyright (c) 2026 Alephant AI LLC. Operator contact: team@openany.si.
