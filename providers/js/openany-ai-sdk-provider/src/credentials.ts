/** Page where a caller creates an OpenAny API key. The key itself is never included. */
export const KEY_PAGE_URL = 'https://openany.si/dashboard?page=api-keys';

const PLACEHOLDER_VALUES = new Set([
  'YOUR_API_KEY',
  'YOUR_OPENANY_KEY',
  'YOUR_OPENANY_API_KEY',
  'OPENANY_API_KEY',
]);

export class OpenAnyCredentialsError extends Error {
  readonly isRetryable = false;

  constructor(message: string) {
    super(message);
    this.name = 'OpenAnyCredentialsError';
  }
}

function isIdent(value: string): boolean {
  if (value.length === 0) {
    return false;
  }
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    const letter =
      (code >= 65 && code <= 90) || (code >= 97 && code <= 122) || code === 95;
    const digit = code >= 48 && code <= 57;
    if (index === 0 ? !letter : !(letter || digit)) {
      return false;
    }
  }
  return true;
}

/**
 * Reject an unexpanded credential template. The supplied value is never copied
 * into the error. This matches the MCP server's placeholder check.
 */
export function isCredentialPlaceholder(material: string): boolean {
  if (material.length === 0) {
    return false;
  }
  if (material.includes('${')) {
    return true;
  }
  if (material.startsWith('$') && isIdent(material.slice(1))) {
    return true;
  }
  if (
    material.length > 2 &&
    material.startsWith('<') &&
    material.endsWith('>') &&
    !/[\s<>]/.test(material.slice(1, -1))
  ) {
    return true;
  }
  return PLACEHOLDER_VALUES.has(material.toUpperCase());
}

/**
 * Accept a raw API key. The value must not include the Bearer scheme.
 * Returns the key, or throws before any network call.
 */
export function assertApiKey(value: string | undefined): string {
  if (value == null || value.trim().length === 0) {
    throw new OpenAnyCredentialsError(
      `OpenAny API key is missing. Set OPENANY_API_KEY or pass apiKey to createOpenAny(). Create a key at ${KEY_PAGE_URL}. Do not paste the key into chat.`,
    );
  }
  if (/\s/.test(value) || value.toLowerCase().startsWith('bearer')) {
    throw new OpenAnyCredentialsError(
      'OpenAny apiKey must be the raw key, without a Bearer prefix or whitespace. The adapter sets Authorization itself.',
    );
  }
  if (isCredentialPlaceholder(value.trim())) {
    throw new OpenAnyCredentialsError(
      `OpenAny API key is an unexpanded placeholder. Set OPENANY_API_KEY in the environment or secret settings and restart the process. Do not paste the key into chat. Create a key at ${KEY_PAGE_URL}.`,
    );
  }
  return value;
}

export function resolveApiKey(explicit: string | undefined): string {
  return assertApiKey(explicit ?? process.env.OPENANY_API_KEY);
}
