/**
 * Adversarial checks against @deepseek-ai/dsh@0.2.2-rc.5.
 *
 * Uses a synthetic credential and local HTTP servers only. It does not call
 * https://mcp.openany.si, run_api, or a model API. It does not print the
 * synthetic value.
 *
 * DSH_PREFIX defaults to /tmp/dsh-rc5, created by `npm install --prefix
 * /tmp/dsh-rc5 --ignore-scripts @deepseek-ai/dsh@0.2.2-rc.5`.
 */
import { createServer } from 'node:http'
import { randomBytes } from 'node:crypto'
import { mkdir, mkdtemp, readFile, rm, writeFile, chmod } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const DSH = process.env.DSH_PREFIX ?? '/tmp/dsh-rc5'
const secret = `synth-dsh-review-${randomBytes(8).toString('hex')}`
const otherSecret = `synth-dsh-other-${randomBytes(8).toString('hex')}`
const findings = []

function redact(value) {
  return String(value).split(secret).join('[redacted]').split(otherSecret).join('[redacted]')
}

function pass(name, detail) {
  findings.push({ name, ok: true, detail })
  console.log(`PASS ${name}${detail ? ` ${redact(detail)}` : ''}`)
}

function fail(name, detail) {
  findings.push({ name, ok: false, detail: redact(detail) })
  console.error(`FAIL ${name} ${redact(detail)}`)
}

async function load(specifier) {
  return import(pathToFileURL(join(DSH, 'node_modules', specifier)).href)
}

function listen(server) {
  return new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () => resolve(server.address().port))
  })
}

function json(res, status, body, headers = {}) {
  const payload = JSON.stringify(body)
  res.writeHead(status, {
    'content-type': 'application/json',
    'content-length': Buffer.byteLength(payload),
    ...headers,
  })
  res.end(payload)
}

async function readBody(req) {
  const chunks = []
  for await (const chunk of req) chunks.push(chunk)
  const text = Buffer.concat(chunks).toString('utf8')
  try {
    return JSON.parse(text)
  } catch {
    return {}
  }
}

function mcpHandler(record) {
  return async (req, res) => {
    const message = await readBody(req)
    const auth = req.headers.authorization ?? ''
    record.hits.push({
      method: message.method ?? req.method,
      hasAuth: auth.length > 0,
      authMatches: auth === `Bearer ${secret}`,
      authIsOther: auth === `Bearer ${otherSecret}`,
    })
    if (req.method === 'DELETE') {
      record.deletes += 1
      if (record.stallClose) {
        req.on('close', () => { record.closeAborted = true })
        return
      }
      res.writeHead(204)
      res.end()
      return
    }
    if (message.method === 'initialize') {
      json(res, 200, {
        jsonrpc: '2.0',
        id: message.id,
        result: {
          protocolVersion: '2025-11-25',
          capabilities: { tools: {} },
          serverInfo: { name: 'review-mock', version: '0' },
        },
      }, { 'mcp-session-id': 'review-session' })
      return
    }
    if (message.method === 'tools/list') {
      json(res, 200, {
        jsonrpc: '2.0',
        id: message.id,
        result: {
          tools: [
            { name: 'openany_overview', description: 'counts', inputSchema: { type: 'object' } },
            { name: 'search_apis', description: 'search', inputSchema: { type: 'object' } },
            { name: 'get_api', description: 'schema', inputSchema: { type: 'object' } },
            { name: 'run_api', description: 'paid', inputSchema: { type: 'object' } },
          ],
        },
      })
      return
    }
    if (message.method === 'tools/call') {
      record.calls += 1
      json(res, 200, {
        jsonrpc: '2.0',
        id: message.id,
        result: { isError: true, content: [{ type: 'text', text: 'local mock error' }] },
      })
      return
    }
    if (message.id === undefined) {
      res.writeHead(202)
      res.end()
      return
    }
    json(res, 200, { jsonrpc: '2.0', id: message.id, result: {} })
  }
}

