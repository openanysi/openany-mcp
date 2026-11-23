import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { realpathSync } from "node:fs";
import { createServer } from "node:http";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL, fileURLToPath } from "node:url";
import test from "node:test";
import { promisify } from "node:util";
import { openanyServerConfig } from "../extensions/index.js";

const exec = promisify(execFile);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repo = path.resolve(root, "../..");

function piBin() {
  return process.env.PI_BIN || "pi";
}

async function havePi() {
  try {
    const { stdout } = await exec(piBin(), ["--version"]);
    return stdout.trim();
  } catch {
    return "";
  }
}

async function runPi(args, options) {
  try {
    const result = await exec(piBin(), args, options);
    return { code: 0, stdout: result.stdout, stderr: result.stderr };
  } catch (error) {
    return { code: error.code ?? 1, stdout: error.stdout || "", stderr: error.stderr || "" };
  }
}

test("manifest points at the skill and extension and carries the pi-package keyword", async () => {
  const manifest = JSON.parse(await readFile(path.join(root, "package.json"), "utf8"));
  assert.equal(manifest.name, "pi-openany");
  assert.equal(manifest.version, "0.1.0");
  assert.ok(manifest.keywords.includes("pi-package"));
  assert.equal(manifest.pi.extensions[0], "./extensions/index.js");
  assert.equal(manifest.pi.skills[0], "./skills");
  assert.equal(manifest.dependencies, undefined);
  assert.equal(manifest.scripts.preinstall, undefined);
  assert.equal(manifest.scripts.install, undefined);
  assert.equal(manifest.scripts.postinstall, undefined);
  for (const relative of ["extensions/index.js", "skills/openany/SKILL.md", "README.md", "LICENSE"]) {
    const text = await readFile(path.join(root, relative), "utf8");
    assert.ok(text.length > 0, relative);
    assert.doesNotMatch(text, /\bsk-[A-Za-z0-9_-]{8,}/);
    assert.doesNotMatch(text, /^(?:export\s+)?OPENANY_API_KEY\s*=/m);
  }
  const skill = await readFile(path.join(root, "skills/openany/SKILL.md"), "utf8");
  assert.match(skill, /^---\nname: openany\n/);
  const description = skill.match(/^description: (.+)$/m)[1];
  assert.ok(description.length <= 1024);
  assert.match(skill, /\/skill:openany/);
  assert.match(skill, /https:\/\/mcp\.openany\.si/);
  assert.match(skill, /4,400\+ APIs/);
  assert.match(skill, /168\+ AI models/);
  assert.doesNotMatch(skill, /\$\{OPENANY_API_KEY\}/);
  assert.doesNotMatch(skill, /3,200/);
});

test("the extension omits Authorization unless OPENANY_API_KEY is non-empty", () => {
  const bare = openanyServerConfig({});
  assert.equal(bare.url, "https://mcp.openany.si");
  assert.equal(bare.type, "streamable-http");
  assert.equal(bare.headers, undefined);
  assert.equal(openanyServerConfig({ OPENANY_API_KEY: "   " }).headers, undefined);
  const authed = openanyServerConfig({ OPENANY_API_KEY: " probe-token " });
  assert.deepEqual(authed.headers, { Authorization: "Bearer probe-token" });
  const seen = [];
  const pi = {
    registerMcpServer(name, config) {
      seen.push({ name, config });
    },
  };
  return import("../extensions/index.js").then((mod) => {
    const previous = process.env.OPENANY_API_KEY;
    delete process.env.OPENANY_API_KEY;
    try {
      mod.default(pi);
    } finally {
      if (previous === undefined) delete process.env.OPENANY_API_KEY;
      else process.env.OPENANY_API_KEY = previous;
    }
    assert.equal(seen.length, 1);
    assert.equal(seen[0].name, "openany");
    assert.equal(seen[0].config.headers, undefined);
  });
});

test("npm pack includes the extension and skill and no secret", async () => {
  const { stdout } = await exec("npm", ["pack", "--dry-run", "--json"], { cwd: root });
  const [packed] = JSON.parse(stdout);
  const names = packed.files.map((file) => file.path);
  for (const expected of [
    "package.json",
    "extensions/index.js",
    "skills/openany/SKILL.md",
    "README.md",
    "LICENSE",
  ]) {
    assert.ok(names.includes(expected), expected);
  }
  assert.equal(names.some((name) => name.includes("node_modules")), false);
  const bundled = await readFile(path.join(root, "extensions/index.js"), "utf8");
  assert.doesNotMatch(bundled, /Bearer [A-Za-z0-9._~+/-]{16,}/);
});

async function loadedPackage(agentDir, work, env) {
  const entry = path.resolve(path.dirname(realpathSync(piBin())), "../index.js");
  const { DefaultResourceLoader } = await import(pathToFileURL(entry).href);
  const loader = new DefaultResourceLoader({ cwd: work, agentDir });
  await loader.reload();
  return loader;
}

