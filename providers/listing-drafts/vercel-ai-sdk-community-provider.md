# Draft only: Vercel AI SDK community provider page

**Not submitted.** The package is not published and OpenAny is not a community provider. Send this only after `openany-ai-sdk-provider` is on npm and a maintainer opens a docs pull request against the AI SDK repository, using the OpenRouter community-provider page as the template: https://ai-sdk.dev/providers/community-providers/custom-providers

---

# OpenAny Provider

[OpenAny](https://openany.si) is the B2A (business to agent) marketplace. The `openany-ai-sdk-provider` package calls the documented text Chat Completions endpoint with your own OpenAny API key. It does not call the MCP marketplace endpoint and it does not grant access to OpenAny's data APIs.

## Setup

The OpenAny provider is available in the `openany-ai-sdk-provider` module.

```bash
npm install openany-ai-sdk-provider
```

## Provider instance

```ts
import { createOpenAny } from 'openany-ai-sdk-provider';

const openany = createOpenAny({
  apiKey: process.env.OPENANY_API_KEY,
});
```

The default base URL is `https://api.openany.si/v1`. Requests go to `POST /chat/completions` with `Authorization: Bearer <key>`.

## Language models

```ts
import { generateText } from 'ai';
import { createOpenAny } from 'openany-ai-sdk-provider';

const openany = createOpenAny();

const { text } = await generateText({
  model: openany(process.env.OPENANY_MODEL!),
  prompt: 'Hello',
});
```

Copy `OPENANY_MODEL` from the current OpenAny listing. Model ids are route-specific.

Chat Completions support does not imply Responses, Anthropic Messages, image, video, audio, or embeddings. This provider is text and SSE only. It rejects tool calling, structured output, and non-text parts. A streamed tool-call payload is not executed. It does not retry a failed or timed-out call. A caller-supplied fetch is trusted and can already have sent the API key before a redirected response is refused.

A non-default base URL requires `allowCustomEndpoint: true`.
