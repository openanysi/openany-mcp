import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { describe, it } from 'node:test'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const root = join(dirname(fileURLToPath(import.meta.url)), '../../..')
const pluginUrl = pathToFileURL(join(root, 'plugins/deepseek-harness/index.js')).href
const plugin = await import(pluginUrl)

const SAMPLE_KEY = 'sk-unit-test-secret-value-should-never-leak'

function harness(options = {}) {
  const registered = []
  const logs = []
  const cleanups = []
  let configured = options.configured === true
  const ctx = {
    credentials: {
      async describe(ref) {
        assert.equal(ref, plugin.CREDENTIAL_REF)
        return {
          configured,
          source: configured ? 'file' : undefined,
          writable: true,
          masked: configured ? 'sk-unit…leak' : undefined,
        }
      },
    },
    mcpManager: {
      register(name, entry) {
        if (options.failRegister) {
          throw new Error(`mcp-manager: server "${name}" failed Authorization: Bearer ${SAMPLE_KEY}`)
        }
        const record = { name, entry, disposed: false }
        registered.push(record)
        return () => {
          record.disposed = true
        }
      },
    },
    logger: {
      info(message) { logs.push(['info', message]) },
      warn(message) { logs.push(['warn', message]) },
    },
    on(event, listener) {
      cleanups.push(['on', event, listener])
      return () => {}
    },
    effect(fn) {
      const dispose = fn()
      cleanups.push(['effect', dispose])
      return dispose
    },
  }
  return {
    ctx,
    registered,
    logs,
    cleanups,
    setConfigured(value) { configured = value },
    listener() {
      const found = cleanups.find((item) => item[0] === 'on')
      assert.ok(found)
      return found[2]
    },
    unload() {
      for (const item of cleanups) {
        if (item[0] === 'effect') item[1]()
      }
    },
  }
}

describe('registration entry', () => {
  it('omits Authorization when the credential is absent', () => {
    const entry = plugin.registrationEntry(false)
    assert.equal(entry.transport, 'streamable-http')
    assert.equal(entry.url, 'https://mcp.openany.si')
    assert.deepEqual(entry.headers, {})
    assert.equal(entry.startupTimeoutMs, 30_000)
    assert.equal(entry.toolCallTimeoutMs, 60_000)
    assert.equal(entry.closeTimeoutMs, 5_000)
    assert.equal(entry.url.endsWith('/mcp'), false)
    assert.equal(entry.url.includes('.si/mcp'), false)
  })

  it('stores the credential template and not a resolved key', () => {
    const entry = plugin.registrationEntry(true)
    assert.deepEqual(entry.headers, {
      Authorization: 'Bearer ${credential:OPENANY_API_KEY}',
    })
    assert.equal(JSON.stringify(entry).includes(SAMPLE_KEY), false)
  })
})

describe('apply', () => {
  it('registers without a header when describe reports the key absent', async () => {
    const host = harness({ configured: false })
    await plugin.apply(host.ctx)
    assert.equal(host.registered.length, 1)
    assert.equal(host.registered[0].name, 'openany')
    assert.deepEqual(host.registered[0].entry.headers, {})
    assert.equal(host.logs.some(([, message]) => message.includes('without an Authorization header')), true)
    assert.equal(JSON.stringify(host.logs).includes(SAMPLE_KEY), false)
  })

  it('registers the template when the key is configured and keeps the secret out of the entry and logs', async () => {
    const host = harness({ configured: true })
    await plugin.apply(host.ctx)
    assert.deepEqual(host.registered[0].entry.headers, {
      Authorization: 'Bearer ${credential:OPENANY_API_KEY}',
    })
    const dumped = JSON.stringify({ entry: host.registered[0].entry, logs: host.logs })
    assert.equal(dumped.includes(SAMPLE_KEY), false)
    assert.equal(dumped.includes('sk-unit'), false)
  })

  it('swaps the registration when the credential appears and disappears', async () => {
    const host = harness({ configured: false })
    await plugin.apply(host.ctx)
    assert.equal(host.registered[0].disposed, false)

    host.setConfigured(true)
    await host.listener()(plugin.CREDENTIAL_REF)
    assert.equal(host.registered[0].disposed, true)
    assert.equal(host.registered[1].entry.headers.Authorization, 'Bearer ${credential:OPENANY_API_KEY}')

    host.setConfigured(false)
    await host.listener()({ toString() { return plugin.CREDENTIAL_REF } })
    assert.equal(host.registered[1].disposed, true)
    assert.deepEqual(host.registered[2].entry.headers, {})
  })

  it('ignores updates for a different credential', async () => {
    const host = harness({ configured: false })
    await plugin.apply(host.ctx)
    await host.listener()('DEEPSEEK_API_KEY')
    assert.equal(host.registered.length, 1)
    assert.equal(host.registered[0].disposed, false)
  })

  it('disposes the registration when the plugin effect unloads', async () => {
    const host = harness({ configured: false })
    await plugin.apply(host.ctx)
    host.unload()
    assert.equal(host.registered[0].disposed, true)
  })

  it('logs a redacted warning when register throws a bearer token', async () => {
    const host = harness({ configured: true, failRegister: true })
    await plugin.apply(host.ctx)
    const warnings = host.logs.filter(([level]) => level === 'warn').map(([, message]) => message)
    assert.equal(warnings.length, 1)
    assert.match(warnings[0], /Bearer \[redacted\]/)
    assert.equal(warnings[0].includes(SAMPLE_KEY), false)
    assert.equal(host.registered.length, 0)
  })
})

