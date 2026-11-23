# Cursor client record, 2026-11-23

This is the local GUI record for commit `471af31347bde22eff348e9d71e619d1174777d1`. It is not a full end-to-end test. It does not include a model chat, a GUI tool call, `run_api`, a saved key, or a paid call.

## Setup

- When: 2026-11-23 10:02, Taipei time.
- Machine: Debian 13, x86_64.
- Client: Cursor 3.31.15. The version came from `cursor --version` on the official deb package. The GUI About box was not checked.
- Account: the Cursor client was signed in.
- Install path: `/home/box/.cursor/plugins/local/openany`.
- Package commit: `471af31347bde22eff348e9d71e619d1174777d1`.

## Passed

- Customize showed the Openany plugin at version 0.1.1.
- Customize showed the openany skill.
- `OPENANY_API_KEY` was shown as Optional and was empty. The key was not entered or saved.
- The MCP server showed Connected.
- The tool list was `get_api`, `openany_overview`, `search_apis`, and `run_api`.
- Disable, then enable, then Reload Window returned the server to Connected.

## Not tested

- A model chat.
- Invoking any tool from the GUI. No debug entry that calls a tool without a model was found.
- `run_api`.
- A valid key.
- A rejected key.
- Whether Cursor expands `${OPENANY_API_KEY}` or `${env:OPENANY_API_KEY}` after a value is saved.
- Uninstall.
- A full quit and restart. Reload Window is a separate action and was the only restart-like step.
- A paid call.
- Coexistence with a hand-written `~/.cursor/mcp.json`.

## Environment note

The OS keyring said it could not recognize the desktop environment. The prompt was closed without choosing a weaker encryption setting. The client continued to work after that. This note does not score that prompt as a plugin pass or failure.
