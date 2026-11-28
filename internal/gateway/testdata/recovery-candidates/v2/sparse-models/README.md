# 网关 HTTP 导出的模型只读候选

这 10 份 JSON 来自网关 `tests/support/model_sparse_recovery.rs` 实际生成的错误响应，仅归一化 recovery_id 和 generated_at；来源 SHA256 及本地运行记录见仓库根目录 `docs/verification/evidence/2026-11-28-model-sparse-schema-fix/fixture-lineage.json`。

覆盖两个模型各自的错误 Provider、route_mode、组合、无可用源 Provider 后的同类型模型；另含无候选与关闭候选引擎两种路由纠正响应。

MCP 回归将这些响应作为本地模拟网关的 HTTP 返回，验证默认入口和 `/mcp-lite` 的错误事实、只读候选及 get_api 查阅；不会调用真实供应商，不自动执行候选，也不将此作为已上线验证。
