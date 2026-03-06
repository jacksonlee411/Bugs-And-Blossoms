# DEV-PLAN-075D 执行日志

## 基本信息
- 计划文档：`docs/archive/dev-plans/075d-orgunit-status-field-active-inactive-selector.md`
- 完成日期：2026-02-07
- 当前状态：已完成

## 阶段交付

### P0 契约冻结
- PR #304（已合并）
- 冻结 075D 的可达性契约（`include_disabled`）、操作矩阵与 fail-closed 规则。

### P1 读路径与展示
- PR #305（已合并）
- 交付内容：
  - 详情/树/搜索支持 `include_disabled`。
  - 详情状态从硬编码改为动态展示（有效/无效）。
  - disabled 记录在详情场景下可达并可恢复。

### P2 写路径对称
- PR #306（本日志对应提交）
- 交付内容：
  - Service 层补齐 `Enable(ctx, tenantID, req)`。
  - Internal API 新增 `POST /org/api/org-units/enable`。
  - routing allowlist 与 authz 映射接入 enable 路由。
  - OrgUnit 记录动作新增 `action=change_status`，后端基于真实当前状态判定 `DISABLE/ENABLE`。
  - UI 增加“状态变更”入口与目标状态下拉，提交文案按目标状态动态显示“启用/停用”。

### P3 测试与门禁
- 补齐测试：
  - `modules/orgunit/services/orgunit_write_service_test.go`
  - `internal/server/orgunit_api_test.go`
  - `internal/server/handler_test.go`
  - `internal/server/authz_middleware_test.go`
  - `internal/server/orgunit_nodes_test.go`
  - `internal/server/orgunit_nodes_read_test.go`
- 本地门禁结果：
  - `go fmt ./...`
  - `go vet ./...`
  - `make check lint`
  - `make test`（覆盖率 100%）
  - `make check routing`
  - `make authz-pack && make authz-test && make authz-lint`

## 验收结论
- 075D 验收目标已满足：
  - 页面可见真实状态字段；
  - 支持显式“有效/无效”双向切换；
  - disabled 记录可达并可恢复；
  - 写入保持 One Door（仅 `ENABLE/DISABLE` 事件）；
  - 未引入 legacy 双链路。
