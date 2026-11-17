import assert from 'node:assert/strict';
import http from 'node:http';
import { once } from 'node:events';
import test from 'node:test';
import { inspect } from 'node:util';
import { generateText, streamText } from 'ai';
import { APICallError, NoSuchModelError } from '@ai-sdk/provider';
import { createOpenAny } from '../dist/index.js';

const FAKE_KEY = 'sk-test-openany-not-a-real-key';
const MODEL_ID = 'listing/model:route-1';

interface RecordedRequest {
  url?: string;
  method?: string;
  authorization?: string;
  body?: unknown;
}

interface MockServer {
  baseURL: string;
  requests: RecordedRequest[];
  close: () => Promise<void>;
}

function listen(
  handler: (request: http.IncomingMessage, response: http.ServerResponse, body: unknown) => void,
): Promise<MockServer> {
  const requests: RecordedRequest[] = [];
  const server = http.createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on('data', chunk => chunks.push(chunk as Buffer));
    request.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8');
      let body: unknown;
      try {
        body = raw ? JSON.parse(raw) : undefined;
      } catch {
        body = raw;
      }
      requests.push({
        url: request.url,
        method: request.method,
        authorization: request.headers.authorization,
        body,
      });
      handler(request, response, body);
    });
  });
  server.listen(0, '127.0.0.1');
  return once(server, 'listening').then(() => {
    const address = server.address();
    if (!address || typeof address === 'string') {
      throw new Error('test server did not bind a port');
    }
    return {
      baseURL: `http://127.0.0.1:${address.port}/v1`,
      requests,
      close: () =>
        new Promise((resolve, reject) => {
          server.close(error => (error ? reject(error) : resolve()));
        }),
    };
  });
}

function jsonResponse(status: number, payload: unknown) {
  return (_request: http.IncomingMessage, response: http.ServerResponse) => {
    const body = JSON.stringify(payload);
    response.writeHead(status, { 'content-type': 'application/json' });
    response.end(body);
  };
}

function providerFor(server: MockServer, apiKey: string = FAKE_KEY) {
  return createOpenAny({
    apiKey,
    baseURL: server.baseURL,
    allowCustomEndpoint: true,
  });
}

test('rejects a missing key before any request', async () => {
  const previous = process.env.OPENANY_API_KEY;
  delete process.env.OPENANY_API_KEY;
  const server = await listen(() => {
    throw new Error('the mock server should not be called');
  });
  try {
    const openany = createOpenAny({ baseURL: server.baseURL, allowCustomEndpoint: true });
    await assert.rejects(
      () => generateText({ model: openany(MODEL_ID), prompt: 'Hello' }),
      /API key is missing/,
    );
    assert.equal(server.requests.length, 0);
  } finally {
    if (previous === undefined) {
      delete process.env.OPENANY_API_KEY;
    } else {
      process.env.OPENANY_API_KEY = previous;
    }
    await server.close();
  }
});

test('rejects placeholder and header-shaped keys before any request', async () => {
  const server = await listen(() => {
    throw new Error('the mock server should not be called');
  });
  try {
    for (const apiKey of ['${OPENANY_API_KEY}', '$OPENANY_API_KEY', '<key>', 'YOUR_API_KEY', `Bearer ${FAKE_KEY}`]) {
      assert.throws(() => createOpenAny({ apiKey, baseURL: server.baseURL, allowCustomEndpoint: true }), /placeholder|raw key/);
    }
    assert.equal(server.requests.length, 0);
  } finally {
    await server.close();
  }
});

test('requires an explicit opt-in before using a non-default endpoint', () => {
  assert.throws(
    () => createOpenAny({ apiKey: FAKE_KEY, baseURL: 'http://127.0.0.1:9/v1' }),
    /allowCustomEndpoint/,
  );
  assert.throws(
    () => createOpenAny({ apiKey: FAKE_KEY, baseURL: 'https://mcp.openany.si/v1', allowCustomEndpoint: true }),
    /MCP host/,
  );
  assert.throws(
    () => createOpenAny({ apiKey: FAKE_KEY, baseURL: `http://127.0.0.1:9/v1?key=${FAKE_KEY}`, allowCustomEndpoint: true }),
    /must not contain the API key/,
  );
  assert.throws(
    () => createOpenAny({ apiKey: FAKE_KEY, baseURL: 'http://user:password@127.0.0.1:9/v1', allowCustomEndpoint: true }),
    /userinfo/,
  );
});

