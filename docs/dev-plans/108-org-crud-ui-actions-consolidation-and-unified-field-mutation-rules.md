# DEV-PLAN-108：Org 模块 CRUD UI 按钮整合与统一字段变更规则（用户操作视角）

**状态**: 规划中（2026-02-18 08:18 UTC，已冻结关键业务语义）

## 0. 实施级冻结摘要（给开发/评审的速览）

1. CRUD 区按钮只保留：`新建组织 / 新建版本 / 插入版本 / 更正 / 删除`。
2. 用户不再选择 `rename/move/enable/disable/set_business_unit` 等动作类型；只编辑字段。
3. 新增统一写入 API（删除除外）：`POST /org/api/org-units/write`，一次提交只落一个事件。
4. 新增 Org 基础事件：`UPDATE`（单事件多字段），用于 append 场景（新建版本/插入版本）。
5. `correct` 必须支持“状态 + 其他字段”同次提交，且仍保持单 `CORRECT_EVENT` 审计链。
6. `UPDATE`/（更正后的 effective replay）执行采用确定性顺序：`parent` 优先、`name` 最后（见 §6.4）。为满足“允许 disabled move / status+BU 同次提交”，内核将移除对 move / set_business_unit 的 `status=active` 硬依赖（见 §6.4、§6.5）。
7. 删除自动判定：多记录删记录（`rescind event`）；仅一条记录删组织（`rescind org`）。
8. 运维入口（字段配置等）不在本计划改造范围；仅要求从 CRUD 认知中解耦。

---

## 1. 背景

当前 OrgUnit 的 UI/接口仍偏“事件动作驱动”：用户要先理解 `RENAME/MOVE/ENABLE/DISABLE/SET_BUSINESS_UNIT`，再组合操作完成一次业务维护。实际用户心智是“编辑这一条组织记录”。

痛点：

1. 操作入口多、概念负担重。
2. 一次变更涉及多个字段时，往往要拆成多次提交，审计链分裂且易触发日期/占位冲突。
3. 多事件执行顺序不稳定，`full_name_path`（尤其 move + rename）可解释性差。

本计划目标是：把写入模型从“动作选择”收敛到“字段变化”，并把多字段同次维护压缩为单事件。

---

## 2. 目标与非目标

### 2.1 目标（冻结）

1. CRUD 按钮收敛为 5 个：
   - 新建组织
   - 新建版本
   - 插入版本
   - 更正
   - 删除
2. 对“新建组织/新建版本/插入版本/更正”，允许用户在一次提交中维护全部业务字段（core + ext）。
3. UI 不再出现“操作类型/变更类型”选择（如 `record_change_type`）。
4. 后端按字段差异自动判定内部子动作，并合并为单事件。
5. 删除动作自动判定“删记录 or 删组织”。
6. 冻结用户确认语义（2026-02-18）：
   - 更正必须支持 `status + 其他字段` 同次提交；
   - 允许在 disabled 记录上执行 move；
   - 允许 `status` 与 `is_business_unit` 同日同次提交（可伴随其他字段）。

### 2.2 非目标（Stopline）

1. 不引入 legacy/双链路；写入仍走 DB Kernel `submit_*`（One Door）。
2. 不改变 day 粒度 Valid Time。
3. 不扩展到其他业务模块。
4. 不改字段配置模块本身（数据结构、运维流程、权限模型均不在本计划）。

### 2.3 与用户诉求逐条对齐

1) UI 仅保留 5 个 CRUD 按钮：已冻结于 §4.1。  
2) 新建/新建版本/插入/更正支持全字段：已冻结于 §4.2、§5.3。  
3) 单次多字段合并一个事件，且顺序固定：已冻结于 §6.4。  
4) 原子 API 会冲突时改为单 API：已冻结于 §5.1、§5.2、§6。  
5) 删除自动判定记录/组织：已冻结于 §4.4、§7。  
6) 取消操作类型选择：已冻结于 §4.2。  
7) 设置 BU/配置字段按钮属运维：已冻结于 §4.1（分区，不并入 CRUD 改造）。

