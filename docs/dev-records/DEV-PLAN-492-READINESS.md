# DEV-PLAN-492 Readiness

## 说明

- 本文件记录 `DEV-PLAN-492` 的阶段性实施进展与验证结果，避免计划文档与实际代码状态漂移。
- 当前仅记录已完成且已验证的后端先行切片；未完成的 children/list/grid/search HTTP 全量迁移、前端 selector 消费与联合 E2E 仍保持待办。

## 2026-05-04 实施记录

### 已完成

- 已新增 `modules/orgunit/services` 的 `OrgUnitReadService` 骨架、request/response 类型、scope filter 和 store port。
- 已补 `modules/orgunit/services` fake-store 单测，覆盖：
  - visible roots
  - 中段 scope 的 visible roots 去重
  - scope-aware children
  - search safe path
  - resolve 范围外 fail-closed
- 已在 `internal/server` 增加 ReadService adapter，并将默认 `GET /org/api/org-units` roots 语义切到 scope-aware visible roots。
- 已把 list DTO 的 `org_node_key` 从隐藏字段改为响应字段，并新增 `has_visible_children`。
- 已补 handler contract 测试，覆盖：
  - roots 响应暴露 `org_node_key`
  - roots 响应暴露 `has_visible_children`
  - 授权范围位于树中段时返回 visible root，而不是物理 root

### 已验证

- `go fmt ./modules/orgunit/services ./internal/server`
- `go test ./modules/orgunit/services ./internal/server`
- `go vet ./...`
- `make check lint`
- `make test`

### 当前结论

- `492 P1` 的 ReadService 骨架已落地，`P2` 中的 visible roots 与 selector-ready DTO 已完成首轮后端切片。
- 目前尚未完成的工作仍包括：
  - children / list-grid / search HTTP 全量迁移到同一 read core
  - `total` / pagination 的全量 scope 收敛
  - 前端 `OrgUnitTreeSelector` / facade 消费
  - 491/492 联合 E2E
