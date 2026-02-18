# DEV-PLAN-082：Org 模块业务字段修改规则全量调查（排除元数据）

**状态**: 已完成（2026-02-11 04:00 UTC）；2026-02-18 起部分写入口径被 `DEV-PLAN-108` 取代

## 0. 与 DEV-PLAN-108 的对齐补充（2026-02-18）

> 本文件最初用于“盘点当时实现”的字段可改范围（以 target_event_type 白名单为核心）。
> 自 `DEV-PLAN-108` 起，OrgUnit 写入将从“动作/事件驱动（rename/move/enable/disable/set BU）”收敛为“字段编辑 + intent 自动判定 + 单事件多字段”。
> 因此，本文件 §4~§7 中关于“更正字段白名单/状态必须 CORRECT_STATUS”的描述将变为**历史实现口径**，不再作为新能力的 SSOT。

108 生效后的关键口径（以 108 为准）：

1. UI/调用方不再选择 `record_change_type`；只提交字段 patch。
2. 更正必须支持 `status + 其他字段` 同次提交（仍保持单 `CORRECT_EVENT` 审计链；replay 视角解释为 `UPDATE` 应用）。
3. append（新建版本/插入版本）允许一次提交多字段，落一个 `UPDATE` 事件（占 day-slot）。
4. 允许 disabled 记录 move；允许同次提交 `status + is_business_unit`（不再因顺序触发 `ORG_INACTIVE_AS_OF`）。

对照关系：

- 旧：`POST /org/api/org-units/{rename|move|enable|disable|business-unit}` + `append-capabilities` / `mutation-capabilities`
- 新：`POST /org/api/org-units/write` + `GET /org/api/org-units/write-capabilities`（intent 维度）

> 注：本文件仍保留其“当时实现盘点”的价值（用于回归/迁移对照），但任何新实现/评审应优先参考 108。

## 1. 背景

近期在 OrgUnit 详情页尝试修改“部门负责人（manager_pernr）”时，出现 `PATCH_FIELD_NOT_ALLOWED`（前端文案：字段不允许更正）。
为避免后续在“字段可改范围”上继续出现认知偏差，本调查汇总当前 Org 模块（OrgUnit）在 **CRUD + 纠错 + 删除（撤销）** 全操作下的字段修改规则，形成单点事实源。

## 2. 调查范围与口径

### 2.1 范围（In Scope）
- Org 模块业务对象：`orgunit`。
- 写入入口：
  - UI 表单：`/org/nodes`（含新增版本/插入版本/状态变更/删除记录/删除组织）
  - API：`/org/api/org-units/*`
  - DB Kernel：`orgunit.submit_*` 与 `orgunit.apply_*`
- 操作类型：
  - Create / Read / Update / Delete（Delete 为事件撤销语义）
  - 同日状态纠错、事件纠错、单记录撤销、整组织撤销

### 2.2 非范围（Out of Scope）
- 元数据字段（本调查明确排除）：`event_uuid/request_id/request_code/initiator_uuid/tx_time/transaction_time/created_at/*snapshot*` 等。
- 非 orgunit 模块（如 staffing/jobcatalog/person）。

### 2.3 业务字段口径
以运行态业务可见字段为主：
- `org_code`
- `effective_date`（对应版本起始日）
- `name`
- `parent_org_code`（内部投影为 `parent_id`）
- `status`
- `is_business_unit`
- `manager_pernr`（内部投影为 `manager_uuid`）

补充：`node_path/full_name_path/validity` 属投影派生字段，不支持外部直接赋值。

## 3. 写入入口总览

1. UI 入口（页面表单）
- `internal/server/orgunit_nodes.go`
- 覆盖动作：
  - 新建组织（Create）
  - 新增版本 / 插入版本（封装为 RENAME/MOVE/SET_BUSINESS_UNIT）
  - 状态变更（DISABLE/ENABLE 或 CORRECT_STATUS）
  - 删除记录（RESCIND_EVENT）
  - 删除组织（RESCIND_ORG）
  - 详情页“保存”（CORRECT_EVENT）

2. API 入口（Internal API）
- `internal/server/orgunit_api.go`
- 覆盖端点：
  - `POST /org/api/org-units`（Create）
  - `POST /org/api/org-units/rename`
  - `POST /org/api/org-units/move`
  - `POST /org/api/org-units/disable`
  - `POST /org/api/org-units/enable`
  - `POST /org/api/org-units/business-unit`
  - `POST /org/api/org-units/corrections`
  - `POST /org/api/org-units/status-corrections`
  - `POST /org/api/org-units/rescinds`
  - `POST /org/api/org-units/rescinds/org`

