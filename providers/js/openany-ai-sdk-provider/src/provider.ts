import { createOpenAICompatible } from '@ai-sdk/openai-compatible';
import {
  APICallError,
  NoSuchModelError,
  UnsupportedFunctionalityError,
  type LanguageModelV4,
  type LanguageModelV4CallOptions,
  type LanguageModelV4GenerateResult,
  type LanguageModelV4StreamPart,
  type LanguageModelV4StreamResult,
  type SharedV4Warning,
} from '@ai-sdk/provider';
import type { FetchFunction } from '@ai-sdk/provider-utils';
import { Agent, fetch as undiciFetch } from 'undici';
import { assertApiKey, resolveApiKey } from './credentials.js';
import { DEFAULT_TIMEOUT_MS, OpenAnyEndpointError, resolveEndpoint } from './endpoint.js';
import { asNonRetryable, redactHeaders, redactValue } from './redact.js';
import { convertOpenAnyUsage } from './usage.js';

const FORBIDDEN_HEADERS = new Set([
  'authorization',
  'proxy-authorization',
  'cookie',
  'set-cookie',
  'x-api-key',
  'xi-api-key',
]);

const ALLOWED_BODY_KEYS = new Set([
  'model',
  'messages',
  'max_tokens',
  'temperature',
  'top_p',
  'frequency_penalty',
  'presence_penalty',
  'stop',
  'seed',
  'stream',
]);

export interface OpenAnyProviderSettings {
  /**
   * Raw OpenAny API key. When omitted, OPENANY_API_KEY is read on each call.
   * Do not include the Bearer prefix.
   */
  apiKey?: string;
  /**
   * API root. The default is https://api.openany.si/v1.
   * Another URL is used only when allowCustomEndpoint is true.
   */
  baseURL?: string;
  /**
   * Explicit opt-in for a non-default baseURL. The API key is not sent to
   * another host unless this is true. The MCP host is still rejected.
   */
  allowCustomEndpoint?: boolean;
  /** Extra request headers. Authorization and cookie headers are rejected. */
  headers?: Record<string, string>;
  /**
   * Optional fetch implementation.
   *
   * This function is trusted code. It receives the request, including the
   * Authorization header. If it follows a redirect, logs the request, or
   * sends the header somewhere else, this package cannot undo that. The
   * guard below only refuses to treat a redirected or unexpected final URL
   * as a successful completion.
   */
  fetch?: FetchFunction;
  /**
   * Positive millisecond budget for response headers and for each idle gap
   * while reading the body. The default is 60000. There is no unlimited value.
   * A timeout is not retried.
   */
  timeoutMs?: number;
}

export interface OpenAnyProvider {
  (modelId: string): LanguageModelV4;
  specificationVersion: 'v4';
  languageModel(modelId: string): LanguageModelV4;
  chatModel(modelId: string): LanguageModelV4;
  embeddingModel(modelId: string): never;
  imageModel(modelId: string): never;
  transcriptionModel(modelId: string): never;
  speechModel(modelId: string): never;
  completionModel(modelId: string): never;
}

function unsupported(functionality: string, message: string): UnsupportedFunctionalityError {
  return new UnsupportedFunctionalityError({ functionality, message });
}

export function assertModelId(modelId: string): string {
  if (typeof modelId !== 'string' || modelId.trim().length === 0 || /[\u0000\r\n]/.test(modelId)) {
    throw unsupported(
      'model id',
      'Pass the model id copied from the current OpenAny listing. This package does not ship a model catalog.',
    );
  }
  return modelId;
}

function assertProviderHeaders(headers: Record<string, string> | undefined, apiKey: string | undefined): Record<string, string> | undefined {
  if (!headers) {
    return undefined;
  }
  const output: Record<string, string> = {};
  for (const [name, value] of Object.entries(headers)) {
    if (FORBIDDEN_HEADERS.has(name.toLowerCase())) {
      throw new OpenAnyHeaderError(`${name} is reserved. The adapter sets Authorization from the OpenAny API key.`);
    }
    if (apiKey && value.includes(apiKey)) {
      throw new OpenAnyHeaderError('Custom headers must not contain the OpenAny API key.');
    }
    output[name] = value;
  }
  return output;
}

class OpenAnyHeaderError extends Error {
  readonly isRetryable = false;
  constructor(message: string) {
    super(message);
    this.name = 'OpenAnyHeaderError';
  }
}