test('posts text chat completions and passes the model id through', async () => {
  const server = await listen(
    jsonResponse(200, {
      id: 'resp-2',
      model: MODEL_ID,
      choices: [{ index: 0, message: { role: 'assistant', content: 'OpenAny connected' }, finish_reason: 'stop' }],
      usage: { prompt_tokens: 11, completion_tokens: 3, total_tokens: 14, prompt_tokens_details: { cached_tokens: 4 } },
    }),
  );
  try {
    const result = await generateText({
      model: providerFor(server)(MODEL_ID),
      prompt: 'Reply with exactly: OpenAny connected',
      maxOutputTokens: 32,
      temperature: 0,
      headers: { Authorization: 'Bearer attacker-should-not-be-sent', 'X-Request-Tag': 'unit' },
    });
    assert.equal(result.text, 'OpenAny connected');
    assert.equal(result.finishReason, 'stop');
    assert.equal(result.usage.inputTokens, 11);
    assert.equal(result.usage.outputTokens, 3);
    assert.equal(result.usage.inputTokenDetails.cacheReadTokens, 4);
    assert.equal(server.requests.length, 1);
    const request = server.requests[0];
    assert.equal(request?.url, '/v1/chat/completions');
    assert.equal(request?.authorization, `Bearer ${FAKE_KEY}`);
    const body = request?.body as { model?: string; stream?: boolean; max_tokens?: number; messages?: unknown[] };
    assert.equal(body.model, MODEL_ID);
    assert.equal(body.stream, undefined);
    assert.equal(body.max_tokens, 32);
    assert.equal(JSON.stringify(request).includes('attacker-should-not-be-sent'), false);
    assert.equal(JSON.stringify(result.response.headers ?? {}).includes(FAKE_KEY), false);
  } finally {
    await server.close();
  }
});

test('does not retry a paid HTTP error and redacts the key', async () => {
  const server = await listen(
    jsonResponse(500, { error: { message: `upstream failed ${FAKE_KEY}` } }),
  );
  try {
    const error = await generateText({
      model: providerFor(server)(MODEL_ID),
      prompt: 'Hello',
    }).then(
      () => {
        throw new Error('expected the request to fail');
      },
      (caught: unknown) => caught,
    );
    assert.equal(server.requests.length, 1);
    assert.ok(APICallError.isInstance(error));
    assert.equal(error.isRetryable, false);
    assert.equal(error.statusCode, 500);
    const rendered = `${error.message}\n${error.responseBody}\n${error.url}`;
    assert.equal(rendered.includes(FAKE_KEY), false);
    assert.match(rendered, /\[redacted\]/);
  } finally {
    await server.close();
  }
});

test('refuses redirects and does not send the key to the next host', async () => {
  const stolen: RecordedRequest[] = [];
  const thief = http.createServer((request, response) => {
    stolen.push({ url: request.url, authorization: request.headers.authorization });
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end('{"choices":[{"message":{"content":"stolen"}}]}');
  });
  thief.listen(0, '127.0.0.1');
  await once(thief, 'listening');
  const thiefAddress = thief.address();
  if (!thiefAddress || typeof thiefAddress === 'string') {
    throw new Error('thief server did not bind');
  }
  const server = await listen((_request, response) => {
    response.writeHead(302, { location: `http://127.0.0.1:${thiefAddress.port}/steal` });
    response.end();
  });
  try {
    await assert.rejects(
      () => generateText({ model: providerFor(server)(MODEL_ID), prompt: 'Hello' }),
      /redirect/,
    );
    assert.equal(server.requests.length, 1);
    assert.equal(stolen.length, 0);
  } finally {
    await server.close();
    await new Promise<void>((resolve, reject) => thief.close(error => (error ? reject(error) : resolve())));
  }
});

