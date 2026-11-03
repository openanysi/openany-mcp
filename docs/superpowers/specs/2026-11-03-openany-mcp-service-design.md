# OpenAny 通用 MCP 服务 Go 实现设计方案

日期：2026-11-03。版本：v2。实现状态：已交付 Go 服务、Python 维护脚本及本地迁移/回填；本文保留已批准设计合同。实际验证与未验证边界见 README.md 和 docs/verification.md。

命名规则修订：固定 UUID 后缀已由用户要求改为无后缀名称、重名时使用 `_2`、`_3`。存量脚本生成名称通过显式参数迁移，详见 `2026-11-03-readable-tool-names-design.md`；本文旧命名章节保留为历史基线。

## 审阅整合说明

本次两个设计审阅子代理均反馈“没有重要遗漏”。Go 专项审阅确认：动态快照与分页、持久命名、数组 query 的上线前核实、逐请求凭证隔离、取消后的执行状态不确定，以及应用和 HTTP Transport 禁止业务重放，已经形成一致且可实施的合同。子代理没有提出新增功能，本版将这些合同收敛为实施和验收基线，不把审阅结论夸大为代码或真实 API 测试通过。

| 审阅确认事项 | 本版明确决定 | 对应章节 |
|---|---|---|
| 持久名称 | 数据库为准；Python 只补 NULL；运行服务不重新计算 | 5、6 |
| 动态注册与分页 | 请求固定一代快照；cursor 绑定 hash 和视图 | 7、8、9 |
| 数组 query | 46 个参数逐项核实编码；缺少规则不能视为已验收 | 11、18、19 |
| 凭证隔离 | 只使用当前 tools/call 请求头；不保存 session Key | 12、13 |
| 取消与超时 | 已发出请求后无法确认结果时明确标记 unknown | 14、15 |
| 禁止重放 | 应用、Go Transport 及代理配置共同检查 | 14、17、19 |
| 实际接入依赖 | 导入方保留新列、公开描述检查与数组编码是上线门槛 | 6、11、18 |

本版另外明确四个不同状态，避免把名称生成数量当成可调用数量：

| 状态 | 判定 |
|---|---|
| 已命名 named | public_mcp_tool_name 非 NULL，且格式/全表唯一约束成立；不要求 active |
| 已发布 published | active、已命名，且本代目录的 schema、路由及必要传输元数据校验通过，能在 tools/list 发现 |
| 支持执行 supported | 已发布且业务响应模式在首版范围内；不代表凭证、余额或上游此刻可用 |
| 本次调用成功 succeeded | 本次网关调用返回符合合同的成功结果；不能只依据 HTTP 200 |

已知 gang stream 可以发布一个明确标注限制的工具定义，但不属于首版支持持续订阅的工具；调用直接返回明确错误。当前 970/956 是 API 及 active API 的目录基线，published/supported/succeeded 均需在实施后另行统计，不在设计阶段填成已经验收的数字。

## 1 目标与已经确定的要求

创建独立项目 `openany-mcp`，将 PostgreSQL 中已有 Agent API 一对一发布为 MCP 工具。常驻服务使用 Go；名称补齐脚本按用户选择保留 Python。服务直接读取数据库元数据，业务请求统一经过现有 OpenAny 网关。

已确定的合同：

| 项目 | 决定 |
|---|---|
| 服务语言 | Go |
| 名称维护 | Python 独立脚本 |
| 项目边界 | 独立仓库、独立构建与部署，不并入 Rust 网关进程 |
| MCP 入口 | 远程 Streamable HTTP，支持多个客户端 |
| 认证 | 每次透传调用方的 OpenAny 凭证，由网关判断有效性 |
| 目录来源 | 只读 PostgreSQL，周期刷新内存快照 |
| 对外工具名 | 新增 `agent_apis.public_mcp_tool_name` 持久保存 |
| 名称生命周期 | 只为 NULL 分配，已保存名称不随 API 路径/描述变化自动改名 |
| 网关职责 | 认证、账户与 Key 限制、路由、上游鉴权、计费、结算、usage 日志 |

本服务负责协议适配、工具目录、参数编码、调用转发及结果转换。不建立第二套账户系统，不持有统一销售 Key，不直接调用供应商，不复制网关的计价与上游 mappings。

“支持多个客户端”指兼容标准协议和不同产品的调用方式。客户端必须能够设置 OpenAny 凭证请求头。仅支持 OAuth 登录、不能配置这些请求头的客户端，不承诺第一版可以直接连接。

## 2 现有目录基线与已知限制

基线来自本次会话此前对 `openany_local` 的只读查询、当前网关源码和 `/private/tmp/openany-mcp-smoke`。这些数字是快照，实施前必须重新对账，不代表数据库在后续时刻仍保持相同数量。

| provider_id | active API |
|---|---:|
| tianyancha | 239 |
| qcc | 204 |
| serpapi | 180 |
| dataforseo | 154 |
| blockrun | 76 |
| yuandian | 40 |
| pkulaw | 36 |
| gang | 24 |
| accurlex | 3 |
| 合计 | 956 |

另有 14 条 disabled 的 fetchin-mock API，共 970 条。active API 中 HTTP 上游 713 条、MCP 上游 243 条；两类都能使用现有网关 HTTP 入口。本服务不因上游协议不同而另开绕过网关的链路。

956 条 active API 均有 `agent_api_request_configs`，其中 23 条无参数。需要覆盖 GET、POST JSON，以及 query/path/body/header 参数。现有正文包括 154 个根数组、245 个根对象，不能把 JSON 请求一律当成对象。两份北大法宝 schema 含本地 `$defs` / `$ref`。原始 inputs 序列化约 3 MB，这不是 token 数估计。

有两项明确的现有缺口：

- `gang GET /api/stream` 的目录说明写明同步网关尚不支持业务 SSE。其名字仍可一对一分配；第一版需要明确标记不支持持续订阅，不声称加一层 MCP 即可修复。
- 8 条 Blockrun API 共 46 个数组 query 参数，现有 inputs 不含 style/explode 等序列化信息；抽查的导入快照也只保留了类型及描述。上线前必须逐项确认数组的实际 HTTP 编码，不能从 `type=array` 推断为重复参数或逗号分隔。

## 3 方案选择与总体架构

沿用用户已选择的 A：独立 MCP 服务只读数据库。不采用新增网关目录接口，也不以静态导出文件作为运行时目录。

```text
API 新增/导入任务
       │
       └── Python 名称补齐脚本（有限写权限）
                     │ 仅补齐 public_mcp_tool_name
                     ▼
                PostgreSQL
                     │ 只读目录及参数定义
                     ▼
MCP 客户端 ──► Go openany-mcp ──► OpenAny 网关 ──► HTTP / MCP 上游
  凭证随请求      协议/校验/编码       认证/限额/计费/执行
```

名称写入与服务读取分开：数据库迁移、Python 维护任务、Go 常驻服务使用不同权限。Go 服务不在启动或刷新时写库，也不自动调用维护脚本。

请求路径只允许指向配置中的网关 origin。客户端选择的是已注册工具名及业务参数，不可指定任意 URL、HTTP method、provider 或另一条 api_id。

## 4 技术栈与项目结构

