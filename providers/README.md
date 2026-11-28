# OpenAny language-model packages

These packages are named client adapters for OpenAny text Chat Completions. They are not the MCP server in the repository root, and they do not change that server.

| Package | Language | Import |
| --- | --- | --- |
| `openany-ai-sdk-provider` | JavaScript / TypeScript | `createOpenAny`, `openany` |
| `langchain-openany` | Python | `ChatOpenAny` |

Both send `POST https://api.openany.si/v1/chat/completions` with `Authorization: Bearer <OPENANY_API_KEY>`. The model id is the string you pass. It is not chosen from a list baked into the package.

OpenAny is the B2A (business to agent) marketplace. One MCP endpoint, https://mcp.openany.si, is how an agent finds data APIs and AI models. These chat packages do not call that MCP endpoint and do not expose those APIs.

Neither package is published, listed, or accepted by Vercel or LangChain. Publishing and listing steps that are still open are in [PUBLISHING.md](PUBLISHING.md).

No paid OpenAny request was made while these packages were added. Tests use local mock HTTP only.