function stripCallHeaders(
  headers: Record<string, string | undefined> | undefined,
): Record<string, string | undefined> | undefined {
  if (!headers) {
    return undefined;
  }
  const output: Record<string, string | undefined> = {};
  for (const [name, value] of Object.entries(headers)) {
    if (!FORBIDDEN_HEADERS.has(name.toLowerCase())) {
      output[name] = value;
    }
  }
  return output;
}

function assertTextMessages(messages: unknown): void {
  if (!Array.isArray(messages)) {
    throw unsupported('messages', 'Chat Completions messages must be an array.');
  }
  for (const message of messages) {
    if (!message || typeof message !== 'object') {
      throw unsupported('messages', 'Each Chat Completions message must be an object with role and content.');
    }
    const record = message as Record<string, unknown>;
    if (record.role === 'tool' || record.tool_calls != null || record.function_call != null) {
      throw unsupported(
        'tool messages',
        'This adapter does not send tool calls. The documented OpenAny chat input is messages with role and content.',
      );
    }
    const content = record.content;
    if (typeof content === 'string') {
      continue;
    }
    if (!Array.isArray(content)) {
      throw unsupported('message content', 'Message content must be text.');
    }
    for (const part of content) {
      if (!part || typeof part !== 'object' || (part as { type?: unknown }).type !== 'text') {
        throw unsupported(
          'non-text message content',
          'This adapter sends text Chat Completions content only. Image, audio, video, and file parts are not supported.',
        );
      }
    }
  }
}

function restrictRequestBody(body: Record<string, any>): Record<string, any> {
  for (const [key, value] of Object.entries(body)) {
    if (value === undefined) {
      continue;
    }
    if (!ALLOWED_BODY_KEYS.has(key)) {
      throw unsupported(
        key,
        `Refusing to send "${key}". This adapter only posts documented text Chat Completions fields.`,
      );
    }
  }
  assertTextMessages(body.messages);
  if (body.stream !== undefined && typeof body.stream !== 'boolean') {
    throw unsupported('stream', 'stream must be a boolean when it is sent.');
  }
  return body;
}

function assertSupportedCall(options: LanguageModelV4CallOptions): void {
  if (options.prompt.length === 0) {
    throw unsupported('empty prompt', 'Chat Completions requires at least one message.');
  }
  for (const message of options.prompt) {
    if (message.role === 'tool') {
      throw unsupported(
        'tool results',
        'This adapter does not send tool results. Tool calling is not a documented OpenAny Chat Completions feature.',
      );
    }
    if (message.role === 'system') {
      continue;
    }
    for (const part of message.content) {
      if (part.type !== 'text') {
        throw unsupported(
          part.type,
          'This adapter sends text Chat Completions messages only. File, image, audio, video, reasoning, and tool parts are not sent.',
        );
      }
    }
  }
  if (options.tools != null && options.tools.length > 0) {
    throw unsupported(
      'tool calling',
      'Tool calling is not documented for OpenAny Chat Completions. This adapter rejects tools before the request.',
    );
  }
  if (options.toolChoice != null && options.toolChoice.type !== 'auto' && options.toolChoice.type !== 'none') {
    throw unsupported('tool choice', 'Tool choice is not supported.');
  }
  if (options.responseFormat != null && options.responseFormat.type !== 'text') {
    throw unsupported(
      'structured output',
      'JSON response format is not a documented OpenAny Chat Completions feature.',
    );
  }
  if (options.reasoning != null && options.reasoning !== 'provider-default') {
    throw unsupported(
      'reasoning effort',
      'reasoning_effort is not part of the documented OpenAny Chat Completions request.',
    );
  }
  if (options.providerOptions != null && Object.keys(options.providerOptions).length > 0) {
    throw unsupported(
      'provider options',
      'Provider options are not forwarded. They could add undocumented request fields.',
    );
  }
  if (options.topK != null) {
    throw unsupported('topK', 'top_k is not part of the documented OpenAny Chat Completions request.');
  }
}

function requestURL(input: Parameters<FetchFunction>[0]): URL {
  if (typeof input === 'string') {
    return new URL(input);
  }
  if (input instanceof URL) {
    return input;
  }
  return new URL(input.url);
}

function assertTimeout(timeoutMs: number | undefined): number {
  const value = timeoutMs ?? DEFAULT_TIMEOUT_MS;
  if (!Number.isFinite(value) || value <= 0) {
    throw new OpenAnyEndpointError(
      'timeoutMs must be a positive finite number of milliseconds. Unlimited waits are not available.',
    );
  }
  return value;
}

function unwrapProviderError(error: unknown): unknown {
  if (
    APICallError.isInstance(error) &&
    error.message === 'Failed to process successful response' &&
    APICallError.isInstance(error.cause)
  ) {
    return error.cause;
  }
  return error;
}

