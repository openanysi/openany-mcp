/**
 * DeepSeek Harness bundle for the hosted OpenAny MCP server.
 *
 * The module calls ctx.mcpManager.register(), the harness lifetime API that
 * mounts @deepseek-ai/dsh-mcp-client. This file does not open its own HTTP
 * client. The registration entry keeps the credential as the template
 * ${credential:OPENANY_API_KEY}; the resolved key stays in the credentials
 * provider and is not copied into this entry.
 */

export const name = 'openany'

/** Web and Desktop mount mcpManager. Other shipped profiles leave this row pending. */
export const inject = ['mcpManager', 'credentials']

export const SERVER_NAME = 'openany'
export const ENDPOINT = 'https://mcp.openany.si'
export const CREDENTIAL_REF = 'OPENANY_API_KEY'
export const AUTHORIZATION_HEADER = 'Bearer ${credential:OPENANY_API_KEY}'
export const STARTUP_TIMEOUT_MS = 30_000
export const TOOL_CALL_TIMEOUT_MS = 60_000
export const CLOSE_TIMEOUT_MS = 5_000

const noop = () => {}

/**
 * Build the manager registration. `authorized` selects the Bearer template.
 * The returned object never contains a resolved key.
 * @param {boolean} authorized
 */
export function registrationEntry(authorized) {
  if (authorized !== true && authorized !== false) {
    throw new TypeError('openany: authorized must be a boolean')
  }
  return {
    transport: 'streamable-http',
    url: ENDPOINT,
    headers: authorized ? { Authorization: AUTHORIZATION_HEADER } : {},
    startupTimeoutMs: STARTUP_TIMEOUT_MS,
    toolCallTimeoutMs: TOOL_CALL_TIMEOUT_MS,
    closeTimeoutMs: CLOSE_TIMEOUT_MS,
  }
}

/**
 * @param {unknown} ref
 */
function sameCredential(ref) {
  return ref === CREDENTIAL_REF || String(ref) === CREDENTIAL_REF
}

/**
 * Keep diagnostics free of bearer tokens when a host error quotes a header.
 * @param {unknown} error
 */
export function safeErrorMessage(error) {
  const raw = error instanceof Error && error.message ? error.message : 'registration failed'
  return raw.replace(/Bearer\s+\S+/gi, 'Bearer [redacted]').slice(0, 500)
}

/**
 * @param {{ logger?: { info?: Function, warn?: Function } }} ctx
 * @param {'info' | 'warn'} level
 * @param {string} message
 */
function log(ctx, level, message) {
  const fn = ctx.logger && ctx.logger[level]
  if (typeof fn === 'function') fn.call(ctx.logger, message)
}

/**
 * Presence only. describe() does not return the secret.
 * @param {{ credentials: { describe: (ref: string) => Promise<{ configured?: boolean } | undefined> } }} ctx
 */
export async function credentialConfigured(ctx) {
  const info = await ctx.credentials.describe(CREDENTIAL_REF)
  return info?.configured === true
}

/**
 * Register OpenAny for this plugin lifetime and refresh the header when
 * OPENANY_API_KEY appears or disappears.
 * @param {any} ctx
 */
export function apply(ctx) {
  let disposeRegistration = noop
  /** @type {boolean | null} */
  let authorized = null
  let stopped = false
  let chain = Promise.resolve()

  function sync() {
    chain = chain.then(async () => {
      if (stopped) return
      const next = await credentialConfigured(ctx)
      if (stopped || next === authorized) return
      const previous = disposeRegistration
      disposeRegistration = noop
      previous()
      if (stopped) return
      disposeRegistration = ctx.mcpManager.register(SERVER_NAME, registrationEntry(next))
      if (typeof disposeRegistration !== 'function') {
        disposeRegistration = noop
        throw new TypeError('openany: mcpManager.register did not return a disposer')
      }
      authorized = next
      log(
        ctx,
        'info',
        next
          ? 'openany: registered https://mcp.openany.si with Authorization Bearer ${credential:OPENANY_API_KEY}'
          : 'openany: registered https://mcp.openany.si without an Authorization header',
      )
    }).catch((error) => {
      log(ctx, 'warn', `openany: MCP registration failed: ${safeErrorMessage(error)}`)
    })
    return chain
  }

  ctx.on('credentials/reference-updated', (ref) => {
    if (!sameCredential(ref)) return undefined
    return sync()
  })

  ctx.effect(() => () => {
    stopped = true
    const previous = disposeRegistration
    disposeRegistration = noop
    previous()
  })

  return sync()
}
