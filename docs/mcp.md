# OpenAny MCP 接入

推荐普通 Agent 使用轻量入口：`https://<你的 MCP 域名>/mcp-lite`。
传输方式为 Streamable HTTP。执行业务 API 时，在每个请求中携带：

```text
Authorization: Bearer YOUR_OPENANY_KEY
```

MCP 不保存备用业务密钥；认证、权限、限流和计费由网关处理。请使用 HTTPS。不要把真实 Key 放进提示词、业务参数或公开配置仓库。

## 选择入口

| 路径 | 内容 |
| --- | --- |
| `/mcp-lite` | search_apis、get_api、run_api 三个固定工具，推荐 |
| `/mcp` | 所有独立 API 工具，适合已有精确调用配置 |
| `/mcp/{provider_id}` | 指定供应商的独立工具 |
| `/docs/mcp` | 本接入文档，轻量功能启用后公开提供 |

通常只配置一个入口。把全量入口同时连接到 Agent，仍会加载完整目录。

在 Grok Bot 或其他支持远程 MCP 的客户端中，填写服务器 URL、Streamable HTTP 和 Authorization 请求头。不同客户端的配置格式不同；这里的 HTTP 联调不代表所有版本的客户端 UI 都已经验证。新配置后重新连接并检查三个工具是否出现。

## Agent 的调用顺序

1. search_apis 搜索需要的能力。
2. get_api 查看选中 API 的完整输入 Schema、价格、调用限制和版本。
3. run_api 执行一次业务请求。

API 标识 `api` 使用已有的公开工具名。不要猜工具名，不要直接传 URL。

```text
search_apis({"query":"backlinks summary","limit":5})
get_api({"api":"<搜索结果中的 api>"})
run_api({
  "api":"<相同的 api>",
  "api_version":"<get_api 返回的 api_version>",
  "input":{...按 input_schema 填写...}
})
```

上面是调用形状示意，不是可原样执行的 JSON。工具调用本身仍使用标准 MCP tools/call 消息。

### search_apis

`query`：最多256个字符的业务关键词。还可按 `provider`、`category`、`data_platform`、`data_product`、`capability` 筛选；至少提供关键词或一个筛选条件。筛选值使用返回的实际标识，capability 接受 ID 或 key。`limit` 默认5、最多10。

每个候选包含原有 api、title、provider、summary、method、path，以及已有分类、平台、产品、能力、purpose、match_reason、required_inputs_brief 和已知 limitations。match_reason 说明命中的字段和词，不代表供应商质量评分。缺失的分类和能力不会凭空生成。返回摘要不包含完整 Schema。

```json
{"query":"帮我查询企业工商信息","limit":5}
```

ranking 为 keyword：同时检索接口名、说明、路径、参数名、已有翻译和分类，支持有限中英文别名和词形归并。优先覆盖核心业务概念，再考虑最新、热点、历史、批量等意图及字段相关性；相同词在多个字段重复出现不会反复累加，英文完整词优先，避免 ai 误匹配 chain。时间意图帮助选择接口，不会自动填写业务日期或时间窗口。识别到查询意图时，避开明显的删除/创建等路由；这只是搜索排序辅助，仍须阅读 API 的实际语义。它不是通用语义搜索，不保证任意中文业务表述都命中。搜索不到时尝试实际接口名、供应商标识或更具体的英文词。

metadata_available=false 表示分类、翻译或示例辅助读取暂不可用；此时可继续按原有接口名、描述或供应商搜索，分类筛选可能找不到结果。翻页携带 next_cursor，保持 query、全部筛选条件和 limit 一致；目录、排序版本更新或游标失效后重新搜索。

### get_api

必填 `api`。返回精确的 `input_schema`、公开 `pricing`、`api_version`、描述、方法、公开路径和 API ID。参数说明可能包含暂不支持的可选数组限制。

`input_schema` 与现有直连工具的参数 Schema 一致。保留 body、query、path、header 参数原有的顶层参数名和 Schema 引用结构，不要自行重组为另一种参数格式。API ID 用于排查问题；执行仍传 api。

新增的调用指导包括：

| 字段 | 含义 |
| --- | --- |
| required_inputs | 声明为必填的 JSON Pointer 路径、类型和已有说明；scope=request 是顶层必填，if_parent_is_object/each_present_item 是对象或数组元素存在时的要求 |
| examples | 从已有公开 HTTP 请求样例转成 MCP input，保留可选业务字段；每个样例必须同时通过当前 Schema 和 HTTP 编码器，模板还需替换占位符 |
| price_summary | 精确美元字符串、计价单位和收费条件提示，is_quote 始终为 false；rules 模式仍需阅读完整规则 |
| response_hint | 公开响应样例的结构、顶层字段或数组长度；supported_media_types 只说明 MCP 支持的响应类型，不保证本 API 的实际响应格式；不返回原始响应样例值 |

