# OpenAny skill for OpenClaw

This folder is an OpenClaw skill for the hosted OpenAny MCP server. It is a separate package from `plugins/openany`, which Hermes loads.

OpenAny (https://openany.si) is a marketplace for agents. One API key covers model and API calls, pay per call, no monthly fee. The skill tells an OpenClaw agent when to use that path, how to connect the lite MCP server, and the order of tool calls.

## Files

`SKILL.md` is the skill. Its front matter sets `name`, `description`, and `metadata.openclaw`. The homepage is https://openany.si. The skill requires the environment variable `OPENANY_API_KEY` and names that variable as `primaryEnv`.

## Connect

Merge the server block from `SKILL.md` into `~/.openclaw/openclaw.json` under `mcp.servers`. Keep `transport` set to `streamable-http`. The header reads `OPENANY_API_KEY` from the environment. Do not put a real key in this folder.

The URL in the skill is `https://mcp.openany.si/mcp-lite`. That server lists `search_apis`, `get_api`, and `run_api`. The manual example at `examples/openclaw.json` still points at `https://mcp.openany.si/mcp`.

## Publish

Publishing this folder to ClawHub is a separate maintainer step after it is on the default branch. Installing the skill locally does not require that publish.