test('rejects tools, files, and other model kinds before the network', async () => {
  const server = await listen(() => {
    throw new Error('the mock server should not be called');
  });
  try {
    const openany = providerFor(server);
    await assert.rejects(
      () =>
        openany(MODEL_ID).doGenerate({
          prompt: [{ role: 'user', content: [{ type: 'text', text: 'Use a tool' }] }],
          tools: [
            {
              type: 'function',
              name: 'lookup',
              inputSchema: { type: 'object', properties: { q: { type: 'string' } } },
            },
          ],
        }),
      /Tool calling/,
    );
    await assert.rejects(
      () =>
        openany(MODEL_ID).doGenerate({
          prompt: [
            {
              role: 'user',
              content: [{ type: 'file', mediaType: 'image/png', data: { type: 'data', data: new Uint8Array([1, 2, 3]) } }],
            },
          ],
        }),
      /image/,
    );
    assert.throws(() => openany.embeddingModel('embed'), NoSuchModelError.isInstance);
    assert.throws(() => openany.imageModel('image'), NoSuchModelError.isInstance);
    assert.throws(() => openany.completionModel('complete'), /text completions/);
    assert.equal(server.requests.length, 0);
  } finally {
    await server.close();
  }
});

test('reassembles fragmented SSE chunks, usage, and finish reason', async () => {
  const events = [
    `data: ${JSON.stringify({ id: 'resp-1', model: MODEL_ID, choices: [{ index: 0, delta: { content: 'He' } }] })}\n\n`,
    `data: ${JSON.stringify({ id: 'resp-1', model: MODEL_ID, choices: [{ index: 0, delta: { content: 'llo 你' } }] })}\n\n`,
    `data: ${JSON.stringify({
      id: 'resp-1',
      model: MODEL_ID,
      choices: [{ index: 0, delta: {}, finish_reason: 'length' }],
      usage: { prompt_tokens: 5, completion_tokens: 2, total_tokens: 7 },
    })}\n\n`,
    'data: [DONE]\n\n',
  ].join('');
  const bytes = Buffer.from(events, 'utf8');
  const splitAt = bytes.indexOf(Buffer.from('你', 'utf8')) + 1;
  const server = await listen((_request, response) => {
    response.writeHead(200, { 'content-type': 'text/event-stream' });
    response.write(bytes.subarray(0, splitAt));
    setTimeout(() => {
      response.write(bytes.subarray(splitAt));
      response.end();
    }, 20);
  });
  try {
    const result = streamText({ model: providerFor(server)(MODEL_ID), prompt: 'Hello' });
    let text = '';
    for await (const chunk of result.textStream) {
      text += chunk;
    }
    assert.equal(text, 'Hello 你');
    assert.equal(await result.finishReason, 'length');
    const usage = await result.usage;
    assert.equal(usage.inputTokens, 5);
    assert.equal(usage.outputTokens, 2);
    assert.equal(server.requests.length, 1);
    const body = server.requests[0]?.body as { stream?: boolean; model?: string };
    assert.equal(body.stream, true);
    assert.equal(body.model, MODEL_ID);
    assert.equal(JSON.stringify(server.requests[0]).includes('stream_options'), false);
  } finally {
    await server.close();
  }
});

test('stops a cancelled stream without a second request', async () => {
  const server = await listen((_request, response) => {
    response.writeHead(200, { 'content-type': 'text/event-stream' });
    response.write(`data: ${JSON.stringify({ choices: [{ index: 0, delta: { content: 'partial' } }] })}\n\n`);
  });
  const controller = new AbortController();
  try {
    const result = streamText({
      model: providerFor(server)(MODEL_ID),
      prompt: 'Hello',
      abortSignal: controller.signal,
    });
    const iterator = result.textStream[Symbol.asyncIterator]();
    const first = await iterator.next();
    assert.equal(first.value, 'partial');
    controller.abort();
    await iterator.return?.(undefined);
    assert.equal(server.requests.length, 1);
  } finally {
    await server.close();
  }
});

