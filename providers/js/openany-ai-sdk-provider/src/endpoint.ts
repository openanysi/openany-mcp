export const DEFAULT_BASE_URL = 'https://api.openany.si/v1';
export const CHAT_COMPLETIONS_PATH = '/chat/completions';
/** Idle/header timeout used when the caller does not pass timeoutMs. */
export const DEFAULT_TIMEOUT_MS = 60_000;
const BLOCKED_HOSTS = new Set(['mcp.openany.si']);

export class OpenAnyEndpointError extends Error {
  readonly isRetryable = false;

  constructor(message: string) {
    super(message);
    this.name = 'OpenAnyEndpointError';
  }
}

export interface ResolvedEndpoint {
  /** API root, with no trailing slash, that `/chat/completions` is appended to. */
  baseURL: string;
  chatCompletionsURL: string;
}

function isLoopback(hostname: string): boolean {
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]' || hostname === '::1';
}

/**
 * Lower-case a host and remove a trailing DNS root dot.
 * `mcp.openany.si.` is the same name as `mcp.openany.si`.
 */
export function canonicalHost(hostname: string): string {
  const host = hostname.trim().toLowerCase().replace(/\.+$/, '');
  if (!host || host.split('.').some(label => label.length === 0)) {
    throw new OpenAnyEndpointError('OpenAny baseURL host is not valid.');
  }
  return host;
}

/**
 * Resolve the Chat Completions API root.
 * The default host is always https://api.openany.si/v1.
 * Any other URL requires allowCustomEndpoint, which is an explicit opt-in.
 * The MCP host is rejected even with that opt-in, including case and trailing-dot forms.
 */
export function resolveEndpoint(
  baseURL: string | undefined,
  allowCustomEndpoint: boolean,
  apiKey?: string,
): ResolvedEndpoint {
  const raw = baseURL ?? DEFAULT_BASE_URL;
  if (apiKey && raw.includes(apiKey)) {
    throw new OpenAnyEndpointError(
      'OpenAny baseURL must not contain the API key. Pass the key as apiKey or OPENANY_API_KEY.',
    );
  }
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new OpenAnyEndpointError('OpenAny baseURL is not a valid URL.');
  }
  if (url.username || url.password) {
    throw new OpenAnyEndpointError('OpenAny baseURL must not include userinfo.');
  }
  if (url.search || url.hash) {
    throw new OpenAnyEndpointError('OpenAny baseURL must not include a query or fragment.');
  }
  const host = canonicalHost(url.hostname);
  if (BLOCKED_HOSTS.has(host)) {
    throw new OpenAnyEndpointError(
      'OpenAny chat requests are not sent to the MCP host. Use https://api.openany.si/v1.',
    );
  }
  const path = url.pathname.replace(/\/+$/, '') || '/';
  if (path !== '/v1') {
    throw new OpenAnyEndpointError(
      'OpenAny baseURL path must be /v1. The adapter appends /chat/completions.',
    );
  }
  if (url.protocol === 'http:' && !isLoopback(host)) {
    throw new OpenAnyEndpointError('OpenAny baseURL must use https, except for loopback tests.');
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') {
    throw new OpenAnyEndpointError('OpenAny baseURL must use https.');
  }
  const defaultPort = url.protocol === 'https:' ? '443' : '80';
  const port = url.port && url.port !== defaultPort ? `:${url.port}` : '';
  const normalized = `${url.protocol}//${host}${port}/v1`;
  const isDefault = normalized === DEFAULT_BASE_URL;
  if (!isDefault && !allowCustomEndpoint) {
    throw new OpenAnyEndpointError(
      'A non-default OpenAny baseURL requires allowCustomEndpoint: true. The API key is not sent to another host unless you set that option.',
    );
  }
  return {
    baseURL: normalized,
    chatCompletionsURL: `${normalized}${CHAT_COMPLETIONS_PATH}`,
  };
}
