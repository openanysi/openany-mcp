# OpenAny MCP 收录执行清单

核对日期：2026 年 11 月 6 日。提交对象是独立的 OpenAny MCP 托管服务，推荐入口为 `https://mcp.openany.si`。`/mcp-lite` 是同一轻量服务。项目方已确认源码仓库暂时私有，本轮使用官网和公开文档申请。先完成英文接入材料和用户接入验证，再申请 MCP 专门目录；Agent 和 Skill 清单按各自范围另行处理。

## 已确认的发布条件

| 项目 | 当前事实 |
| --- | --- |
| 官网 | https://openany.si/ |
| MCP 地址 | https://mcp.openany.si |
| 传输 | Streamable HTTP |
| 工具 | openany_overview、search_apis、get_api、run_api |
| 认证 | 业务执行使用调用者自己的 OpenAny Bearer Key；公开发现检查没有使用 Key |
| 公网发现验证 | initialize、tools/list、search_apis、get_api 均返回 HTTP 200；详见 verification.md |
| 英文材料 | connect.en.md，可用于发布英文 MCP 接入页面 |
| 现有 Skill | https://openany.si/SKILL.md 已公开，内容主要是 CLI 发现、执行和账户工作流；不能再笼统声称 OpenAny 没有 Skill |
| 代码仓库 | openanysi/openany-mcp 暂时私有；本轮不作为对外提交链接 |
| 许可口径 | 本轮申请托管服务，不声明开源、不选择代码许可证 |
| 联系邮箱 | 官网公开地址 team@openany.si，可填入提交表单；尚未验证该邮箱接收通知的流程 |
| Registry 查重 | 官方 API 本轮搜索 openany 返回 0 条；名称尚未注册或保留，发布前复查 |

本地实现比本轮公网发现结果包含更多功能。公网 run_api 尚未公布本地已有的 output 参数，search_apis 也没有公布新增分类筛选；本次英文接入材料以公网 Schema 为准。

## 执行顺序

1. 发布英文 MCP 接入页。connect.en.md 已填入正式地址；现有 `/docs/mcp` 仍为中文且含域名占位符，需要在对应部署中更新。
2. 验证新用户可以注册、创建自己的 Key 并完成接入。官网已有 Key 创建入口，但本轮没有注册账户、创建 Key 或验证收费调用。录制一个真实客户端的完整任务，保留脱敏后的结果和 request_id。
3. 并行准备 MCP 官方 Registry 和 Glama。Registry 使用 server.json 候选元数据；Glama 收录后记录实际 connector URL 和徽章，不能自行编造徽章路径。
4. 投递 mcpservers.org、TensorBlock、Backblaze 和 Farol。根据各站点要求选择表单、Issue 或 PR，使用 submissions.md 中对应文案。
5. 获得 Glama connector 后，再投递 punkpeye 的远程 MCP 清单。
6. 如要推广源码，先决定代码公开范围和许可证，再补充外部用户可复现的部署说明；当前服务需要 OpenAny 目录数据库和网关，公开源码不等于普通用户已经能独立安装运行。
7. Skill 路线复用现有公开 Skill，先核对许可、公开源码地址、真实客户端测试与社区使用证据。不要为申请 Skill 收录重复创建一个仅含 MCP URL 的包。

## 首批目标

以下顺序根据匹配度和前置依赖安排，不代表流量或收录概率排名。提交前再次查重并阅读最新规则。