test('reads the key from OPENANY_API_KEY when the constructor omits it', async () => {
  const previous = process.env.OPENANY_API_KEY;
  process.env.OPENANY_API_KEY = FAKE_KEY;
  const server = await listen(
    jsonResponse(200, {
      choices: [{ message: { role: 'assistant', content: 'from env' }, finish_reason: 'stop' }],
    }),
  );
  try {
    const result = await generateText({
      model: createOpenAny({ baseURL: server.baseURL, allowCustomEndpoint: true })('env-model'),
      prompt: 'Hello',
    });
    assert.equal(result.text, 'from env');
    assert.equal(result.usage.inputTokens, undefined);
    assert.equal(server.requests[0]?.authorization, `Bearer ${FAKE_KEY}`);
    assert.equal((server.requests[0]?.body as { model?: string }).model, 'env-model');
  } finally {
    if (previous === undefined) {
      delete process.env.OPENANY_API_KEY;
    } else {
      process.env.OPENANY_API_KEY = previous;
    }
    await server.close();
  }
});

function deadline<T>(work: Promise<T>, ms: number): Promise<T> {
  let timer: NodeJS.Timeout;
  const timeout = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => reject(new Error(`deadline ${ms}ms`)), ms);
  });
  return Promise.race([work, timeout]).finally(() => clearTimeout(timer));
}

test('rejects trailing-dot and mixed-case MCP hosts', () => {
  for (const baseURL of ['https://mcp.openany.si./v1', 'https://MCP.OpenAny.SI./v1', 'https://mcp.openany.si.../v1']) {
    assert.throws(
      () => createOpenAny({ apiKey: FAKE_KEY, baseURL, allowCustomEndpoint: true }),
      /MCP host/,
    );
  }
  assert.doesNotThrow(() => createOpenAny({ apiKey: FAKE_KEY, baseURL: 'https://API.OpenAny.SI./v1' }));
});

test('does not expose the API key through inspect or JSON', () => {
  const provider = createOpenAny({ apiKey: FAKE_KEY });
  const model = provider(MODEL_ID);
  const rendered = [
    JSON.stringify(model) ?? '',
    JSON.stringify(provider) ?? '',
    inspect(model),
    inspect(provider),
    inspect(model, { showHidden: true, depth: 8 }),
    inspect(provider, { showHidden: true, depth: 8 }),
  ].join('\n');
  assert.equal(rendered.includes(FAKE_KEY), false);
});

test('streamText completes a tool-call SSE without executing tools', async () => {
  const events = [
    `data: ${JSON.stringify({
      choices: [{ index: 0, delta: { content: 'Hi', tool_calls: [{ index: 0, id: 'call-1', type: 'function', function: { name: 'lookup', arguments: '{"q":"x"}' } }] } }],
    })}\n\n`,
    `data: ${JSON.stringify({
      choices: [{ index: 0, delta: {}, finish_reason: 'tool_calls' }],
      usage: { prompt_tokens: 2, completion_tokens: 1, total_tokens: 3 },
    })}\n\n`,
    'data: [DONE]\n\n',
  ].join('');
  const server = await listen((_request, response) => {
    response.writeHead(200, { 'content-type': 'text/event-stream' });
    response.end(events);
  });
  try {
    const result = streamText({ model: providerFor(server)(MODEL_ID), prompt: 'Hi' });
    const parts: string[] = [];
    await deadline((async () => {
      for await (const part of result.fullStream) {
        parts.push(part.type);
      }
    })(), 1000);
    const text = await deadline(result.text, 1000);
    const finish = await deadline(result.finishReason, 1000);
    const toolCalls = await deadline(result.toolCalls, 1000);
    assert.equal(text, 'Hi');
    assert.notEqual(finish, 'tool-calls');
    assert.equal(toolCalls.length, 0);
    assert.equal(parts.some(type => type === 'tool-call' || type === 'tool-input-start'), false);
    assert.equal(server.requests.length, 1);
  } finally {
    await server.close();
  }
});