---

## 3. SSOT 与约束引用

- One Door/No Legacy：`AGENTS.md`、`DEV-PLAN-004M1`、`DEV-PLAN-026`
- OrgUnit 写模型与 replay：`DEV-PLAN-026`、`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`
- 删除语义（rescind）：`DEV-PLAN-075C`
- 状态显示与切换：`DEV-PLAN-075D`
- add/insert 日期口径：`DEV-PLAN-101I`
- capabilities 策略单点：`DEV-PLAN-083/083A/100E1`
- ext 写入快照：`DEV-PLAN-100D/100E1`

---

## 4. UI 实施设计（实现级）

### 4.1 页面分区与按钮

#### 4.1.1 CRUD 区（本计划改造范围）

- `新建组织`
- `新建版本`
- `插入版本`
- `更正`
- `删除`

#### 4.1.2 运维区（本计划不改行为）

- `字段配置` 等运维入口保留（若存在 `设置 BU` 运维入口，保持现状）
- 但它们不计入 CRUD 收敛，不参与“5 按钮”验收

### 4.2 表单与交互

1. 新建组织/新建版本/插入版本/更正都使用“字段编辑表单”，不展示动作类型下拉。
2. 字段集合（统一）：
   - core：`name`, `parent_org_code`, `status`, `is_business_unit`, `manager_pernr`
   - ext：`ext[field_key]`
3. 提交策略：patch 语义
   - 未变化字段不提交
   - 显式清空提交 `null`
4. 日期规则：
   - add/insert 继续复用 `DEV-PLAN-101I`
   - correct 继续复用 `DEV-PLAN-075/075A`
5. 交互语义冻结：
   - disabled 记录也允许在表单里改 `parent_org_code`（move）；
   - 允许同次改 `status + is_business_unit`，并允许同时改其它字段；
   - move + rename 同次提交时，用户无需拆两次保存（后端保证 parent 先、name 后）。

### 4.3 删除按钮行为

- 点击删除后，前端先根据版本数量自动选择删除类型：
  - `versions.length > 1` -> `rescind event`
  - `versions.length == 1` -> `rescind org`
- 用户只看到一个按钮与一个确认流程（必须输入 `reason`）。

### 4.4 前端代码改造落点（冻结）

