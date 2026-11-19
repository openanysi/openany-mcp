# OpenAny for DeepSeek Harness

OpenAny is the B2A (business to agent) marketplace. This bundle registers one remote MCP server, `https://mcp.openany.si`, with DeepSeek Harness. The server exposes `openany_overview`, `search_apis`, `get_api`, and `run_api` for 3,200+ APIs and 168+ AI models.

Searching, schema reads, and price checks are free and do not need a key. `run_api` uses your own OpenAny API key and can charge per call. This package does not call `run_api`, does not contain a key, and is not a DeepSeek product.

The Go server in this repository is unchanged. The root `package.json` is the installable bundle. `dsh plugin add github:openanysi/openany-mcp` installs that root. A subdirectory package by itself is not the install command.

## What it registers

| Field | Value |
| --- | --- |
| Package | `dsh-openany` `0.1.0` |
| Server name | `openany` |
| Transport | `streamable-http` through `@deepseek-ai/dsh-mcp-client`, mounted by `ctx.mcpManager.register()` |
| URL | `https://mcp.openany.si` |
| Startup budget | 30 seconds |
| Tool-call budget | 60 seconds |
| Close budget | 5 seconds |
| Credential reference | `OPENANY_API_KEY` |
| Header, when that reference is configured | `Authorization: Bearer ${credential:OPENANY_API_KEY}` |
| Header, when it is absent | omitted |

The header template is the string stored in the registration. The harness credentials provider resolves it. This plugin reads `describe()`, which reports whether the reference is set, and does not copy the value into the entry, a log line, or another origin.

Web and Desktop profiles mount `@deepseek-ai/dsh-mcp-manager`, so the row activates there. Headless, sdk, sdk-minimal, and acp do not mount that manager. On those profiles the row stays pending and does not open a connection. npm `latest` on 2026-11-19 is `@deepseek-ai/dsh@0.2.2-rc.1`, whose web profile also has no manager. `engines.dsh` refuses that release.

## Compatible DeepSeek Harness versions

`engines.dsh` is `>=0.2.2-rc.2 <0.3.0-0`. `engines.node` is `>=24.9.0`, matching the published CLI.

`@deepseek-ai/dsh-mcp-manager` is published from `0.2.2-rc.2` upward. The `register()` type in `0.2.2-rc.2` and `0.2.2-rc.5` accepts the same fields this plugin sets. The install and live load recorded with this change used `@deepseek-ai/dsh@0.2.2-rc.5` (the npm `next` tag on 2026-11-19) on Node 24.22.3. That run is not a test of every release inside the range, and it is not a test of the Desktop application.

Install the tested CLI when `dsh --version` prints `0.2.2-rc.1` or older:

```sh
npm install -g @deepseek-ai/dsh@0.2.2-rc.5
dsh --version
```

## Install

From a git checkout, which is the same package root a `github:` install uses:

```sh
dsh plugin --profile web add /path/to/openany-mcp
```

From the packed tarball, after `npm pack` in this repository:

```sh
dsh plugin --profile web add ./dsh-openany-0.1.0.tgz
```

From GitHub, after this commit is on the public default branch. Pin a commit. This package has no `prepare` script, so pnpm does not need an `allowBuilds` entry:

```sh
dsh plugin --profile web add github:openanysi/openany-mcp#<commit>
```

Confirm the layer without booting:

```sh
dsh --profile web --dump-config
```

The dump includes a `dsh-openany` layer and an `openany` row whose `name` is `dsh-openany`. Then start the profile:

```sh
dsh web
```

`dsh web` is `dsh --profile web`. The npm CLI refuses the reserved `desktop` profile. Installing into Desktop uses the launcher shipped inside the DeepSeek Harness Desktop app, which was not run here.

## API key

Create a key at https://openany.si/dashboard?page=api-keys. Store it as the credential reference `OPENANY_API_KEY`. Do not paste it into chat, this repository, a shell trace, or a log.

The local credentials file is `$DSH_HOME/.credentials.yaml`, usually `~/.dsh/.credentials.yaml`. The current document is versioned:

```yaml
version: 1
refs:
  OPENANY_API_KEY: YOUR_OPENANY_KEY
```

Replace `YOUR_OPENANY_KEY` with the key from the dashboard. On POSIX the file must be readable only by your user (`chmod 600`). The harness refuses a file that other users can read.

Resolution order in `@deepseek-ai/dsh-credentials-local`, from the 0.2.2-rc.5 package docs:

1. The stored file.
2. The environment that launched `dsh`.
3. The `.env` file in the directory where `dsh` was started.
4. `$DSH_HOME/.env`.

An empty value is absent. A value saved in the file outranks the launch environment until you remove that `refs` entry. The launch environment is snapshotted when `dsh` starts, so export the variable before the process starts.

With no configured reference, the plugin registers the server with no `Authorization` header. `initialize`, `tools/list`, `openany_overview`, `search_apis`, and `get_api` work. `run_api` is rejected by the OpenAny server as `openany_mcp_credentials_missing` and does not reach the gateway.

When the reference becomes configured, the plugin disposes the registration and registers again with the Bearer template. The harness client sends that header to `https://mcp.openany.si`. Removing the reference registers the server again without the header. There is no OAuth flow.

A displayed price is not a price lock. A `run_api` timeout after the server has accepted the call has an unknown charging state. Do not retry it automatically.

## Timeouts and failures

The registration sets three budgets on the official client:

- 30 seconds for template resolution, connect, and the first tool list. A failed attempt follows the client reconnect policy. After the attempt budget is exhausted the client withdraws the tools until the server is restarted from the harness.
- 60 seconds for one tool call. A timeout is reported by the client as `MCP_TIMEOUT`. The call outcome is unknown.
- 5 seconds to close the transport when the registration is removed.

If another server already uses the name `openany`, `register()` throws and this plugin logs that failure. The log line is passed through a redaction that replaces `Bearer` tokens. Rename the other server or remove it before loading this one.

## Remove

```sh
dsh plugin --profile web remove dsh-openany
```

Removal drops the bundle layer. The plugin unload runs the registration disposer, which unmounts the MCP client and withdraws its tools. The OpenAny account stays as it is. `OPENANY_API_KEY` stays in the credentials file until you delete that `refs` entry.

## Topic discovery

GitHub topic discovery reads a public repository's root `package.json`. This repository's root manifest declares `dsh.bundle.patch` as `./cordis.patch.yml`, and that file is in the tree. Remaining steps, which this change does not perform:

1. Merge the pull request through review. An open PR is not a published plugin.
2. Confirm `openanysi/openany-mcp` is public.
3. Add the GitHub topic `dsh-plugin` in the repository settings. A topic is not a DeepSeek endorsement.
4. Optional later: publish a GitHub Release `.tgz` for marketplaces that prefer a release asset. `npm publish` is not required for the git or tarball install above, and this change does not publish.

## Limits

The checked UI, model conversation, and Desktop flows were not run. The automated load test boots the published manager and client against `https://mcp.openany.si` with no API key and checks `tools/list`. It does not call `run_api`.