test('generateText keeps text and does not execute tool calls in a JSON response', async () => {
  const server = await listen(
    jsonResponse(200, {
      choices: [
        {
          message: {
            role: 'assistant',
            content: 'Hi',
            tool_calls: [{ id: 'call-1', type: 'function', function: { name: 'lookup', arguments: '{"q":"x"}' } }],
          },
          finish_reason: 'tool_calls',
        },
      ],
    }),
  );
  try {
    const result = await generateText({ model: providerFor(server)(MODEL_ID), prompt: 'Hi' });
    assert.equal(result.text, 'Hi');
    assert.equal(result.toolCalls.length, 0);
    assert.equal(server.requests.length, 1);
  } finally {
    await server.close();
  }
});

test('default fetch times out a stalled body and a missing response once', async () => {
  for (const phase of ['headers', 'body'] as const) {
    const requests: string[] = [];
    const server = http.createServer((request, response) => {
      const chunks: Buffer[] = [];
      request.on('data', chunk => chunks.push(chunk as Buffer));
      request.on('end', () => {
        requests.push(request.url ?? '');
        if (phase === 'body') {
          response.writeHead(200, { 'content-type': 'text/event-stream' });
          response.write(`data: ${JSON.stringify({ choices: [{ index: 0, delta: { content: 'x' } }] })}\n\n`);
        }
        setTimeout(() => {
          if (!response.writableEnded) {
            response.end();
          }
        }, 2000);
      });
    });
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    const address = server.address();
    if (!address || typeof address === 'string') {
      throw new Error('stall server did not bind');
    }
    try {
      const model = createOpenAny({
        apiKey: FAKE_KEY,
        baseURL: `http://127.0.0.1:${address.port}/v1`,
        allowCustomEndpoint: true,
        timeoutMs: 200,
      })(MODEL_ID);
      const run = phase === 'headers' ? generateText({ model, prompt: 'Hello' }) : streamText({ model, prompt: 'Hello' }).text;
      const error = await deadline(
        run.then(
          () => {
            throw new Error('expected a timeout');
          },
          (caught: unknown) => caught,
        ),
        1500,
      );
      assert.match(String((error as Error).message), /timed out|timeout/i);
      assert.equal(String(error).includes(FAKE_KEY), false);
      if (APICallError.isInstance(error)) {
        assert.equal(error.isRetryable, false);
      }
      await new Promise(resolve => setTimeout(resolve, 50));
      assert.equal(requests.length, 1);
    } finally {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close(closeError => (closeError ? reject(closeError) : resolve())));
    }
  }
});

test('default fetch does not follow a same-origin 307', async () => {
  const stolen: string[] = [];
  const server = await listen((request, response) => {
    if (request.url === '/steal') {
      stolen.push(request.headers.authorization ?? '');
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ choices: [{ message: { content: 'stolen' } }] }));
      return;
    }
    response.writeHead(307, { location: '/steal' });
    response.end();
  });
  try {
    await assert.rejects(
      () => generateText({ model: providerFor(server)(MODEL_ID), prompt: 'Hello' }),
      /redirect/,
    );
    assert.equal(stolen.length, 0);
    assert.equal(server.requests.length, 1);
  } finally {
    await server.close();
  }
});

test('a custom fetch that follows redirects is not treated as success', async () => {
  const stolen: Array<string | undefined> = [];
  const server = await listen((request, response) => {
    if (request.url === '/steal') {
      stolen.push(request.headers.authorization);
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ choices: [{ message: { content: 'stolen' } }] }));
      return;
    }
    response.writeHead(307, { location: '/steal' });
    response.end();
  });
  try {
    const model = createOpenAny({
      apiKey: FAKE_KEY,
      baseURL: server.baseURL,
      allowCustomEndpoint: true,
      fetch: (input, init) => fetch(input, { ...init, redirect: 'follow' }),
    })(MODEL_ID);
    await assert.rejects(
      () => generateText({ model, prompt: 'Hello' }),
      /may already have been sent/,
    );
    assert.equal(stolen.length, 1);
    assert.equal(stolen[0]?.includes(FAKE_KEY), true);
  } finally {
    await server.close();
  }
});