function timeoutError(url: string): APICallError {
  return new APICallError({
    message: 'OpenAny chat request timed out. It was not retried.',
    url,
    requestBodyValues: {},
    isRetryable: false,
  });
}

function sameChatUrl(actual: URL, expected: URL): boolean {
  return actual.origin === expected.origin && actual.pathname === expected.pathname && !actual.search && !actual.hash;
}

function combineSignals(signals: Array<AbortSignal | null | undefined>): AbortSignal | undefined {
  const present = signals.filter((signal): signal is AbortSignal => signal != null);
  if (present.length === 0) {
    return undefined;
  }
  if (present.length === 1) {
    return present[0];
  }
  return AbortSignal.any(present);
}

function raceTimeout<T>(
  work: Promise<T>,
  timeoutMs: number,
  signal: AbortSignal | undefined,
  killer: AbortController,
  url: string,
): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      const error = timeoutError(url);
      killer.abort(error);
      reject(error);
    }, timeoutMs);
    const onAbort = () => {
      clearTimeout(timer);
      const reason = signal?.reason;
      reject(reason instanceof Error ? reason : new DOMException('The operation was aborted.', 'AbortError'));
    };
    if (signal?.aborted) {
      onAbort();
      return;
    }
    signal?.addEventListener('abort', onAbort, { once: true });
    work.then(
      value => {
        clearTimeout(timer);
        signal?.removeEventListener('abort', onAbort);
        resolve(value);
      },
      error => {
        clearTimeout(timer);
        signal?.removeEventListener('abort', onAbort);
        reject(error);
      },
    );
  });
}

function watchBody(
  response: Response,
  timeoutMs: number,
  signal: AbortSignal | undefined,
  killer: AbortController,
  onDone?: () => void,
): Response {
  let settled = false;
  const finish = () => {
    if (!settled) {
      settled = true;
      onDone?.();
    }
  };
  if (!response.body) {
    finish();
    return response;
  }
  const reader = response.body.getReader();
  const stream = new ReadableStream<Uint8Array>({
    async pull(controller) {
      try {
        const next = await raceTimeout(reader.read(), timeoutMs, signal, killer, response.url || 'https://api.openany.si/v1/chat/completions');
        if (next.done) {
          finish();
          controller.close();
          return;
        }
        controller.enqueue(next.value);
      } catch (error) {
        finish();
        try {
          await reader.cancel(error);
        } catch {
          // The timed-out body may already be unusable.
        }
        controller.error(error);
      }
    },
    cancel(reason) {
      finish();
      return reader.cancel(reason);
    },
  });
  return new Response(stream, {
    status: response.status,
    statusText: response.statusText,
    headers: response.headers,
  });
}

function createGuardedFetch(expected: string, inner: FetchFunction | undefined, timeoutMs: number): FetchFunction {
  const expectedURL = new URL(expected);
  return async (input, init) => {
    const url = requestURL(input);
    if (url.username || url.password || url.search || url.hash || !sameChatUrl(url, expectedURL)) {
      throw unsupported(
        'request URL',
        'OpenAny chat adapter only posts to the configured /v1/chat/completions URL.',
      );
    }
    const killer = new AbortController();
    const signal = combineSignals([init?.signal, killer.signal]);
    const agent = inner ? undefined : new Agent({ headersTimeout: timeoutMs, bodyTimeout: timeoutMs, connectTimeout: timeoutMs });
    const fetchImpl: FetchFunction =
      inner ??
      ((requestInput, requestInit) =>
        undiciFetch(requestInput as Parameters<typeof undiciFetch>[0], {
          ...(requestInit as object),
          dispatcher: agent,
        }) as unknown as ReturnType<FetchFunction>);
    let response: Response;
    try {
      response = await raceTimeout(
        Promise.resolve(fetchImpl(input, { ...init, redirect: 'manual', signal })),
        timeoutMs,
        signal,
        killer,
        expected,
      );
    } catch (error) {
      await agent?.close();
      throw error;
    }
    if (response.status >= 300 && response.status < 400 || response.type === 'opaqueredirect') {
      await response.body?.cancel().catch(() => undefined);
      await agent?.close();
      throw unsupported(
        'redirect',
        inner
          ? 'OpenAny refused a redirect response. A custom fetch may already have sent the API key; refusing the response does not undo that send.'
          : 'OpenAny refused a redirect response and did not follow it.',
      );
    }
    let finalURL = url;
    if (response.url) {
      try {
        finalURL = new URL(response.url);
      } catch {
        finalURL = url;
      }
    }
    if (!sameChatUrl(finalURL, expectedURL) || response.redirected) {
      await response.body?.cancel().catch(() => undefined);
      await agent?.close();
      throw unsupported(
        'response URL',
        'OpenAny refused a response whose final URL is not the configured chat completions URL. If a custom fetch followed a redirect, the API key may already have been sent; this check does not undo that send.',
      );
    }
    return watchBody(response, timeoutMs, signal, killer, () => {
      void agent?.close();
    });
  };
}

