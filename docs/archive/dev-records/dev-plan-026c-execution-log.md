# DEV-PLAN-026C 执行日志

> 目的：记录 DEV-PLAN-026C 的修订落地与核实结论。

## 变更摘要

- 026B 对齐：补充 `org_unit_codes` 时间字段语义（非业务/审计时间），明确占位 org_code 规则与迁移清单标记（`ZZ-<org_id>` + `source=placeholder`）。
- 修复 SetID UI 输入链路：`org_code` 不再提前 Trim，确保首尾空白按 `org_code_invalid` 语义处理。
- N+1 风险收敛：Positions API 列表已改为批量解析 `org_code`（去重后调用一次 `ResolveOrgCodes`），避免按 `org_unit_id` 循环查询；SQL 形态见下文证据。
- 未新增专门针对 “SetID UI 首尾空白” 的测试用例（现有 `org_code invalid` 覆盖仍通过）。

## N+1 证据记录（列表/批量请求）
- 触发点：`internal/server/staffing_handlers.go` 的 Positions API 列表，先去重 `org_unit_id` → `[]int`，再调用一次 `ResolveOrgCodes`。
- SQL 形态：`pkg/orgunit/resolve.go` 中的批量查询  
  `SELECT org_id, org_code FROM orgunit.org_unit_codes WHERE tenant_uuid = $1::uuid AND org_id = ANY($2::int[])`。
- 调用次数：每次 Positions 列表请求最多 1 次批量查询（与 `org_unit_id` 数量无关）。

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