建议实施基线如下，版本依据在文末给出；实际项目使用锁定版本，不在部署时跟随 main/latest。

| 能力 | 选择 | 原因 |
|---|---|---|
| Go 工具链 | Go 1.27.3 | 本次核实的稳定版本；官方 SDK v1.8.0 最低要求 Go 1.25 |
| MCP | `github.com/modelcontextprotocol/go-sdk v1.8.0` | 使用官方协议与 HTTP transport 实现 |
| HTTP | 标准库 `net/http` | 不为少量路由引入 Gin、Echo 等框架 |
| PostgreSQL | `github.com/jackc/pgx/v5 v5.11.0` 与 pgxpool | 直接查询、连接池、显式事务，无 ORM |
| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3` | 支持现有 draft-07 与 2020-12 校验需求 |
| JSON/日志/配置 | `encoding/json`、`log/slog`、环境变量 | 保留数字精度，减少依赖和配置层 |
| Python 维护任务 | Python 3.12、psycopg 3 | 单用途数据库脚本，不运行 HTTP 服务 |

Go 服务用 `go.mod` / `go.sum` 锁依赖；Python 脚本用独立依赖锁文件锁版本。Python 不进入 Go 服务运行镜像，脚本由运维环境或独立任务镜像执行。

建议目录：

```text
openany-mcp/
  cmd/openany-mcp/main.go
  internal/
    config/config.go
    catalog/postgres.go       目录查询
    catalog/snapshot.go       不可变快照与健康状态
    catalog/refresh.go        周期刷新
    schema/compile.go         inputs → inputSchema + validator
    transport/encode.go       path/query/body/header 编码
    gateway/client.go         HTTP 客户端、连接上限与逐次凭证
    gateway/result.go         网关结果与错误转换
    mcpserver/server.go       动态工具注册及 SDK handler
    mcpserver/pagination.go   绑定目录版本的分页
    httpserver/server.go      路由、Origin/Host、健康检查
  config/catalog-overrides.json
  migrations/001_add_public_mcp_tool_name.sql
  scripts/sync_mcp_tool_names.py
  scripts/requirements.lock
  testdata/                  脱敏元数据及模拟响应
  Dockerfile
  .env.example
  README.md
  go.mod
  go.sum
```

模块边界按实际代码量可合并，不提前建立插件框架或多层 repository/service 抽象。不生成 956 个 Go 函数；所有工具复用一套执行逻辑和按配置生成的 binding。

## 5 数据库字段与迁移

现有 `mcp_tool_name` 是上游 MCP 工具名，允许重复且保留大小写。它不能改作对外名称。新增字段如下：

| 字段 | 类型及约束 | 含义 |
|---|---|---|
| `public_mcp_tool_name` | text，可为 NULL | 本服务对外的唯一工具名 |
| 非 NULL 格式 | `^[a-z][a-z0-9_]{0,63}$` | 小写 ASCII、数字、下划线，字母开头，最长 64 字符 |
| 唯一范围 | 全表非 NULL 值唯一 | disabled 记录也占用名称 |
| 默认值 | NULL | 尚未完成名称分配 |

64 字符是项目兼容性约束，不是 MCP 规范的长度上限。NULL 允许新 API 先入库，维护脚本随后补齐；空字符串和纯空白均非法。停用不清除名称，重新启用时仍用原名。

迁移文件的核心 SQL：

```sql
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE public.agent_apis
    ADD COLUMN public_mcp_tool_name text;

ALTER TABLE public.agent_apis
    ADD CONSTRAINT agent_apis_public_mcp_tool_name_format_check
    CHECK (
        public_mcp_tool_name IS NULL
        OR public_mcp_tool_name ~ '^[a-z][a-z0-9_]{0,63}$'
    );

CREATE UNIQUE INDEX agent_apis_public_mcp_tool_name_unique
    ON public.agent_apis (public_mcp_tool_name);

COMMIT;
```

迁移作为有版本的独立 SQL 文件执行，不由服务或补齐脚本隐式创建。首次仅加列及约束，不把 970 条回填塞进 DDL。重复部署须识别迁移已完成；若对象已存在，检查其定义是否一致，不能仅凭同名就当作迁移成功。

当前约 970 行可采用普通唯一索引；若实施时数据量显著增加，迁移者重新评估建索引锁与窗口。失败时事务回滚。回滚服务版本时保留新增列和名称数据；不以删除列作为常规回滚动作。

## 6 名称分配算法与 Python 脚本

### 6.1 生成规则

完整 `agent_apis.id` 是内部身份。`public_mcp_tool_name` 是公开且持久的调用名称。

首次分配算法：

1. 拼接 `provider_id + '_' + lower(method) + '_' + path_template`。
2. 转小写，把连续的非 ASCII 字母数字字符替换为 `_`，去掉首尾 `_`，得到可读前缀。
3. UUID 去掉 `-`，首次取前 8 位作为后缀。
4. 前缀最多取 55 字符并去掉末尾 `_`，得到 `prefix + '_' + id8`。
5. 若名字已被占用，仅对本条待分配记录改用 12、16、32 位 UUID 后缀；前缀长度改为 `64 - 1 - suffixLength`。
6. 候选仍冲突或格式无效则报错，不覆盖另一 API，也不重命名已有 API。

例如：

```text
provider_id = blockrun
method = GET
path_template = /api/v1/crypto/list
api_id = e1183d75-4cb5-41b3-a351-0b027f17dea9

public_mcp_tool_name = blockrun_get_api_v1_crypto_list_e1183d75
```

合法的人工指定名称，例如 `blockrun_crypto_list`，不受自动生成规则覆盖。自动名称的后缀帮助区分，不替代数据库唯一索引。

### 6.2 脚本模式与执行过程

提供 `scripts/sync_mcp_tool_names.py`。唯一必需连接配置为 `POSTGRES_DATABASE_URL`，运行时注入脚本专用凭证。

```sh
# 默认只预览；不会保存数据库变更
python3 scripts/sync_mcp_tool_names.py

# 显式写入
python3 scripts/sync_mcp_tool_names.py --apply
```

脚本处理 active 和 disabled 中所有名称为 NULL 的 API。已命名行全部跳过；先读取全部占用名称，再按 api_id 排序分配，确保同一数据库状态下预览结果稳定。

预览输出机器可读 JSON，包含模式、总行数、已命名数、待补齐数，以及每条拟分配的 api_id/status/tool_name。不输出数据库连接串、凭证或业务正文。无待补齐记录时成功退出。

`--apply` 的流程：

1. 检查新列、格式约束和唯一索引已经存在且定义符合约定。
2. BEGIN，设置 5 秒锁超时和 30 秒语句超时，取得本脚本专用事务 advisory lock；不复用网关的缓存发布锁。
3. 锁定待填充行，并在本事务内重新读取已有名称与待填充记录；不把早先 dry-run 当作无条件执行清单。
4. 生成、校验所有候选；在内存占用集合中立即保留本批已分配值。
5. 逐条执行条件 UPDATE，绑定 SQL 参数：

```sql
UPDATE public.agent_apis
SET public_mcp_tool_name = %s, updated_at = now()
WHERE id = %s AND public_mcp_tool_name IS NULL;
```

6. 逐条检查影响行数为 1；非预期并发、约束或权限错误整体回滚，并非零退出。全部成功后一次 COMMIT，输出实际更新数。

advisory lock 串行化协作脚本；行锁、NULL 条件和唯一索引共同应对其他数据库写入方。脚本不自动重试失败的写事务，便于运维明确看到失败原因；重新运行仍只处理剩余 NULL。

仅更新新列及 updated_at，不修改 upstream `mcp_tool_name`、状态、价格或网关 revision。只为新列赋值无需刷新网关 Redis；Go 服务通过完整内容 hash 感知变更。

### 6.3 名称生命周期与新增流程

API 的中文名、描述、method、provider 或 path 变化，都不触发自动改名。服务用 api_id 读取最新路由，但保留数据库中已保存的名称。参数/行为变化仍需评估客户端兼容性；确需改工具名时作为显式变更，不提供批量 force-rename。

后续新增链路：

```text
新增/导入 API → 名称为 NULL 或人工合法值
              → Python 脚本补齐 NULL
              → Go 目录刷新
              → active 且配置合法的工具对外出现