3. 领域服务与 Kernel
- 服务层：`modules/orgunit/services/orgunit_write_service.go`
- 持久化层：`modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`
- Kernel SQL：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`

## 4. CRUD + 全操作规则（字段视角）

## 4.1 Create（新增组织）
- 可写字段：`org_code`、`effective_date`、`name`、`parent_org_code`、`is_business_unit`、`manager_pernr(API)`。
- 规则要点：
  - UI 创建路径当前不提供 `manager_pernr` 输入；API Create 支持 `manager_pernr`。
  - 根组织创建时 `is_business_unit` 必须为 `true`（或不显式传 false）。
  - `org_code` 创建后写入 `org_unit_codes`，后续无 rename org_code 能力。

## 4.2 Read（读取）
- 不修改任何业务字段。
- `tree_as_of/effective_date` 仅影响视图选中版本，不直接写库。

## 4.3 Update（更新类）

### A) 事件型更新（新增一条业务事件）
- `RENAME`：仅改 `name`（payload key: `new_name`）
- `MOVE`：仅改 `parent_org_code`（payload key: `new_parent_id`）
- `SET_BUSINESS_UNIT`：仅改 `is_business_unit`
- `DISABLE/ENABLE`：仅改 `status`

### B) 纠错型更新（CORRECT_EVENT）
以“目标生效日对应的有效事件类型”为准，字段白名单如下：
- 目标事件 = `CREATE`：可更正
  - `effective_date`
  - `name`
  - `parent_org_code`
  - `is_business_unit`
  - `manager_pernr`（映射到 `manager_uuid`）
- 目标事件 = `RENAME`：仅可更正 `effective_date`、`name`
- 目标事件 = `MOVE`：仅可更正 `effective_date`、`parent_org_code`
- 目标事件 = `SET_BUSINESS_UNIT`：仅可更正 `effective_date`、`is_business_unit`
- 目标事件 = `DISABLE/ENABLE`：仅可更正 `effective_date`（状态本身走 CORRECT_STATUS）

不在白名单内将返回：`PATCH_FIELD_NOT_ALLOWED`。

### C) 同日状态纠错（CORRECT_STATUS）
- 仅针对目标事件类型为 `ENABLE/DISABLE` 的记录。
- 只允许改目标状态：`target_status in {active, disabled}`。
- 若目标事件不是 `ENABLE/DISABLE`：`ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET`。

## 4.4 Delete（删除语义）
本模块无物理删除写路径，UI 的“删除”均为撤销事件：
- 删除记录：`RESCIND_EVENT`
- 删除组织：`RESCIND_ORG`（批量撤销组织下有效事件）

含义：通过“事件失效/撤销”实现逻辑删除，不做业务主表硬删除。

## 5. 字段 × 操作矩阵（排除元数据）

| 字段 | Create | Rename | Move | Set BU | Disable/Enable | Correct Event | Correct Status | Delete Record/Org |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `org_code` | 可写（初次） | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 |
| `effective_date` | 可写 | 可写 | 可写 | 可写 | 可写 | 可更正（所有目标事件） | 不改日期 | 不改日期（目标定位） |
| `name` | 可写 | 可写（new_name） | 不支持 | 不支持 | 不支持 | 仅目标 CREATE/RENAME 可更正 | 不支持 | 不支持 |
| `parent_org_code` | 可写 | 不支持 | 可写（new_parent_id） | 不支持 | 不支持 | 仅目标 CREATE/MOVE 可更正 | 不支持 | 不支持 |
| `status` | 默认 active | 不支持 | 不支持 | 不支持 | 通过 ENABLE/DISABLE 改 | 不支持（除日期） | 可更正 active/disabled | 通过撤销影响存在性 |
| `is_business_unit` | 可写 | 不支持 | 不支持 | 可写 | 不支持 | 仅目标 CREATE/SET_BUSINESS_UNIT 可更正 | 不支持 | 不支持 |
| `manager_pernr` | API 可写（映射 `manager_uuid`） | 不支持 | 不支持 | 不支持 | 不支持 | **仅目标 CREATE 可更正** | 不支持 | 不支持 |
| `full_name_path/node_path/validity` | 派生 | 派生更新 | 派生更新 | 派生更新 | 派生更新 | 重建派生 | 重建派生 | 重建派生 |

## 6. 你遇到“字段不允许更正”的直接原因

当版本详情显示事件类型为 `RENAME` 时，提交 `manager_pernr` 会命中服务层白名单拒绝：
- `manager_pernr` 仅在目标事件 `CREATE` 时允许更正。
- 前端把 `PATCH_FIELD_NOT_ALLOWED` 映射为“字段不允许更正”。

这与当前实现一致，不是偶发错误。

## 7. 关键约束（跨操作）

- 生效日更正必须满足“前后版本区间约束 + 同日唯一约束”（越界/冲突会失败）。
- 高风险重排保护：特定 create 重排场景会触发 `ORG_HIGH_RISK_REORDER_FORBIDDEN`。
- 根组织约束：
  - 根组织不能 MOVE。
  - 根组织不能被置为非业务单元。
  - 根组织不允许删除（`ORG_ROOT_DELETE_FORBIDDEN`）。

## 8. 结论与后续建议

### 8.1 结论
- 当前 org 模块对“字段可改范围”采取**事件类型白名单**模型，而不是“详情页字段自由编辑”模型。
- UI 编辑表单暴露了 `manager_pernr` 输入，但当目标版本事件不是 CREATE 时会被后端拒绝，存在体验落差。

### 8.2 建议（后续可立项）
1. UI 按目标事件类型动态禁用/隐藏不允许更正字段（至少在保存前提示）。
2. 若业务要求“任意版本可改负责人”，需新增明确事件语义（例如 Manager 变更事件）并更新服务层白名单与回归测试。

## 9. 证据索引（代码事实源）

- API 写入口与错误码：`internal/server/orgunit_api.go`
- UI 写入口（record actions / edit 保存）：`internal/server/orgunit_nodes.go`
- 前端错误文案映射（PATCH_FIELD_NOT_ALLOWED）：`internal/server/orgunit_nodes.go`
- 服务层字段白名单：`modules/orgunit/services/orgunit_write_service.go`
- 白名单测试（manager_not_allowed / manager_success）：`modules/orgunit/services/orgunit_write_service_test.go`
- Kernel 事件提交与应用逻辑：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`
- 业务表字段定义（org_events/org_unit_versions/org_unit_codes）：`modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql`
