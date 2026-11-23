---
name: openany
description: Discover models and APIs for your agent through OpenAny's marketplace. Use when the user wants a model, API, skill, data source, or service from OpenAny and has agreed to that connection.
---

# OpenAny

OpenAny is the marketplace for AI agents. Discover and buy the models, APIs, skills, data, and services your agent needs.

Use this skill when the user wants one of those from OpenAny. The Cursor plugin connects to the hosted MCP server at `https://mcp.openany.si`. Queries and tool inputs are sent to OpenAny. They leave this machine.

This plugin does not use OAuth. It does not run the catalog on the local machine. Do not describe it as zero-configuration or as a local-only tool.

## Key

The plugin variable is `OPENANY_API_KEY`. The header placeholder in `.mcp.json` is `Authorization: Bearer ${OPENANY_API_KEY}`. Set the value in Cursor plugin settings. Do not paste the key into chat, a prompt, a screenshot, or a file in this repository. Do not print the header.

The plugin is MIT licensed. Installing it and using the plugin is free. OpenAny models, APIs, and other listings are a separate service and can charge under that listing's current terms. Installing the plugin, connecting, and saving the key do not approve that charge.

On 2026-11-23, Cursor 3.31.15 connected with `OPENANY_API_KEY` empty and unsaved, and listed the tools below. That session did not call a tool and did not show whether a saved value expands `${OPENANY_API_KEY}`. A saved key was not tested.

A hand-written `~/.cursor/mcp.json` entry is a different file. That file uses `${env:OPENANY_API_KEY}`, which is Cursor's MCP config interpolation, not this plugin variable.

## Tools

The server lists `openany_overview`, `search_apis`, `get_api`, and `run_api`.

1. `openany_overview` returns the current catalog snapshot, including counts. Treat those counts as live. Do not memorize them or quote them as a fixed catalog size.
2. `search_apis` finds candidate models and APIs. Do not invent a name.
3. `get_api` reads the selected item, including its schema and the price and billing unit shown for that item. A displayed price is not a lock, and it is not a rule for every other listing. Listings can differ in authentication, payment, execution, and delivery.
4. `run_api` only for an item `get_api` returned, on a path that response supports, and only after the user explicitly approves that call. Pass the same `api`, the `api_version` from `get_api`, and `input` that matches `input_schema`.

Asking for that approval is usage guidance. This plugin does not add a technical block that enforces it.

`run_api` can charge and can have side effects. If its execution or charging state is unknown, do not retry it. Report the result that came back. HTTP success is not proof of the charge.
