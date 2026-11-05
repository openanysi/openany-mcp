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

`query`：最多256个字符的关键词；`provider`：可选供应商标识。至少提供一个。`limit` 默认5、最多10。返回 api、title、provider、summary、method、path、total、ranking 和 next_cursor。返回摘要不包含完整 Schema。

第一版 ranking 为 keyword，支持有限的中英文别名，不承诺通用语义搜索。搜索不到时尝试接口名、供应商名或更具体的英文关键词。翻页时携带 next_cursor，并保持 query、provider、limit 一致。游标过期时重新搜索。

### get_api

必填 `api`。返回精确的 `input_schema`、公开 `pricing`、`api_version`、描述、方法、公开路径和 API ID。参数说明可能包含暂不支持的可选数组限制。

`input_schema` 与现有直连工具的参数 Schema 一致。保留 body、query、path、header 参数原有的顶层参数名和 Schema 引用结构，不要自行重组为另一种参数格式。API ID 用于排查问题；执行仍传 api。

### run_api

必填 `api`、`api_version`、`input`。input 是一个符合刚读取的 input_schema 的对象，也就是原直连工具的 arguments。大整数按 JSON 数值准确传入。

参数、版本、接口状态检查通过后，服务按原有编码规则向固定网关发出业务请求。返回 http_status、request_id、data 等原有结果信息；工具错误包含 code、stage、execution_state。MCP 不自动重试业务调用。

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
- execution_state=unknown：请求可能已经执行或扣费，不要因为超时就重复提交。保存 request_id 供排查。

api_version 包含该接口的 revision、Schema、公开价格和转发规则，只用于发现已知定义变化，不是认证凭证、幂等键或网关锁价令牌。本版本没有通用的自动去重或长任务恢复工具。结果大小限制沿用现有服务；业务结果过大时不会伪装成截断成功。

## 部署与自检

轻量入口默认开启，更新二进制后重启服务即可使用。需要关闭时设置 `MCP_LITE_ENABLED=false`；关闭后不加载轻量价格和搜索索引，原有入口照常工作。数据库角色仍为只读。价格使用独立刷新任务和最多1个连接的独立只读连接池，避免占用原目录连接池；数据库连接配额应预留这个额外连接。

公开价格辅助查询需要 agent_apis 的公开计费字段、收费条件字段，以及 agent_api_pricing_policies 的 api_id、policy_version、parameters、sale_policy 只读权限。无需读取 supplier_policy 或 supplier_price_micro_usd。由数据库管理员按最小列权限授权；不需要新增业务表或改名。

```sh
./scripts/run-local.sh --check-catalog
./scripts/run-local.sh
```

变量未设置或为空时默认开启。如果 .env 中已显式设置为 false，仍会关闭；删除该配置或改为 true 即可开启。源码开发也需要先加载环境变量。run-local.sh 运行 bin/openany-mcp，修改代码后先重新构建。

/readyz 中 ready 继续表示原目录状态；启用后还会显示 lite_ready 和 pricing_available。价格辅助读取失败不让原目录失败，get_api 明确返回价格 unavailable。目录尚未完成轻量准备时，/mcp-lite 暂时返回503。/docs/mcp 不依赖目录就绪。

生产环境需设置 MCP_PUBLIC_HOSTS；有浏览器 Origin 时配置 MCP_ALLOWED_ORIGINS。反向代理应放行 /mcp-lite 和 /docs/mcp，按现有要求透传调用方凭证并关闭业务重试。已有环境配置中的显式 false 会覆盖默认开启行为。

三个入口共用同进程资源与业务并发额度。需要更强隔离时，用同一镜像单独部署轻量实例，再把 /mcp-lite 路由到该实例。
