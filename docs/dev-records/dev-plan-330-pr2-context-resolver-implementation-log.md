# DEV-PLAN-330 PR-2 记录：Context Resolver 单点落地

**状态**: 已完成（2026-04-11 14:34 CST）

## 1. 范围

1. [X] 在 `internal/server` 建立统一 `Context Resolver`，把 `business_unit_org_code -> business_unit_node_key -> resolved_setid -> setid_source` 收敛为正式边界。
2. [X] 将 explain、`/internal/rules/evaluate`、OrgUnit 字段启用候选 / field options 的重复解析逻辑切到同一 Resolver。
3. [X] 为 `Context Resolver` 增加独立测试，证明该边界不是“文档概念”，而是可直接验收的运行时组件。
4. [X] 保持当前接口的错误优先级与对外行为不倒退，不在 PR-2 顺手切 schema、主查询或 PDP。

## 2. 代码交付

1. [X] 新增统一 Resolver：
   [setid_context_resolver.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_context_resolver.go)
2. [X] explain 链路改为复用统一 Resolver，并补充 `setid_source` 输出：
   [setid_explain_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_explain_api.go)
3. [X] `/internal/rules/evaluate` 改为复用统一 Resolver，并在内部上下文中显式带出 `setid_source`：
   [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go)
4. [X] OrgUnit 字段启用候选与 field options 预览链路改为复用统一 Resolver，同时保留原错误返回口径：
   [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go)
5. [X] 新增 Resolver 独立测试：
   [setid_context_resolver_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_context_resolver_test.go)

## 3. 实施要点

### 3.1 统一 Resolver 输出已固定

1. [X] `ResolvePolicyContext(...)` 输出以下正式字段：
   - `business_unit_org_code`
   - `business_unit_node_key`
   - `resolved_setid`
   - `setid_source`
2. [X] `ResolveOrgContext(...)` 提供可复用的 org 级上下文解析，供 explain / preview / internal evaluate 在不改 schema 的前提下复用。

### 3.2 当前 `setid_source` 归类规则已单点化

1. [X] `DEFLT` → `deflt`
2. [X] `SHARE` → `share_preview`
3. [X] 其他非空 SetID → `custom`

### 3.3 错误优先级保持不变

1. [X] 对 `org_code` 非法 / 不存在的场景，仍优先返回 `org_code_invalid` / `org_code_not_found`，而不是被 `setid_resolver_missing` 抢先覆盖。
2. [X] 对 SetID 无法解析或返回空值的场景，仍维持现有 handler 口径：
   - explain / internal evaluate：继续按上下文不匹配拒绝
   - field metadata preview：继续返回 `setid_missing` 或底层稳定错误码
3. [X] PR-2 只收解析边界，不切主查询、不切双轴 schema、不替换 PDP。

## 4. 验证

1. [X] `go test ./internal/server`
2. [X] `go vet ./...`
3. [X] `make check lint`
4. [X] `make test`

## 5. 对 PR-3 的冻结前提

1. [X] `Context Resolver` 已存在且有独立测试，因此 `PR-3` 可以直接承接双轴 schema 与历史回填，不再需要先补解析层。
2. [X] `PR-3` 之前仍不得声称“双轴主查询已完成”，因为当前 Registry schema / unique key / 主查询仍未纳入 `resolved_setid`。
3. [X] `PR-4` 之前仍不得声称“唯一 PDP 已完成”，因为 `internal/server` 与 `modules/orgunit/infrastructure` 的平行 PDP 仍未合并。