function withUnsupportedWarning(
  warnings: SharedV4Warning[],
  feature: string,
  details: string,
): SharedV4Warning[] {
  return [...warnings, { type: 'unsupported', feature, details }];
}

function sanitizeGenerateResult(
  result: LanguageModelV4GenerateResult,
  secret: string,
): LanguageModelV4GenerateResult {
  let strippedTools = false;
  let strippedFiles = false;
  const content = result.content.flatMap(part => {
    if (part.type === 'tool-call' || part.type === 'tool-result' || part.type === 'tool-approval-request') {
      strippedTools = true;
      return [];
    }
    if (part.type === 'file' || part.type === 'reasoning-file') {
      strippedFiles = true;
      return [];
    }
    return [part];
  });
  let warnings = result.warnings;
  let finishReason = result.finishReason;
  if (strippedTools) {
    warnings = withUnsupportedWarning(
      warnings,
      'tool calls',
      'The response included tool calls. They were not executed and are not returned as tool calls.',
    );
    if (finishReason.unified === 'tool-calls') {
      finishReason = { unified: 'other', raw: finishReason.raw };
    }
  }
  if (strippedFiles) {
    warnings = withUnsupportedWarning(
      warnings,
      'file output',
      'The response included file output. This adapter does not return image, audio, or video payloads.',
    );
  }
  return {
    ...result,
    content,
    warnings,
    finishReason,
    response: result.response
      ? {
          ...result.response,
          headers: redactHeaders(result.response.headers, secret),
          body: redactValue(result.response.body, secret),
        }
      : undefined,
    providerMetadata: redactValue(result.providerMetadata, secret) as LanguageModelV4GenerateResult['providerMetadata'],
    request: result.request
      ? { ...result.request, body: redactValue(result.request.body, secret) }
      : undefined,
  };
}

function sanitizeStreamPart(part: LanguageModelV4StreamPart, secret: string): LanguageModelV4StreamPart | undefined {
  switch (part.type) {
    case 'tool-input-start':
    case 'tool-input-delta':
    case 'tool-input-end':
    case 'tool-call':
    case 'tool-result':
    case 'tool-approval-request':
    case 'file':
    case 'reasoning-file':
      return undefined;
    case 'error':
      return { type: 'error', error: asNonRetryable(part.error, secret) };
    case 'finish':
      return part.finishReason.unified === 'tool-calls'
        ? { ...part, finishReason: { unified: 'other', raw: part.finishReason.raw } }
        : part;
    default:
      return part;
  }
}

function isDroppedStreamPart(part: LanguageModelV4StreamPart): boolean {
  return (
    part.type === 'tool-input-start' ||
    part.type === 'tool-input-delta' ||
    part.type === 'tool-input-end' ||
    part.type === 'tool-call' ||
    part.type === 'tool-result' ||
    part.type === 'tool-approval-request' ||
    part.type === 'file' ||
    part.type === 'reasoning-file'
  );
}

function guardStream(
  stream: ReadableStream<LanguageModelV4StreamPart>,
  secret: string,
): ReadableStream<LanguageModelV4StreamPart> {
  const reader = stream.getReader();
  let sawFinish = false;
  return new ReadableStream<LanguageModelV4StreamPart>({
    async pull(controller) {
      try {
        while (true) {
          const next = await reader.read();
          if (next.done) {
            if (!sawFinish) {
              controller.enqueue({
                type: 'finish',
                finishReason: { unified: 'other', raw: undefined },
                usage: convertOpenAnyUsage(undefined),
              });
            }
            controller.close();
            return;
          }
          if (isDroppedStreamPart(next.value)) {
            continue;
          }
          const part = sanitizeStreamPart(next.value, secret);
          if (!part) {
            continue;
          }
          if (part.type === 'finish') {
            sawFinish = true;
          }
          controller.enqueue(part);
          return;
        }
      } catch (error) {
        controller.error(asNonRetryable(unwrapProviderError(error), secret));
      }
    },
    cancel(reason) {
      return reader.cancel(reason);
    },
  });
}

