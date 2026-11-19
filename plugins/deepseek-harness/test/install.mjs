/**
 * Fresh-package install and official-manager load for dsh-openany.
 *
 * This is the CLI and runtime test. plugins/deepseek-harness/test/unit.test.js
 * is the mock-only suite and does not start DeepSeek Harness or open a socket.
 *
 * Requires Node >= 24.9, pnpm >= 10, and network access to npm and
 * https://mcp.openany.si. It installs @deepseek-ai/dsh@0.2.2-rc.5 into a
 * temp prefix, packs this repository, and installs that tarball with
 * `dsh plugin`. It then loads the installed module under the published
 * mcp-manager and mcp-client with no API key. It does not call run_api.
 */
import { execFile } from 'node:child_process'
import { chmod, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const DSH_VERSION = '0.2.2-rc.5'
const repoRoot = join(import.meta.dirname, '../../..')

delete process.env.OPENANY_API_KEY

function fail(message) {
  console.error(message)
  process.exitCode = 1
  throw new Error(message)
}

async function run(command, args, options = {}) {
  const result = await execFileAsync(command, args, {
    cwd: options.cwd,
    env: options.env,
    timeout: options.timeout ?? 180_000,
    maxBuffer: 8 * 1024 * 1024,
  })
  return result
}

async function main() {
  const nodeMajor = Number(process.versions.node.split('.')[0])
  if (nodeMajor < 24) fail(`Node ${process.versions.node} is below the tested DSH runtime`)

  const work = await mkdtemp(join(tmpdir(), 'openany-dsh-'))
  const prefix = join(work, 'dsh')
  const home = join(work, 'home')
  const packDir = join(work, 'pack')
  const credDir = join(work, 'cred')
  await chmod(work, 0o700)
  const { mkdir } = await import('node:fs/promises')
  await mkdir(packDir, { recursive: true })

  const env = {
    ...process.env,
    npm_config_ignore_scripts: 'true',
    DSH_HOME: home,
  }
  delete env.OPENANY_API_KEY

  console.log(`installing @deepseek-ai/dsh@${DSH_VERSION}`)
  await run('npm', ['install', '--prefix', prefix, `--ignore-scripts`, `@deepseek-ai/dsh@${DSH_VERSION}`], {
    cwd: work,
    env,
    timeout: 300_000,
  })
  const dsh = join(prefix, 'node_modules', '.bin', 'dsh')
  const version = (await run(dsh, ['--version'], { cwd: work, env })).stdout.trim()
  if (version !== DSH_VERSION) fail(`dsh --version printed ${version}`)

  console.log('packing dsh-openany')
  const packedName = (await run('npm', ['pack', '--pack-destination', packDir, '--ignore-scripts'], {
    cwd: repoRoot,
    env,
  })).stdout.trim().split('\n').at(-1)
  const tarball = join(packDir, packedName)
  if (!packedName.startsWith('dsh-openany-')) fail(`unexpected pack name ${packedName}`)

  console.log('installing the tarball into a fresh web profile')
  const add = await run(dsh, ['plugin', '--profile', 'web', 'add', tarball], { cwd: work, env })
  const dump = await run(dsh, ['--profile', 'web', '--dump-config'], { cwd: work, env })
  const dumpText = `${dump.stdout}\n${dump.stderr}`
  if (!dumpText.includes('dsh-openany')) fail('dump-config did not name dsh-openany')
  if (!dumpText.includes('id: openany')) fail('dump-config did not insert the openany row')
  const profileManifest = JSON.parse(await (await import('node:fs/promises')).readFile(join(home, 'profiles', 'web', 'package.json'), 'utf8'))
  const bundles = profileManifest.dsh?.profile?.bundles ?? []
  if (!bundles.includes('dsh-openany')) fail(`profile bundles missing dsh-openany: ${bundles.join(', ')}`)
  if (bundles[0] !== '@deepseek-ai/dsh-base' || !bundles.includes('@deepseek-ai/dsh-web-app')) {
    fail(`web profile bundles were ${bundles.join(', ')}`)
  }

  const installedEntry = join(home, 'profiles', 'web', 'node_modules', 'dsh-openany', 'plugins', 'deepseek-harness', 'index.js')
  console.log('loading the installed package through the published MCP manager')
  const load = await loadInstalled({
    dshPrefix: prefix,
    installedEntry,
    credDir,
    env,
  })

  console.log('removing the bundle')
  await run(dsh, ['plugin', '--profile', 'web', 'remove', 'dsh-openany'], { cwd: work, env })
  const removed = JSON.parse(await (await import('node:fs/promises')).readFile(join(home, 'profiles', 'web', 'package.json'), 'utf8'))
  const removedBundles = removed.dsh?.profile?.bundles ?? []
  if (removedBundles.includes('dsh-openany')) fail('remove left dsh-openany in the profile')

  const directoryHome = join(work, 'directory-home')
  const directoryEnv = { ...env, DSH_HOME: directoryHome }
  console.log('installing the repository root the way a github: install resolves the package')
  await run(dsh, ['plugin', '--profile', 'web', 'add', repoRoot], { cwd: work, env: directoryEnv })
  const directoryDump = await run(dsh, ['--profile', 'web', '--dump-config'], { cwd: work, env: directoryEnv })
  if (!`${directoryDump.stdout}\n${directoryDump.stderr}`.includes('dsh-openany')) {
    fail('directory install did not produce a dsh-openany layer')
  }
  await run(dsh, ['plugin', '--profile', 'web', 'remove', 'dsh-openany'], { cwd: work, env: directoryEnv })

  const summary = {
    dsh: version,
    node: process.versions.node,
    pnpm: (await run('pnpm', ['--version'], { cwd: work, env })).stdout.trim(),
    tarball: packedName,
    profileBundlesAfterAdd: bundles,
    connectedToolCount: load.toolCount,
    toolNames: load.toolNames,
    disposed: load.disposed,
    pluginAddStderrBytes: Buffer.byteLength(add.stderr),
    pendingWithoutManager: load.pendingWithoutManager,
  }
  console.log(JSON.stringify(summary, null, 2))
  await rm(work, { recursive: true, force: true })
  // The published credentials watcher and MCP transport keep the event loop
  // alive after the profile directory is gone. The assertions above already
  // settled, so exit explicitly.
  process.exit(0)
}

async function loadInstalled({ dshPrefix, installedEntry, credDir, env }) {
  const { mkdir } = await import('node:fs/promises')
  await mkdir(credDir, { recursive: true, mode: 0o700 })
  await chmod(credDir, 0o700)
  const credFile = join(credDir, '.credentials.yaml')
  await writeFile(credFile, '', { mode: 0o600 })

  const importFrom = async (specifier) => import(pathToFileURL(join(dshPrefix, 'node_modules', specifier)).href)
  const { Context } = await importFrom('@deepseek-ai/cordis/lib/index.js')
  const credentialsModule = await importFrom('@deepseek-ai/dsh-credentials-local/lib/index.js')
  const managerModule = await importFrom('@deepseek-ai/dsh-mcp-manager/lib/index.js')
  const openany = await import(pathToFileURL(installedEntry).href)

  const ctx = new Context()
  const seen = []
  ctx.plugin({
    name: 'tools-stub',
    apply(context) {
      context.provide('tools', {
        register(definition) {
          seen.push(definition?.name ?? definition?.tool?.name ?? 'unknown')
          return () => {}
        },
      })
    },
  })
  const Credentials = credentialsModule.default
  const Manager = managerModule.default
  ctx.plugin(Credentials, { path: credFile, dshHome: credDir })
  ctx.plugin(Manager, {})
  const fiber = ctx.plugin({
    name: openany.name,
    inject: openany.inject,
    apply: openany.apply,
  })
  await fiber

  const deadline = Date.now() + 45_000
  let view
  while (Date.now() < deadline) {
    view = ctx.mcpManager.list().find((item) => item.name === 'openany')
    if (view?.status?.state === 'connected') break
    if (view?.status?.state === 'failed') {
      fail(`official client failed: ${JSON.stringify(view.status)}`)
    }
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  if (view?.status?.state !== 'connected') {
    fail(`official client did not connect: ${JSON.stringify(view ?? null)}`)
  }
  const toolCount = view.status.toolCount
  if (toolCount < 4) fail(`expected at least 4 tools, got ${toolCount}`)
  const wanted = ['openany_overview', 'search_apis', 'get_api', 'run_api']
  for (const name of wanted) {
    if (!seen.some((item) => String(item).includes(name))) fail(`tool ${name} was not registered; saw ${seen.join(', ')}`)
  }

  await fiber.dispose()
  const afterDeadline = Date.now() + 15_000
  let stillThere = true
  while (Date.now() < afterDeadline) {
    stillThere = ctx.mcpManager.list().some((item) => item.name === 'openany' && item.configured)
    if (!stillThere) break
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  if (stillThere) fail('registration remained after plugin dispose')

  const pendingCtx = new Context()
  let applied = false
  const pending = pendingCtx.plugin({
    name: openany.name,
    inject: openany.inject,
    apply(context) {
      applied = true
      return openany.apply(context)
    },
  })
  await new Promise((resolve) => setTimeout(resolve, 300))
  if (applied) fail('plugin apply ran without mcpManager')
  if (pending.state === 2) fail('plugin became active without mcpManager')
  await pending.dispose()

  return {
    toolCount,
    toolNames: wanted.filter((name) => seen.some((item) => String(item).includes(name))),
    disposed: true,
    pendingWithoutManager: true,
  }
}

main().catch((error) => {
  console.error(error && error.stack ? error.stack : error)
  process.exit(1)
})
