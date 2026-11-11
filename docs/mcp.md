# OpenAny MCP 接入

本文默认英文。`Accept-Language` 偏好 zh 时返回中文，也可以用 `?lang=zh` 或 `?lang=en` 指定。请求的 `Accept` 包含 `text/html` 时返回 HTML，否则返回 text/plain。

推荐把 `https://mcp.openany.si` 粘贴到任意 MCP 客户端。传输方式为 Streamable HTTP。`/mcp-lite` 是同一轻量服务。执行业务 API 时，在每个请求中携带：

```text
Authorization: Bearer YOUR_OPENANY_KEY
```

MCP 不保存备用业务密钥；认证、权限、限流和计费由网关处理。请使用 HTTPS。不要把真实 Key 放进提示词、业务参数或公开配置仓库。

## 密钥

openany_overview、search_apis、get_api 读取目录，不需要密钥，也不会调用收费网关。run_api，以及 `/mcp` 上的业务工具，需要调用方自己的 OpenAny API key。

没有携带任何凭证时，服务在本地拒绝，不访问网关。错误码是 `openany_mcp_credentials_missing`，`execution_state` 为 `not_started`。到 https://openany.si/dashboard?page=api-keys 创建密钥，然后在 MCP 客户端的安全密钥或连接器设置里配置 `Authorization: Bearer <key>`。不要让用户把密钥粘贴进聊天或提示词。也接受非空的 `X-Api-Key` 或 `Xi-Api-Key`。

密钥存在但被网关拒绝时，保留网关错误码、HTTP 状态和 request_id。线上无效密钥是 `openany_key_invalid`。同一说明也用于 `openany_authentication_invalid`，以及 `openany_key_expired`、`openany_key_revoked`、`openany_authentication_expired`、`openany_authentication_revoked`。这表示密钥无效、过期或已撤销。在客户端的安全设置里更换密钥，不要把新密钥发到聊天里。其他网关错误仍用原来的通用说明。

本服务没有 OAuth。OpenAny 也没有 MCP URL elicitation 完成页。客户端即使声明了 URL elicitation，缺少密钥时也只返回上述文字说明，不会用表单 elicitation 收集密钥。表单模式不能用来收集密钥。网站若要支持 URL 模式，需要一个 HTTPS 页面：用掩码输入收集密钥，把密钥交给 MCP 客户端（而不是模型）并写进后续请求的 Authorization 头，再向该客户端发送 `notifications/elicitation/complete`。本服务是无状态的，不保存密钥，因此完成通知本身不会让下一次调用带上密钥。

## 选择入口

| 路径 | 内容 |
| --- | --- |
| `/` | 与 `/mcp-lite` 相同的轻量服务。MCP POST 走工具协议；浏览器 GET 返回本文 |
| `/mcp-lite` | openany_overview、search_apis、get_api、run_api，与 `/` 相同 |
| `/mcp` | 所有独立 API 工具，不含模型，适合已有精确调用配置 |
| `/mcp/{provider_id}` | 指定供应商的独立工具 |
| `/docs/mcp` | 本接入文档，轻量功能启用后公开提供 |

通常只配置一个入口。把全量入口同时连接到 Agent，仍会加载完整目录，而且那里没有模型。

在 Grok Bot 或其他支持远程 MCP 的客户端中，填写服务器 URL、Streamable HTTP 和 Authorization 请求头。不同客户端的配置格式不同；这里的 HTTP 联调不代表所有版本的客户端 UI 都已经验证。新配置后重新连接并检查四个工具是否出现。

## Agent 的调用顺序

用户问 OpenAny 有什么时，先调用 openany_overview，按类别介绍数据 API，按模态介绍 AI 模型，并给出两边的数量。然后：

1. search_apis 搜索需要的能力。
2. get_api 查看选中 API 或模型的完整输入 Schema、价格、调用限制和版本。
3. 向用户确认 get_api 返回的当前价格。
4. run_api 执行一次业务请求。

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

### openany_overview

无参数。读取当前目录快照，不调用收费网关。返回 `api_count`、`model_count`、按供应商和人类类别分组的 API，以及按主模态分组的模型 id。类别为 social/content、crypto/on-chain、company & leads、SEO/search/scraping、finance/macro、legal、e-commerce/real estate/travel、other。模型模态为 chat/reasoning、image、video、audio；没有文本输出、图像、视频或音频的模型归入 other。数量来自当前快照，不要在文档或提示词里写死。`models_available=false` 时模型数量不能当成产品里没有模型。

### search_apis

`query`：最多256个字符的业务关键词。还可按 `kind`、`provider`、`category`、`data_platform`、`data_product`、`capability` 筛选。`kind` 取 `api`、`model` 或 `all`，默认 `all`。`kind=api` 只返回 API，排序与只索引 API 时一致。至少提供关键词或一个筛选条件；只传 `kind=model` 时按模型 id 浏览。筛选值使用返回的实际标识，capability 接受 ID 或 key。`limit` 默认5、最多10。

