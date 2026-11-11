#!/bin/sh
# CONFIRMED authentication header: Authorization: Bearer <OPENANY_API_KEY>.
# The header is optional. Browsing, search, and pricing work without a key.
# run_api needs OPENANY_API_KEY in the private shell environment. Never hardcode a key.
# The CLI may persist the expanded header in personal settings. Do not enable shell tracing.
set -eu
if [ -n "${OPENANY_API_KEY:-}" ]; then
  claude mcp add --transport http openany https://mcp.openany.si --header "Authorization: Bearer $OPENANY_API_KEY"
else
  claude mcp add --transport http openany https://mcp.openany.si
fi
