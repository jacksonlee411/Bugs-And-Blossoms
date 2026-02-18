# DEV-PLAN-106B：OrgUnit 更正语义收敛——生效日更正的“粘性”与后续更正兼容（根因修复）

**状态**: 已实施（2026-02-18；修复迁移已落库）

## 1. 背景

在 `DEV-PLAN-106A`（字典字段 `d_<dict_code>` + 启用时自定义展示名）验证过程中，出现如下用户可见失败：

- 页面：`/app/org/units/<org_code>?as_of=2026-02-18`
- 操作：通过“更正（CORRECT_EVENT）”维护某个字典字段（例如“字典字段01”）的值
- 现象：提交失败，前端展示 `orgunit_correct_failed`

排障结论（已复现并定位）：

- `orgunit_correct_failed` 是 UI 展示的兜底 message；真实稳定错误码为 `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`。
- 失败并非字段未启用，而是 **“生效日更正后的记录”在后续更正时发生 effective_date 回退**，导致 DB Kernel 在回放/重建版本时用“旧 effective_date”校验 ext 字段 enabled window，从而 fail-closed。

> 相关历史：`DEV-PLAN-080B` 解决的是 correction kernel privileges 与错误码提取；本计划解决的是 **corrections 语义层面的有效日期推导**（根因修复），避免要求 UI 强行携带 `patch.effective_date` 做绕过。

### 1.1 在 DEV-PLAN-108 完成后的复核结论（2026-02-18，修复前）

`DEV-PLAN-108` 已将 OrgUnit 写入口收敛为统一写 API（`POST /org/api/org-units/write`），但该问题**并未被 108 自动消除**，原因是：

1. 108 的 unified write（intent=`correct`）只有在“更正生效日发生变化”时才会把 `effective_date` 写入 `CORRECT_EVENT` 的 payload：
   - `modules/orgunit/services/orgunit_write_service.go`：`if effectiveDate != targetDate { payload["effective_date"] = effectiveDate }`
2. 一旦存在“先更正 effective_date（payload 含 effective_date）→ 再更正 ext（payload 不含 effective_date）”的两次更正链路，第二次更正会触发本计划描述的 effective_date 回退风险：
   - 修复前：`orgunit.org_events_effective` 与 `orgunit.org_events_effective_for_replay(...)` 使用 “latest correction payload 是否含 `effective_date`” 来决定 effective_date（详见 §3），因此会把 effective_date 回落到 base event 的日期。
   - 修复后：两处改为 sticky 语义（`latest_effective_date_corrections` + `COALESCE(sticky_effective_date, base_effective_date)`），不再回落。

结论：`DEV-PLAN-106B` 是 108 之后的真实缺陷修复项（并且更容易在新 UI 的“字段编辑表单”里触发跨两次更正的场景）；现已按本计划落地修复（见 §7.1）。

## 2. 问题定义（冻结）

### 2.1 现象（冻结）

当某条 OrgUnit 记录发生过“生效日更正”（即对某 target event 通过 `CORRECT_EVENT` 修改了 `effective_date`）后：

- 用户再次对该记录做 `CORRECT_EVENT`（只修改 `patch.ext` 等字段，不包含 `patch.effective_date`）；
- DB Kernel 在“重建 versions（replay）”过程中，把该记录的 effective_date 计算回原始日期；
- 从而触发 `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`（或其他依赖 as_of 的 fail-closed 规则）。

### 2.2 期望语义（冻结）

**“生效日更正”必须对同一 target event 的后续更正保持粘性（sticky）**：

1. 对同一 `target_event_uuid`，一旦某次 `CORRECT_EVENT` 将 effective_date 更正为 `D_new`，则后续针对该 `target_event_uuid` 的更正（即便 patch 未包含 effective_date）也不得让 effective_date 回退到原始值。
2. effective_date 的最终值应当等价于：
   - “最后一次显式更正 effective_date 的 correction”的 effective_date（按 `tx_time, id` 取最新），若存在；
   - 否则为原始事件的 effective_date。
3. 对除 effective_date 之外的字段（含 `ext/ext_labels_snapshot`），“最新 correction wins”的语义不变（仍按最新 correction payload 合并计算）。

> 直觉解释：effective_date 是“记录在时间轴上的位置”；不应被一次“只改字段值”的更正隐式重置。

## 3. 根因分析（冻结）

修复前 OrgUnit Kernel 的 replay 入口（以 `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` 为准）对 corrections 的处理方式是：

- `orgunit.org_events_effective`（view）与 `orgunit.org_events_effective_for_replay(...)`（function）
  - 对每个 `target_event_uuid` 只选取 **latest_corrections**（按 `tx_time DESC, id DESC`）
  - effective_date 的计算逻辑是：仅当 “latest correction payload 含 `effective_date`” 才改变 effective_date，否则回落到原 event effective_date。

