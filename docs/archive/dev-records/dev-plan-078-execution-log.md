# DEV-PLAN-078 执行日志

> 目的：记录 DEV-PLAN-078 的落地变更与可复现验证入口（命令/时间/样本/结果/提交号）。

## 变更摘要
- [x] 删除 replay 相关函数/入口与错误码映射。
- [x] 修正/撤销改为增量区间运算（无 replay）。
- [x] 审计链收敛到 `org_events`；删除 `org_events_audit` 与 `org_event_corrections_*`；完成表级重基线。
- [x] 回归与一致性测试通过（替代 replay 对照）。
- [x] 最小 E2E 样板通过（tp060 + m3-smoke）。

## 环境与样本
- 环境：开发早期测试环境（可清库重建）。
- 样本规模：E2E 自动生成（见 `make e2e` 运行日志/报告）。

## 本地验证
- 2026-02-09 06:23 UTC：`make orgunit plan && make orgunit lint && make orgunit migrate up && make orgunit plan`
  - 结果：通过
- 2026-02-09 06:23 UTC：`rg -n "replay_org_unit_versions|ORG_REPLAY_FAILED" modules/orgunit migrations/orgunit internal/server`
  - 结果：有命中（已清理，见 11:13 记录）
- 2026-02-09 06:23 UTC：`go fmt ./... && go vet ./... && make check lint && make test`
  - 结果：通过
- 2026-02-09 11:13 UTC：`rg -n "org_events_audit|org_event_corrections_" modules/orgunit/infrastructure/persistence/schema internal/sqlc/schema.sql internal/server`
  - 结果：无命中
- 2026-02-09 11:13 UTC：`rg -n "replay_org_unit_versions|ORG_REPLAY_FAILED" modules/orgunit/infrastructure/persistence/schema internal/sqlc/schema.sql internal/server`
  - 结果：无命中
- 2026-02-09 11:13 UTC：`make check no-legacy`
  - 结果：通过
- 2026-02-09 11:13 UTC：`make check doc`
  - 结果：通过
- 2026-02-09 11:13 UTC：`make check lint && make test`
  - 结果：通过
- 2026-02-09 11:13 UTC：`make e2e`
  - 结果：通过

## 指标与验收
- replay 已彻底删除（代码/DB 入口均不可调用）：通过（rg 无命中）
- M4 性能与稳态验收：取消（按指令）

## 证据与提交号
- Commit/PR：PR #316（merge commit `9ab27e3`）
- 关联说明：审计链收敛到 `org_events`，对齐 DEV-PLAN-080；完成 078C/078D/078E 验收