- `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
  - 移除 rename/move/set_business_unit 独立 CRUD 按钮
  - 移除记录向导 `record_change_type`
  - 接入统一写接口 `write`
- `apps/web/src/pages/org/OrgUnitsPage.tsx`
  - 新建组织改调统一写接口 `write(intent=create_org)`
- `apps/web/src/pages/org/orgUnitAppendIntent.ts`
  - 逐步替换为 `orgUnitWritePatch.ts`（以统一 patch 构造为准）
- `apps/web/src/pages/org/orgUnitCorrectionPatch.ts`
  - 与 append patch 构造收敛为同一工具

---

## 5. 接口契约（实现级冻结）

## 5.1 统一写入 API（新增）

- `POST /org/api/org-units/write`

### 5.1.1 Request

```json
{
  "intent": "create_org | add_version | insert_version | correct",
  "org_code": "A001",
  "effective_date": "2026-02-20",
  "target_effective_date": "2026-02-10",
  "request_code": "ui-write-20260220-001",
  "patch": {
    "name": "Finance Shared Service",
    "parent_org_code": "P001",
    "status": "active",
    "is_business_unit": true,
    "manager_pernr": "1234",
    "ext": {
      "org_type": "DEPARTMENT",
      "x_custom_text": null
    }
  }
}
```

字段约束：

1. `intent` 必填：
   - `create_org`：创建组织
   - `add_version`：追加新版本
   - `insert_version`：区间插入
   - `correct`：更正选中记录
2. `request_code` 必填：用于写入幂等（落库为 `org_events.request_code`）；禁止同时存在 `request_id`（避免双字段同一事实漂移）。
3. `target_effective_date` 仅 `correct` 必填。
4. `patch` 必须是 object；未知字段一律 400（`DisallowUnknownFields` + fail-closed）。
5. 禁止客户端提交 `ext_labels_snapshot`。

### 5.1.2 Response

```json
{
  "org_code": "A001",
  "effective_date": "2026-02-20",
  "event_type": "CREATE | UPDATE | CORRECT_EVENT",
  "event_uuid": "...",
  "fields": {
    "name": "Finance Shared Service",
    "parent_org_code": "P001",
    "status": "active"
  }
}
```

### 5.1.3 路由/权限

- route class：`internal_api`
- 权限：沿用 orgunit admin 写权限
- 路由注册：`internal/server/handler.go`

## 5.2 删除接口（保持不变，调用策略调整）

- `POST /org/api/org-units/rescinds`
- `POST /org/api/org-units/rescinds/org`

前端根据版本数自动选择调用，不新增删除 API。

## 5.3 write-capabilities 契约收敛（冻结）

为避免 UI 写规则硬编码，冻结为“一个查询入口 + intent 维度”：

- `GET /org/api/org-units/write-capabilities?intent=...&org_code=...&effective_date=...&target_effective_date=...`

返回最小契约：

```json
{
  "intent": "create_org | add_version | insert_version | correct",
  "enabled": true,
  "deny_reasons": [],
  "allowed_fields": ["name", "parent_org_code", "status", "is_business_unit", "manager_pernr", "...ext keys..."],
  "field_payload_keys": {
    "name": "name",
    "parent_org_code": "parent_id",
    "status": "status",
    "is_business_unit": "is_business_unit",
    "manager_pernr": "manager_pernr"
  }
}
```

规则：

1. `allowed_fields` 与 `field_payload_keys` 必须一致（前端 fail-closed）。
2. `correct` 返回值必须覆盖 `status` 与其它 core/ext 字段并存能力（对应用户冻结语义）。
3. 旧端点（`append-capabilities` / `mutation-capabilities`）在过渡期可保留，但仅作为 `write-capabilities` 薄包装输出，避免双规则漂移。

---

## 6. 事件与内核改造（实现级冻结）

### 6.1 新增事件类型：`UPDATE`（有效事件日槽位）

在 OrgUnit **有效事件**基础集合新增：`UPDATE`。用途：

- `add_version/insert_version` 场景下，一次提交多字段变化 -> 一个 `UPDATE` 事件（占用 day-slot）。
- `correct` 场景：DB 仍写 `CORRECT_EVENT`（保持 target_event_uuid 审计链），但 **replay 视角**下会把“被更正后的目标事件”按需解释为 `UPDATE`，以支持 `status + 其他字段` 同次更正（见 §6.5）。

### 6.2 schema / 约束变更清单（冻结）

需要同步修改：

1. `org_events.event_type` CHECK 增加 `UPDATE`
2. `is_org_event_snapshot_presence_valid(...)` 将 `UPDATE` 纳入 before/after 必填集合
3. `org_events_effective` 与 `org_events_effective_for_replay` 的基础事件集合加入 `UPDATE`
4. `is_org_ext_payload_allowed_for_event(...)` 允许 `UPDATE`
5. `submit_org_event(...)` 允许 `p_event_type=UPDATE`
6. day-slot 守卫把 `UPDATE` 纳入“占槽位”的有效事件集合，确保同 org 同日仍只有一个有效事件（对齐 UI 目标）。落点：新增一条迁移调整 `orgunit.guard_org_events_one_per_day_effective()` 的 `NEW.event_type IN (...)` 集合（不要修改历史迁移文件）。

涉及文件：

- `modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql`
- `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`
- `modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql`

### 6.3 update 语义（冻结）

`UPDATE` payload 允许 key（canonical）：

- `name`
- `parent_id`
- `status`（`active|disabled`）
- `is_business_unit`
- `manager_uuid` / `manager_pernr`
- `ext`
- `ext_labels_snapshot`

兼容 key（legacy / correction merge）：

- `new_name`（等价 `name`）
- `new_parent_id`（等价 `parent_id`）

说明：

- `UPDATE` 的核心目标是“单事件承载多字段”；payload key 层面允许兼容旧 key，以便 `CORRECT_EVENT` merge 后的 replay 不需要为每个 target event type 走不同 key 形态。

### 6.4 UPDATE 执行顺序（冻结）

在 `apply_update_logic(...)` 内固定顺序（一次 split，一次性最终重建 path）：

1. parent（move）
2. status（enable/disable）
3. is_business_unit
4. manager
5. ext
6. name（最后）

并要求（对应用户语义冻结）：

- `rebuild_full_name_path_subtree(...)` 在最终 parent + name 确定后执行（一次性最终重建）。
- `apply_move_logic(...)` 与 `apply_set_business_unit_logic(...)` 需要去除 `status='active'` 的硬依赖，以满足：
  - disabled 记录允许 move；
  - `status` 与 `is_business_unit` 可同次提交、且不因顺序触发 `ORG_INACTIVE_AS_OF`。

### 6.5 更正（correct）与“状态 + 其他字段”合并

为满足“更正必须支持一次改 `status + 其它字段`”，冻结实现（以 replay 解释为准）：

1. `correct` intent 仍提交 `CORRECT_EVENT`（保持 `target_event_uuid` 审计链；UI 不再调用 `status-corrections`）。
2. correction payload 采用与 `UPDATE` 一致的字段 key（`status/name/parent_id/is_business_unit/manager_uuid/ext/...`），避免按 target event type 做 key 变形。
3. 在 `org_events_effective_for_replay(...)`（仅 replay 视角）中：当 `lc.correction_type='CORRECT_EVENT'` 且 merge 后 payload 命中任一 `UPDATE` 字段 key（含兼容 key）时，将 **被更正的目标事件** 的 `event_type` 输出为 `UPDATE`。
4. `rebuild_org_unit_versions_for_org_with_pending_event(...)` 增加 `UPDATE` 分支：调用 `apply_update_logic(...)`，并在 parent+name 最终确定后只重建一次 `full_name_path`。

备注：

- `org_events_effective`（读视角）不必把 event_type 改写成 `UPDATE`：它仍用于“有效事件流”与 day-slot 守卫；而 `for_replay` 是“重放执行口径”，可独立解释 event_type。

---

## 7. Go 服务层改造（实现级）

### 7.1 新增/调整服务接口

在 `modules/orgunit/services/orgunit_write_service.go`：

1. 新增请求模型：`WriteOrgUnitRequest`（intent + date + patch）
2. 新增方法：`Write(ctx, tenantID, req)`
3. 现有 `Create/Rename/Move/Disable/Enable/SetBusinessUnit` 逐步改为薄包装（内部调用 `Write`），避免双规则

### 7.2 patch 标准化流水线

1. 解析 patch -> 标准化字段（org_code/parent/status/manager/ext）
2. 解析 `manager_pernr` -> `manager_uuid`
3. 解析 DICT ext -> `ext_labels_snapshot`
4. 校验 allowed fields（policy 单点）
5. 生成 kernel payload

### 7.3 policy/capabilities 收敛

- 将 append + correction 的字段白名单能力收敛到一套 `write-capabilities` 输出（契约见 §5.3）。
- 前端所有可编辑判断统一依赖 capabilities；失败一律 fail-closed。

---

## 8. 删除自动判定（实现级）

前端逻辑（`OrgUnitDetailsPage`）：

1. 读 `versions`。
2. `len>1` -> 调 `rescinds`（带 `effective_date`）。
3. `len==1` -> 调 `rescinds/org`。
4. 统一确认框，必须输入 `reason`。

后端无需新增删除 API。

---

## 9. 错误码与返回口径（冻结）

继续复用稳定码，新增仅在必要时补充：

- `EVENT_DATE_CONFLICT`
- `EFFECTIVE_DATE_OUT_OF_RANGE`
- `ORG_PARENT_NOT_FOUND_AS_OF`
- `ORG_CYCLE_MOVE`
- `ORG_ROOT_CANNOT_BE_MOVED`
- `ORG_ROOT_BUSINESS_UNIT_REQUIRED`
- `ORG_INACTIVE_AS_OF`
- `ORG_EVENT_NOT_FOUND`
- `ORG_REQUEST_ID_CONFLICT`（幂等键冲突：同 request_code 不同请求体；错误码命名沿用历史稳定码）
- `PATCH_FIELD_NOT_ALLOWED`
- `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`
- `ORG_EXT_LABEL_SNAPSHOT_REQUIRED`

新增建议码：

- `ORG_UPDATE_PATCH_EMPTY`（无字段变化）
- `ORG_INTENT_NOT_SUPPORTED`（非法 intent）

---

## 10. 实施步骤（细化到文件）

1. [ ] 契约冻结（先文档）
   - [ ] 更新 `DEV-PLAN-075A/075D/083A/101I` 与本计划对齐
2. [ ] DB
   - [ ] `00002/00003/00015` 增加 `UPDATE` 事件全链路支持（含 replay 分支）
   - [ ] 新增迁移：day-slot 守卫 trigger/函数把 `UPDATE` 视为有效事件（避免同日多事件）
   - [ ] 补齐更正 replay 解释：更正 payload 命中 UPDATE 字段时，将目标事件 replay 为 `UPDATE`
   - [ ] 放宽内核约束：move 目标 org 不再要求 `status='active'`；set_business_unit 不再要求 active（仍保留 root guard）
3. [ ] Go
   - [ ] `orgunit_write_service.go` 新增 `Write(...)`
   - [ ] `orgunit_pg_store.go` 保持 `SubmitEvent`，增加 `UPDATE` payload 提交路径
   - [ ] `internal/server/orgunit_api.go` 新增 `handleOrgUnitsWriteAPI`
   - [ ] `internal/server/handler.go` 注册 `/org/api/org-units/write`
4. [ ] Web
   - [ ] `OrgUnitDetailsPage.tsx` 移除动作类型下拉与原子按钮
   - [ ] `OrgUnitsPage.tsx` 新建组织接入统一写接口
   - [ ] patch 构造工具统一
5. [ ] 测试
   - [ ] SQL 回归 + Go 单测 + API 契约 + 前端单测 + E2E
6. [ ] 证据
   - [ ] `docs/dev-records/dev-plan-108-execution-log.md`

---

## 11. 测试计划（实现级）

### 11.1 Kernel/SQL

1. `UPDATE` 同时改 `parent + name`，验证 path 与 full_name_path 正确。
2. `UPDATE` 同时改 `status + is_business_unit`，验证 root guard 与状态约束。
3. `UPDATE` 含 DICT ext，验证 label snapshot 必填/清空语义。
4. `CORRECT_EVENT` 同时改 `status + 其它字段`（如 `name+parent`），验证 replay 按 `UPDATE` 解释且单次提交生效。
5. disabled 记录 `UPDATE` move：验证允许移动且 cycle/root guard 正常。

### 11.2 Go/API

1. `/org/api/org-units/write`：unknown field/empty patch/invalid intent。
2. manager_pernr 校验、parent 解析、dict label 解析。
3. 删除自动判定逻辑（前端）对应后端错误码映射稳定。

### 11.3 前端

1. CRUD 区仅 5 按钮可见。
2. 无 `record_change_type` 下拉。
3. 一次提交多字段写入成功（含 ext + parent + name）。
4. 删除多版本/单版本自动分流。

### 11.4 E2E 最小闭环

1. 新建组织 -> 新建版本（多字段）-> 列表/详情回显。
2. 插入版本（多字段）-> 审计链单事件可见。
3. 更正（含状态 + 名称）-> 审计与详情一致。
4. 删除自动判定两路径都通过。

---

## 12. 风险与缓解

1. **风险：引入 UPDATE 后与既有原子事件并存，规则漂移**
   - 缓解：旧接口改薄包装到统一写核心，禁止并行两套校验。
2. **风险：CORRECT_EVENT 在 replay 中按 UPDATE 解释后，兼容 key 与 canonical key 可能错配**
   - 缓解：统一 correction payload key（优先 canonical），并补齐 replay 键兼容测试。
3. **风险：按钮收敛后用户找不到运维入口**
   - 缓解：明确 CRUD 与运维分区，运维入口保留但视觉降权。

---

## 13. 验收标准（DoD）

1. CRUD 区仅 5 按钮，无动作类型下拉。
2. 新建版本/插入版本/更正可一次提交多字段，后端只落一个事件。
3. move+rename 同次提交时，路径与名称最终一致，且审计可解释。
4. 删除自动判定按版本数分流，均为 rescind 语义。
5. 所有命中门禁通过并留执行证据。

---

## 14. 需对齐/修订的既有计划文档（以 108 为准，先登记后修改）

> 说明：以下文档中有“已完成”的历史结论，但其部分契约/交互在 108 生效后会变成**过时或冲突**。
> 本节作为“改动登记册（register）”，先列清单再逐一回写，避免遗漏导致后续评审口径漂移。

### 14.1 登记清单（按冲突强度排序）

- [x] `docs/archive/dev-plans/082-org-module-field-mutation-rules-investigation.md`
  - 冲突：按 target_event_type 白名单限制更正字段（含 status 只能 CORRECT_STATUS），与 108“更正支持 status+其它字段同次提交”冲突。
  - 修订：新增“108 对齐补充”章节，更新字段矩阵为 intent/字段编辑视角，并标注旧矩阵为历史实现口径。
- [x] `docs/archive/dev-plans/075a-orgunit-records-ui-and-editing-issues.md`
  - 冲突：add/insert 强依赖 `record_change_type`；与 108“取消动作类型选择、统一字段编辑表单”冲突。
  - 修订：标注向导/变更类型部分为过渡形态；将目标口径迁移到 108。
- [x] `docs/archive/dev-plans/101i-orgunit-effective-date-record-add-insert-ui-and-constraints.md`
  - 冲突：明确“不引入 UPDATE/不支持全字段”，与 108 的 `UPDATE`/全字段表单冲突。
  - 修订：新增“108 生效后的新口径（取代 record_change_type）”章节，并把 101I 定位为 108 前过渡实现。
- [x] `docs/archive/dev-plans/075d-orgunit-status-field-active-inactive-selector.md`
  - 冲突：冻结“状态变更必须独立动作、correct 不承载状态”，与 108“状态是字段，可同次提交”冲突。
  - 修订：保留 include_disabled/可达性 SSOT；调整写入口与交互矩阵到 108 的字段编辑模式；`status-corrections` 标注为兼容入口（UI 不再主路径依赖）。
- [x] `docs/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
  - 冲突：依赖 `CORRECT_STATUS` 作为同日纠错主路径；108 改为 `CORRECT_EVENT` 支持 status 合并（replay 解释 UPDATE）。
  - 修订：新增“108 对齐补充”：同日纠错 UI 不再独立入口；接口保留兼容但不再作为推荐路径。