examples.status 为 available、template_only、missing、stale 或 no_safe_valid_sample。items[].kind 区分 example（完整公开样例）、template（占位模板）和 empty_request（校验通过的空请求）。模板通过 placeholder_paths 指明待替换字段，不能原样调用；template_only 表示尚无完整请求样例。没有完整样例但 input {} 通过当前 Schema 和 HTTP 编码校验时，提供 empty_request；不会用空请求掩盖过期样例。最多展示三个条目，完整或空请求样例优先于模板。服务最多读取每接口前三个、单个请求不超过16KiB的公开样例；缺少可用样例时不编造公司名、地址、Token 或其他业务参数。认证请求头不转成 input；发现凭证形态的声明参数则放弃该样例。示例校验范围由 validation=mcp_schema_and_http_encoding、supplier_verified=false 标明：它证明 MCP 能接收和编码请求，不能证明上游数据、权限、额外业务规则一定满足。详情仍以 input_schema 为准，条件分支和跨字段约束不能只看必填摘要。

### run_api

必填 `api`、`api_version`、`input`。input 是一个符合刚读取的 input_schema 的对象，也就是原直连工具的 arguments。大整数按 JSON 数值准确传入。

参数、版本、接口状态检查通过后，服务按原有编码规则向固定网关发出业务请求。返回 http_status、request_id、data 等原有结果信息；工具错误包含 code、stage、execution_state、next_action。参数 Schema 错误还返回最多10条 details（path、keyword、message），不回显提交的值；details_scope=input 时路径相对 input，arguments 时相对整个工具参数对象。MCP 不自动重试业务调用。