每个候选包含 `kind`（`api` 或 `model`）、api、title、provider、summary、method、path，以及已有分类、平台、产品、能力、purpose、match_reason、required_inputs_brief 和已知 limitations。模型候选另有 `model_id`。目录里实际有的 `modalities`、`context_length`、`pricing_brief` 才会出现；没有的字段不会编造。模型分类固定包含 `models` 和 `llm`，`category=ai-generation` 也会命中模型。`capability=chat` 只命中目录模态或特性里包含 chat 的模型。match_reason 说明命中的字段和词，不代表供应商质量评分。缺失的 API 分类和能力不会凭空生成。返回摘要不包含完整 Schema。

模型名或厂商品牌词（例如 deepseek、qwen、claude、anthropic）会把模型排在前面。任务词把模型和相关 API 混排。翻译、写作、代码、对话，以及 translate、write、code、chat、completion，只命中输出模态包含 chat、coding 或 reasoning 的模型。输入模态里的 chat 只表示提示词通道，统一的 `/v1/chat/completions` 路径也不是聊天模型信号，生图和视频模型不会因此命中这些词，也不会排在文本模型前面。看图和 vision 命中接受图像且输出文本的模型。画图、生图和 image generation 命中输出图像的模型。支持有限中英文别名，不是通用语义搜索。`models_available=false` 表示还没有成功读取过模型目录；`models_stale=true` 表示最近一次刷新失败，结果仍来自上一份成功目录。

```json
{"query":"帮我查询企业工商信息","limit":5}
```

ranking 为 keyword：同时检索接口名、说明、路径、参数名、已有翻译和分类，支持有限中英文别名和词形归并。明确提及供应商时优先匹配该供应商，随后优先考虑标题/能力中的核心业务概念覆盖，再考虑最新、热点、历史、批量等意图及字段相关性；说明中对其他接口的交叉引用只作为弱信号。相同词在多个字段重复出现不会反复累加，英文完整词优先，避免 ai 误匹配 chain。时间意图帮助选择接口，不会自动填写业务日期或时间窗口。识别到查询意图时，避开明显的删除/创建等路由；这只是搜索排序辅助，仍须阅读 API 的实际语义。它不是通用语义搜索，不保证任意中文业务表述都命中。搜索不到时尝试实际接口名、供应商标识或更具体的英文词。

metadata_available=false 表示分类、翻译或示例辅助读取暂不可用；此时可继续按原有接口名、描述或供应商搜索，分类筛选可能找不到结果。翻页携带 next_cursor，保持 query、全部筛选条件和 limit 一致；目录、排序版本更新或游标失效后重新搜索。

### get_api

必填 `api`。API 返回精确的 `input_schema`、公开 `pricing`、`api_version`、描述、方法、公开路径和 API ID。参数说明可能包含暂不支持的可选数组限制。

模型的 `kind` 为 `model`。`input_schema` 是 chat completions 形状：必填 `messages`，每项需要 `role` 和 `content`，其余字段可以附加。`model` 可省略，省略时执行阶段会填入 `model_id`；如果传入，必须等于该 `model_id`。`stream: true` 会在本地拒绝。`pricing` 是模型目录原始价格，`pricing_brief` 只在目录给出非零价格时出现。目录里的 prompt、completion、input_cache_read 按每个 token 的美元计价，`pricing_brief` 把这三项乘以 1000000 后写成每百万 token；image、request、duration 按目录原值展示。展示价格不是锁价。

`input_schema` 与现有直连工具的参数 Schema 一致。保留 body、query、path、header 参数原有的顶层参数名和 Schema 引用结构，不要自行重组为另一种参数格式。API ID 用于排查问题；执行仍传 api。

新增的调用指导包括：

| 字段 | 含义 |
| --- | --- |
| required_inputs | 声明为必填的 JSON Pointer 路径、类型和已有说明；scope=request 是顶层必填，if_parent_is_object/each_present_item 是对象或数组元素存在时的要求 |
| examples | 从已有公开 HTTP 请求样例转成 MCP input，保留可选业务字段；每个样例必须同时通过当前 Schema 和 HTTP 编码器，模板还需替换占位符 |
| price_summary | 精确美元字符串、计价单位和收费条件提示，is_quote 始终为 false；rules 模式仍需阅读完整规则 |
| response_hint | 公开响应样例的结构、顶层字段或数组长度；supported_media_types 只说明 MCP 支持的响应类型，不保证本 API 的实际响应格式；不返回原始响应样例值 |

examples.status 为 available、template_only、missing、stale 或 no_safe_valid_sample。items[].kind 区分 example（完整公开样例）、template（占位模板）和 empty_request（校验通过的空请求）。模板通过 placeholder_paths 指明待替换字段（含字符串内部的 ${...}、{{...}}）；占位路径超过40项时放弃该样例，避免遗漏替换提示。模板不能原样调用；template_only 表示尚无完整请求样例。没有完整样例但 input {} 通过当前 Schema 和 HTTP 编码校验时，提供 empty_request；不会用空请求掩盖过期样例。最多展示三个条目，完整或空请求样例优先于模板。服务最多读取每接口前三个、单个请求不超过16KiB的公开样例；缺少可用样例时不编造公司名、地址、Token 或其他业务参数。认证请求头不转成 input；发现凭证形态的声明参数则放弃该样例。示例校验范围由 validation=mcp_schema_and_http_encoding、supplier_verified=false 标明：它证明 MCP 能接收和编码请求，不能证明上游数据、权限、额外业务规则一定满足。详情仍以 input_schema 为准，条件分支和跨字段约束不能只看必填摘要。

