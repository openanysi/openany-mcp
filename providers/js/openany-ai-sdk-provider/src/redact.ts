import { APICallError } from '@ai-sdk/provider';

const SENSITIVE_HEADER = /^(authorization|proxy-authorization|cookie|set-cookie|x-api-key|xi-api-key)$/i;

export function redactString(value: string, secret: string | undefined): string {
  if (!secret || secret.length === 0 || !value.includes(secret)) {
    return value;
  }
  return value.split(secret).join('[redacted]');
}

export function redactValue(value: unknown, secret: string | undefined): unknown {
  if (typeof value === 'string') {
    return redactString(value, secret);
  }
  if (Array.isArray(value)) {
    return value.map(item => redactValue(item, secret));
  }
  if (value && typeof value === 'object') {
    const output: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      output[key] = SENSITIVE_HEADER.test(key) ? '[redacted]' : redactValue(item, secret);
    }
    return output;
  }
  return value;
}

export function redactHeaders(
  headers: Record<string, string> | undefined,
  secret: string | undefined,
): Record<string, string> | undefined {
  if (!headers) {
    return undefined;
  }
  const output: Record<string, string> = {};
  for (const [key, value] of Object.entries(headers)) {
    output[key] = SENSITIVE_HEADER.test(key) ? '[redacted]' : redactString(value, secret);
  }
  return output;
}

function isAbortError(error: unknown): boolean {
  return (
    (error instanceof Error && error.name === 'AbortError') ||
    (typeof DOMException !== 'undefined' && error instanceof DOMException && error.name === 'AbortError')
  );
}

/** Rebuild provider errors so paid calls are not marked retryable and secrets are removed. */
export function asNonRetryable(error: unknown, secret: string | undefined): unknown {
  if (isAbortError(error)) {
    return error;
  }
  if (APICallError.isInstance(error)) {
    return new APICallError({
      message: redactString(error.message, secret),
      url: redactString(error.url, secret),
      requestBodyValues: redactValue(error.requestBodyValues, secret),
      statusCode: error.statusCode,
      responseHeaders: redactHeaders(error.responseHeaders, secret),
      responseBody: error.responseBody ? redactString(error.responseBody, secret) : undefined,
      cause: error.cause,
      isRetryable: false,
      data: redactValue(error.data, secret),
    });
  }
  if (error instanceof Error) {
    const wrapped = new Error(redactString(error.message, secret));
    wrapped.name = error.name;
    wrapped.cause = error.cause;
    return wrapped;
  }
  return new Error('OpenAny chat request failed.');
}