```

脚本失败不影响已有名称。未命名 active API 不发布，但要报告数量及 API ID。若需要全自动，将 `--apply` 作为导入成功后的任务步骤；本方案不新增数据库触发器或常驻 Python 服务。

所有 API 导入/更新方必须保留未提交的新列，不能在未传字段时写 NULL；删除后重建同一 API 的流程须显式保留原名。实施时检查实际写入方，这属于本项目的接入依赖，不假设现有控制面已经支持该列。

配套 `2026-11-03-openany-mcp-tool-mapping.json` 保存 970 条初次回填提案。tool_name 是建议落库值，尚未执行回填；其中 956 条 exposed=true 仅表示回填后满足 active 状态条件，最终发布还须通过 schema 与传输元数据检查。

## 7 只读目录查询与内存模型

### 7.1 读取范围

服务使用 pgxpool。一次查询同时取得 API 元数据和 inputs，不逐 API 发 SQL。只读必要列，不 SELECT *，不读取上游 Key、固定上游鉴权头或供应商成本。

示意查询：

```sql
SELECT a.id, a.name, a.description, a.provider_id,
       a.public_mcp_tool_name, a.method, a.path_template,
       a.public_path, a.status, a.revision, a.upstream_protocol,
       a.request_content_type, a.unknown_parameters,
       a.max_request_bytes, a.max_response_bytes,
       c.api_id AS request_config_api_id, c.inputs
