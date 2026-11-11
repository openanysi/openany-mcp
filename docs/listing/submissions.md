# OpenAny MCP submission materials

These drafts describe the hosted service. The project team confirmed that the source repository is currently private. Use the public website and documentation for this submission. Replace marked fields, check for existing entries and open submissions, and confirm the current target rules before submitting. Disclose affiliation. No claim of open-source licensing, universal client compatibility or verified execution of the entire API catalog is included.

## Shared description

Name: OpenAny MCP

Website: https://openany.si/

Endpoint: `https://mcp.openany.si`

Transport: Streamable HTTP

Auth: API key through `Authorization: Bearer YOUR_OPENANY_KEY` for business execution; public discovery was tested without credentials.

Short description:

> Discover APIs, inspect schemas and pricing, and execute requests through a unified gateway.

Long description:

> OpenAny MCP provides three tools: search_apis discovers API candidates, get_api returns the selected API's input schema, public pricing and version, and run_api executes a request through the OpenAny gateway using the caller's own key. API execution can incur usage charges. The server does not automatically retry business calls, and a timeout after dispatch can leave execution and charging status unknown.

Existing public MCP reference: https://mcp.openany.si/docs/mcp

English reference: publish connect.en.md and insert its final public URL before applying where English documentation is required. The existing MCP reference is currently Chinese.

Affiliation statement:

> Submitted on behalf of OpenAny. This is a hosted commercial service; business API calls use the caller's OpenAny key and can incur charges.

