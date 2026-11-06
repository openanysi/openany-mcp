# OpenAny MCP 收录准备设计

日期：2026 年 11 月 6 日。状态：提案，尚未实施。对应项目：openany-mcp。

## 目标和边界

让现有托管 MCP 服务能够说明自己的调用流程、向客户端声明三工具的行为、提供正式英文接入文档，并使部署版本与对外描述一致。源码仓库继续私有，业务入口保持 `https://mcp.openany.si/mcp-lite`，业务流程保持 search_apis → get_api → run_api。

本次补充协议元数据和文档交付；业务鉴权、网关执行、计费、参数校验、api_version 校验和错误恢复继续沿用现有实现。无需新增业务工具、数据库表、依赖或后台服务。

## 路径选择

推荐沿用当前 Go embed 和 HTTP Handler，将已有英文指南嵌入同一二进制。文档会与代码一起构建和发布，代价是更新文档也要重建镜像。

另一种可行方式是在官网单独托管英文页，更新更独立，但需要跨仓库协调文档和服务版本。本次提案采用前一种方式，保留官网作为产品和账户入口。

## 初始化操作指引

在 internal/mcpserver/lite.go 的 newLiteServer 中设置 ServerOptions.Instructions，使用简短英文：

```text
Use search_apis to find an API, then get_api to inspect its current input_schema, pricing, limitations and api_version. Pass that api and api_version to run_api with input matching the schema. Execute only within the user's authorized task and spending limits, using the caller's OpenAny credentials. run_api can incur charges and business side effects. If execution_state is unknown after a timeout or disconnect, do not retry automatically; retain the request_id when available and investigate the original call. Documentation: https://mcp.openany.si/docs/mcp/en
```

这段说明只用于轻量入口。全量与供应商入口直接暴露各 API 工具，不套用轻量三工具顺序。现有 Tool.Description 仍保留各工具的具体参数及限制。

## 工具行为标注

为轻量入口的三个 mcp.Tool 设置 Annotations。字段是客户端提示，实际权限与执行限制仍由现有代码落实。

| 工具 | readOnlyHint | destructiveHint | idempotentHint | openWorldHint |
| --- | --- | --- | --- | --- |
| search_apis | true | 不设置 | false，读取场景不依赖此值 | false |
| get_api | true | 不设置 | false，读取场景不依赖此值 | false |
| run_api | false | true | false | true |

前两项只读 OpenAny 目录；run_api 可调用多种外部业务 API，因此按可能产生破坏性变更和不可幂等处理。不把“某次只查询数据”当成整个 run_api 都只读，也不对全量 API 的行为做未经证实的统一承诺。

## 中英文公共文档

| URL | 内容 |
| --- | --- |
| /docs/mcp | 继续提供中文 docs/mcp.md |
| /docs/mcp/en | 提供现有 docs/listing/connect.en.md |

docs/embed.go 新增 MCPEnglish string，使用 `//go:embed listing/connect.en.md`。复用已有英文文件，不维护第二份相同正文。中文指南中的 MCP 域名占位符替换为正式地址；两份指南增加彼此链接。

两个文档 URL 都沿用当前合同：轻量入口开启时支持 GET、HEAD；其他方法返回 405 且 Allow 为 GET, HEAD；无需 Key、无需目录就绪；仍经过 Host 和携带 Origin 时的检查。轻量入口关闭时两条文档路由都为 404。Content-Type 为 text/plain; charset=utf-8，并保留 X-Content-Type-Options: nosniff。

反向代理须放行新增英文路径。docs/listing/submissions.md 的英文参考地址在正式部署后统一改为 https://mcp.openany.si/docs/mcp/en。

## 版本和部署

建议本次补丁版本使用 0.1.1。internal/mcpserver/server.go 定义单一常量 serverVersion，轻量、全量和供应商视图的 Implementation.Version 共同引用。serverInfo.name 保持现有名称。

在同一变更中将 docs/listing/server.json 的候选版本更新为 0.1.1。该文件仍是发布材料，更新文件不触发 Registry 发布。构建和部署使用同一个经过测试的镜像，记录源提交和镜像 digest；复用现有部署方式，不新增 CI/CD 系统。

线上验收先检查文档、initialize、tools/list、search_apis、get_api。确认文档 URL、版本、三工具元数据，以及本次发布源码中已有的筛选参数与 output 参数一致。辅助元数据或价格暂不可用时，按既有降级合同展示，不伪造可用结果。

用真实 MCP 客户端完成凭证配置和三工具发现。真实 run_api 验收另行确定具体 API、输入和费用预算；未经该授权不执行收费业务调用。所有验收记录区分发现成功、认证成功和业务执行成功。

回滚使用上一镜像，同时回退嵌入文档；如果 Registry 尚未发布则暂缓发布候选元数据。沿用当前服务的排空和超时配置，避免强制中断在途调用；中断后的执行状态依旧可能未知。

## 完成条件

1. 轻量初始化返回正确指引；三个工具及其输入 Schema 保持兼容，工具列表仍满足现有大小限制。
2. 工具行为标注符合上表，客户端提示不代替权限检查。
3. 中英文 URL 在目录未就绪时可读，HEAD、405、404、Host 和 Origin 边界符合约定。
4. 三种 MCP 视图报告同一版本，候选 Registry 版本一致；现有凭证隔离与业务调用测试通过。
5. 发布验收记录明确列出已通过项、未执行的收费调用和回滚镜像。