FROM public.agent_apis a
LEFT JOIN public.agent_api_request_configs c ON c.api_id = a.id
WHERE a.status = 'active'
ORDER BY a.id;
```

必须使用 LEFT JOIN：已命名 API 缺少配置要可见并报错，不能被 INNER JOIN 静默丢掉。有配置且 inputs=[] 是合法无参工具，不等于缺少配置。

NULL 名称记录先进入待命名统计，不发布。其他已命名 active 行逐项检查：工具名格式/唯一性、HTTP 方法与编码、公开路径、参数描述、schema 引用和传输策略。`public_path` 必须与 `/v1/agent-apis/{provider_id}` 加 path_template 一致；不接受 URL scheme、host、fragment 或任意 query 被写入路由。

只支持 GET/none 与 POST/application-json 这些已覆盖合同；未来出现 multipart、任意新方法或未知编码时明确报告“不支持的目录配置”，不能猜测编码。

### 7.2 核心内存对象

每个发布工具建立一个只读 ToolBinding，保存：api_id、持久名称、provider、method、公开路径模板、输入描述、生成的 inputSchema、编译后的 validator、正文构造策略、业务 header 白名单及必要的传输例外。

每次刷新生成 CatalogSnapshot，保存内容 hash、构建时间、全量名称索引、供应商索引、排序后的工具列表，以及已经构造好的各视图 SDK server。所有 map/slice 在发布后禁止修改。

使用 `atomic.Pointer[CatalogSnapshot]` 发布当前快照。请求开始时只 Load 一次；本次发现、校验与执行均引用同一代绑定，不混用两代 schema 和路由。长请求持有旧快照直到完成；Go GC 在无引用后回收，不靠手工关闭共享对象回收。

## 8 自动刷新与一致性规则

建议每 30 秒执行一次目录查询，查询限时 10 秒，同一实例的刷新不可重入。规范化元数据及 overrides 后计算 SHA-256；只有内容变化才重新编译和构建 SDK server。hash 覆盖删除、停用、名称、路径、inputs、公开描述和传输配置，不仅比较 max(revision) 或 max(updated_at)。

刷新过程：读取一致结果 → 检查待命名行 → 校验全部可发布记录 → 编译全部 schema → 构建完整视图及 handler → 原子切换指针。

状态处理：

| 情况 | 处理 |
|---|---|
| 第一次加载成功 | 发布快照，ready=true |
| 第一次加载失败 | HTTP 健康端点仍可响应，ready=false；MCP 入口返回 503 |
| hash 未变、查询与检查成功 | 保留同一快照，但更新最后成功核验时间 |
| 新快照完整有效 | 一次替换，后续请求使用新代 |
| SQL 或已命名记录编译失败 | 保留旧快照，记录 API ID/字段路径/错误类别 |
| 新增 active 行尚未命名 | 跳过该行并告警，已有工具继续服务 |
| 距最后成功核验超过 120 秒 | ready=false，拒绝新的 MCP 请求；已接收的业务调用继续按自身预算执行 |

必须区分快照构建时间与最后成功核验时间：数据库内容一直不变不能导致服务 120 秒后被误判过期。配置错误连续导致刷新失败时最终停止服务，避免无限发布不可信的旧目录。

首版选择整批校验、整批发布：一条已命名 active API 的配置无效也会拒绝这一代新快照；若连续失败超过 120 秒，整个入口会变为未就绪。这是明确的故障策略，不是已经实现逐 API 隔离。日常新增和更新必须先跑只读目录检查，再执行名称补齐或发布更新。未命名行只告警跳过，不按配置失败处理；已知业务 SSE 限制以显式工具错误表达，也不属于编译失败。

MCP 快照与网关 Redis 不是一个分布式事务。30 秒内可能出现“目录已看到新工具，但网关尚未同步”或“目录仍显示已停用工具，网关已经拒绝”的窗口；最终调用结果由网关决定。

不能把同一 provider/method/public_path 立即转给另一 api_id，并假设缓存中的旧工具仍有精确身份保证。当前 HTTP 入口没有 api_id/revision 条件断言；若业务需要这种强一致保证，需另扩展网关合同。本期通过禁止短时间跨 API 复用公开路由、发布顺序及目录刷新核对控制风险，不宣称 HTTP 转发已经具有版本锁定能力。

## 9 MCP 协议、入口与动态注册

### 9.1 HTTP 入口

| 地址 | 作用 |
|---|---|
| `/mcp` | 全量有效工具 |
| `/mcp/{provider_id}` | 该供应商视图，工具名与全量视图相同 |
| `/healthz` | 进程存活，不调用数据库或业务 API |
| `/readyz` | 目录是否有可用且未过期的快照 |

不存在的供应商入口返回 404；存在但暂时没有可发布工具的已知供应商可返回空列表。供应商视图同时限制 list 与 call，不能只隐藏列表却仍允许调用其他供应商工具。视图只用于工具选择，不代替认证或权限。

### 9.2 Go SDK 集成方式

使用官方 `mcp.NewServer`、`Server.AddTool` 和 `NewStreamableHTTPHandler`。选择低层 `Server.AddTool`，因为 inputs 来自数据库，不能依赖静态 Go struct 推导 schema；参数校验、结果包装由本项目明确完成。

每次快照先构造全量及供应商 SDK server，注册对应 ToolBinding 的执行闭包，随后一并发布。HTTP middleware 将当前快照固定到请求 context；SDK getServer 回调从该固定快照选择视图，不能在同一请求的多次回调中反复读取全局最新指针。

工具 handler 从 `CallToolRequest.Extra.Header` 获取本次请求头；不从初始化 session 保存凭证。共享 handler 只持有不可变 binding 和共享 HTTP client，不捕获用户 token。

明确设置 `StreamableHTTPOptions.Stateless=true`。第一版验收协议版本为 2025-03-26、2025-06-18、2025-11-25、2026-07-28，并通过 SDK SupportedProtocolVersions 收窄到这些版本。版本协商、旧式 initialize 与现代 discover 由 SDK 处理，不手写完整 JSON-RPC transport。

第一版只声明 tools 能力，listChanged=false。无 sampling、elicitation、roots、业务订阅、旧式独立 `/sse` 入口或 stdio 交付。不承诺实时推送目录变化；客户端重新 tools/list 或重连后获取新目录。SDK 默认推断的 logging/listChanged 等能力须显式收窄，不能宣称未实现的能力。

不根据 GET/POST 自动推断 readOnly、destructive 或 idempotent 等工具语义。需要这些 annotations 时只能使用经过确认的元数据；尤其不能把“只读查询”当作“免费”或“可以安全重复扣费调用”。

Streamable HTTP 在新旧版本上的 GET/session 行为不同，由 SDK 按协商版本处理，不把所有 GET 一律视为业务查询，也不把所有版本都写死成必须初始化。

### 9.3 分页与多客户端的大目录处理

工具按持久 tool_name 排序，默认每页最多 50 条。分页不是授权，也不会自动减少客户端最终放进模型上下文的工具数量；按供应商连接仍是主要减量方式。

通过 SDK receiving middleware 处理标准 tools/list 结果及游标，使 cursor 显式包含版本号、目录 hash、视图、最后一个名称。cursor 使用有长度上限的 base64url JSON；它不是凭证，不存敏感信息，无需每实例随机密钥。解码后验证字段、视图、目录 hash、位置及长度。

目录或视图不匹配时返回标准 InvalidParams，提示重新从第一页读取，不拼接新旧代。多实例内容相同即得到相同 hash，游标可跨副本；滚动刷新期若 hash 不同，允许客户端重新枚举，不要求粘性 session。不依赖 SDK 内部私有游标格式或 fork SDK。

分页实现只接管标准 tools/list 的结果切片和游标，不重新解析或实现协议生命周期。调用则继续走同一快照中 SDK 注册的工具 handler，确保“列出来的定义”和“执行所用的 binding”属于同一代。

## 10 inputSchema 编译与参数合同

### 10.1 外层结构

MCP arguments 始终为 JSON 对象。使用 inputs 中原始 name 作为属性名：

- GET 的 query/path/header input 作为同名顶层参数。
- 根正文 input 使用 `body` 参数，类型保留 object 或 array。
- 非根正文 input 使用其原始 name，并记录其 HTTP JSON Pointer。
- 无参工具发布 `{type:object, additionalProperties:false}`。

当前 active API 内没有重复 input.name。未来出现跨位置同名 input 时明确拒绝该目录配置，不静默覆盖，也不临时自动改名改变客户端合同。

### 10.2 编译规则

优先保留 input.json_schema 的具体结构；缺少时根据 type/item_type 构建。结合 inputs 层 required、enum_values、minimum、maximum、nullable 及描述。数据库中字符串形式的数值约束按十进制数字解析，不能当作字符串比较。

required 表示属性必须存在，nullable 表示值能否为 null，不能混为一谈。false、0、空字符串、空数组与未传值严格区分。default 只作为说明，不在 MCP 层自动补值或改变网关定价输入。

把 OpenAPI nullable 转为明确 JSON Schema 表达；与原 schema 明确矛盾的约束报配置错误，不悄悄放宽。将 draft-07 片段规范化到公布的 2020-12 外层，处理 definitions/$defs、dependencies、数组 tuple 等语义差异；遇到未支持转换的关键字，阻止新快照发布并报告，不直接删除关键字假装编译成功。

根 schema 嵌到 `properties.body` 后，`#/$defs/...` 不再天然指向原来的位置。编译器需搬移/命名空间化本地定义并改写本地引用，或在有限、无循环的情况下展开；不可机械复制导致引用失效。禁止编译器自动访问外网或本地文件解析外部 `$ref`。未打包的外部引用必须先在元数据准备阶段解决。

使用同一份规范化 schema 进行发现及运行时校验，避免客户端看到的约束与服务实际应用的约束不同。编译后的 validator 在快照内复用，不在每次调用时重新编译。

### 10.3 JSON 与未知参数

arguments 使用标准 JSON decoder 的 UseNumber，校验前拒绝重复属性、尾随额外 JSON 和过深结构。涉及大整数、金额或精确标识的值不能先转换为 float64；响应转发也保留 json.Number/json.RawMessage，不把大整数舍入后再输出。

unknown_parameters=reject 时，外层未知参数报输入错误。ignore 时可以接收但不编码未声明的外层参数；不会把 `url`、method、认证头之类未知字段当作控制参数。根 body 内是否允许额外属性由原始 body schema 决定，不能因存在一个 body 容器就擅自删除其内部字段。

## 11 HTTP 请求编码与传输例外

### 11.1 规则表

| 输入 | HTTP 编码 | 错误处理 |
|---|---|---|
| path | 按占位符替换并单段转义 | 缺失、空段或会改变路由结构的值拒绝 |
| query 标量 | 用 url.Values 构造，bool 为 true/false，数字保留十进制语义 | 不把 false/0 当作未提供 |
| query 数组 | 使用明确的 csv/repeated/json 规则 | 元数据未确定时不猜测 |
| 根 body | body 值直接序列化为完整正文 | 不能额外套一层 body，也不能把数组改为对象 |
| 非根 body | 根据 pointer，或默认 /name，构造 JSON 对象 | 重叠/冲突指针拒绝，不覆盖另一参数 |
| 业务 header | 仅声明的 header，转成明确标量字符串 | CR/LF、保留头和无法表达的 null 拒绝 |

path 值与静态模板分别处理，不对最终路径使用会清理 `..` 或改变路由含义的 JoinPath。对每个用户值只做一次正确编码，并与网关 decode_segment 的限制一致；不把用户输入先反复 URL 解码。

JSON Pointer 按 RFC 6901 的 `~0` / `~1` 解析。根 body 与 path 共存合法，例如 RPC network 路径；根 body 与其他 body input 冲突时报告配置错误。未来需要新增嵌套数组索引构造时，必须有明确父节点 schema 与边界，不稀疏扩容任意巨大数组。

