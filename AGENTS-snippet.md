# OpenAny agent instruction snippets

## English one-liner

Read https://openany.si/AGENT.md and connect OpenAny (MCP: https://mcp.openany.si/mcp; key via OPENANY_API_KEY).

## 中文一句话

阅读 https://openany.si/AGENT.md 并接入 OpenAny（MCP：https://mcp.openany.si/mcp；通过 OPENANY_API_KEY 安全提供密钥）。

## Optional English block

Use Streamable HTTP at https://mcp.openany.si/mcp. Tool discovery through
`tools/list` is public; resource execution through `tools/call` requires an
OpenAny API key. No OAuth is available. The confirmed authentication header
is `Authorization: Bearer <OPENANY_API_KEY>`. Read the
current tool schema and price, request explicit approval for a spending limit,
and report discovery separately from execution. Use the private
`OPENANY_API_KEY` environment variable; never request or reveal a key in chat.

## 可选中文说明

使用 Streamable HTTP 连接 https://mcp.openany.si/mcp。`tools/list` 工具发现
无需密钥；通过 `tools/call` 执行资源调用需要 OpenAny API key，不使用 OAuth。
已确认认证头为 `Authorization: Bearer <OPENANY_API_KEY>`。
具体任务仍需通过获授权的实际调用验证，不能将配置完成当成调用成功。执行前检查当前工具结构和价格，
获得明确的预算授权，并区分“已发现工具”与“已成功执行”。通过私有环境变量
`OPENANY_API_KEY` 提供密钥，不在聊天中索取或展示密钥。