当存在“先更正 effective_date，再更正 ext”两次 correction 时（修复前）：

- 第二次 correction（只改 ext）成为 latest correction，但其 payload 不含 effective_date；
- effective_date 在 view/for_replay 中回落到原始值；
- replay 阶段调用 `apply_org_event_ext_payload(..., p_effective_date=<回落后的旧日期>, payload.ext=...)`，触发 enabled window 校验失败（`ORG_EXT_FIELD_NOT_ENABLED_AS_OF`）。

补充（108 后的触发路径，修复前）：

- unified write 的 `correct` 仍会写入 `CORRECT_EVENT`（保持 target_event_uuid 审计链），且当“第二次更正不是生效日更正”时，`CORRECT_EVENT` payload 默认不包含 `effective_date`（见 §1.1），因此满足本缺陷的触发条件（修复前会失败；修复后按 sticky 语义可通过）。

## 4. 目标与非目标

### 4.1 目标（冻结）

1. 修复 OrgUnit corrections 的 effective_date 推导，使其满足 §2.2 “粘性语义”。
2. 修复后不要求 UI 在“仅更正字段值”时额外提交 `patch.effective_date` 作为绕过。
3. `orgunit.org_events_effective` 与 replay 使用的 `orgunit.org_events_effective_for_replay(...)` 在 effective_date 推导上保持一致，避免 UI/versions 与实际 replay 语义漂移。
4. 保持 One Door：仍通过 Kernel 函数写入与重建链路完成变更，不引入第二写入口（SSOT：`AGENTS.md` §3.7）。

### 4.2 非目标（Stopline）

1. 不新增数据库表（如需新增表必须另起变更并获得用户手工确认，遵循仓库红线）。
2. 不改变 corrections 的“最新 patch 合并”语义（除 effective_date 推导外），不引入“多 correction 顺序叠加”的全新规则体系。
3. 不在本计划解决 UI 错误展示优先级问题（UI 优先展示 message 的现状保持；错误码提取与映射已由 `DEV-PLAN-080B` 处理）。

## 5. 设计方案（冻结）

### 5.1 核心思路：effective_date 与 correction payload 解耦

在 view/function 中对同一 `target_event_uuid` 分别计算：

1. **v_sticky_effective_date**：来自“最新一次包含 `effective_date` 字段的 CORRECT_EVENT”；
2. **v_latest_correction_payload**：来自“最新一次 correction（CORRECT_EVENT/CORRECT_STATUS/RESCIND_*）”用于合并 payload / event_type；

并最终满足：

- `effective_date = COALESCE(v_sticky_effective_date, base_event.effective_date)`
- `payload =` 仍按现有合并逻辑使用 **latest correction payload**（只影响字段值，不影响 effective_date 的取值）

### 5.2 需要修改的 Kernel 结构（冻结）

必须同时修改两处（保持一致性）：

1. `CREATE OR REPLACE VIEW orgunit.org_events_effective`：
   - 将 effective_date 的计算从“latest correction 是否含 effective_date”改为“latest effective_date-correction（sticky）”。
2. `orgunit.org_events_effective_for_replay(...)`：
   - 与 view 同步；保证 replay 阶段看到的 effective_date 与 UI/versions 一致。

建议的实现形态（冻结为意图，不冻结具体 SQL 细节）：

- 在 `latest_corrections` 之外新增一个 `latest_effective_date_corrections` CTE：
  - 仅筛选 `CORRECT_EVENT` 且 `payload ? 'effective_date'`
  - 按 `(tenant_uuid, target_event_uuid)` 取 `tx_time DESC, id DESC`
  - 产出 `sticky_effective_date = NULLIF(btrim(payload->>'effective_date'), '')::date`
- 最终 effective_date 用 `COALESCE(sticky_effective_date, base_effective_date)`

### 5.3 不变量与边界（冻结）

1. sticky effective_date 只来自 `CORRECT_EVENT`（状态更正/撤销不改变日期）。
2. 若 sticky effective_date 的值非法/空：Kernel 侧已有校验会拒绝写入，因此 view/function 不需要额外容错；但仍需防御性 `NULLIF/btrim`。
3. RESCIND 语义不变：被 rescind 的 target event 在 view/for_replay 中应被排除（沿用现有 `COALESCE(correction_type,'') NOT IN ('RESCIND_EVENT','RESCIND_ORG')` 过滤）。
4. 不引入 legacy 分支：不允许同时保留旧/新两套 effective_date 推导逻辑（对齐 `DEV-PLAN-004M1`）。

## 6. 契约影响面（冻结）

### 6.1 API 契约（冻结）