- [x] `docs/archive/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
  - 冲突：capabilities 以 `correct_event/correct_status/...` 结构为 SSOT；108 收敛为 `write-capabilities(intent=...)`。
  - 修订：新增“108 后 SSOT 迁移”章节：`mutation-capabilities/append-capabilities` 过渡为 `write-capabilities` 的薄包装，避免双规则。
- [x] `docs/archive/dev-plans/083a-orgunit-append-actions-capabilities-policy-extension.md`
  - 冲突：冻结“不新增事件语义（不引入 UPDATE）”，与 108 引入 `UPDATE` 冲突。
  - 修订：标注为 108 前 append 阶段性收口；新增迁移策略：append-capabilities -> write-capabilities。
- [x] `docs/dev-plans/073-orgunit-crud-implementation-status.md`
  - 冲突：以动作型 endpoint（rename/move/enable/disable/set BU）作为交付粒度；108 改为统一 write。
  - 修订：补“108 统一写入”章节与迁移路线（旧端点薄包装到 write，避免双规则）。
- [x] `docs/dev-plans/096-org-module-full-migration-and-ux-convergence-plan.md`
  - 冲突：UI 以多个 Dialog（Rename/Move/Enable/Disable）为目标形态；108 收敛为 5 按钮 + 字段编辑表单。
  - 修订：新增“108 后 UX 收敛口径”章节，更新验收表的 UI 形态描述。
- [x] `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
  - 冲突：更正 UI 依赖 `mutation-capabilities` 作为长期 SSOT；108 计划新增 `write-capabilities`。
  - 修订：补充“capabilities SSOT 迁移到 write-capabilities”的说明（不推翻 ext_fields/label snapshot 口径）。