不复用 agent_api_request_configs.mappings 再做一次上游转换，不注入 fixed_body/fixed_headers；这些由网关执行。本服务只构造网关公开 HTTP 合同。

### 11.2 现有数组 query 的补全方式

在 `config/catalog-overrides.json` 中按 api_id 和 input.name 保存经过来源核实的编码规则。采用如下固定值集合：

- csv：`["A","B"]` 编为一个参数 `ticker=A,B`，再统一 URL 转义。
- repeated：编为 `ticker=A&ticker=B`。
- json：编为一个参数，值为 JSON 字符串 `["A","B"]`。

配置同时明确空数组是省略还是传空值。查询数组含逗号等特殊值时遵循对应 API 合同，不能因选择 csv 就丢失值边界。nullable 的 query/path/header 若没有明确表示规则，作为不支持配置处理；当前快照没有 nullable query。

实现前核实上述 8 条 API 的全部 46 个数组参数，引用官方文档或已存在的请求样例形成覆盖清单，再提交 overrides。对于 rules 计价路由，网关当前拒绝重复 query key，因此不能为这种路由盲目使用 repeated；若上游必须 repeated 而网关不支持，列为网关能力缺口。

该文件还可为夹杂上游鉴权/内部说明的 description 提供经过审阅的公开业务描述，以及明确的业务响应模式限制。它不保存凭证、不改价格、不改 URL、不重新命名工具。配置变更通过服务发布生效并计入目录 hash；数据库元数据更新继续按周期自动刷新。

一个已经命名但传输规则不完整的新 API 不能被假装为完整可用工具：新快照校验报告具体 API ID 和缺失字段。在完成历史清单前，不把全部 956 条工具视为已验收。

### 11.3 请求示例

| 场景 | MCP arguments | 网关请求 |
|---|---|---|
| Crypto 列表 | `{"limit":3}` | GET 对应公开路径，query 为 limit=3 |
| 路径变量 | `{"id":"abc"}` | GET /v1/agent-apis/gang/api/agent-details/abc |
| DataForSEO 根数组 | `{"body":[{"keywords":["seo"]}]}` | POST 正文直接为数组 |
| 上游 MCP API | `{"body":{"keyword":"劳动合同"}}` | POST 正文为普通参数对象，由网关封装上游 MCP |
| 非根 JSON Pointer | `{"urls":["https://example.com"]}` | JSON 正文为 `{"urls":[...]}` |

## 12 凭证透传与 HTTP 边界

按每个 tools/call 的 RequestExtra.Header 复制以下凭证头到新的网关请求：Authorization、x-api-key、xi-api-key。保持值及既有网关解析优先级，不在 MCP 中验证账户，不缓存认证结果，不把 token 填进工具 arguments。

共享 http.Client 不带默认用户凭证；每个新 http.Request 都建立独立 header map。初始化用户 A 后收到用户 B 的调用，必须仍按当前 B 请求头转发。没有凭证则不补固定 Key，调用网关后由网关返回认证错误。

只允许转发凭证白名单和该 API inputs 声明的业务 header。禁止业务参数覆盖 Authorization、API Key、Host、Cookie、Connection、Content-Length，以及 MCP session/protocol 控制头。Content-Type 由正文编码器确定；网关 Accept 使用 application/json，以接收其既有 JSON 心跳及最终 JSON。

initialize/discover/tools/list 是公开元数据操作，不调用收费 API，不要求 MCP 账户认证，也不做按用户过滤。公开描述经过准备和审阅，仅含业务用途、限制、公开路由与参数；上游 Key、内部地址、固定鉴权头、供应商认证操作说明及内部成本说明不进入工具列表。

Origin/Host 检查属于 HTTP 边界，不是账户认证。Origin 缺省的原生客户端正常接入；若携带 Origin，必须在允许列表，否则 403。Host 必须匹配部署域名或本地地址；反向代理前置时只信任明确配置的代理，不无条件使用 X-Forwarded-Host。

网关 base URL 在启动时解析并固定，仅允许 http/https、无 userinfo/query/fragment，首版只接受空路径或 `/`。生产公开 MCP 入口由反向代理提供 HTTPS；网关地址可按受控内网网络方案使用 HTTP。禁止跳过 TLS 证书验证、跟随重定向或自动套用未配置的环境代理。

本方案使用 OpenAny 自有 API Key，不实现 MCP OAuth。不能把任意第三方 OAuth access token 当作网关 Key 透传并宣称合规 OAuth 接入；未来需要此能力时，由网关承担对应 MCP 资源身份与令牌校验。

## 13 一次工具调用的完整流程

1. HTTP middleware 检查入口、Host/Origin、请求大小及目录新鲜度，并固定当前快照与视图。
2. SDK 解析协议，找到当前视图内的工具。未知工具返回协议错误；不会按客户端自带 api_id 找路由。
3. 检查工具的业务响应模式限制。已知不支持的持续订阅返回明确工具错误，不发起注定无法完成的 20 分钟网关请求。
4. 对 arguments 执行结构、schema 与输入边界校验；失败不调用网关。
5. 尝试取得进程级并发槽。无槽立即返回工具执行错误 `openany_mcp_busy`，不建立无界队列。
6. 构造 URL/query/body/header；只在这里把当前调用凭证放进请求。
7. 在调用 context 的总预算内发送一个业务 HTTP 请求。
8. 有界读取响应；识别 JSON 心跳后的最终正文、公开错误及内容类型。
9. 转换为 MCP CallToolResult，记录安全日志，释放并发槽。

MCP request ID 与网关 request_id 不是同一个概念。内部日志可以同时记录二者，但结果中的网关 request_id 必须来自网关响应头，不伪造或拿 MCP request ID 替代。网关响应早期错误若没有 request_id，返回 null。

## 14 超时、取消、并发及不重放

### 14.1 实际网关预算

已核实 `AgentApiService` 的上游 timeout 来自全局 `settings.key_market.upstream_timeout_secs`。当前源码默认 1200 秒，连接超时默认 60 秒；prepare 总预算 30 秒，JSON 心跳默认 55 秒。不能用 agent_apis.timeout_ms 或示例固定 30 秒当作真实调用总预算。

建议 MCP → 网关调用总超时默认 1260 秒，留出 prepare 和结算余量。部署前按实际网关配置调整，而非机械采用默认值。接入方可使用更短客户端超时，但必须知晓它会先于网关完成。

HTTP server 的 ReadHeaderTimeout 建议 5 秒，IdleTimeout 60 秒。不设置比工具调用预算短的全局 WriteTimeout；业务调用用 context.WithTimeout 控制总时长。连接网关的拨号预算建议 5 秒；不使用 30 秒 ResponseHeaderTimeout，因为未启用心跳或较长准备阶段可能还未返回响应头。

反向代理的总体请求预算必须长于 MCP 调用预算，例如 1300 秒。不能只把 proxy_read_timeout 设置很长，却保留其他更短的网关/平台总超时。旧客户端等待单个 JSON 响应的能力和现代请求级 SSE 的能力分别联调，不凭 MCP transport 支持 SSE 就保证所有产品会等待 20 分钟。

### 14.2 取消与执行状态不确定