Contact email: `team@openany.si`, published on the [OpenAny contact page](https://openany.si/contact/). Confirm the team monitors this mailbox before submitting.

## mcpservers.org

Submit through [the website form](https://mcpservers.org/submit), not a PR to wong2/awesome-mcp-servers.

| Field | Value |
| --- | --- |
| Server Name | OpenAny MCP |
| Category | Choose the closest available API integration category; do not invent an unavailable form option |
| Short Description | Use the shared short description |
| Repository, Website or Documentation | https://openany.si/ or the published English MCP guide |
| Official MCP Registry Name | Leave blank until actually registered; proposed name is si.openany/openany |
| Supports remote connections | Yes |
| Contact Email | team@openany.si |
| Submission plan | Free; do not purchase the optional paid review without authorization |

## Glama connectors

Open [Glama connectors](https://glama.ai/mcp/connectors) and use Add Server. Provide the shared endpoint, transport, auth and description, plus the published English guide. Follow the actual form if it asks for further details; a directory page alone does not establish every current form field.

After it is listed, record the real connector URL and copy the actual badge URL from that connector. Do not infer either URL from the proposed Registry name.

## punkpeye awesome remote MCP servers

[Rules](https://github.com/punkpeye/awesome-remote-mcp-servers/blob/main/CONTRIBUTING.md). Add one three-line entry alphabetically in the appropriate README category. Complete the Glama fields first; the placeholder below is intentionally not ready to submit.

```markdown
- [OpenAny](https://openany.si/) `https://mcp.openany.si`
  [![OpenAny MCP connector](GLAMA_BADGE_URL)](GLAMA_CONNECTOR_URL)
  🔑 - Discover APIs, inspect schemas and pricing, and execute requests through a unified gateway.
```

PR title: `Add OpenAny remote MCP server`

PR body:

> Adds the OpenAny hosted MCP service. Its public Streamable HTTP endpoint exposes search_apis, get_api and run_api. Execution uses the caller's OpenAny API key and can incur usage charges. Public initialize, tools/list, search_apis and get_api checks passed on November 6, 2026; authenticated business execution and individual client compatibility are separate checks. See the linked documentation and the actual Glama connector badge in the entry. Submitted on behalf of OpenAny.

Current rules require the submitting account to Star the repository. Keep the API-key marker even though initialization and discovery work without credentials. Inspect any CI disagreement before modifying the entry.

## TensorBlock awesome MCP servers

Use the [Add MCP server Issue form](https://github.com/TensorBlock/awesome-mcp-servers/issues/new?template=add-mcp-server.yml), or modify only the best matching `docs/*.md` category following neighboring entries. Do not treat generated `data/catalog.json` as the manual entry point.

Issue title: `Add OpenAny MCP — API discovery, schema inspection and execution`

Paste the shared service metadata and add:

> Connection requires no local server installation. Configure the hosted Streamable HTTP endpoint and the caller's Authorization header. The public discovery check confirmed exactly three tools. The source repository is private; this submission describes the hosted service, not an open-source package. Product-specific client validation and authenticated execution evidence should be attached when available.

If the form requires a public source repository or license and does not allow hosted-only submissions, resolve that requirement with the maintainers instead of substituting a private link or an invented license.

## Backblaze awesome agent infrastructure

[Rules](https://github.com/backblaze-labs/awesome-agent-infrastructure/blob/main/CONTRIBUTING.md). Edit `entries.yaml` only, one entry per PR. The current `categories.yaml` defines `tool-protocols-and-servers` for MCP/tool infrastructure.

The following is a field draft. Confirm any additional required fields against the current repository before turning it into a patch:

```yaml
- name: OpenAny MCP
  url: https://openany.si/
  docs_url: https://mcp.openany.si/docs/mcp
  description: Discover APIs, inspect schemas and pricing, and execute requests through a unified gateway.
  category: tool-protocols-and-servers
  last_verified: 2026-11-06
```

The verified date describes the public discovery check. Supply a verifiable maintenance signal from the last 12 months. If the target schema requires a license or source field, use its documented hosted-service representation; do not assert a code license that has not been granted.

PR title: `Add OpenAny MCP to agent tool infrastructure`

PR body:

> Adds a developer-accessible MCP service for agents to discover external APIs, inspect their contracts and public prices, and execute requests through a gateway. The public discovery endpoint was checked on November 6, 2026. Execution requires the caller's OpenAny key and can incur charges. Submitted on behalf of OpenAny; public docs and a developer access path are provided.

## Farol awesome agent services

[Rules](https://github.com/farol-team/awesome-agent-services/blob/main/contributing.md). Add the following row to Tools & Integrations, following the actual ordering of that section. The guide mentions both appending and alphabetical order; place OpenAny alphabetically and mention the choice if the maintainer requests otherwise.

```markdown
| [OpenAny MCP](https://openany.si/) | Lets agents discover APIs, inspect schemas and public pricing, and execute requests through one MCP endpoint | Hosted service / Usage-based API calls |
```

PR title: `Add OpenAny MCP to Tools & Integrations`

PR body:

> OpenAny MCP is used directly by agents: search_apis discovers candidate APIs, get_api supplies the selected contract and public price, and run_api executes the selected request. This proposal lists the agent-facing tool service, with a public Streamable HTTP endpoint and documented developer access. API execution requires the caller's OpenAny key and may incur charges. Submitted on behalf of OpenAny.

## Additional catalogs if the source is published later

For [michielhdoteth/awesome-ai-agent-tools](https://github.com/michielhdoteth/awesome-ai-agent-tools/blob/main/contributing.md), use `mcps/catalog.json`. Copy an existing entry from the live catalog and fill its exact schema. Candidate values are id `openany-mcp`, name `OpenAny MCP`, and the shared description; choose an existing category. Add a verified public GitHub URL and a real supported connection command/configuration, then run `node scripts/validate-catalogs.js` in that repository. Do not invent Stars, license, or an npm install command for this Go service.

For [junminhong/awesome-agent-skills](https://github.com/junminhong/awesome-agent-skills/blob/main/CONTRIBUTING.md), a future MCP listing can use Tooling & Integrations and `Type: MCP`. Both READMEs must have identical project name, source URL, type and documented platform names. Disclose affiliation and use Verified commits. A hosted URL alone does not satisfy its public source and license requirements.

For Skill listings, reuse the public OpenAny workflow only after its repository, license, actual testing and required usage evidence are ready. VoltAgent's candidate short description is: `Discover APIs, inspect pricing, and execute authorized requests.` Do not submit it as a newly created, proven community Skill.

## Official MCP Registry

The included server.json is `si.openany/openany` version `0.1.2`. The remote URL is `https://mcp.openany.si` (Streamable HTTP, the lite server). The Authorization header is optional. Browsing does not need a key. `run_api` does.

Follow the official [remote server guide](https://modelcontextprotocol.io/registry/remote-servers) and [authentication guide](https://modelcontextprotocol.io/registry/authentication) to verify `openany.si` through DNS or HTTP ownership proof. Keep publisher signing keys private. This ownership verification is independent of end-user API-key authentication; it does not add OAuth to the MCP service.

Once ownership has been verified and the final metadata reviewed, copy the candidate file to `server.json` in the publisher working directory, then use the official `mcp-publisher publish` command. Confirm the exact name and version in the [Registry API](https://registry.modelcontextprotocol.io/v0.1/servers?search=si.openany%2Fopenany) afterward. An accepted Registry record does not automatically submit or guarantee inclusion in any Awesome list.