### run_api

必填 `api`、`api_version`、`input`。API 的 input 是一个符合刚读取的 input_schema 的对象，也就是原直连工具的 arguments。大整数按 JSON 数值准确传入。

模型调用使用同一组参数。服务把 input 发到模型源站的 `POST /v1/chat/completions`，并附上本次请求的调用方凭证。未写 `model` 时使用目录中的 `model_id`。这可能产生费用。`stream: true` 不会发出请求。超时后的执行和扣费状态同样视为未知，不要自动重试。

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

openany_overview、search_apis、get_api 读取目录，不会执行收费调用。run_api 对 API 和模型都可能产生费用和业务副作用。模型价格来自 `GET /v1/models`，不套用下面的 fixed、usage、rules 形状。

pricing.status=available 表示公开价格数据可读，不代表已经为当前参数报价，也不保证网关接受执行。

- fixed：price_micro_usd 是精确整数微美元字符串，结合 unit 和 charge_condition 理解收费条件。
- usage：还包含 unit_quantity、quantity_scale、rounding、usage_pointer 等用量口径，总费用可能执行后才能确定。
- rules：policy 包含公开 parameters 和 sale 规则，包括条件价、数量价、舍入与上限。
- unavailable：公开价格暂不可用，不能当成免费。

1 USD = 1,000,000 微美元。不要使用浮点数累加金额。展示价格不是锁价承诺，实际结算由网关决定。响应中未提供实际费用时，不要自行编造一个费用数字。

## 错误与重试

- openany_mcp_credentials_missing：请求没有密钥，网关未被调用。到 https://openany.si/dashboard?page=api-keys 创建密钥，并在客户端安全设置里配置 `Authorization: Bearer <key>`。不要把密钥贴进聊天。
- openany_key_invalid、openany_key_expired、openany_key_revoked、openany_authentication_invalid、openany_authentication_expired、openany_authentication_revoked：密钥已发送但被网关拒绝。保留原错误码、HTTP 状态和 request_id。在客户端安全设置里更换密钥，不要把新密钥贴进聊天。
- openany_mcp_api_changed：定义或展示价格变化，重新 get_api，再决定是否执行。
- openany_mcp_api_unavailable：接口不在当前可发布目录，重新搜索。
- openany_mcp_invalid_search：按错误说明修正 search_apis 的 query、筛选条件或 limit；游标失效时仅去掉 cursor，保留当前 query、筛选条件和 limit，从第一页重新调用 search_apis。未调用业务网关。
- openany_mcp_invalid_input：参数不符合 Schema，在本地被拒绝。details_scope=arguments 时按当前 MCP 工具的 inputSchema 和 error.details 修正工具参数；details_scope=input 时按 get_api.input_schema 修正业务 input，避免把搜索参数当成业务参数。
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

/readyz 中 ready 继续表示原目录状态；启用后还会显示 lite_ready、pricing_available 和 metadata_available。模型目录启用后另外显示 models_available、models_stale 和 models_published。价格、辅助资料或模型目录读取失败不让原目录失败。模型目录失败时保留上一份成功结果，并标记 models_stale。get_api 明确返回价格 unavailable 或缺少可用示例，search_apis/get_api 也报告 metadata_available。辅助资料刷新不改变原目录 hash；示例单独更新会刷新展示但不会让旧 api_version 失效。模型目录不进入 `/mcp` 的目录 hash。目录尚未完成轻量准备时，`/` 的 MCP 请求和 /mcp-lite 暂时返回503。浏览器 GET `/` 与 /docs/mcp 不依赖目录就绪。

模型目录默认从 `https://api.openany.si/v1/models` 读取，不发送 API key。可用 `OPENANY_MODELS_BASE_URL` 改成另一个源站，只接受 origin，或带 `/v1` 路径的同一 origin。进程需要能访问该源站。模型执行发往同一源站的 `/v1/chat/completions`，使用调用方自己的 key。不在服务配置里保存模型调用密钥。

生产环境需设置 MCP_PUBLIC_HOSTS；有浏览器 Origin 时配置 MCP_ALLOWED_ORIGINS。反向代理应放行 `/`、/mcp-lite 和 /docs/mcp，按现有要求透传调用方凭证并关闭业务重试。已有环境配置中的显式 false 会覆盖默认开启行为。

`/`、/mcp-lite 和全量入口共用同进程资源与业务并发额度。需要更强隔离时，用同一镜像单独部署轻量实例，再把 `/` 和 /mcp-lite 路由到该实例。
