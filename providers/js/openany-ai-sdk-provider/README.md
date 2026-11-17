# openany-ai-sdk-provider

Named [Vercel AI SDK](https://ai-sdk.dev) provider for OpenAny text Chat Completions.

This package is version `0.1.0` and is **not published**. The npm name `openany-ai-sdk-provider` was not in the registry when this package was added. Publishing needs a registry account and an approved name. See `providers/PUBLISHING.md`.

It is **not** an official Vercel community provider and **not** listed at https://ai-sdk.dev/providers/community-providers.

## What it calls

- `POST https://api.openany.si/v1/chat/completions`
- `Authorization: Bearer <OPENANY_API_KEY>`
- The model id you pass is sent as `model`. Copy it from the current listing. This package does not embed a model catalog.

The root MCP URL `https://mcp.openany.si` is a different product surface. This provider never posts chat requests there.

OpenAny is the B2A (business to agent) marketplace. The marketplace also lists 3,200+ APIs and 168+ AI models. This chat package does not grant access to those APIs. Searching the marketplace and checking prices are separate from this SDK.

## Install

Peer dependency: `ai` 7.x. The versions executed in this repository are stable `ai@7.0.173`, `@ai-sdk/provider@4.0.38`, `@ai-sdk/provider-utils@5.0.81`, and `@ai-sdk/openai-compatible@3.0.92`. The provider implements Language Model Specification V4 (`specificationVersion: 'v4'`) for text chat only. It is not a claim of full AI SDK provider conformance.

```bash
npm install ai openany-ai-sdk-provider
```

Node.js 22.19.0 or newer. That floor is `undici@8.12.4`, whose package metadata declares `engines.node` `>=22.19.0`. The AI SDK packages used here declare `>=22`, which is lower. Node older than 22.19.0 is unsupported. npm warns about `engines` on install; it does not refuse the install unless the installer sets `engine-strict`.

## Use

```ts
import { generateText, streamText } from 'ai';
import { createOpenAny } from 'openany-ai-sdk-provider';

const openany = createOpenAny(); // or createOpenAny({ apiKey })

const { text } = await generateText({
  model: openany(process.env.OPENANY_MODEL!), // id from the current listing
  prompt: 'Hello',
  maxOutputTokens: 32,
});
```

`createOpenAny()` reads `OPENANY_API_KEY` when `apiKey` is omitted. The value is the raw key, without a `Bearer` prefix. A missing key, a blank key, or an unexpanded placeholder such as `${OPENANY_API_KEY}` throws before a request is sent.

Create a key at https://openany.si/dashboard?page=api-keys and keep it in the environment or a secret store. Do not paste it into chat, a URL, or a log.

## Supported

- Text prompts and text output through Chat Completions
- Streaming Server-Sent Events (`stream: true`), including chunks split across TCP reads
- Model id pass-through
- Sampling fields only when you set them: `maxOutputTokens` (`max_tokens`), `temperature`, `topP`, `presencePenalty`, `frequencyPenalty`, `stopSequences`, and `seed`
- `usage` and `finishReason` when the response includes them (`stop`, `length`, `content_filter`, and other documented Chat Completions reasons)
- Aborting a stream
- A 60 second default budget for response headers and for each idle gap in the body. Pass `timeoutMs` to change it. There is no unlimited wait. A timeout is not retried
- One request per call. HTTP 408, 409, 429, 5xx, and timeouts are marked `isRetryable: false` so the AI SDK does not repeat a call that may already have been billed

## Not supported

These are rejected before the request, or the corresponding provider method throws:

- Tool calling and tool results
- JSON schema / structured output
- Image, audio, video, and file input or output
- Embeddings, image generation, speech, transcription, and legacy text completions
- Responses API and Anthropic Messages
- `reasoning` effort and provider options (they are not part of the documented chat request)
- `topK`
- Automatic retries
- Following redirects with the default fetch. A 3xx response is an error and the default fetch does not follow it
- A non-default `baseURL`, unless you set `allowCustomEndpoint: true`
- The MCP host, including `https://MCP.openany.si./v1`, even with that opt-in

A route may still reject a sampling field. That failure is returned once.

If a response includes tool calls or file output, those parts are not returned as executable tool calls or files. A tool-call stream is consumed and completed; it is not left open and the tool calls are not executed. A missing usage object stays empty; this package does not fill counts with zeros.

`stream_options` is not sent. Usage on a stream is reported only when a chunk includes `usage`.

The default fetch does not follow redirects and does not use `HTTP_PROXY` / `HTTPS_PROXY`, including when `NODE_USE_ENV_PROXY=1` is set for Node's global `fetch`. A `fetch` function you pass is trusted code. It is given the `Authorization` header. If that function follows a redirect, logs the header, or sends it elsewhere, this package cannot undo the send. It only refuses to treat an unexpected final URL as a successful completion.

No live paid call was made to verify this package. Tests use a local mock HTTP server.

## Endpoint override

```ts
createOpenAny({
  apiKey: process.env.OPENANY_API_KEY,
  baseURL: 'https://example.invalid/v1',
  allowCustomEndpoint: true,
});
```

`baseURL` is the API root and must use path `/v1`. The client appends `/chat/completions`. `http` is accepted only for loopback. The URL must not contain the key, a user, a query, or a fragment.

## License

MIT. Copyright (c) 2026 Alephant AI LLC. Operator contact: team@openany.si.