async function withContext(setup) {
  const { Context } = await load('@deepseek-ai/cordis/lib/index.js')
  const ctx = new Context()
  const logs = []
  ctx.logger.exporter({
    export(message) {
      logs.push(redact(JSON.stringify(message)))
    },
  })
  const fibers = []
  const api = {
    ctx,
    logs,
    plugin(plugin, config) {
      const fiber = ctx.plugin(plugin, config)
      fibers.push(fiber)
      return fiber
    },
    async stop() {
      for (const fiber of fibers.reverse()) {
        try { await fiber.dispose() } catch { /* already disposed */ }
      }
    },
  }
  try {
    return await setup(api)
  } finally {
    await api.stop()
  }
}

function toolsStub(seen) {
  return {
    name: 'tools-stub',
    apply(context) {
      context.provide('tools', {
        register(definition) {
          seen.push(definition)
          return () => {}
        },
      })
    },
  }
}

async function waitFor(predicate, timeoutMs, label) {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    const value = await predicate()
    if (value) return value
    await new Promise((resolve) => setTimeout(resolve, 50))
  }
  throw new Error(`timeout: ${label}`)
}

async function credentialPrecedence() {
  const { createLaunchEnvironmentSnapshot } = await load('@deepseek-ai/dsh-launch-environment/lib/index.js')
  const credentialsModule = await load('@deepseek-ai/dsh-credentials-local/lib/index.js')
  const dir = await mkdtemp(join(tmpdir(), 'openany-cred-'))
  await chmod(dir, 0o700)
  const credFile = join(dir, '.credentials.yaml')
  await writeFile(credFile, `version: 1\nrefs:\n  OPENANY_API_KEY: ${secret}\n`, { mode: 0o600 })
  const snapshot = createLaunchEnvironmentSnapshot([
    { source: 'process', values: { OPENANY_API_KEY: otherSecret } },
    { source: 'project-env', values: { OPENANY_API_KEY: 'project-should-lose' } },
    { source: 'user-env', values: { OPENANY_API_KEY: 'user-should-lose' } },
  ])
  try {
    await withContext(async ({ plugin, ctx }) => {
      plugin({
        name: 'launch-env',
        apply(context) {
          context.provide('launchEnvironment', snapshot)
        },
      })
      const fiber = plugin(credentialsModule.default, { path: credFile, dshHome: dir, watch: false })
      await fiber
      const ref = 'OPENANY_API_KEY'
      const described = await ctx.credentials.describe(ref)
      const resolved = await ctx.credentials.resolve(ref)
      if (described.source !== 'file' || described.configured !== true) {
        fail('credential-file-beats-env', `source ${described.source} configured ${described.configured}`)
      } else if (resolved.value !== secret || resolved.source !== 'file') {
        fail('credential-file-beats-env', 'resolved value did not come from the file')
      } else if (JSON.stringify(described).includes(secret)) {
        fail('credential-describe-hides-value', 'describe() included the raw value')
      } else {
        pass('credential-file-beats-env', `source ${described.source}`)
        pass('credential-describe-hides-value', 'describe has no raw value')
      }
    })
    await writeFile(credFile, 'version: 1\nrefs: {}\n', { mode: 0o600 })
    await withContext(async ({ plugin, ctx }) => {
      plugin({
        name: 'launch-env',
        apply(context) {
          context.provide('launchEnvironment', snapshot)
        },
      })
      await plugin(credentialsModule.default, { path: credFile, dshHome: dir, watch: false })
      const resolved = await ctx.credentials.resolve('OPENANY_API_KEY')
      if (resolved?.source === 'env' && resolved.value === otherSecret) {
        pass('credential-env-when-file-absent', 'source env')
      } else {
        fail('credential-env-when-file-absent', `source ${resolved?.source}`)
      }
    })
  } finally {
    await rm(dir, { recursive: true, force: true })
  }
}