describe('package layout', () => {
  it('declares a root bundle whose patch and entry exist', () => {
    const manifest = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8'))
    assert.equal(manifest.name, 'dsh-openany')
    assert.equal(manifest.dsh.bundle.patch, './cordis.patch.yml')
    assert.equal(manifest.main, './plugins/deepseek-harness/index.js')
    assert.equal(manifest.engines.dsh, '>=0.2.2-rc.2 <0.3.0-0')
    assert.equal(manifest.engines.node, '>=24.9.0')
    const patch = readFileSync(join(root, manifest.dsh.bundle.patch), 'utf8')
    assert.match(patch, /name: dsh-openany/)
    assert.match(patch, /id: openany/)
    assert.equal(patch.includes('https://mcp.openany.si/mcp'), false)
    const source = readFileSync(join(root, 'plugins/deepseek-harness/index.js'), 'utf8')
    assert.equal(source.includes(SAMPLE_KEY), false)
    assert.match(source, /mcpManager\.register/)
    assert.equal(source.includes("from '@modelcontextprotocol"), false)
    for (const relative of manifest.files) {
      readFileSync(join(root, relative))
    }
  })

  it('packs the bundle files and leaves the Go tree out', async () => {
    const { stdout } = await execFileAsync('npm', ['pack', '--dry-run', '--json'], {
      cwd: root,
      env: { ...process.env, npm_config_ignore_scripts: 'true' },
    })
    const [packed] = JSON.parse(stdout)
    const names = packed.files.map((file) => file.path)
    for (const required of [
      'package.json',
      'cordis.patch.yml',
      'plugins/deepseek-harness/index.js',
      'plugins/deepseek-harness/DETAIL.md',
      'locale/en.json',
      'locale/zh.json',
    ]) {
      assert.equal(names.includes(required), true, required)
    }
    for (const banned of [
      'go.mod',
      'cmd/openany-mcp/main.go',
      'internal/mcpserver/server.go',
      '.env',
      'plugins/deepseek-harness/test/unit.test.js',
      'plugins/deepseek-harness/test/install.mjs',
      'plugins/deepseek-harness/test/review.mjs',
    ]) {
      assert.equal(names.includes(banned), false, banned)
    }
    assert.equal(names.some((name) => name.startsWith('providers/')), false)
  })

  it('ships the saved marketplace introduction through the DSH display fields', () => {
    const intro = 'The business-to-agent marketplace. One MCP endpoint for 3,200+ APIs across social, SEO/search, company/leads, finance and legal, plus 168+ AI models for chat, reasoning, image, video and audio—including GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo and Kling. Browse free; pay per call.'
    const manifest = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8'))
    const english = JSON.parse(readFileSync(join(root, 'locale/en.json'), 'utf8'))
    const chinese = JSON.parse(readFileSync(join(root, 'locale/zh.json'), 'utf8'))
    const detail = readFileSync(join(root, 'plugins/deepseek-harness/DETAIL.md'), 'utf8')
    const readme = readFileSync(join(root, 'plugins/deepseek-harness/README.md'), 'utf8')
    assert.equal(manifest.description, intro)
    assert.equal(english.meta.description, intro)
    assert.equal(english.meta.detail, 'plugins/deepseek-harness/DETAIL.md')
    assert.equal(manifest.icon, './plugins/deepseek-harness/icon.svg')
    assert.equal(manifest.exports['./locale/*.json'], './locale/*.json')
    assert.equal(manifest.exports['./icon'], './plugins/deepseek-harness/icon.svg')
    assert.equal(detail.includes(intro), true)
    assert.equal(readme.startsWith('# OpenAny for DeepSeek Harness\n\n' + intro + '\n'), true)
    assert.equal(chinese.meta.description.startsWith('The business-to-agent marketplace'), true)
    assert.equal(chinese.meta.description.includes('3,200+'), true)
    assert.equal(chinese.meta.description.includes('168+'), true)
    assert.equal(chinese.meta.description.includes('crypto'), false)
    assert.equal(chinese.meta.description.includes('加密'), false)
    for (const name of ['GPT', 'Claude', 'Gemini', 'DeepSeek', 'Qwen', 'Midjourney', 'Veo', 'Kling']) {
      assert.equal(chinese.meta.description.includes(name), true, name)
    }
  })

  it('does not tell readers the current default branch lacks the bundle', () => {
    const readme = readFileSync(join(root, 'plugins/deepseek-harness/README.md'), 'utf8')
    const rootReadme = readFileSync(join(root, 'README.md'), 'utf8')
    const bare = /github:openanysi\/openany-mcp(?!#)/
    assert.equal(bare.test(readme), true)
    assert.equal(readme.includes('Until this change is merged'), false)
    assert.equal(readme.includes('does not install this bundle'), false)
    assert.match(readme, /c5006a83f5ce10d1d24b7ec132fb1825683adfe9/)
    assert.match(readme, /675f24c553580756739164ea88338f5794373b92/)
    assert.equal([...readme.matchAll(/```sh\n([\s\S]*?)```/g)].some((match) => bare.test(match[1]) && !match[1].includes('#')) , false)
    assert.equal([...rootReadme.matchAll(/```sh\n([\s\S]*?)```/g)].some((match) => match[1].includes('github:openanysi/openany-mcp') && !match[1].includes('#')), false)
  })
})
