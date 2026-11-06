# OpenAny MCP Listing Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 本方案建议由当前会话内的实施者顺序完成。

**Goal:** 为现有托管 MCP 增加操作指引、工具行为标注、英文文档路由及一致的发布版本。

**Architecture:** 沿用现有 mcpserver Handler 和 Go MCP SDK，轻量入口继续使用三个工具。文档通过现有 docs 包嵌入，版本常量供轻量和完整视图共同引用。

**Tech Stack:** 现有 Go、modelcontextprotocol/go-sdk、embed、net/http/httptest；不新增依赖。

**Spec:** ../specs/2026-11-06-mcp-listing-readiness-design.md

## Global Constraints

- 源码仓库继续私有，业务入口保持 `https://mcp.openany.si/mcp-lite`，业务流程保持 search_apis → get_api → run_api。
- 无需新增业务工具、数据库表、依赖或后台服务。
- 操作指引只用于轻量入口；候选发布版本为 0.1.1，各视图共用 serverVersion。
- /docs/mcp 保留中文；/docs/mcp/en 提供已有 docs/listing/connect.en.md。
- 文档沿用 LiteEnabled、Host、Origin、GET/HEAD、405、404 和目录未就绪时可读的合同。
- 业务鉴权、计费、执行及不自动重试合同沿用现有实现；未获得具体任务及预算授权时不执行真实收费 API。

## Review Focus

- 目录尚未就绪：两种文档仍可公开读取；由任务 2 的启动前文档测试覆盖。
- LiteEnabled=false 或近似路径：文档不被意外开放；由任务 2 的关闭和路径测试覆盖。
- 非允许 Host 或携带未允许 Origin：英文路径不得绕过原边界；由任务 2 的边界测试覆盖。
- run_api 路由到有副作用的 API：不得误标只读或幂等；由任务 1 的工具元数据测试覆盖，既有调用测试覆盖实际权限隔离。
- 同一镜像不同视图或发布材料版本不一致：由任务 1 的初始化测试、任务 3 的元数据核对和线上验收覆盖。

## 文件映射

| 文件 | 责任 |
| --- | --- |
| internal/mcpserver/lite.go | 设置轻量 Instructions、三工具 Annotations，引用共享版本 |
| internal/mcpserver/server.go | 定义共享版本，扩展现有文档路由 |
| internal/mcpserver/metadata_test.go | 新增初始化、版本和行为标注的 HTTP 验证 |
| docs/embed.go | 嵌入现有英文指南为 MCPEnglish |
| docs/mcp.md、docs/listing/connect.en.md | 正式域名、中英文互链与已发布功能说明 |
| internal/mcpserver/lite_test.go | 扩展现有公共文档测试 |
| docs/listing/server.json、docs/listing/submissions.md | 候选版本和正式英文文档 URL |
| docs/verification.md | 追加本次本地与上线验证记录 |

## Task 1: 初始化说明、行为标注和统一版本

**Files:** 修改 internal/mcpserver/lite.go、internal/mcpserver/server.go；新增 internal/mcpserver/metadata_test.go。

**Interfaces:** 现有 newLiteServer() *mcp.Server 和 server(snapshot *catalog.Snapshot, scope string) *mcp.Server 保持签名；新增包内 `const serverVersion = "0.1.1"`。通过现有 MCP initialize 和 tools/list 对外提供元数据，不新增 RPC 方法。

- [ ] 在 metadata_test.go 增加 `TestLiteInitializeInstructions`：用现有 catalog.NewManagerWithDiscovery + 固定 demo 记录建立就绪目录和 httptest 服务，发送协议 2025-11-25 的 initialize，断言 instructions 包含三个工具名、input_schema、api_version、费用/授权提示、unknown 状态及英文文档 URL；说明不得声称价格锁定或超时后可以安全重试。
- [ ] 增加 `TestLiteToolAnnotations`：通过真实 HTTP tools/list 断言工具名集合严格为三项，Annotations 与设计表一致；保留输入 Schema，并继续满足既有 10 KiB 工具列表上限。读取场景不使用 destructiveHint 作判断。
- [ ] 增加 `TestMCPViewsReportSameReleaseVersion`：对 /mcp-lite、/mcp、/mcp/demo 初始化，断言三处 serverInfo.version 均为 0.1.1，名称仍分别为 OpenAny Lite 和 OpenAny；完整入口不返回轻量三工具指引。
- [ ] 运行 `go test ./internal/mcpserver -run 'TestLiteInitializeInstructions|TestLiteToolAnnotations|TestMCPViewsReportSameReleaseVersion' -count=1`，确认缺少元数据/版本的断言失败，再实施。
- [ ] 在 server.go 定义 serverVersion；两个 NewServer 调用引用该常量。在 lite.go 按设计文本设置 Instructions，按设计表设置三个工具的 Annotations；不改执行分支或工具参数定义。可在原 definitions 结构增加 annotations 字段，避免按名称另建注册层。
- [ ] 重跑上述命令及 `go test ./internal/mcpserver -count=1`；应全部通过，已有凭证隔离和请求等价性断言仍通过。
- [ ] 审查差异；在实施分支提交本任务涉及的代码和测试，不混入既有未跟踪的收录材料。

