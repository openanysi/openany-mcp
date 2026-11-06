#!/bin/sh
# CONFIRMED authentication header: Authorization: Bearer <OPENANY_API_KEY>.
# Requires OPENANY_API_KEY in the private shell environment. Never hardcode a key.
# The CLI may persist the expanded header in personal settings. Do not enable shell tracing.
set -eu
: "${OPENANY_API_KEY:?Set OPENANY_API_KEY privately before running this script}"
claude mcp add --transport http openany https://mcp.openany.si/mcp --header "Authorization: Bearer $OPENANY_API_KEY"
