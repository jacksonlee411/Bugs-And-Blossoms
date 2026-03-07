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

## 追加收尾记录（2026-02-12）

- 详情页记录操作区补齐“插入版本”显式入口（`insert_record`），与“新建版本/修正当前”并列。
- 日期语义文案分离：
  - 树筛选：`树视图日期（tree_as_of）` + 浏览提示文案。
  - 记录写入：`记录生效日期`。
  - 创建组织：`建档生效日期` + 与 `tree_as_of` 关系提示。
- 测试补点：
  - `internal/server/orgunit_nodes_test.go` 新增渲染断言（插入入口、日期语义文案）。
  - `e2e/tests/tp060-02-orgunit-record-wizard.spec.js` 新增“插入入口可见”断言。

### 本地验证（2026-02-12）

- 已通过：`go test ./internal/server -count=1`
- 已通过：`make e2e`（5 passed）
- 排障记录：首次失败原因为端口占用（`127.0.0.1:4433` 的 kratosstub 旧进程与 `:8080` 的旧 server 进程）；清理后复跑通过。