支持有限 JSON（application/json、application/*+json）、UTF-8 text/plain 和 text/markdown。JSON 解析行为保持一致；文本和 Markdown 保留在原有结果结构的 data 字符串中，content_type 标明媒体类型。缺省或 utf-8 字符集的文本必须是有效 UTF-8。HTML、二进制和 SSE 不作为普通文本放行；JSON 类型但内容无效时也不会伪装成文本成功。

成功读取响应后增加 summary，说明真实 JSON 结构、字段或实际返回数组项数；字符串说明 type=string 和 characters（Unicode 码点数），不生成新闻摘要等业务解释。business_status=not_interpreted。HTTP 200 和这个摘要均不表示业务操作一定成功，不会推断供应商的总记录数、实际扣费或生成下载链接。

可以通过可选 output 控制展示：

```json
{
  "api":"<get_api 返回的 api>",
  "api_version":"<get_api 返回的 api_version>",
  "input":{"<实际参数名>":"<实际参数值>"},
  "output":{"mode":"compact","items_path":"/rows","fields":["id","name"],"max_items":5}
}
```

上面仅示意形状；只有实际响应 data.rows 是数组、元素具有对应字段时才适用。

| mode | 行为 |
| --- | --- |
| raw（默认） | 完整保留 data，另加结构摘要 |
| summary | 只返回结构摘要和 HTTP/请求编号，省略 data；文本也只报告类型与长度 |
| compact | 对根对象或指定数组元素选择 fields；数组最多显示 max_items 项，默认10，允许1至100 |

items_path 是相对原始 data 的 RFC6901 JSON Pointer，空字符串选择根数组；嵌套数组必须显式指定路径。fields 为最多20个当前对象或数组元素的字面字段名，不是嵌套路径；不存在的字段会被省略。items_path、fields、max_items 仅用于 compact；items_path 或 max_items 要求目标是数组。服务不会自动寻找并截断某个嵌套数组，实际删减会标记 presentation.truncated=true。

可提前判断的非法 output 配置在转发前拒绝。若执行后才发现路径不存在或结构不符，则返回 openany_mcp_output_unavailable、execution_state=completed，保留原始 data 和 request_id，明确提示不要为修正展示而重跑。字符串不支持 compact 字段/数组投影，处理方式相同：返回原文与 completed 提示，不要为更改展示而重新调用。输出裁剪不修改业务请求或计费，也不能挽救已超出网关响应/MCP原始结果限制的数据。

## 价格与计费

search_apis、get_api 读取目录，不会执行收费业务 API。run_api 可能产生费用和业务副作用。

pricing.status=available 表示公开价格数据可读，不代表已经为当前参数报价，也不保证网关接受执行。

- fixed：price_micro_usd 是精确整数微美元字符串，结合 unit 和 charge_condition 理解收费条件。
- usage：还包含 unit_quantity、quantity_scale、rounding、usage_pointer 等用量口径，总费用可能执行后才能确定。
- rules：policy 包含公开 parameters 和 sale 规则，包括条件价、数量价、舍入与上限。
- unavailable：公开价格暂不可用，不能当成免费。

1 USD = 1,000,000 微美元。不要使用浮点数累加金额。展示价格不是锁价承诺，实际结算由网关决定。响应中未提供实际费用时，不要自行编造一个费用数字。

## 错误与重试

- openany_mcp_api_changed：定义或展示价格变化，重新 get_api，再决定是否执行。
- openany_mcp_api_unavailable：接口不在当前可发布目录，重新搜索。
- openany_mcp_invalid_input：参数不符合 Schema，在本地被拒绝。
- openany_mcp_array_policy_missing：省略该可选数组，或等待配置编码规则；未向网关发送请求。
- openany_mcp_busy：本地业务调用并发已满，此次调用未开始。
- openany_mcp_invalid_output：输出选项组合无效，未转发到网关；类型或范围错误也可能返回 invalid_input。
- openany_mcp_output_unavailable：请求已经执行，无法按要求展示；先使用已返回的原始 data，不要自动重跑。
- openany_mcp_unsupported_response / openany_mcp_invalid_response：响应格式、编码或 JSON 内容不受支持，保留 HTTP 状态、请求编号及有界 content_type 信息，不回显原始错误页面。HTTP 200 仍可能在 MCP 结果处理中失败，不能据此认定未执行或未扣费。
- execution_state=unknown：请求可能已经执行或扣费，不要因为超时就重复提交。保存 request_id 供排查。

api_version 包含该接口的 revision、Schema、公开价格和转发规则，只用于发现已知定义变化，不是认证凭证、幂等键或网关锁价令牌。本版本没有通用的自动去重或长任务恢复工具。结果大小限制沿用现有服务；业务结果过大时不会伪装成截断成功，添加摘要后也会重新检查输出预算。

## 部署与自检

轻量入口默认开启，更新二进制后重启服务即可使用。需要关闭时设置 `MCP_LITE_ENABLED=false`；关闭后不加载轻量价格和搜索索引，原有入口照常工作。数据库角色仍为只读。价格和公开辅助资料共用独立刷新任务与最多1个连接的独立只读连接池；查询不占原目录连接池的连接。数据库连接配额仍预留这一个额外连接。

公开价格辅助查询需要 agent_apis 的公开计费字段、收费条件字段，以及 agent_api_pricing_policies 的 api_id、policy_version、parameters、sale_policy 只读权限。无需读取 supplier_policy 或 supplier_price_micro_usd。新增辅助查询只读取现有公开字段：

- agent_apis：id、revision、status、categories、data_platform、data_product、capability_id、upstream_key_id、name_translations、description_translations。
- agent_api_request_configs：api_id、samples；仅投影请求样例与响应结构，不把原始响应样例值存入 MCP 目录。
- agent_api_capabilities：id、key、name、description、status、name_translations。
- key_market_upstream_keys：仅 id、status，用于与公开目录的可用性条件保持一致，不读取密钥内容。

不使用整行 JSON 投影，不加载供应商成本、内部映射或固定认证配置。无需新增表、列、Redis、向量库或额外服务。由数据库管理员按现有角色核对必要公开列的只读权限；本次实现不执行授权或数据库迁移。如果部署环境缺少辅助列/表/权限，辅助信息降级，原目录和调用继续工作。已知停用的能力或上游密钥对应 API 会从轻量发现中隐藏；网关仍负责实际权限判断。

```sh
./scripts/run-local.sh --check-catalog
./scripts/run-local.sh
```

变量未设置或为空时默认开启。如果 .env 中已显式设置为 false，仍会关闭；删除该配置或改为 true 即可开启。源码开发也需要先加载环境变量。run-local.sh 运行 bin/openany-mcp，修改代码后先重新构建。

/readyz 中 ready 继续表示原目录状态；启用后还会显示 lite_ready、pricing_available 和 metadata_available。价格或辅助资料读取失败不让原目录失败；get_api 明确返回价格 unavailable 或缺少可用示例，search_apis/get_api 也报告 metadata_available。辅助资料刷新不改变原目录 hash；示例单独更新会刷新展示但不会让旧 api_version 失效。目录尚未完成轻量准备时，/mcp-lite 暂时返回503。/docs/mcp 不依赖目录就绪。

生产环境需设置 MCP_PUBLIC_HOSTS；有浏览器 Origin 时配置 MCP_ALLOWED_ORIGINS。反向代理应放行 /mcp-lite 和 /docs/mcp，按现有要求透传调用方凭证并关闭业务重试。已有环境配置中的显式 false 会覆盖默认开启行为。

三个入口共用同进程资源与业务并发额度。需要更强隔离时，用同一镜像单独部署轻量实例，再把 /mcp-lite 路由到该实例。
