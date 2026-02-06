# DEV-PLAN-075A 执行日志

> 目的：记录 DEV-PLAN-075A 的落地变更、验证结果与已知限制。

## 变更摘要

- OrgUnit 详情页“记录操作”改为三分区：新版本操作（新增/插入）、历史修正（修正入口）、危险操作（删除）。
- 记录弹层升级为轻量四步向导（意图 → 日期 → 字段 → 确认），保持单表单、单次提交，不引入草稿态或后端状态机。
- 新增/插入支持 `record_change_type`：`rename` / `move` / `set_business_unit`，并按类型显示字段与默认值。
- 后端记录动作按 `record_change_type` 走单事件写入（`RenameNodeCurrent` / `MoveNodeCurrent` / `SetBusinessUnitCurrent`），移除“最早插入隐式 correction”路径。
- 新增 E2E 用例 `e2e/tests/tp060-02-orgunit-record-wizard.spec.js`，覆盖四步向导主路径与危险区入口可见性。

## 本地验证（2026-02-06）

- 已通过：`go test ./internal/server -count=1`
- 已通过：`make check lint`
- 已通过：`make generate && make css`
- 已通过：`make check doc`
- 已通过：`make e2e`（5 passed）

## 已知门禁状态

- `make preflight` 未全绿：`make check no-legacy` 命中仓内既有 legacy 标记（`internal/server/staffing_handlers.go`、`internal/server/staffing_test.go` 及 vendor package.json）。
- 上述问题与 DEV-PLAN-075A 本次改动无直接关系，本次未在 075A 范围内处理。

## CI 证据

- 待 PR 触发 CI 后补充。
