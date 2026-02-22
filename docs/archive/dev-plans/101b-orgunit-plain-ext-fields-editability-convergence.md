# [Archived] DEV-PLAN-101B：OrgUnit PLAIN 扩展字段编辑能力收敛（新建/插入记录/修正）

**状态**: 已完成（2026-02-16 20:55 UTC）

> SSOT 依赖：`DEV-PLAN-083`（capabilities 单点）、`DEV-PLAN-100D/100E1`（`patch.ext` 写入契约）、`DEV-PLAN-100E`（详情页编辑态）与 `AGENTS.md`（触发器与门禁）。

## 1. 背景

当前 OrgUnit 扩展字段里，`PLAIN` 字段在部分编辑场景被 UI 强制只读（MVP 早期收敛策略），导致和现阶段诉求不一致：

- 新建（CREATE）需要可编辑 PLAIN 扩展字段；
- 插入记录（append：RENAME/MOVE/ENABLE/DISABLE/SET_BUSINESS_UNIT）需要可编辑 PLAIN 扩展字段；
- 修正（CORRECT_EVENT）需要可编辑 PLAIN 扩展字段；
- 同类场景应保持一致行为，避免“某动作可改、某动作只读”的体验漂移。

## 2. 目标与边界

### 2.1 目标

1. [X] 在 OrgUnit UI 中，`PLAIN` 扩展字段（`text/int/bool/date/uuid`）在以下动作里按 capabilities 允许进行编辑：
   - [X] 新建（CREATE）
   - [X] 插入记录（append 动作）
   - [X] 修正（CORRECT_EVENT）
2. [X] 继续遵循 capabilities-driven + fail-closed：字段是否可编辑仍以 `allowed_fields/field_payload_keys` 为唯一事实源。
3. [X] 保持 One Door：写入仍走既有 `create/*append*/corrections` API，不新增旁路写入口。

### 2.2 非目标

- 不新增 DB schema/迁移；
- 不改变 `PLAIN 无 options` 契约（仍禁止对 PLAIN 调用 options endpoint）；
- 不在本计划引入“业务数据多语言 label 存储”。

## 3. 设计冻结

1. **编辑能力判定**
   - `PLAIN` 字段：可编辑性 = `actionEditableByCapabilities`；
   - 控件按 `value_type` 映射：`text/uuid -> TextField`、`int -> number`、`bool -> select(true/false/null)`、`date -> date input`。
2. **提交载荷**
   - 仍复用 `field_payload_keys` 映射到 `patch.ext.<field_key>` 或 append payload 的 `ext.<field_key>`；
   - UI 不提交 `ext_labels_snapshot`（仅 DICT 由服务端生成）。
3. **空值语义**
   - `CREATE`（新建）使用 `omit_empty`：文本输入为空（trim 后空串）时不提交该字段（保持“未赋值”）。
   - `append/correct`（插入记录/修正）使用 `null_empty`：文本输入为空（trim 后空串）时提交 `null`，用于显式清空扩展字段。
   - `int/uuid/date` 在 UI 侧做轻量格式校验；无效输入禁止提交并在字段下给出错误提示。

## 4. 实施步骤

1. [X] 文档与契约对齐
   - [X] 新增本计划（101B）并挂入 Doc Map（`AGENTS.md`）。
   - [X] 在 `DEV-PLAN-100E` 中标注：PLAIN 只读的历史限制已由 101B 收敛为“PLAIN 按 capabilities 可编辑（控件按 value_type 映射）”。
2. [X] 前端实现（`apps/web/src/pages/org/`）
   - [X] `OrgUnitDetailsPage`：append/correct 的扩展字段表单中放开 `PLAIN` 输入（按 `value_type` 映射控件）。
   - [X] `OrgUnitsPage`：新建弹窗补齐 `PLAIN` 扩展字段输入区，并纳入 create payload。
3. [X] 测试与回归
   - [X] 补充/更新前端单测（intent/patch 相关）覆盖 PLAIN 可编辑提交与空值清空。
   - [X] 通过前端最小门禁（lint/typecheck/test）。

## 5. 验收标准

- [X] 新建：当 create capabilities 允许该字段时，PLAIN 字段可输入并成功写入；
- [X] 插入记录：append 动作里 PLAIN 字段可输入并成功写入；
- [X] 修正：correct 动作里 PLAIN 字段可输入并成功写入；
- [X] capabilities 不可用或字段不在 `allowed_fields` 时，字段禁用（fail-closed）；
- [X] 提交 payload 不含 `ext_labels_snapshot`（UI 侧）。

## 6. 关联文档

- `docs/archive/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
- `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
