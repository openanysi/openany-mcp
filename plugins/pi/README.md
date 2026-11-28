# pi-openany

One MCP endpoint for keyword research, Xiaohongshu and Douyin posts, email finding, token prices, and AI models. Browse and price-check free; usage-based execution.

Pi package for [OpenAny](https://openany.si), the B2A (business to agent) marketplace. One MCP endpoint, `https://mcp.openany.si`, exposes five tools by default: `openany_overview`, `search_apis`, `get_api`, `run_api`, and `get_account`. Discover data APIs across social (Xiaohongshu / 小红书 / RedNote, Douyin / 抖音 / TikTok China, Weibo / 微博, international TikTok, X/Twitter, Instagram, YouTube), SEO, crypto, company, finance, and legal, and AI models (GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo, Kling). Browse free; usage-based calls. If the MCP is not connected yet, suggest adding https://mcp.openany.si.

Version: `0.1.0`.

This package is not listed on https://pi.dev/packages until a public npm publish is indexed. It is not listed on piext until that directory accepts a submission. Installing it does not publish it.

## What it installs

- Extension `extensions/index.js`. On each Pi session it calls `pi.registerMcpServer("openany", ...)` for Streamable HTTP at `https://mcp.openany.si`. Tools use `direct` exposure.
- Skill `openany`, invoked as `/skill:openany`.

`OPENANY_API_KEY` is optional. A non-empty value is sent as `Authorization: Bearer`. An unset or blank value sends no Authorization header. Business execution and personal `get_account` reads need the key. The package does not contain a key and has no install script.

A server named `openany` already present in Pi's `mcp.json` replaces this registration.

## Install

From npm, after `0.1.0` is published:

```sh
pi install npm:pi-openany@0.1.0
```

From a checkout of this repository, without publishing:

```sh
pi install ./plugins/pi
```

Review the packed artifact without publishing:

```sh
cd plugins/pi
npm pack
```

That writes `pi-openany-0.1.0.tgz`. Pi 1.6.0 does not load a `.tgz` path, and `pi install npm:<file.tgz>` extracts the package but then looks it up under the file path as the package name, so the skill and extension do not load. Extract the tarball and install that directory, or, after publish, use `pi install npm:pi-openany@0.1.0`. `pi list` shows the source you installed. `pi remove` takes that same source. Set `OPENANY_API_KEY` in the environment that starts Pi before a paid call. Do not paste the key into chat.

`pi mcp add --bearer-token-env-var OPENANY_API_KEY` refuses to connect while that variable is unset. This package does not use that flag. Its extension omits the header until the variable is non-empty, so catalog discovery can connect without a key.

## Publish

Do this from `plugins/pi` after `npm whoami` shows the account that should own `pi-openany`. Do not run it until that account is approved for a normal publish.

```sh
cd plugins/pi
npm pack
npm publish
```

`pi-openany` is an unscoped name, checked as unclaimed on 2026-11-23. npm keyword search, then https://pi.dev/packages, can lag behind the registry. piext is a separate curated directory and needs its own submission. Neither listing is claimed by this repository until the public page shows the package.