收到客户端取消或断线时，取消本进程对应的网关 HTTP context，以释放连接和并发资源。但现有网关业务执行放在独立任务内，断开连接不等于网关停止上游调用或不收费。

请求已发出后遇到网络中断、超时或响应超限，错误注明 `execution_state=unknown`；不能声称“未执行”“未扣费”或提示自动重试。若网关已返回 request_id，保留它供查询 usage；未返回则明确为 null。

### 14.3 不重放的具体约束

应用层不因 401/429/5xx/超时/断线重试，不切换协议后重发业务 tools/call，不以发现失败为由重复收费请求。

Go 的默认 Transport 在某些复用连接故障下会透明重试 GET；本项目的 GET 也可能是收费 API，不能忽略这层行为。首版网关业务 client 使用 HTTP/1.1、DisableKeepAlives=true、禁用重定向，不启用 HTTP/2 自动恢复；共享 client 和连接上限仍可复用配置，但不复用业务 TCP 连接。这样以额外建连成本换取清晰的单次转发边界，并用故障注入测试核实。

不声称这提供分布式 exactly-once。客户端自己重复发送同一工具调用仍可能执行两次；当前 MCP request ID 不是网关幂等键。未来若需要安全重试与连接池性能优化，先建立网关可验证的幂等合同，再调整传输策略。

### 14.4 并发与大小

建议首版进程级并发默认 16，可配置；账户/Key 限流仍由网关负责，不在 MCP 中解析 token 推断账户。Go 信号量只限制本进程资源，无可用槽立即失败，不无限排队。

建议边界：MCP 入站正文 8 MiB，网关响应解码后 8 MiB，最终 MCP result 20 MiB，JSON 嵌套深度 128。传输层和解压后的字节数均需受限；不依赖供应商 Content-Length。现有快照中 max_response_bytes 为 8 MiB 或 64 KiB，但旧网关路径未必执行该限制；本服务的全局边界独立生效，不能把旧元数据限额误当成已实现约束。

同时返回 structuredContent 和文本 JSON 会增加输出体积，必须计入最终 result 上限。超限报错，不截断出一个看似成功的残缺结果。并发 16 与 8 MiB 不等于固定内存占用；压测需测量 JSON 解码、文本包装及慢客户端下的峰值，再设置容器内存限制。

## 15 返回结果与错误分类

### 15.1 正常结果

工具统一返回以下逻辑对象，同时作为 structuredContent 和文本 JSON：

```json
{
  "http_status": 200,
  "request_id": "gateway-request-id",
  "content_type": "application/json",
  "data": {"symbols": ["BTC-USD", "ETH-USD"]}
}
```

data 可以是对象、数组或其他合法 JSON 值。对于上游 MCP API，保留网关返回的完整 MCP result，不只抽 structuredContent，不重新执行其中的工具或链接。

第一版不声明供应商级 outputSchema，避免对未建模的上游 JSON 作错误的类型保证；统一包装字段按本文合同构造并测试。MCP 协议外壳及不同版本所需 resultType 等由 SDK 序列化，不自行拼装第二套协议响应。

第一版支持完整、有限的 JSON 业务响应。普通文本、HTML、二进制和无限业务 SSE 返回明确不支持错误；这些响应模式若要成为产品能力，需要另做输出合同。已知 gang stream 在目录说明中标记限制，调用时返回 `openany_mcp_unsupported_response_mode`。

### 15.2 错误分层

| 错误类别 | 对外处理 |
|---|---|
| 无效 MCP JSON/方法/未知工具 | SDK 标准 JSON-RPC 错误 |
| 过大 HTTP 请求、Host/Origin 不合法 | HTTP 413/403，业务未转发 |
| 目录未就绪或超过过期阈值 | MCP HTTP 入口 503，业务未转发 |
| 业务参数不合法 | CallToolResult.isError=true，stage=validate |
| 本进程并发槽不足 | isError=true，code=openany_mcp_busy |
| 网关 401/403/429/5xx | isError=true，保留网关状态及安全错误码 |
| HTTP 200 但正文 type=openany_error | isError=true，不按 HTTP 状态误判成功 |
| 超时/网络/读取超限 | isError=true，注明是否 execution_state=unknown |
| 有限 JSON 中的供应商业务码 | 保留 data，不额外重写供应商成功/计费判断 |

工具执行错误可返回：

```json
{
  "http_status": 401,
  "request_id": null,
  "error": {
    "code": "网关公开错误码",
    "message": "安全、可操作的错误说明",
    "stage": "gateway",
    "execution_state": "rejected"
  }
}
```

不要把网关业务 401 直接变成 MCP HTTP 层 OAuth challenge。MCP 的 HTTP 传输成功可以返回 200，而 CallToolResult 标记错误；调用者仍能看到网关原始状态。

已有网关 JSON 心跳可能先提交 HTTP 200，再返回 `type=openany_error`。Go client 应将空白视为 JSON 前导内容，等待最终对象，并结合公开错误类型判断；不能只调用 StatusCode 判断成功。非 JSON 错误页不原样塞入结果暴露内部代理信息。

## 16 配置、数据库权限与日志

### 16.1 配置

以下值是首版建议默认值，属于设计选择而非已部署配置：

| 环境变量 | 默认值/要求 | 作用 |
|---|---|---|
| POSTGRES_DATABASE_URL | 必填，运行角色 | 只读目录连接 |
| OPENANY_GATEWAY_BASE_URL | 必填 | 唯一允许转发的网关 origin |
| MCP_LISTEN_ADDR | 127.0.0.1:3333 | 容器内可显式设为 0.0.0.0:3333 |
| MCP_PUBLIC_HOSTS | 生产必须配置 | 允许的 Host |
| MCP_ALLOWED_ORIGINS | 默认仅本地测试来源 | 携带 Origin 的请求允许列表 |
| MCP_CATALOG_REFRESH_INTERVAL | 30s | 周期核验 |
| MCP_CATALOG_MAX_STALENESS | 120s | 目录过期阈值 |
| MCP_DATABASE_QUERY_TIMEOUT | 10s | 查询预算 |
| MCP_DATABASE_MAX_CONNS | 4 | pgxpool 上限 |
| MCP_GATEWAY_TIMEOUT | 1260s | 业务请求总预算 |
| MCP_GATEWAY_CONNECT_TIMEOUT | 5s | 网关拨号预算 |
| MCP_MAX_CONCURRENT_CALLS | 16 | 进程资源上限 |
| MCP_REQUEST_MAX_BYTES | 8388608 | MCP 入站正文上限 |
| MCP_GATEWAY_RESPONSE_MAX_BYTES | 8388608 | 网关响应上限 |
| MCP_RESULT_MAX_BYTES | 20971520 | 包装后结果上限 |
| MCP_CATALOG_OVERRIDES_FILE | config/catalog-overrides.json | 已核实传输规则和公开描述覆盖 |
| MCP_SHUTDOWN_TIMEOUT | 1300s | 停止接入后等待已接收调用 |
| LOG_LEVEL | info | slog 日志级别 |

50 条分页、JSON 深度 128 等内部常量先固定，不为每项都增加环境开关。启动时校验 URL、时长关系、正整数边界及配置文件；配置缺失或相互矛盾时退出。禁止把连接 URL、凭证打印到错误日志。