## Task 2: 发布中英文接入文档

**Files:** 修改 docs/embed.go、docs/mcp.md、docs/listing/connect.en.md、internal/mcpserver/server.go、internal/mcpserver/lite_test.go。

**Interfaces:** docs 包保留 MCP string，新增 MCPEnglish string。原 HTTP Handler 识别 /docs/mcp 和 /docs/mcp/en，使用相同的访问与方法检查，且在目录 readiness 检查前返回文档。

- [ ] 扩展 `TestLitePublicDocsAvailableBeforeCatalog` 为双路径表测试：未 Refresh 的目录下 GET 均 200；中文包含正式 MCP 地址，英文包含三个工具名及凭证格式；Content-Type 和 nosniff 正确。HEAD 为 200 且正文为空；POST 为 405 且 Allow 为 GET, HEAD。
- [ ] 增加 `TestLitePublicDocsRespectBoundaries`：两条路径在 LiteEnabled=false 时均 404；非允许 Host、携带未允许 Origin 均 403；/docs/mcp/en/extra 为 404。
- [ ] 运行 `go test ./internal/mcpserver -run 'TestLitePublicDocs' -count=1`，确认英文路由相关断言失败。
- [ ] 使用 `//go:embed listing/connect.en.md` 声明 MCPEnglish；在已有文档分支选择对应正文。修正中文域名占位符，两种指南互链；示例保留明确占位符，实际 api、参数和版本通过实时 get_api 获取。
- [ ] 英文指南继续以公网可复现的基础流程为主；如记录新增参数，只描述此次发布源码实际支持的字段。整理末尾历史验证段落，避免把上一轮检查日期或旧 Schema 写成这次版本已经验收。
- [ ] 重跑文档测试及 `go test ./internal/mcpserver -count=1`，应全部通过；无需新增文档服务器、模板引擎或配置项。
- [ ] 将实际作为 embed 输入的英文文件一并纳入提交；检查它目前处于未跟踪的 docs/listing/ 目录，不能只提交 embed.go 导致全新检出构建失败。单独提交本任务变更。

## Task 3: 发布一致性和验收

**Files:** 修改 docs/listing/server.json、docs/listing/submissions.md、docs/verification.md；按现有方式构建 Docker 镜像，不新增部署系统。

**Interfaces:** 使用前两任务的 serverVersion、HTTP 文档和元数据；候选 Registry JSON 的 version 为 0.1.1，remote URL 不变；正式英文 URL 为 https://mcp.openany.si/docs/mcp/en。

- [ ] 同步候选 server.json 版本和提交文案的英文 URL；检查 JSON、目标 URL 和版本。按官方 Schema 校验 server.json；更新文件不等于发布 Registry。
- [ ] 依次运行 `go test ./...`、`go test -race ./...`、`go vet ./...`。数据库集成测试若因未配置专用测试库而跳过，单独记录；不为本次元数据变更操作业务数据库。
- [ ] 运行 `docker build -t openany-mcp:0.1.1 .`，确认全新构建包含两个嵌入文档。记录源提交和镜像 digest，后续环境使用同一镜像。
- [ ] 在测试环境放行 /docs/mcp/en，检查两个文档 GET/HEAD、initialize 指引、三工具列表和 Annotations。确认 /mcp 与已存在的供应商视图版本一致。
- [ ] 完成一次无 Key 的 search_apis → get_api 验收，并核对 tools/list 已公布本次镜像中的筛选参数和 output 参数。示例、价格或辅助元数据不可用时记录真实降级结果，不编造成功。
- [ ] 使用一个实际可用的 MCP 产品客户端验证连接、三工具发现和凭证配置，记录客户端名称/版本。真实 run_api 需先确定 API、输入、费用预算和授权；未执行时明确记为待验收，不用目录成功代替执行成功。
- [ ] 经单独部署授权后，按现有流程上线同一镜像并重复无收费验收；保留旧镜像及当前排空/超时设置。发现版本或 Schema 不一致时暂停目录提交，检查实际镜像、多副本和代理路由，不能只修改文案掩盖差异。
- [ ] 在 docs/verification.md 记录本地命令结果、跳过项、UTC/北京时间、线上版本、客户端验证范围和回滚镜像。审查并提交本任务的发布材料。

## 方案自查

四项要求均有落点：操作指引和行为标注在任务 1，公共文档在任务 2，发布一致性在任务 1 和 3。五类 Review Focus 均有对应测试或验收。没有要求增加业务工具、开放源码、改变鉴权或自动执行收费调用。

该方案尚未实施，测试与上线步骤是待办，不代表已经通过。建议先评审本方案，再由当前会话内的实施者按三个任务顺序完成。
