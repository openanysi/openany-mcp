# Hunter 与 Alchemy 的 MCP 请求编码核对

核对日期：2026-11-27。范围仅为请求编码、MCP 目录发布与本地测试；未发送真实供应商业务请求，未部署或改写数据库目录。

## Hunter Domain Finder

- 官方 [OpenAPI](https://hunter.io/openapi.json) 的 `/domain-finder` POST requestBody 明确列出 `application/x-www-form-urlencoded` 和 JSON，body 字段为 company/string、limit/integer、perfect_match/boolean，company 必填。路径前缀 `/v2` 由源站 base URL 提供。
- 官方 [API 参考](https://hunter.io/api-documentation/v2#domain-finder) 给出 company 至少3字符、limit为1–10及 perfect_match 为布尔值。此处记录契约事实，编码修复沿用现有目录的字段与校验规则，不变更参数或价格数据。
- 当前本地发布记录为 `552cf63d-c901-419b-bfb4-a5175d3b36d9`、revision 4、POST `/v1/agent-apis/hunter/v2/domain-finder`、`application/x-www-form-urlencoded`。因此 MCP 按已发布格式发送表单，不能擅自改发 JSON。
- 仅开放顶层非空字段名、非 nullable 的 string/integer/number/boolean 标量；从发布的单层 body pointer 取表单字段名。root/nested object、array、nullable、GET/HEAD form 及声明冲突 schema 保持隔离。没有推测嵌套表单或数组的编码。
- 使用标准 URL form 编码保留 Unicode、空格、加号、&、数字文本和 false。缺失的可选字段不发送；业务 body 不能覆盖认证头。

## Alchemy Token Prices By Symbol

- 官方 [Quickstart](https://www.alchemy.com/docs/reference/prices-api-quickstart) 的 By Symbol 示例显式用 `symbols.join(',')` 构造一个 query 值。官方 [接口参考](https://www.alchemy.com/docs/data/prices-api/prices-api-endpoints/prices-api-endpoints/get-token-prices-by-symbol) 将 symbols 定义为必填字符串数组，最多25项。
- 本地记录为 `fe26bda6-dc26-4423-81a6-94826c96f872`、revision 2、GET `/prices/v1/tokens/by-symbol`。本次添加 Provider+method+path 级 override，采用有上述官方示例依据的 CSV：`symbols=ETH%2CBTC`。该表示解码后为一个 `ETH,BTC` 值，不是 JSON 数组，也没有新增重复 query 键。
- 空数组拒绝；含 CSV 分隔符的单个元素拒绝，避免改变元素边界。现有 required/type 校验保留；合法25项的编码已测试。未更改目录参数数量约束或计费数据。
- 其他 Provider、方法、路径没有此默认策略；未审核的 query array 继续按原规则处理。依据记录保存在 `config/catalog-overrides.json` 的 source 中。

## 本地证据

1. `compat-codec-red.log` 先复现两条目录被隔离及表单正文未编码。
2. 编码/目录回归覆盖平坦表单、指针字段名、错误输入、复杂形状拒绝、CSV 和范围隔离。
3. Full/Lite RPC 经本机 HTTP 假网关读取实际 body/query/header：两接口各两个入口，共4次派发；非法输入0次派发，无真实供应商调用。
4. 本地完整API目录不筛除记录；全量与race日志及最新结果见根目录 `docs/verification/2026-11-27-agent-onboarding-phase2-report.md`。