async function headerAndRedirect() {
  const record = { hits: [], deletes: 0, calls: 0, stallClose: false, closeAborted: false }
  const otherHits = []
  const local = createServer(mcpHandler(record))
  const other = createServer(async (req, res) => {
    const auth = req.headers.authorization ?? ''
    otherHits.push({
      hasAuth: auth.length > 0,
      authMatches: auth === `Bearer ${secret}`,
    })
    json(res, 200, { jsonrpc: '2.0', id: 1, result: {} })
  })
  const redirect = createServer((req, res) => {
    res.writeHead(302, { location: `http://127.0.0.1:${other.address().port}/steal` })
    res.end()
  })
  const localPort = await listen(local)
  await listen(other)
  const redirectPort = await listen(redirect)
  const dir = await mkdtemp(join(tmpdir(), 'openany-hdr-'))
  await chmod(dir, 0o700)
  const credFile = join(dir, '.credentials.yaml')
  await writeFile(credFile, `version: 1\nrefs:\n  OPENANY_API_KEY: ${secret}\n`, { mode: 0o600 })
  const credentialsModule = await load('@deepseek-ai/dsh-credentials-local/lib/index.js')
  const managerModule = await load('@deepseek-ai/dsh-mcp-manager/lib/index.js')
  const seen = []
  try {
    await withContext(async ({ plugin, ctx, logs }) => {
      plugin(toolsStub(seen))
      await plugin(credentialsModule.default, { path: credFile, dshHome: dir, watch: false })
      await plugin(managerModule.default, {})
      const dispose = ctx.mcpManager.register('reviewlocal', {
        transport: 'streamable-http',
        url: `http://127.0.0.1:${localPort}/mcp`,
        headers: { Authorization: 'Bearer ${credential:OPENANY_API_KEY}' },
        startupTimeoutMs: 5000,
        toolCallTimeoutMs: 2000,
        closeTimeoutMs: 1000,
      })
      const view = await waitFor(async () => {
        const found = ctx.mcpManager.list().find((item) => item.name === 'reviewlocal')
        return found?.status?.state === 'connected' ? found : undefined
      }, 8000, 'local connect')
      const authed = record.hits.filter((hit) => hit.authMatches)
      const leaked = JSON.stringify(logs).includes(secret) || JSON.stringify(ctx.mcpManager.list()).includes(secret)
      if (view.status.toolCount !== 4 || authed.length === 0) {
        fail('official-client-sends-bearer-to-local-only', `tools ${view.status.toolCount} authed ${authed.length}`)
      } else if (otherHits.length !== 0) {
        fail('official-client-sends-bearer-to-local-only', `other origin received ${otherHits.length} requests`)
      } else if (leaked) {
        fail('registration-list-and-logs-hide-key', 'secret appeared in list() or logs')
      } else {
        pass('official-client-sends-bearer-to-local-only', `toolCount ${view.status.toolCount} authedRequests ${authed.length}`)
        pass('registration-list-and-logs-hide-key', 'no raw value')
      }
      const stored = [...(ctx.mcpManager.registrations?.entries?.() ?? [])]
      const storedHeader = stored.find(([name]) => name === 'reviewlocal')?.[1]?.headers?.Authorization
      if (storedHeader === 'Bearer ${credential:OPENANY_API_KEY}') {
        pass('stored-registration-keeps-template', 'template only')
      } else if (storedHeader === undefined) {
        pass('stored-registration-not-public', 'registrations map is not readable; list() had no header')
      } else if (storedHeader.includes(secret)) {
        fail('stored-registration-keeps-template', 'stored header contained the resolved key')
      } else {
        fail('stored-registration-keeps-template', 'unexpected stored header shape')
      }
      dispose()
    })

    const redirected = await fetch(`http://127.0.0.1:${redirectPort}/mcp`, {
      method: 'POST',
      redirect: 'follow',
      headers: {
        'content-type': 'application/json',
        authorization: `Bearer ${secret}`,
      },
      body: '{}',
    })
    await redirected.arrayBuffer()
    const forwarded = otherHits.some((hit) => hit.authMatches)
    if (forwarded) {
      fail('cross-origin-redirect-forwards-authorization', 'fetch forwarded the bearer token to the other origin')
    } else if (otherHits.length === 0) {
      fail('cross-origin-redirect-forwards-authorization', 'redirect target was not requested')
    } else {
      pass('cross-origin-redirect-strips-authorization', `target requests ${otherHits.length} withAuth ${otherHits.filter((hit) => hit.hasAuth).length}`)
    }
  } finally {
    local.close()
    other.close()
    redirect.close()
    await rm(dir, { recursive: true, force: true })
  }
}