- [x] `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
  - 冲突：100E1 冻结了“改生效日必须独立提交（排他模式）”，与 108 的“允许改生效日 + 改其它字段同次提交”冲突。
  - 修订：标注为 108 前口径，并明确 108 后移除该排他规则。

### 14.2 执行证据（实施时新增）

- [ ] `docs/dev-records/dev-plan-108-execution-log.md`

## 15. 关联文档

- `AGENTS.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- `docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- `docs/archive/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
- `docs/archive/dev-plans/109-request-code-unification-and-gate.md`

---

## 16. 场景覆盖盘点与缺口（以 108 目标为准）

### 16.1 已覆盖的主场景（108 DoD 对齐）

1. **新建组织（create_org）**：一次提交可维护 core + ext（含 parent/status/is_business_unit/manager/name）。
2. **新建版本（add_version）**：一次提交可同时改多个字段，落单一 `UPDATE` 事件。
3. **插入版本（insert_version）**：同上，落单一 `UPDATE` 事件，并沿用既有 add/insert 日期区间约束。
4. **更正（correct）**：必须支持 `status + 其他字段` 同次提交，且仍保持单 `CORRECT_EVENT` 审计链（replay 解释为 `UPDATE` 应用）。
5. **删除（delete）**：自动判定“删记录（rescind event）/删组织（rescind org）”，且仍是一键入口。

