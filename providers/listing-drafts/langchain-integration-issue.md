# Draft only: LangChain integration listing issue

**Not submitted.** Do not file this until `langchain-openany` is on PyPI from a repository LangChain's publish guide accepts. New integrations are not pull requests to `langchain-ai` code repositories. The listing path is an issue in `langchain-ai/docs`. A maintainer applies `integration-run`. Automation opens the listing pull request. Do not open a manual docs pull request.

Guide: https://docs.langchain.com/oss/python/contributing/publish-langchain

This package does not have 50,000 monthly downloads and is not featured. Request the default external YAML row only. Do not request a hosted MDX guide.

## Suggested form fields

- Display name: `ChatOpenAny`
- Language: Python
- Component: Chat model
- Package name: `langchain-openany`
- Docs URL: the provider README or docs page after publication. Prefer that page, then the GitHub repository, then PyPI.
- Short description (max 160 characters): `Chat model for OpenAny text Chat Completions. Bring your own OpenAny API key. Tool calling is not supported.`

## Capability flags to report honestly

- `stream`: yes
- `tool_calling`: no
- Structured output: no
- Image input: no

## Maintainer notes to keep in the issue body

`ChatOpenAny` posts to `https://api.openany.si/v1/chat/completions` with the caller's `OPENANY_API_KEY`. It passes the model id through. It does not call the OpenAny MCP server. Unit tests include LangChain `ChatModelUnitTests` and mocked HTTP tests. Live integration tests were not run. OpenAny is not claiming a hosted LangChain guide.