test("Pi CLI installs and removes the local package and the packed tarball", async () => {
  const version = await havePi();
  if (!version) {
    return;
  }
  const agentDir = await mkdtemp(path.join(tmpdir(), "pi-openany-"));
  const work = await mkdtemp(path.join(tmpdir(), "pi-openany-work-"));
  const env = { ...process.env, PI_CODING_AGENT_DIR: agentDir, HOME: agentDir };
  delete env.OPENANY_API_KEY;
  try {
    await exec(piBin(), ["install", root], { cwd: work, env });
    const listed = await exec(piBin(), ["list"], { cwd: work, env });
    assert.match(listed.stdout, new RegExp(root.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
    const local = await loadedPackage(agentDir, work, env);
    assert.ok(local.skills.some((skill) => skill.name === "openany"));
    assert.equal(local.extensionsResult.runtime.mcpServers.list()[0].name, "openany");
    assert.equal(local.extensionsResult.runtime.mcpServers.list()[0].config.headers, undefined);
    assert.equal(local.extensionsResult.runtime.mcpServers.list()[0].config.url, "https://mcp.openany.si");
    await exec(piBin(), ["remove", root], { cwd: work, env });
    const removed = await exec(piBin(), ["list"], { cwd: work, env });
    assert.doesNotMatch(removed.stdout, /plugins\/pi/);

    const packed = await exec("npm", ["pack", "--json"], { cwd: root });
    const [info] = JSON.parse(packed.stdout);
    const tarball = path.join(root, info.filename);
    const extracted = path.join(work, "extracted");
    try {
      await exec("tar", ["-xzf", tarball, "-C", work]);
      const packageDir = path.join(work, "package");
      await exec(piBin(), ["install", packageDir], { cwd: work, env });
      const fromTarball = await exec(piBin(), ["list"], { cwd: work, env });
      assert.match(fromTarball.stdout, /package/);
      const packedLoad = await loadedPackage(agentDir, work, env);
      assert.ok(packedLoad.skills.some((skill) => skill.name === "openany" && skill.filePath.startsWith(`${packageDir}${path.sep}`)));
      const registered = packedLoad.extensionsResult.runtime.mcpServers.list()[0];
      assert.equal(registered.config.headers, undefined);
      assert.equal(registered.config.url, "https://mcp.openany.si");
      assert.ok(registered.extensionPath.startsWith(`${packageDir}${path.sep}`));
      await exec(piBin(), ["remove", packageDir], { cwd: work, env });
      const after = await exec(piBin(), ["list"], { cwd: work, env });
      assert.doesNotMatch(after.stdout, new RegExp(packageDir.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
    } finally {
      await rm(tarball, { force: true });
      await rm(extracted, { recursive: true, force: true });
    }
  } finally {
    await rm(agentDir, { recursive: true, force: true });
    await rm(work, { recursive: true, force: true });
  }
});

test("Pi MCP list connects without a key and sends the bearer only when the env var is set", async () => {
  const version = await havePi();
  if (!version) return;
  const hits = [];
  const server = createServer((request, response) => {
    const chunks = [];
    request.on("data", (chunk) => chunks.push(chunk));
    request.on("end", () => {
      const body = Buffer.concat(chunks).toString("utf8");
      let rpc = {};
      try {
        rpc = JSON.parse(body);
      } catch {
        rpc = {};
      }
      hits.push({
        authorization: request.headers.authorization || "",
        method: rpc.method || "other",
      });
      const result = rpc.method === "tools/list"
        ? { tools: [{ name: "openany_overview", description: "counts", inputSchema: { type: "object" } }] }
        : {
            protocolVersion: "2025-06-18",
            capabilities: { tools: {} },
            serverInfo: { name: "probe", version: "0" },
          };
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id ?? 0, result }));
    });
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address();
  const agentDir = await mkdtemp(path.join(tmpdir(), "pi-openany-mcp-"));
  const work = await mkdtemp(path.join(tmpdir(), "pi-openany-mcp-work-"));
  const env = { ...process.env, PI_CODING_AGENT_DIR: agentDir, HOME: agentDir };
  delete env.OPENANY_API_KEY;
  try {
    const url = `http://127.0.0.1:${port}/mcp`;
    await exec(piBin(), ["mcp", "add", "openany-probe", "--url", url], { cwd: work, env });
    hits.length = 0;
    const open = await runPi(["mcp", "list", "--json"], { cwd: work, env });
    assert.equal(open.code, 0, open.stdout);
    assert.equal(hits.some((hit) => hit.method === "initialize"), true);
    assert.ok(hits.every((hit) => hit.authorization === ""));
    const openJson = JSON.parse(open.stdout);
    assert.equal(openJson.servers[0].state, "connected");
    assert.doesNotMatch(open.stdout, /probe-token/);
    await exec(piBin(), ["mcp", "remove", "openany-probe"], { cwd: work, env });

    await exec(
      piBin(),
      ["mcp", "add", "openany-probe", "--url", url, "--bearer-token-env-var", "OPENANY_API_KEY"],
      { cwd: work, env },
    );
    hits.length = 0;
    const missing = await runPi(["mcp", "list", "--json"], { cwd: work, env });
    assert.equal(hits.length, 0);
    assert.match(missing.stdout, /OPENANY_API_KEY/);
    assert.equal(JSON.parse(missing.stdout).servers[0].state, "failed");
    const saved = await readFile(path.join(agentDir, "mcp.json"), "utf8");
    assert.match(saved, /\$\{OPENANY_API_KEY\}/);
    assert.doesNotMatch(saved, /probe-token/);

    hits.length = 0;
    env.OPENANY_API_KEY = "probe-token";
    const authed = await runPi(["mcp", "list", "--json"], { cwd: work, env });
    assert.equal(authed.code, 0, authed.stdout);
    assert.ok(hits.some((hit) => hit.authorization === "Bearer probe-token"));
    assert.equal(JSON.parse(authed.stdout).servers[0].state, "connected");
    const savedAfter = await readFile(path.join(agentDir, "mcp.json"), "utf8");
    assert.doesNotMatch(savedAfter, /probe-token/);
  } finally {
    delete env.OPENANY_API_KEY;
    server.close();
    await rm(agentDir, { recursive: true, force: true });
    await rm(work, { recursive: true, force: true });
  }
});

test("the repository license is the package license", async () => {
  const packaged = await readFile(path.join(root, "LICENSE"), "utf8");
  const canonical = await readFile(path.join(repo, "LICENSE"), "utf8");
  assert.equal(packaged, canonical);
});