async function duplicateAndTimeouts() {
  const record = { hits: [], deletes: 0, calls: 0, stallClose: true, closeAborted: false }
  const server = createServer(mcpHandler(record))
  const port = await listen(server)
  const hang = createServer(() => {})
  const hangPort = await listen(hang)
  const credentialsModule = await load('@deepseek-ai/dsh-credentials-local/lib/index.js')
  const managerModule = await load('@deepseek-ai/dsh-mcp-manager/lib/index.js')
  const plugin = await import(pathToFileURL(join(import.meta.dirname, '../index.js')).href)
  const dir = await mkdtemp(join(tmpdir(), 'openany-dup-'))
  await chmod(dir, 0o700)
  const credFile = join(dir, '.credentials.yaml')
  await writeFile(credFile, `version: 1\nrefs:\n  OPENANY_API_KEY: ${secret}\n`, { mode: 0o600 })
  const seen = []
  try {
    await withContext(async ({ plugin: mount, ctx, logs }) => {
      mount(toolsStub(seen))
      await mount(credentialsModule.default, { path: credFile, dshHome: dir, watch: false })
      await mount(managerModule.default, {
        servers: {
          openany: {
            transport: 'streamable-http',
            url: `http://127.0.0.1:${port}/mcp`,
            headers: {},
          },
        },
      })
      await waitFor(async () => {
        const found = ctx.mcpManager.list().find((item) => item.name === 'openany')
        return found?.status?.state === 'connected' ? found : undefined
      }, 8000, 'preconfigured connect')
      const before = record.hits.length
      await plugin.apply(ctx)
      const warned = logs.some((line) => line.includes('already configured') || line.includes('MCP registration failed'))
      const authedAfter = record.hits.slice(before).some((hit) => hit.authMatches)
      if (!warned) fail('duplicate-server-name', 'plugin did not log the collision')
      else if (authedAfter) fail('duplicate-server-name', 'collision still sent the bearer token')
      else if (logs.some((line) => line.includes(secret))) fail('duplicate-server-name', 'collision log contained the key')
      else pass('duplicate-server-name', 'existing server kept, no bearer sent')
    })

    const started = Date.now()
    await withContext(async ({ plugin: mount, ctx }) => {
      mount(toolsStub([]))
      await mount(credentialsModule.default, { path: credFile, dshHome: dir, watch: false })
      await mount(managerModule.default, {})
      ctx.mcpManager.register('hang', {
        transport: 'streamable-http',
        url: `http://127.0.0.1:${hangPort}/mcp`,
        headers: {},
        startupTimeoutMs: 1000,
      })
      const view = await waitFor(async () => {
        const found = ctx.mcpManager.list().find((item) => item.name === 'hang')
        const state = found?.status?.state
        return state === 'failed' || state === 'reconnecting' ? found : undefined
      }, 8000, 'startup budget')
      const elapsed = Date.now() - started
      if (elapsed > 7000) fail('startup-budget', `left connecting for ${elapsed}ms`)
      else pass('startup-budget', `${view.status.state} after ${elapsed}ms`)
    })

    const closeStarted = Date.now()
    await withContext(async ({ plugin: mount, ctx }) => {
      mount(toolsStub([]))
      await mount(credentialsModule.default, { path: credFile, dshHome: dir, watch: false })
      await mount(managerModule.default, {})
      const dispose = ctx.mcpManager.register('slowclose', {
        transport: 'streamable-http',
        url: `http://127.0.0.1:${port}/mcp`,
        headers: {},
        startupTimeoutMs: 5000,
        closeTimeoutMs: 1000,
      })
      await waitFor(async () => {
        const found = ctx.mcpManager.list().find((item) => item.name === 'slowclose')
        return found?.status?.state === 'connected' ? found : undefined
      }, 8000, 'close target connect')
      const t0 = Date.now()
      dispose()
      let last = 'missing'
      let gone = false
      try {
        await waitFor(async () => {
          const found = ctx.mcpManager.list().find((item) => item.name === 'slowclose')
          last = found ? `${found.configured}:${found.status?.state}` : 'absent'
          if (!found) {
            gone = true
            return true
          }
          return undefined
        }, 4000, 'instance removal')
      } catch {
        gone = false
      }
      const elapsed = Date.now() - t0
      if (!gone) fail('close-budget', `still listed as ${last} after ${elapsed}ms; httpDeletes ${record.deletes}`)
      else pass('close-budget', `instance left the manager list in ${elapsed}ms; httpDeletes ${record.deletes} (rc.5 transport close aborts, it does not require DELETE)`)
    })
    if (Date.now() - closeStarted > 20000) fail('close-budget-wall', 'disposal wall clock exceeded 20s')
  } finally {
    server.close()
    hang.close()
    await rm(dir, { recursive: true, force: true })
  }
}

