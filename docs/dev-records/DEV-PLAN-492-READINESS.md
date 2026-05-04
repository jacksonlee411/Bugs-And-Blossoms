# DEV-PLAN-492 Readiness

## 说明

- 本文件记录 `DEV-PLAN-492` 的阶段性实施进展与验证结果，避免计划文档与实际代码状态漂移。
- 当前记录已完成且已验证的后端先行切片；前端 selector 消费、组织管理页读取规则进一步下沉、SQL 级 scoped pagination 优化与联合 E2E 仍保持待办。

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
- PR-3 已将默认 children HTTP 与 search HTTP 迁移到 `OrgUnitReadService`：
  - children 响应通过 ReadService 统一返回 `org_node_key` 与 `has_visible_children`
  - children 请求对范围外 parent fail-closed，不回退物理父节点展开
  - search 通过 ReadService 处理 scope-aware candidates 与 safe `path_org_codes`
  - search 命中深层节点时 `path_org_codes` 从当前 visible root 开始，不泄露范围外祖先
  - `all_org_units=true` + pagination 的 HTTP 响应 total 已通过全量候选过滤后再分页，保持对外 scope 裁剪语义一致

### 已验证

- `go fmt ./modules/orgunit/services ./internal/server`
- `go test ./modules/orgunit/services ./internal/server`
- `go vet ./...`
- `make check lint`
- `make test`

### 当前结论

- `492 P1/P2/P3` 的首轮后端 contract 已落地：roots、children、search 均已通过 ReadService 对外提供 selector-ready DTO 与 scope-aware safe path。
- 目前尚未完成的工作仍包括：
  - list/grid 的 SQL 级 scoped pagination 优化；当前 HTTP 层已保证对外 total/page 以 scope 裁剪后的结果为准
  - 组织管理页局部读取规则继续向 492 ReadService 下沉
  - 前端 `OrgUnitTreeSelector` / facade 消费
  - 491/492 联合 E2E
