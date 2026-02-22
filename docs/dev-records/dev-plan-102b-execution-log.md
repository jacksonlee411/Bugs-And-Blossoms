# DEV-PLAN-102B 执行日志

> 目的：记录 DEV-PLAN-102B（070/071 时间口径显式化）在代码、测试与门禁层的实施与验证结果。

## 变更摘要

- API 层显式化（去除默认 today）：
  - `internal/server/setid_api.go`：`POST /org/api/setids` 强制 `effective_date`；`GET /org/api/setid-bindings` 强制 `as_of`。
  - `internal/server/setid_scope_api.go`：`scope-packages/owned-scope-packages/scope-subscriptions/global-scope-packages` 全部显式日期必填；`POST /org/api/scope-packages/{package_id}/disable` 新增并强制 `effective_date`。
  - `internal/server/jobcatalog_api.go`：`GET /jobcatalog/api/catalog` 强制 `as_of`；`POST /jobcatalog/api/catalog/actions` 强制 `effective_date`。
  - `internal/server/staffing_handlers.go`、`modules/staffing/presentation/controllers/assignments_api.go`：`positions/positions:options/assignments` 读写接口统一显式日期必填，移除 `as_of -> effective_date` 回填。
- Store/Kernel 层显式化：
  - `internal/server/setid.go`：`DisableScopePackage(...)` 签名改为显式接收 `effectiveDate`，PG/Memory 实现不再使用 `time.Now()`。
  - `modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`：
    - `submit_setid_event` 的 `CREATE/BOOTSTRAP` 去除 `current_date` 兜底并强制 `payload.effective_date`；
    - 订阅有效性判断改为 `@> v_effective_date`；
    - `ensure_setid_bootstrap` 去掉 `validity @> current_date` 依赖。
  - `internal/sqlc/schema.sql` 同步 00006 变更，保持 schema SoT 一致。
- 测试收敛（删除 default today 预期，改为 required fail-closed）：
  - 更新 `internal/server/*_test.go` 与 `modules/staffing/presentation/controllers/assignments_api_test.go`，将原“默认 as_of/effective_date 成功”改为“缺失即 `400 invalid_*`”。
  - 所有成功路径用例补齐显式 `as_of` / `effective_date`。
- 新增门禁（M5）：
  - `scripts/ci/check-as-of-explicit.sh`：阻断 070/071 关键路径中 `as_of/effective_date` 的隐式回填及 SQL `current_date` 业务兜底模式。
  - `Makefile`：新增 `make check as-of-explicit`，并接入 `make preflight`。
  - `.github/workflows/quality-gates.yml`：新增 `As-Of Explicit Gate (always)`。

## 本地验证

- 已通过（2026-02-22）：
  - `go test ./internal/server ./modules/staffing/presentation/controllers`
  - `make test`（覆盖率门禁 `100.00%`）
  - `make check as-of-explicit`
  - `make check routing`
  - `make check doc`
  - `make sqlc-generate`（生成后工作区无漂移）
  - `make e2e`
  - `make preflight`

## 冲突文档改写对照（M1）

- `DEV-PLAN-071`：移除 `as_of/effective_date` 默认 `current_date` 描述，改为显式必填。
- `DEV-PLAN-071A`：订阅/包治理调用口径对齐显式日期必填。
- `DEV-PLAN-071B`：补齐 `STD-002` 引用，冻结口径来源。
- `DEV-PLAN-102`：旧矩阵降级为历史附录（非现行），现行跳转到 `DEV-PLAN-102B`。
- `DEV-PLAN-063`：移除“`effective_date` 缺省默认为 `as_of`”。
- `DEV-PLAN-026A`：增加历史勘误，冲突口径以 `STD-002/DEV-PLAN-102B` 为准。

## PR 与交付归档

- 实施提交：`012a070 feat: implement dev-plan-102b explicit time semantics`
- 覆盖率补测提交：`357d1ad test: close 102b coverage gaps for explicit date paths`
- 合并 PR：`#399`（`https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/399`）
- 合并时间：`2026-02-22`（UTC）
- 合并提交：`00dac4c60c6b72e785b4dca9e39207a44c2c8c35`

## 约束符合性

- 未引入 legacy fallback、双链路或特性开关（符合 No Legacy）。
- 未新增数据库表（无额外人工审批需求）。
- 时间语义保持“业务时间显式输入，系统时钟仅用于审计时间”。