- `POST /org/api/org-units/write`（intent=`correct`）：请求体不变；**不新增“必须提交 patch.effective_date”要求**。
- `POST /org/api/org-units/corrections`：若仍保留该入口，请求体亦不变；同样**不新增“必须提交 patch.effective_date”要求**。
- `GET /org/api/org-units/versions` / `GET /org/api/org-units/details`：返回结构不变，但 effective_date 的稳定性提升（不会因后续 ext 更正回退）。

### 6.2 数据契约（冻结）

- `orgunit.org_events_effective` 是“effective 事件视图”的 SSOT；其 effective_date 语义需与 replay 入口一致。
- replay 重建链路必须使用修复后的 effective_date 参与 ext enabled window 校验，保证 fail-closed 正确性。

## 7. 实施步骤（草案）

> 门禁与命令入口引用 `AGENTS.md` 与 `DEV-PLAN-012`；此处不复制完整命令矩阵。

1. [x] 契约固化：
   - 在本文件冻结语义与验收用例（见 §2、§8）。
2. [x] 更新 orgunit engine（schema）：
   - 修改 `orgunit.org_events_effective` view 的 effective_date 推导为 sticky 语义（§5）。
   - 修改 `orgunit.org_events_effective_for_replay(...)` 同步 sticky 语义（§5）。
3. [x] 新增 orgunit 迁移（Goose）：
   - 以模块闭环方式落地（Atlas+Goose，按 `DEV-PLAN-024`）。
4. [x] 同步汇总 schema：
   - 更新 `internal/sqlc/schema.sql`（与模块 schema 一致）。
   - 运行 `make sqlc-generate` 并确保生成物闭环（按 `AGENTS.md`）。
5. [x] 补充测试（至少两类）：
   - schema 断言测试：通过 `internal/server/orgunit_effective_date_sticky_sql_test.go` 断言 sticky 语义已落入 view + replay 两处（避免回归漂移）。
   - （可选）DB 层集成测试：构造“先更正 effective_date，再更正 ext”并断言第二次更正不触发 `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`（未来若 CI 需要更强保障可补齐）。
6. [ ] 回归验证：
   - 跑通 `DEV-PLAN-106A` 的字典字段更正闭环（无需 UI 绕过）。

### 7.1 实施记录（2026-02-18）

- Schema SSOT：
  - `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`：为 `org_events_effective` / `org_events_effective_for_replay(...)` 引入 `latest_effective_date_corrections`，并用 `COALESCE(sticky_effective_date, base_effective_date)` 输出 effective_date（sticky 生效）。
- DB Migration（Goose）：
  - `migrations/orgunit/20260218183000_orgunit_correction_effective_date_sticky.sql`
  - `migrations/orgunit/atlas.sum` 已更新（Atlas hash）。
- sqlc：
  - `make sqlc-generate` 已执行（`internal/sqlc/schema.sql` 与 gen 更新闭环）。
- Tests：
  - `internal/server/orgunit_effective_date_sticky_sql_test.go`：通过 schema 片段断言 sticky 语义已落入 view + replay 两处，避免回归漂移。

- 本地门禁（执行通过）：
  - `make orgunit plan && make orgunit lint && make orgunit migrate up`
  - `go fmt ./... && go vet ./... && make check lint && make test`

## 8. 验收标准（DoD）

1. 在存在“生效日更正”的 OrgUnit 记录上：
   - 只提交 `patch.ext`（不含 `patch.effective_date`）的更正能够成功落库；
   - 不再出现 `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`。
2. `GET /org/api/org-units/versions` 返回的 effective_date 不会因后续 ext 更正而回退（sticky 生效）。
3. replay 重建链路稳定：
   - `orgunit.rebuild_org_unit_versions_for_org_with_pending_event(...)` 在上述场景下可完成，不因 effective_date 回落触发 ext enabled 校验失败。
4. 通过对应门禁：
   - 迁移闭环、sqlc 生成物闭环、相关测试通过（入口引用 `AGENTS.md`）。

## 9. 风险与回滚策略（冻结）

1. 风险：effective_date 语义改变会影响依赖 `org_events_effective` 的下游查询排序/窗口计算。
   - 缓解：本计划将 view 与 replay 同步，避免“UI 看起来正确但 replay 错”的分裂；并以集成测试覆盖。
2. 回滚：按仓库 No Legacy 原则，不引入双链路。
   - 若修复引入新问题，回滚应通过“环境级保护（停写/只读）+ 修复后重试”完成（SSOT：`AGENTS.md` §3.7）。

## 10. 关联文档

- `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md`
- `docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md`
- `docs/dev-plans/080b-orgunit-correction-failure-investigation-and-remediation.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`
- `docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
