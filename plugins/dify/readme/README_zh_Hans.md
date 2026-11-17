# OpenAny Dify 工具插件

The B2A (business to agent) marketplace：一个 MCP 端点接入 3,200+ API 和 168+ AI 模型。API 覆盖社交内容（X、TikTok 等）、SEO 与搜索、加密货币与链上数据、企业工商与销售线索、金融宏观和法律；模型包括 GPT、Claude、Gemini、DeepSeek、Qwen、Kimi，以及 Midjourney、FLUX、Veo、Kling 等图像和视频模型。浏览、搜索、查价免费，只有 `run_api` 需要 API Key，按次付费。

这是 Dify 工具插件，不是模型供应商插件。端点固定为 `https://mcp.openany.si`（根路径，不是 `/mcp` 或 `/mcp-lite`）。

工具：`openany_overview`、`search_apis`、`get_api`、`run_api`。前三个不需要 Key，也不走付费网关。`run_api` 使用 Dify 提供方密钥 `openany_api_key`，该字段是 `secret-input` 且不是必填，也不要把它当成工具参数。Key 在 https://openany.si/dashboard?page=api-keys 创建。保存密钥时只在本地检查格式，不会发起付费调用。

安装与英文说明见仓库内 `plugins/dify/README.md`。支持联系 team@openany.si。制作者：Pistis Wang / OpenAny。