class OpenAnyChatLanguageModel implements LanguageModelV4 {
  readonly specificationVersion = 'v4' as const;
  readonly provider = 'openany.chat';
  readonly modelId: string;
  readonly supportedUrls = {};
  readonly #inner: LanguageModelV4;
  readonly #resolveKey: () => string;

  constructor(inner: LanguageModelV4, resolveKey: () => string) {
    this.#inner = inner;
    this.#resolveKey = resolveKey;
    this.modelId = inner.modelId;
  }

  private currentKey(): string {
    return this.#resolveKey();
  }

  async doGenerate(options: LanguageModelV4CallOptions): Promise<LanguageModelV4GenerateResult> {
    const key = this.currentKey();
    assertSupportedCall(options);
    try {
      const result = await this.#inner.doGenerate({
        ...options,
        headers: stripCallHeaders(options.headers),
      });
      return sanitizeGenerateResult(result, key);
    } catch (error) {
      throw asNonRetryable(unwrapProviderError(error), key);
    }
  }

  async doStream(options: LanguageModelV4CallOptions): Promise<LanguageModelV4StreamResult> {
    const key = this.currentKey();
    assertSupportedCall(options);
    try {
      const result = await this.#inner.doStream({
        ...options,
        headers: stripCallHeaders(options.headers),
      });
      return {
        ...result,
        request: result.request
          ? { ...result.request, body: redactValue(result.request.body, key) }
          : undefined,
        response: result.response
          ? { ...result.response, headers: redactHeaders(result.response.headers, key) }
          : undefined,
        stream: guardStream(result.stream, key),
      };
    } catch (error) {
      throw asNonRetryable(unwrapProviderError(error), key);
    }
  }
}

/**
 * Create an OpenAny provider for the Vercel AI SDK.
 * Model ids are passed through. They are not an enum in this package.
 */
export function createOpenAny(options: OpenAnyProviderSettings = {}): OpenAnyProvider {
  if (options.apiKey !== undefined) {
    assertApiKey(options.apiKey);
  }
  const endpoint = resolveEndpoint(options.baseURL, options.allowCustomEndpoint === true, options.apiKey);
  const headers = assertProviderHeaders(options.headers, options.apiKey);
  const timeoutMs = assertTimeout(options.timeoutMs);
  const explicitKey = options.apiKey;
  const resolveKey = () => {
    const key = resolveApiKey(explicitKey);
    for (const value of Object.values(headers ?? {})) {
      if (value.includes(key)) {
        throw new OpenAnyHeaderError('Custom headers must not contain the OpenAny API key.');
      }
    }
    return key;
  };
  const inner = createOpenAICompatible({
    name: 'openany',
    baseURL: endpoint.baseURL,
    apiKey: resolveKey,
    headers,
    fetch: createGuardedFetch(endpoint.chatCompletionsURL, options.fetch, timeoutMs),
    includeUsage: false,
    supportsStructuredOutputs: false,
    supportsMultiPartToolContent: false,
    supportedUrls: () => ({}),
    transformRequestBody: body => restrictRequestBody(body),
    convertUsage: convertOpenAnyUsage,
  });

  const createModel = (modelId: string): LanguageModelV4 => {
    const id = assertModelId(modelId);
    return new OpenAnyChatLanguageModel(inner.chatModel(id), resolveKey);
  };

  function provider(this: unknown, modelId: string): LanguageModelV4 {
    if (new.target) {
      throw new Error('The OpenAny model factory cannot be called with new.');
    }
    return createModel(modelId);
  }

  const callable = provider as OpenAnyProvider;
  callable.specificationVersion = 'v4';
  callable.languageModel = createModel;
  callable.chatModel = createModel;
  callable.embeddingModel = modelId => {
    throw new NoSuchModelError({
      modelId,
      modelType: 'embeddingModel',
    });
  };
  callable.imageModel = modelId => {
    throw new NoSuchModelError({ modelId, modelType: 'imageModel' });
  };
  callable.transcriptionModel = modelId => {
    throw new NoSuchModelError({ modelId, modelType: 'transcriptionModel' });
  };
  callable.speechModel = modelId => {
    throw new NoSuchModelError({ modelId, modelType: 'speechModel' });
  };
  callable.completionModel = () => {
    throw unsupported(
      'text completions',
      'This adapter only calls POST /v1/chat/completions. Legacy text completions are not supported.',
    );
  };
  return callable;
}

export const openany = createOpenAny();

export { APICallError };
