# DEV-PLAN-015D：Staffing Assignment 分层回流修复（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 08:42 CST）

## 背景

`DEV-PLAN-015A` 已确认当前存在一个明确的分层回流点：

- `modules/staffing/infrastructure/persistence/assignment_pg_store.go` 直接依赖 `modules/staffing/services`

该模式违反 `DEV-PLAN-015` 对 DDD 分层的目标叙事，也属于 `DEV-PLAN-015B` 在 `P1` 中明确要清理的“`infrastructure -> services` 反向依赖”。

本计划用于将 Assignment 相关的输入准备、JSON canonicalize、确定性事件 ID 生成逻辑，从 `services` 下沉到 `domain` 层，消除当前最典型的历史回流点。

## 目标与非目标

### 目标

1. [ ] 将 Assignment Prepare/Canonicalize 逻辑从 `modules/staffing/services` 下沉到 `modules/staffing/domain`。
2. [ ] 清理 `modules/staffing/infrastructure/persistence -> modules/staffing/services` 的反向依赖。
3. [ ] 保持现有外部行为与错误语义不变。

### 非目标

1. [ ] 本计划不处理 `internal/server` 的整体瘦身。
2. [ ] 本计划不一次性重构所有 staffing 边界，只处理 Assignment 这一条最明确的回流点。
3. [ ] 本计划不扩大到其他模块的类似问题。

## 实施步骤

1. [X] 新建 `015D` 文档，冻结本次收口范围。
2. [X] 将 Assignment 规则准备逻辑迁移到 `modules/staffing/domain/assignmentrules`。
3. [X] 更新 `services`、`infrastructure`、`internal/server` 调用点，移除对 `services/assignment_rules.go` 的依赖。
4. [X] 删除旧的 `modules/staffing/services/assignment_rules.go`。
5. [X] 执行最小验证：`go test ./modules/staffing/...`、`go test ./internal/server/...`、`make check lint`（2026-04-09 08:42 CST，本地通过）

## 测试与覆盖率

覆盖率与门禁口径遵循仓库 SSOT：

- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`

本计划命中的最小验证为：

1. [ ] `go test ./modules/staffing/...`
2. [ ] `go test ./internal/server/...`
3. [ ] `make check lint`

## 验收标准

1. [ ] `modules/staffing/infrastructure/**` 不再 import `modules/staffing/services`。
2. [ ] Assignment 相关规则准备逻辑已迁入 domain 层。
3. [ ] 现有测试与 lint 通过。
