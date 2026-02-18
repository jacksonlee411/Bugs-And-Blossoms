# DEV-PLAN-109：Org 模块 `request_id` → `request_code` 统一改造与门禁

**状态**: 进行中（2026-02-18 01:42 UTC）

## 1. 背景

`DEV-PLAN-108` 已冻结：Org CRUD 统一写入应以“字段变化”驱动，业务幂等键统一为 `request_code`。  
当前仓库仍存在 `request_id`（JSON 字段、Go 字段、错误文案、测试数据等），造成命名双轨，不利于实现和验收一致性。

同时，命名口径并非本计划临时约定，而是承接仓库级 SSOT：`DEV-PLAN-026A` 与 `DEV-PLAN-072` 已冻结 `request_id -> request_code` 的命名收敛方向。

本计划作为专门收口文档，目标是把 Org 业务写入链路中的幂等字段统一为 `request_code`，并加入 CI 门禁防止回流。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. Org 业务写入请求体统一使用 `request_code`（替代 `request_id`）。
2. Org 写服务入参与返回字段统一为 `RequestCode/request_code`。
3. 相关前端 API 类型与调用参数统一为 `request_code`。
4. 新增质量门禁，阻断新增 `request_id/RequestID` 业务字段漂移。
5. 保持 One Door / No Legacy：不引入长期双字段兼容窗口。

### 2.2 非目标（Stopline）

1. 不改变 DB 幂等事实键（`org_events.request_code` 已是事实源）。
2. 不修改链路追踪 Header：`X-Request-ID` 仍仅用于 tracing，不作为业务幂等键。
3. 不改变通用错误返回 envelope 中的 `request_id` 字段：它用于链路追踪（与业务幂等无关）。
4. 不在本计划内变更错误码常量名 `ORG_REQUEST_ID_CONFLICT`（仅语义解释统一到 request_code）。

## 3. 统一口径（冻结）

1. **命名来源（SSOT）**：遵循 `DEV-PLAN-026A`/`DEV-PLAN-072`，`request_code` 是业务幂等字段唯一命名。
2. **业务幂等键唯一命名**：`request_code`。
3. **禁止双字段并存**：同一业务请求体不得同时出现 `request_id` 与 `request_code`。
4. **错误语义统一**：参数缺失文案统一为 `request_code is required`。
5. **历史错误码保留**：`ORG_REQUEST_ID_CONFLICT` 暂不改码名，仅在文档/UI 文案解释为“request_code 冲突”。
6. **语义隔离（Tracing vs 业务幂等）**：`X-Request-ID` 与通用错误 envelope 的 `request_id` 保持 tracing 语义，不作为业务幂等键。

## 4. 改造范围（实现清单）

1. 服务端 API 层（`internal/server/orgunit_api.go` 及测试）：
   - 请求 JSON 字段改为 `request_code`；
   - 响应字段改为 `request_code`（删除相关接口）。
2. 服务层（`modules/orgunit/services/orgunit_write_service.go` 及测试）：
   - 请求结构体字段改为 `RequestCode`；
   - 参数校验文案改为 `request_code is required`；
   - 返回 `fields` 中的 `request_id` 键改为 `request_code`。
3. 前端（`apps/web/src/api/orgUnits.ts` / `apps/web/src/pages/org/OrgUnitDetailsPage.tsx` / `apps/web/src/api/errors.ts`）：
   - 请求与响应类型统一 `request_code`；
   - 表单输入与提交参数统一 `request_code`。
4. 文档与契约：
   - 108/109/012 与 AGENTS Doc Map 对齐，避免命名漂移。

### 4.1 本计划覆盖的写入接口（冻结）

1. `POST /org/api/org-units/write`
2. `POST /org/api/org-units/rescinds`
3. `POST /org/api/org-units/rescinds/org`

以上接口的业务幂等入参统一为 `request_code`；不得新增 `request_id` 业务字段。

## 5. 门禁设计（冻结）

### 5.1 Gate-A（已落地）：增量阻断

- 新增脚本：`scripts/ci/check-request-code.sh`。
- 规则：扫描本次变更的新增行（Org 相关 Go/SQL/TS/TSX），禁止新增业务字段 `request_id` / `RequestID`。
- 允许：`X-Request-ID` tracing 语义（不命中该规则）。

### 5.2 Gate 接入（已落地）

1. `Makefile` 新增目标：`make check request-code`。
2. `make preflight` 纳入 `request-code` 检查。
3. CI `Quality Gates` 的 Code Quality job 新增 `Request-Code Gate (always)`。

### 5.3 Gate-B（本计划后续收口）

- 在业务链路完成迁移后，把 Gate 升级为“全量扫描零容忍”（不再仅限新增行）。
- **扫描范围（冻结）**：仅扫描业务实现路径
  - `internal/server/**/*.go`
  - `modules/orgunit/**/*.{go,sql}`
  - `apps/web/src/**/*.{ts,tsx}`
- **排除项（冻结）**：
  - 构建产物：`internal/server/assets/web/**`
  - API 错误适配层：`apps/web/src/api/errors.ts`（保留 envelope `request_id` tracing 语义）
- **允许语义（冻结）**：`X-Request-ID` header 与通用错误 envelope `request_id`（tracing）不属于业务幂等字段改造对象。
- **目标对象（冻结）**：阻断“业务幂等字段命名漂移”（JSON 字段、Go struct 字段、TS/TSX 请求与响应类型字段）；历史迁移文件/历史审计文案不作为 Gate-B 阻断对象。
- **落地顺序（冻结）**：
  1. dry-run 全量报告（只出清单不阻断）；
  2. 清零后切换为 blocking；
  3. 纳入 `make check request-code` 与 CI `Request-Code Gate (always)`。

## 6. 实施步骤（Checklist）

1. [X] 完成 API/服务/前端 `request_code` 命名替换并通过编译测试（2026-02-18 01:42 UTC）。
2. [X] 对齐相关测试断言（请求体、返回体、错误文案）（2026-02-18 01:42 UTC）。
3. [X] 执行并记录（2026-02-18 01:42 UTC）：
   - `make check request-code`（通过）
   - `make check doc`（通过）
4. [X] 在 `docs/dev-records/` 新增 109 执行日志并登记关键命令结果：`docs/dev-records/dev-plan-109-execution-log.md`。
5. [ ] 迁移完成后升级 Gate-B（全量零容忍）。

## 7. 验收标准（DoD）

1. Org 业务写入请求体中不再使用 `request_id`。
2. UI 到 API 到服务层使用同一命名：`request_code`。
3. CI 对新增 `request_id/RequestID` 具备阻断能力。
4. 109 执行证据可在 `docs/dev-records/` 追溯。

## 8. 关联文档

- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- `docs/dev-plans/026a-orgunit-id-uuid-code-naming.md`
- `docs/dev-plans/072-repo-wide-id-code-naming-convergence.md`
- `AGENTS.md`
