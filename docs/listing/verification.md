# OpenAny MCP 收录材料验证记录

日期：2026 年 11 月 6 日，Asia/Shanghai。

## 公网验证

目标：`https://mcp.openany.si/mcp-lite`。未使用 API Key，未调用 run_api，未创建账户、Key 或支付订单。

| 检查 | 结果 | 北京时间 |
| --- | --- | --- |
| initialize | HTTP 200；OpenAny Lite 0.1.0，协议 2025-11-25 | 14:15:17 起 |
| tools/list | HTTP 200；get_api、run_api、search_apis 三个工具 | 14:15:17 起 |
| search_apis | HTTP 200；backlinks summary 查询返回候选 | 14:15:18 起 |
| get_api | HTTP 200；候选包含 input_schema、api_version、pricing.status=available | 14:16:33 起 |
| /healthz | HTTP 200；ok=true | 14:14:37 |
| /docs/mcp | HTTP 200；中文接入指南，仍含域名占位符 | 14:14:38 |
| 官网及 Key 入口 | 官网公开可读，Key 入口跳转登录 | 本轮检查 |
| /SKILL.md | 官网已公开提供 name=openany 的 CLI 工作流 Skill | 本轮检查 |
| Registry 查重 | 官方 API 搜索 openany 返回 count=0 | 本轮检查 |
| 联系邮箱和分类 | 官网 contact 页面列出 team@openany.si；Backblaze 分类包含 tool-protocols-and-servers | 本轮检查 |

原始成功发现检查记录见 public-probe.json，含 UTC 时间、无凭证请求和实际响应。get_api 使用 search_apis 当次返回的 API 标识，未猜测名称。

初次沙箱内请求受 DNS 限制。沙箱外第一次请求曾收到 HTTP 403；随后使用 `OpenAnyListingAudit/1.0` User-Agent 的健康、文档、初始化和发现检查成功。尚未确定早先 403 的原因；不能据此宣称已排除 WAF、地区或目录机器人访问限制。提交后应查看目录自己的 CI 握手结果。

## 文案核对

- 英文指南依据公网实际工具 Schema，只说明已经发现的公共参数。
- 本地源码已有新增筛选和 output 展示参数，公网 tools/list 暂未公布；文案没有把本地实现当作已部署能力。
- 候选 Registry 版本 0.1.0 与公网 serverInfo 一致；命名空间和首发版本仍须在发布前确认。
- 未把官网 API 条目数当作 MCP 可调用工具数，也未声称所有 API 已通过执行验证。
- 未生成未知 Glama 徽章、虚构社区使用证据或声明未选定的开源许可证。

## 尚需补充的验收

1. 新用户公开注册、创建自己的 Key，以及该 Key 在 MCP 客户端中的鉴权。
2. 至少一个真实产品客户端的完整任务与脱敏 request_id；需要付费调用时使用明确的任务与预算授权。
3. 英文指南的公开部署，及线上中文指南中占位符的更新。
4. Registry 名称查重、命名空间所有权验证和实际发布；Glama connector 注册及徽章解析。
5. 各目标目录查重、最新分类/表单/schema、最终 PR 差异和目录自身 CI。

本次改动是收录准备文档和候选元数据，不改变服务行为。没有重新运行应用全量单测；公网发现验证也不替代计费调用或产品客户端验收。

## 材料静态检查

server.json 已使用官方 `2025-12-11/server.schema.json` 和项目现有的 `github.com/santhosh-tekuri/jsonschema/v6` 校验通过。该检查确认 JSON 元数据结构符合 Schema，不代表已完成命名空间认证或 Registry 发布。

交付前同时检查 JSON 可解析、Markdown 代码块成对闭合、文档内相对链接存在，并核对公网探测记录中只有 initialize、tools/list、search_apis、get_api，没有 run_api。