async function dumpConfigLeak() {
  const home = await mkdtemp(join(tmpdir(), 'openany-dump-'))
  await chmod(home, 0o700)
  await mkdir(join(home, 'empty'), { recursive: true })
  const cred = join(home, '.credentials.yaml')
  await writeFile(cred, `version: 1\nrefs:\n  OPENANY_API_KEY: ${secret}\n`, { mode: 0o600 })
  const dsh = join(DSH, 'node_modules', '.bin', 'dsh')
  const env = { ...process.env, DSH_HOME: home }
  delete env.OPENANY_API_KEY
  const repo = join(import.meta.dirname, '../../..')
  await execFileAsync(dsh, ['plugin', '--profile', 'web', 'add', repo], { env, cwd: home })
  const dump = await execFileAsync(dsh, ['--profile', 'web', '--dump-config'], { env, cwd: home })
  const text = `${dump.stdout}\n${dump.stderr}`
  if (text.includes(secret)) fail('dump-config-hides-credential-file', 'dump contained the key')
  else if (!text.includes('dsh-openany')) fail('dump-config-hides-credential-file', 'bundle layer missing')
  else pass('dump-config-hides-credential-file', 'layer present, key absent')
  await rm(home, { recursive: true, force: true })
}

async function main() {
  if (Number(process.versions.node.split('.')[0]) < 24) {
    throw new Error(`Node ${process.versions.node} is below the rc.5 runtime`)
  }
  const version = (await execFileAsync(join(DSH, 'node_modules', '.bin', 'dsh'), ['--version'])).stdout.trim()
  if (version !== '0.2.2-rc.5') throw new Error(`expected dsh 0.2.2-rc.5, found ${version}`)
  console.log(`runtime dsh ${version} node ${process.versions.node}`)
  await credentialPrecedence()
  await headerAndRedirect()
  await duplicateAndTimeouts()
  await dumpConfigLeak()
  const failed = findings.filter((item) => !item.ok)
  console.log(JSON.stringify({ passed: findings.length - failed.length, failed: failed.length, names: findings.map((item) => item.name) }, null, 2))
  process.exit(failed.length === 0 ? 0 : 1)
}

main().catch((error) => {
  console.error(redact(error && error.stack ? error.stack : error))
  process.exit(1)
})
