# DEV-PLAN-101I：OrgUnit 生效日期记录新增/插入（MUI）可实施方案

**状态**: 已实施（2026-02-17）；2026-02-18 起写入口径由 `DEV-PLAN-108` 进一步收敛

## 0. 与 DEV-PLAN-108 的对齐补充（2026-02-18）

本计划完成时，add/insert 仍基于 append 原子事件（`RENAME/MOVE/SET_BUSINESS_UNIT/ENABLE/DISABLE`）与 `record_change_type`。
该口径在 108 生效后调整为“字段编辑 + intent 自动判定”。

108 对本计划的覆盖与替代关系：

1. 本计划 §5.1 的步骤 1（选择 `record_change_type`）被 108 取消；
2. 本计划 §3.2 的“不新增 UPDATE”不再成立：108 明确引入 `UPDATE` 以承载 add/insert 多字段单事件；
3. capabilities 主口径从 append/mutation 分裂，迁移到 `write-capabilities(intent=...)`；
4. 本计划的日期约束（add/insert 的区间规则）继续保留并复用，属于仍有效的约束资产。

执行口径：

- 日期规则（add/insert 上下界）仍以本计划和 `DEV-PLAN-075` 为约束来源；
- 写入交互与接口契约以 `DEV-PLAN-108` 为准。

## 1. 背景

用户诉求聚焦两点：

1. 在 UI 上可明确执行“新增一条生效日期记录（add）”；
2. 在 UI 上可明确执行“插入一条记录（insert）”，并在提交前知道限制范围。

当前 MUI 详情页虽然可通过 append 动作（rename/move/set BU/enable/disable）写入新 `effective_date`，但存在两类问题：

- 默认日期等于当前选中版本日期，首击容易触发 `EVENT_DATE_CONFLICT`；
- 缺少“add/insert”显式语义与区间提示，用户需自行推断合法日期。

## 2. SSOT 与硬约束引用

- MUI-only 前端唯一链路：`docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`
- 有效期区间与插入语义口径：`docs/dev-plans/075-orgunit-effective-date-backdating-assessment.md`
- 历史问题与后续能力诉求（含 `SET_MANAGER`/`UPDATE` 方向）：`docs/dev-plans/075a-orgunit-records-ui-and-editing-issues.md`
- Append capabilities/policy 单点：`docs/dev-plans/083a-orgunit-append-actions-capabilities-policy-extension.md`
- 扩展字段可编辑口径：`docs/dev-plans/101b-orgunit-plain-ext-fields-editability-convergence.md`

## 3. 目标与非目标

### 3.1 本计划目标（可直接实施，P0）

1. [x] 在 OrgUnit 详情页提供“新增版本（add）/插入版本（insert）”显式入口与统一向导流程。
2. [x] 将日期规则前置到 UI：默认值、允许区间、无可用槽位提示（提交前阻断明显无效输入）。
3. [x] 保持 capabilities-driven：动作可用性、字段可编辑性仍以 append-capabilities 为唯一事实源（fail-closed）。
4. [x] 明确“新增时可编辑信息”的当前上限：仅可编辑所选 `record_change_type` 对应字段集，不引入新内核事件语义。

### 3.2 非目标（本计划不做）

1. [ ] 不新增 DB schema / 迁移 / sqlc。
2. [ ] 不新增 `UPDATE` / `SET_MANAGER` 等新事件类型（该方向另立计划，承接 `075A` 的 P1）。
3. [ ] 不引入 legacy 双链路或绕过 One Door 写入口。

## 4. 冻结决策（Simple 优先）

### 4.1 不变量（必须始终成立）

- One Door：写入仅走现有 `/org/api/org-units/*` append/correction 接口。
- 一天一槽位：同 org 同 `effective_date` 只能有一条 effective 事件（冲突码 `EVENT_DATE_CONFLICT`）。
- fail-closed：capabilities 失败/不可用时，动作与字段禁用，不做“乐观放开”。
- 不复制后端规则：前端做可解释的预校验；最终判定仍以后端/kernel 为准。

### 4.2 “新增可编辑全部信息”口径冻结

本计划冻结为：**“全部信息”按当前事件语义理解为“所选变更类型允许的全部字段”**，而非“一次提交修改所有 core 字段”。

原因：当前 append 为原子事件矩阵（`RENAME/MOVE/SET_BUSINESS_UNIT/ENABLE/DISABLE`），且同日槽位唯一；不新增事件语义时无法单次写全量 core 字段。

## 5. 方案设计（实现级）

### 5.1 UI 交互与信息架构

在 `OrgUnitDetailsPage` 增加显式入口：

- 新增版本（add）
- 插入版本（insert）

两者进入同一“记录向导”弹窗，步骤固定：

1) 选择变更类型 `record_change_type`（`rename/move/set_business_unit/enable/disable`）  
2) 选择 `effective_date`（显示默认值与合法区间）  
3) 编辑字段（按 capabilities）  
4) 确认提交

### 5.2 日期规则（实现口径，解决“最早记录歧义”）

设版本升序为 `d0 < d1 < ... < dn`，当前选中版本为 `ds`（索引 `s`）：

#### A) add（新增版本）

- 规则：`effective_date > dn`
- 默认值：`dn + 1 day`

