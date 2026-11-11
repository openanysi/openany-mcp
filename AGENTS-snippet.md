# OpenAny agent instruction snippets

## English one-liner

Read https://openany.si/AGENT.md and connect OpenAny (MCP: https://mcp.openany.si, Streamable HTTP, tools openany_overview, search_apis, get_api, run_api; optional key via OPENANY_API_KEY for run_api).

## 中文一句话

阅读 https://openany.si/AGENT.md 并接入 OpenAny（MCP：https://mcp.openany.si，Streamable HTTP，工具 openany_overview、search_apis、get_api、run_api；浏览不需要密钥，执行 run_api 时通过 OPENANY_API_KEY 安全提供密钥）。

## Optional English block

Use Streamable HTTP at https://mcp.openany.si. That URL is the lite server:
`openany_overview`, `search_apis`, `get_api`, and `run_api`. Tool discovery,
search, and pricing are public. Only `run_api` requires an OpenAny API key.
`Authorization: Bearer <OpenAny API key>` is optional. No OAuth is available.
The confirmed authentication header is `Authorization: Bearer <OPENANY_API_KEY>`.
Read the current tool schema and price, request explicit approval for a spending
limit, and report discovery separately from execution. Use the private
`OPENANY_API_KEY` environment variable when executing; never request or reveal a key in chat.

## 可选中文说明

使用 Streamable HTTP 连接 https://mcp.openany.si。这是轻量服务，工具为
`openany_overview`、`search_apis`、`get_api`、`run_api`。浏览、搜索和查看价格
无需密钥；只有 `run_api` 需要 OpenAny API key，不使用 OAuth。
`Authorization: Bearer <OpenAny API key>` 是可选请求头。
已确认认证头为 `Authorization: Bearer <OPENANY_API_KEY>`。
具体任务仍需通过获授权的实际调用验证，不能将配置完成当成调用成功。执行前检查当前工具结构和价格，
获得明确的预算授权，并区分“已发现工具”与“已成功执行”。执行时通过私有环境变量
`OPENANY_API_KEY` 提供密钥，不在聊天中索取或展示密钥。
