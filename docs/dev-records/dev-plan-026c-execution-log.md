# DEV-PLAN-026C 执行日志

> 目的：记录 DEV-PLAN-026C 的修订落地与核实结论。

## 变更摘要

- 026B 对齐：补充 `org_unit_codes` 时间字段语义（非业务/审计时间），明确占位 org_code 规则与迁移清单标记（`ZZ-<org_id>` + `source=placeholder`）。
- 修复 SetID UI 输入链路：`org_code` 不再提前 Trim，确保首尾空白按 `org_code_invalid` 语义处理。
- N+1 风险评估：Positions API 列表对每个唯一 `org_unit_id` 调用一次 `ResolveOrgCode`，存在潜在 N+1；建议后续用批量查询替代。
- 未新增专门针对 “SetID UI 首尾空白” 的测试用例（现有 `org_code invalid` 覆盖仍通过）。

## N+1 风险评估结论（简述）
- 触发点：`internal/server/staffing_handlers.go` 中列表返回时按唯一 `org_unit_id` 循环调用 `ResolveOrgCode`。
- 影响范围：Positions API 列表（按唯一 org_unit_id 数量线性增长）。
- 建议：新增批量解析接口或联表查询，一次性取回 org_code 映射。

## 本地验证

- 已通过（2026-02-02 23:46 UTC）：
  - `GOCACHE=/tmp/go-build-cache go fmt ./...`
  - `GOCACHE=/tmp/go-build-cache go vet ./...`
  - `GOCACHE=/tmp/go-build-cache GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make check lint`
  - `GOCACHE=/tmp/go-build-cache make test`
  - `make check doc`

## 待办/依赖

- 迁移样本统计仍待提供与记录（org_code 长度/字符集分布）。
- 026D（增量投射方案）按计划另行实施与验证。