| 目标 | 提交方式 | 当前下一步 |
| --- | --- | --- |
| [MCP 官方 Registry](https://modelcontextprotocol.io/registry/remote-servers) | 发布远程 server.json 元数据 | 确定名称及版本，完成命名空间所有权验证；无需改写 Go 或发布 npm 包 |
| [Glama connectors](https://glama.ai/mcp/connectors) | 通过站点 Add Server 流程 | 填入正式 endpoint、文档和 API Key 要求，保存实际 connector 标识 |
| [punkpeye/awesome-remote-mcp-servers](https://github.com/punkpeye/awesome-remote-mcp-servers/blob/main/CONTRIBUTING.md) | README PR | 公网 initialize 已验证；补 Glama 徽章及开放注册证据。使用 🔑 标记，分类内按名称排序；当前规则还要求提交账号 Star 仓库 |
| [wong2/awesome-mcp-servers](https://github.com/wong2/awesome-mcp-servers) | [mcpservers.org 表单](https://mcpservers.org/submit) | 不向仓库提 PR；表单需要联系人邮箱，先使用免费提交选项即可 |
| [TensorBlock/awesome-mcp-servers](https://github.com/TensorBlock/awesome-mcp-servers) | docs 分类 PR 或 [Add MCP server Issue](https://github.com/TensorBlock/awesome-mcp-servers/issues/new?template=add-mcp-server.yml) | 优先填写结构化 Issue；分类、接入、认证、文档及工具信息见 submissions.md。源码和许可必须如实填写 |
| [backblaze-labs/awesome-agent-infrastructure](https://github.com/backblaze-labs/awesome-agent-infrastructure/blob/main/CONTRIBUTING.md) | 仅修改 entries.yaml，一条目一个 PR | 分类 tool-protocols-and-servers 已核实；提供公开文档、开发者接入路径及近 12 个月维护信号 |
| [farol-team/awesome-agent-services](https://github.com/farol-team/awesome-agent-services/blob/main/contributing.md) | README 表格 PR 或推荐 Issue | 适合 Tools & Integrations；解释 Agent 如何直接发现、检查和执行 API，披露项目方身份 |

## 原有清单中需要调整的目标

| 目标 | 处理决定及依据 |
| --- | --- |
| [appcypher/awesome-mcp-servers](https://github.com/appcypher/awesome-mcp-servers/blob/main/CONTRIBUTING.md) | 移除提交队列。仓库已于 2026 年 8 月 1 日归档，只读 |
| [michielhdoteth/awesome-ai-agent-tools](https://github.com/michielhdoteth/awesome-ai-agent-tools/blob/main/contributing.md) | 本轮暂缓。规则要求有效公开 GitHub 链接和接入命令，私有仓库不满足。以后投递须改 mcps/catalog.json 并运行其验证脚本，不手改生成的 README |
| [junminhong/awesome-agent-skills](https://github.com/junminhong/awesome-agent-skills/blob/main/CONTRIBUTING.md) | 本轮暂缓。虽允许 MCP 归入 Tooling & Integrations，但要求公开源码、文档及许可；还须中英文 README 同步、披露关联关系、所有提交显示 Verified |
| [e2b-dev/awesome-ai-sdks](https://github.com/e2b-dev/awesome-ai-sdks) | 次优先级。范围也包含 Agent 工具；公开接入条件齐备后可评估，不能把 OpenAny 描述成尚不存在的 SDK |
| [kyrolabs/awesome-agents](https://github.com/kyrolabs/awesome-agents/blob/main/CONTRIBUTING.md) | 暂缓。要求开源、英文、维护和真实使用积累；还须符合 Agent 框架生态范围 |
| [e2b-dev/awesome-ai-agents](https://github.com/e2b-dev/awesome-ai-agents) | 当前不投。其定位是自主 Agent，OpenAny MCP 本身是工具服务 |
| [VoltAgent/awesome-agent-skills](https://github.com/VoltAgent/awesome-agent-skills/blob/main/CONTRIBUTING.md) | 后续用真实 Skill 申请。需要公开仓库、可用 Skill 和真实社区使用记录，新建后立即投不符合要求；条目描述不超过 10 个英文单词 |
| [travisvn/awesome-claude-skills](https://github.com/travisvn/awesome-claude-skills/blob/main/CONTRIBUTING.md) | 本次不投。规则限制商业 API 包装型 Skill、AI 辅助提交，并设有至少 10 Stars 的自动关闭条件；现有商业服务接入 Skill 与其范围冲突 |
| [ComposioHQ/awesome-claude-skills](https://github.com/ComposioHQ/awesome-claude-skills/blob/master/CONTRIBUTING.md) | 暂缓。需要有实际测试的 Skill；页面当前还显示仅协作者能创建 PR，不能承诺普通 Fork PR 可提交。后续先核实维护者开放的入口 |
| [punkpeye/awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers/blob/main/CONTRIBUTING.md) | 独立部署版本就绪后再考虑。规则面向有公开 GitHub 仓库、可安装运行的服务；纯托管 URL 应投其远程清单 |

## 发布前待办

- [ ] 把英文 MCP 指南发布到稳定公共 URL，并更新中文指南中的域名占位符。
- [ ] 表单联系人使用官网公开的 team@openany.si，确认团队能收到维护者反馈。
- [ ] 新用户完成公开注册和 Key 创建；至少一个真实产品客户端完成连接、三工具发现和授权范围内的完整任务。
- [x] 本轮采用托管服务路线，代码仓库暂时私有；现有 Skill 的可再分发许可留待 Skill 路线单独处理。
- [ ] 官方 Registry 的候选名为 `si.openany/openany`；先核对是否已有登记，并按官方文档完成 openany.si 所有权验证。
- [ ] 获得实际 Glama connector URL 和 badge，替换 submissions.md 的占位符。
- [ ] 提交前检查已合并条目、打开的 PR 和 Issue；每个目标只保留一条申请。

## 材料与进度

| 文件 | 用途 |
| --- | --- |
| [connect.en.md](connect.en.md) | 英文托管 MCP 快速接入指南 |
| [submissions.md](submissions.md) | 网站表单、Issue、PR 条目和说明文案 |
| [server.json](server.json) | 官方 Registry 候选元数据；尚未发布 |
| [verification.md](verification.md) | 本轮验证结论及未覆盖范围 |
| [public-probe.json](public-probe.json) | 无凭证公网发现检查的原始 JSON 记录 |

所有目标当前均为“材料准备”，未提交、未收录。提交后记录目标、日期、申请链接、CI 结果、维护者反馈和最终公开条目 URL；PR 合并后仍需确认目录网站已更新。