### 16.2 关键组合场景（本计划显式覆盖）

- disabled 记录允许 move（parent 变更）。
- 允许同次提交 `status + is_business_unit`，且可伴随其他字段（name/parent/ext 等）。
- move + rename 同次提交：parent 先、name 后，保证最终 `full_name_path` 可解释。

### 16.3 当前仍存在的缺口/需冻结的边界（实现前必须确认）

1. **允许：更正可“改生效日（effective_date correction）+ 改其他字段”同次提交（已冻结 2026-02-18）**
   - 校验与执行口径（冻结）：
     1) 先以“更正后 effective_date”（即 `patch.effective_date`）作为最终生效日参与所有校验（区间边界、同日冲突、父级有效性、cycle/root guard 等）。
     2) `ext` enabled-as-of 与 DICT label snapshot 解析使用 **更正后 effective_date** 作为 as-of（避免“写入通过但回显消失/label 不一致”）。
     3) 若同次提交包含 `parent_org_code`：父级存在性与有效性以更正后 effective_date 判定。
2. **允许：目标父级可为 disabled（已冻结 2026-02-18）**
   - 语义约束（冻结）：
     - move 的 parent 存在性校验不再要求 `status='active'`；只要 parent 在该日有效即可（`validity @> as_of`）。
     - UI 必须提示：若目标上级为 disabled，默认视图（`include_disabled=0`）可能不可达；需要开启“显示无效组织（include_disabled=1）”查看/恢复。
3. **请求幂等字段命名统一方案（已冻结 2026-02-18）**
   - 统一为：**`request_code`**（JSON 字段名 + Go 字段名 + DB `org_events.request_code` 一致）。
   - `request_id` 视为历史命名：108 新增接口禁止出现 `request_id` 字段（严格解码 + unknown field 400）。
   - `X-Request-ID` HTTP Header 仅用于链路追踪/日志关联，不作为业务幂等键（避免与 body 幂等混淆）。
   - 具体改造与门禁实施由 `DEV-PLAN-109` 承接。