脚本也读取 POSTGRES_DATABASE_URL，但运行环境注入的是写入角色凭证，不与 Go 服务共用部署 Secret。配置清单不新增 OPENANY_API_KEY 服务端统一 Key。

### 16.2 三类数据库角色

| 角色 | 权限 |
|---|---|
| 迁移角色 | 执行新列与约束 DDL |
| 名称维护角色 | SELECT 必要命名字段，UPDATE(public_mcp_tool_name, updated_at) |
| MCP 运行角色 | SELECT 必要目录列及 inputs，无 UPDATE/INSERT/DELETE |

运行角色不需要读取 upstream_keys、定价策略或账户表。可以用受控列权限或公开元数据视图提供这些字段；不使用开发环境 postgres 超级用户上线。迁移后验证服务角色写入失败、脚本角色修改价格失败。

### 16.3 日志与健康

slog 输出结构化日志，最少含 stage、mcp_request_id、api_id、tool_name、provider_id、catalog_hash、gateway_request_id、http_status、duration_ms、error_code。凭证、完整请求头、业务输入/输出不写日志。

目录刷新日志记录 active/named/published/pending/invalid 数量、版本 hash 和耗时；对失败记录具体 API ID 与字段路径，不打印原始配置正文。/readyz 只暴露必要状态、最后核验年龄及计数，不公开连接串或原始数据库错误。

记录调用成功/失败/超时/执行不确定、并发占用与目录刷新失败，先通过结构化日志接入已有采集系统；首版不为 metrics 另引入 Web 服务或观测框架。

## 17 构建、部署与优雅退出

### 17.1 构建与运行

Go 构建命令在实施后为：

```sh
CGO_ENABLED=0 go build -trimpath -o bin/openany-mcp ./cmd/openany-mcp
./bin/openany-mcp
```

增加一个只读检查模式 `--check-catalog`：读取与服务相同的元数据、overrides 和 schema 编译器，输出目录检查报告后退出，不监听 HTTP、不调用业务 API、不生成或写入名称。该模式也检查待命名 active 行的参数/编码，帮助在运行 Python 回填前发现问题。

常驻镜像采用多阶段构建，运行层只包含 Go 二进制、必要 CA 根证书及已审阅的 overrides 文件，使用非 root 用户。Python 任务独立安装锁定依赖执行，不能在每次 Go 容器启动时自动运行回填。

开发环境可连接本机数据库和 `http://127.0.0.1:8888` 网关。容器内的 127.0.0.1 代表容器自身，需配置可访问的网关服务地址；不能直接照搬本机 URL。数据库连接和公开入口域名由环境注入，不写入仓库。

### 17.2 启动、就绪与关闭

启动顺序：解析配置 → 建只读 pgxpool → 载入 overrides → 开始 HTTP 健康端点 → 首次目录查询和编译 → ready=true。首次失败时保持 ready=false，后续刷新可恢复；错误必须可观察，不能空目录静默成功。

生产反向代理提供 HTTPS，并将 `/mcp` 原路径及本次凭证头传给服务；不缓存 tools/call、不启用代理自动重试。对可能的 MCP SSE 响应关闭不适合的代理缓冲，整体超时与第 14 节一致。网关前其他代理也须检查是否会自动重放请求；MCP 的单次发送不能约束其他基础设施的行为。

SIGTERM 后立即 ready=false、停止新接入、停止周期刷新；用 http.Server.Shutdown 等待已接收请求。建议关闭预算 1300 秒，平台终止宽限至少 1310 秒，以覆盖默认 1260 秒调用。排空后关闭 HTTP transport 与 pgxpool。

如果运维选择更短宽限，必须接受部分在途调用被截断并进入“执行状态不确定”；服务不能在重启后自动补发这些请求。长调用预算与快速滚动发布之间存在实际取舍，不能同时假设两者无成本。

### 17.3 多副本

副本间无共享 MCP session，也无需 Redis。每个副本独立刷新，内容相同则 hash 相同。数据库池上限与并发上限按“每副本”计算，扩容会同时增加数据库连接和网关业务并发，需要与网关容量协调。

Python 补齐任务可以重复启动，但数据库锁会串行化；它不是每个副本必须执行的 sidecar。名称由数据库唯一约束统一管理，而不是由各副本自己生成。

## 18 上线、日常维护与回滚顺序

首次上线按以下顺序进行：

1. 创建新项目并完成 Go/Python 依赖锁定、配置和脱敏测试材料。
2. 检查实际 API 导入/更新方，确认新增字段不会被覆盖清空；必要修改在对应控制面项目交付。
3. 执行加列迁移，核对列、格式约束和唯一索引；建立三类最小权限角色。
4. 完成公开描述检查、46 个 query 数组编码核实及已知 SSE 限制配置。
5. 执行 `openany-mcp --check-catalog`，对待命名 active API 也做编译与传输检查。
6. 运行 Python 默认预览，核对当前 API 总数与候选名称；显式 --apply 完成回填。
7. 再次运行只读目录检查，核对持久名称与本次导出提案；对于此前快照，目标为 970 个已命名、956 个 active 对应工具定义，持续订阅限制需明确标注。
8. 部署 Go 服务，通过离线模拟验收及至少两种产品客户端联调，再开放入口。

后续 API 新增/更新流程：检查元数据和必要的传输规则 → 只读目录检查 → 补齐 NULL 名称 → 等待 Go 刷新 → 确认工具可发现与当前网关路由可调用。批量导入可将这些步骤串成任务；本设计不自动创建定时任务或代替运维决定调度方式。

停用只改 status，不清空工具名；Go 下一次刷新后移出目录。紧急停用由网关立即执行，MCP 刷新不是安全或计费的最终闸门。

回滚以服务镜像和 overrides 版本为单位。保留 public_mcp_tool_name 列和已经分配的数据，暂停有问题的导入或命名任务；不批量清空名称、不删除已注册值后重新生成。旧网关不读取新列，因此无需为 MCP 服务回滚而回滚网关业务代码。

## 19 测试矩阵与验收标准

### 19.1 Go 单元及组件测试

| 方面 | 必须覆盖 |
|---|---|
| 目录 | NULL 名称、无参配置、缺失配置、非法路径、重名、停用/删除、公开字段白名单 |
| 快照 | 无变化仍更新核验时间、原子切换、旧请求固定旧代、刷新失败保留旧代、120 秒过期、恢复 |
| schema | 必填/可空、false/0、enum/min/max、数组元素、嵌套对象、draft-07、nullable、$defs/$ref、外部引用禁止 |
| JSON | 大整数精度、重复 key、深度及尾随 JSON；不将原始 JSON 数字转为 float64 |
| 编码 | path/query/body/header、根数组、根对象、缺 pointer 默认字段、指针冲突、array 的三种编码 |
| 视图与分页 | 稳定排序、跨供应商调用拒绝、hash 变化、cursor 损坏/超长/越界、相同 hash 跨副本 |
| 凭证 | A/B 并发不串 Key、无凭证由网关拒绝、业务参数不能覆盖凭证、日志不含凭证 |
| 结果 | JSON 对象/数组、大整数、网关 401/403/429/5xx、200+openany_error、前导空白心跳、非 JSON、超限 |
| 执行边界 | 无效输入不转发、全局并发有界、断线取消、超时 unknown、禁止重定向及环境代理、无业务重放 |
| 协议 | 四个目标版本、旧式 initialize、现代 discover、无状态请求、只声明实际能力 |