#### B) insert（插入版本）

- 若 `s < n`（选中非最晚）：
  - 上界：`max = d(s+1) - 1 day`
  - 下界：
    - 若 `s > 0`：`min = d(s-1) + 1 day`
    - 若 `s == 0`：`min = ds + 1 day`（冻结：不允许小于当前最早版本，避免与 `ORG_NOT_FOUND_AS_OF` 口径冲突）
  - 合法：`min <= effective_date <= max` 且 `effective_date != ds`
  - 若 `min > max`：显示“无可插入日期槽位”，禁用确认按钮
- 若 `s == n`（选中最晚）：
  - 插入视同新增：`effective_date > dn`
  - 默认值：`dn + 1 day`

### 5.3 capabilities 与字段编辑

- `record_change_type` 决定目标 emitted event；
- 以 append-capabilities 对应 event 的 `allowed_fields`/`field_payload_keys` 驱动字段可编辑与 payload 组装；
- 弹窗内 `effective_date` 变化时，重新按“当前日期 + 当前 event”校验可用性（capabilities 不可用则 fail-closed）。

### 5.4 错误与提示（最小稳定集）

- `EVENT_DATE_CONFLICT`：该日期已存在记录，请更换日期。
- `ORG_NOT_FOUND_AS_OF`：目标日期组织不存在（通常是日期过早）。
- `ORG_PARENT_NOT_FOUND_AS_OF` / `ORG_CYCLE_MOVE`：移动目标无效。
- `ORG_ROOT_CANNOT_BE_MOVED` / `ORG_ROOT_BUSINESS_UNIT_REQUIRED`：根组织保护规则触发。

## 6. 实施步骤（可执行清单）

1. [x] 文档冻结与 i18n 文案补齐  
   - `docs/dev-plans/101i-orgunit-effective-date-record-add-insert-ui-and-constraints.md`  
   - `apps/web/src/i18n/messages.ts`（新增 add/insert/区间提示/无槽位提示）

2. [x] 详情页增加 add/insert 显式入口与向导状态  
   - `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`

3. [x] 日期计算与前端预校验实现  
   - 新增/扩展 `apps/web/src/pages/org/` 下日期规则辅助函数（纯函数 + 单测）
   - 覆盖 add/insert/最晚回退/无槽位/非法输入

4. [x] capabilities 联动重构  
   - 以“弹窗内 effective_date + record_change_type”为维度计算 append 可用性
   - 保持 fail-closed

5. [x] payload 组装与提交收口  
   - 复用 `orgUnitAppendIntent`，保证仅按 `field_payload_keys` 输出

6. [x] 测试补齐  
   - 前端单测：日期区间、默认值、禁用条件、payload 组装
   - E2E：可发现性（add/insert 按钮可见）、插入成功与冲突提示

7. [x] 记录执行证据  
   - 在 `docs/dev-records/` 增补本计划执行日志（命令、结果、时间戳）

## 7. 验收标准（DoD）

1. [x] 用户可在详情页直接看到“新增版本/插入版本”入口，并可完成提交。
2. [x] add 默认日期命中 `max+1`，不再默认落在冲突日期。
3. [x] insert 在中间版本场景展示并执行区间校验；无槽位时可解释且不可提交。
4. [x] 选中最晚版本时 insert 自动按 add 规则工作。
5. [x] 字段编辑严格受 capabilities 控制；capabilities 异常时动作 fail-closed。
6. [x] 不新增内核事件语义；现有 API/KERNEL 约束下行为一致且可解释。

## 8. 门禁与证据（按触发器）

> 本次预期命中：MUI Web UI、文档、（若改 E2E 则含 E2E）。

- [x] `pnpm --dir apps/web lint`
- [x] `pnpm --dir apps/web typecheck`
- [x] `pnpm --dir apps/web test`
- [x] `make generate && make css`
- [ ] `git status --short`（确认无遗漏生成物；仍需忽略/处理本地 .pid 噪音文件）
- [x] `make check doc`
- [ ] 若 E2E 有改动：`make e2e`
- [ ] PR 前建议：`make preflight`

## 11. 实施落点（代码与证据）

- UI：`apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
- 日期规则：`apps/web/src/pages/org/orgUnitRecordDateRules.ts`
- 单测：`apps/web/src/pages/org/orgUnitRecordDateRules.test.ts`
- i18n：`apps/web/src/i18n/messages.ts`
- 执行日志：`docs/dev-records/dev-plan-101i-execution-log.md`

## 9. 风险与缓解

- 风险：前端区间与后端真实约束不一致。  
  缓解：前端仅做“可解释预校验”，提交后仍统一展示稳定错误码；不在前端复制全部内核规则。

- 风险：新增 add/insert 入口导致状态复杂化。  
  缓解：统一向导状态机，`record_change_type` + `mode(add/insert)` 两个维度，不新增并行弹窗实现。

- 风险：用户继续理解为“一次提交可改全字段”。  
  缓解：在向导内明确“当前变更类型可编辑字段”，并在文案中说明“一次提交对应一次事件变更”。

## 10. 交付物

- 可实施设计文档（本文件）；
- 对应实现 PR（UI + 测试 + 执行证据）；
- `docs/dev-records/` 中的执行日志与门禁结果。