使用 httptest.NewServer 模拟网关，InMemoryTransport 测协议处理，并增加真实本地 Streamable HTTP 测试，不能只靠内存协议测试代表 HTTP 接入正确。

对 Transport 的透明重试专门做故障注入：网关替身记录收到的业务次数，在写出/读取响应前后断开连接，验证一次 tools/call 不会再次触发业务请求。代理测试检查自动重试配置，而不只检查应用代码没有 retry 循环。

### 19.2 Python 与 PostgreSQL 集成测试

纯命名测试覆盖规范化、截断、8/12/16/32 位冲突回退、人工名保留和确定性排序。用临时 PostgreSQL 数据库测试：

- dry-run 无写入，首次补齐与后续只补新增行；重复运行零更新。
- active/disabled 名称全局唯一，格式约束允许多个 NULL、拒绝空字符串/重名。
- 修改路径和描述后已保存名称不变。
- 两个脚本并发，其他写入方竞争，条件 UPDATE 与唯一索引仍生效。
- 中途失败全部回滚；超时和权限失败非零退出。
- 实际控制面更新保留新列，Go 角色不可写、脚本角色不可改价格。

集成测试只用一次性数据库及固定虚构凭证，不向用户指定的业务库建临时测试表或写测试数据。

### 19.3 全量目录与产品验收

从实施时的数据库只读取得新基线，输出 total/active/named/pending/invalid/published 计数及每条排除原因。970/956 是当前参考值，不写成永远固定的生产断言。

验收报告另列 supported 数量及已知限制，不把 active 或 published 直接当作全部可执行。真实调用结果按 request_id 记录 succeeded/failed/unknown，不以一次目录检查替代业务验收。

名称、路由与 schema 一对一检查全部记录；46 个 query 数组参数必须有可追溯编码规则。已知 gang stream 的限制作为明确例外，不能当作普通成功调用计入验收。

官方 Go SDK 客户端、MCP Inspector，以及至少两个不同产品客户端验证：连接、发现/分页、凭证配置、实际调用、错误展示与长调用等待。未指定某个产品必须首批支持，因此具体产品名单在实施时按实际可用客户端选择；仅做了一次 Grok smoke 不代表通用接入已完成。

真实网关联调先选受控替身或确认当前销售价为零的 API；供应商介绍中的 free 字样不替代网关销售价。避免为“全量验收”把全部付费 API 执行一遍，目录/schema 覆盖与真实付费调用分别记录。

实现完成后执行 `go test ./...`、`go test -race ./...`、`go vet ./...` 及 Python 单元/临时 PG 集成测试；原设计阶段尚未运行项目测试；实施阶段的实际结果见 docs/verification.md。用约 1000 工具和并发 16 的模拟流量记录启动、刷新、p95 本地处理开销与内存峰值，测试前不承诺吞吐数字。

## 20 实施分阶段交付

| 阶段 | 交付内容 | 阶段完成条件 |
|---|---|---|
| 1 元数据 | 数据库迁移、Python 命名脚本、权限说明、控制面保留字段合同 | 预览/回填/幂等/并发/回滚测试通过 |
| 2 目录与编译 | Go 查询、schema、overrides、只读检查模式、不可变快照 | 全量目录可对账，缺口有明确原因，引用与数字精度正确 |
| 3 MCP 调用 | SDK 注册、全量/供应商视图、分页、凭证、HTTP 编码与结果 | 模拟网关下协议、请求和错误合同通过 |
| 4 运行交付 | 配置、健康、日志、资源边界、容器、排空/回滚说明 | 无状态多副本及故障行为可验证 |
| 5 接入验收 | 两种以上产品客户端、受控真实网关联调 | 名称/发现/凭证/调用/错误与计费记录可关联 |

实施不得以“956 个名字都生成了”替代“参数和转发全部正确”，也不能以“Go SDK 支持协议”替代产品客户端验收。数据库改动、脚本、服务和控制面配合分别交付，避免只完成其中一半。

## 21 已定决策、实施细节与风险边界

已经确定：Go 服务、Python 补齐、独立项目、数据库直读、新列持久化、只填 NULL、不做第二套认证、所有业务经过网关、无自动业务重试。

实施者可自行确定：内部文件合并方式、测试 fixture 划分、CI/镜像仓库、生产域名/Origin、数据库角色名称，以及根据实际部署调整资源与超时。依赖版本调整必须重新验证协议及 schema 合同，不默默跟随升级。

上线前必须处理的真实缺口是数组 query 编码、公开描述与实际导入方字段保留；业务 SSE 和 OAuth-only 客户端是首版明确边界。若增加 OAuth、持久订阅、任意 URL 工具、自动重试或按用户隐藏目录，均需另补相应合同，而不是夹带在通用代理中实现。

## 22 依据与版本来源

本地依据：

- src/agent_api/catalog.rs：active 路由与 request_contract 的加载方式。
- src/agent_api/service.rs：认证、公开路径、上游协议分流、prepare 预算及独立业务任务。
- src/agent_api/request.rs 与 rules.rs：公开请求、query 处理及重复 query key 限制。
- src/agent_api/heartbeat.rs 与 src/gateway/error.rs：空白心跳与 type=openany_error 合同。
- src/config/mod.rs：默认上游预算 1200 秒、连接 60 秒、心跳 55 秒。

外部依据（2026-11-03 核实，以下是技术依据，非已通过的本项目测试）：

- [Go 官方稳定版本列表](https://go.dev/dl/?mode=json)：Go 1.27.3。
- [官方 Go SDK v1.8.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0) 与 [go.mod](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/go.mod)：SDK 基线及最低 Go 版本。
- [SDK 协议实现说明](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/docs/protocol.md)：新旧协议及无状态 HTTP 行为。
- [SDK server API](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/server.go)、[request context/header](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/shared.go)、[HTTP handler](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/streamable.go)：动态注册、中间件与当前请求头接入点。
- [MCP Streamable HTTP 规范](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http) 与 [工具规范](https://modelcontextprotocol.io/specification/2026-07-28/server/tools)：传输与工具合同。
- [pgx v5.11.0](https://github.com/jackc/pgx/releases/tag/v5.11.0)：PostgreSQL 驱动基线。
- [JSON Schema v6.0.3](https://github.com/santhosh-tekuri/jsonschema/releases/tag/v6.0.3) 与 [支持方言说明](https://github.com/santhosh-tekuri/jsonschema/blob/v6.0.3/README.md)：schema 编译器选择。
- [Go net/http Transport](https://pkg.go.dev/net/http#Transport)：连接重试与协议/连接配置边界。
- [PostgreSQL 约束说明](https://www.postgresql.org/docs/current/ddl-constraints.html)：字段格式和 nullable unique 的数据库约束。
- [Psycopg 事务说明](https://www.psycopg.org/psycopg3/docs/basic/transactions.html)：Python 写入事务。
- [MCP token passthrough 说明](https://modelcontextprotocol.io/docs/2025-11-25/tutorials/security/security_best_practices#token-passthrough)：OAuth access token 透传边界。
