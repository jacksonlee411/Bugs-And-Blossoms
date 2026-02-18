# DEV-PLAN-100E1：OrgUnit Mutation Policy 单点化 + 更正链路支持 `patch.ext`（作为 DEV-PLAN-100E 前置）

**状态**: 已完成（2026-02-15 03:26 UTC）

> 定位：本计划只补齐 **DEV-PLAN-100E（Phase 4A UI）** 所依赖的后端前置改造，确保“capabilities-driven 编辑 + 扩展字段写入”具备可实施的契约与实现基础。
>
> SSOT：能力模型与对外契约以 `DEV-PLAN-083` 为准；扩展字段读写与错误码以 `DEV-PLAN-100C/100D/100D2` 为准；门禁/触发器以 `AGENTS.md` 与 `DEV-PLAN-012` 为准。

## 0. Stopline（本计划明确不做）

- 不新增/变更 DB schema、迁移、sqlc（如需新增表/列必须另起 dev-plan 且先获用户确认）。
- 不实现/调整 MUI UI（由 `DEV-PLAN-100E/101` 承接）。
- 不引入 legacy/双链路；写入仍必须走 DB Kernel 的 `submit_*`（One Door，见 `AGENTS.md`、`DEV-PLAN-026/100C`）。

## 1. 背景与问题陈述

`DEV-PLAN-100E` 依赖“capabilities 外显 + capabilities 驱动编辑 + 扩展字段写入”三件事同时成立：

1. `GET /org/api/org-units/mutation-capabilities` 必须与 `DEV-PLAN-083` 冻结口径一致，且输出稳定（`allowed_fields/field_payload_keys/deny_reasons`）。
2. `POST /org/api/org-units/corrections` 必须支持扩展字段 patch：`patch.ext`，并对其做 **fail-closed** 校验，且与 capabilities 一致。
3. DICT 字段写入必须由服务端生成 `ext_labels_snapshot`（UI 不提交；对齐 `DEV-PLAN-100D`）。

本计划开始时仓库状态（以当时代码为准，作为问题陈述）：

- capabilities API 已存在（`internal/server/orgunit_mutation_capabilities_api.go`），但其“扩展字段并入 allowed_fields”的规则与 `DEV-PLAN-083` 冻结选择存在漂移风险（`DEV-PLAN-083` 要求把 enabled ext 字段集合 `E` 并入 allowed_fields；现实现曾以 target=CREATE 作收紧）。
- 更正写入链路（`internal/server/orgunit_api.go` → `modules/orgunit/services/orgunit_write_service.go`）尚未接收 `patch.ext`，因此 **100E 的编辑闭环无法成立**。
- `DEV-PLAN-083` 约定的策略单点（`ResolvePolicy/AllowedFields/ValidatePatch`）尚未落地，导致 capabilities 与写入校验仍可能分叉。

因此需要一个专门的“前置改造计划”来收敛：**策略单点 + capabilities 对齐 + corrections 支持 ext patch**。

## 2. 目标（Goals）

### 2.1 必达目标（完成后 100E 才能开工）

1. [x] 落地 `DEV-PLAN-083` 的策略单点（最小覆盖 `correct_event`）：
   - [x] 在 `modules/orgunit/services/` 增加 `orgunit_mutation_policy.go`（或等价命名），实现：
     - `ResolvePolicy(...)`
     - `AllowedFields(...)`
     - `ValidatePatch(...)`
   - [x] 单测覆盖（至少覆盖 `correct_event` 的 core 矩阵 + ext 合并规则 + deny_reasons 顺序）。
2. [x] capabilities API 对齐 `DEV-PLAN-083`（避免 UI 猜测）：
   - [x] `allowed_fields` 与 `field_payload_keys` 一致、稳定排序；
   - [x] `deny_reasons` 稳定排序（复用既有优先级规则即可）；
   - [x] **扩展字段合并规则**：对 `effective_date` 下 enabled 的 ext 字段集合 `E`，并入 `allowed_fields`（见 `DEV-PLAN-083` §5.3）。
3. [x] corrections 写入支持 `patch.ext`（108 前口径）：
   - [x] `POST /org/api/org-units/corrections` 请求体支持 `patch.ext`（object；key 为 `field_key`）。
   - [x] 服务端必须按策略单点（AllowedFields/ValidatePatch）对 `patch` 做 fail-closed 校验（禁止“禁用但仍随请求提交”）。
   - [x] DICT：服务端基于 options resolver 生成 `patch.ext_labels_snapshot[field_key]=canonical_label`；UI 侧不得提交该字段（对齐 `DEV-PLAN-100D`）。
   - [x]（已被 108 取代）当 `patch.effective_date` 与 target 不一致时：进入“生效日更正模式”，除 `effective_date` 外其它字段（含 ext）一律拒绝（对齐 `DEV-PLAN-100E` 的风险控制）。
   - 108 新口径：允许“改生效日 + 改其它字段”同次提交（校验/label snapshot as-of 以更正后 effective_date 为准；SSOT：`DEV-PLAN-108`）。

### 2.2 交付物（Deliverables）

- [x] `modules/orgunit/services/orgunit_mutation_policy.go` + `modules/orgunit/services/orgunit_mutation_policy_test.go`
- [x] capabilities API 的契约测试/回归测试补齐（`internal/server/..._test.go`）
- [x] 更正链路（handler + service + store 适配）支持 `patch.ext` 的测试补齐
- [x] 执行日志：`docs/dev-records/dev-plan-100e1-execution-log.md`（按 `DEV-PLAN-010` 口径记录门禁证据）

## 3. 非目标（Non-Goals）

- 不在本计划内实现 ENTITY options（若当前字段清单未命中 ENTITY，保持 fail-closed；如需支持，另立 dev-plan）。
- 不在本计划内重写 OrgUnit 写入语义（只做“策略收口 + 能力外显 + 扩展字段更正接入”，保持行为等价或更严格 fail-closed）。

## 4. 关键设计决策（KDD）

### 4.1 策略单点位置（冻结）

- 按 `DEV-PLAN-083`，策略实现落在 `modules/orgunit/services/`（例如 `orgunit_mutation_policy.go`），由写入链路直接消费，并由 capabilities API 复用，避免两套规则漂移。
- 本计划最小覆盖：`correct_event` 的 `ResolvePolicy/AllowedFields/ValidatePatch`；其余动作（`correct_status/rescind_*`）不在本计划强制范围内。

### 4.2 共享元数据包落点（冻结）

为避免 services 侧生成 DICT label 快照时“再造一份 registry”，本计划冻结一个**唯一共享包**，用于承载 OrgUnit 扩展字段的静态元数据与 DICT 解析逻辑：

- 包路径（冻结）：`modules/orgunit/domain/fieldmeta`（域内专用，不跨模块复用）
- 共享包必须提供的最小 API（冻结）：
  - `LookupFieldDefinition(fieldKey) (def, ok)`
  - `ListFieldDefinitions() []def`
  - `DictCodeFromDataSourceConfig(raw json.RawMessage) (dictCode, ok)`（读取 `tenant_field_configs.data_source_config`）
  - `LookupDictLabel(dictCode, value) (label, ok)`（canonical label；不随 UI locale）
  - （可选但推荐）`ListDictOptions(dictCode, keyword, limit)`（用于 server 的 options endpoint 复用）
- `internal/server` 与 `modules/orgunit/services` 必须都从该共享包读取 field-definitions/DICT registry，禁止保留两份并行实现。

### 4.3 写入侧元数据读取能力（冻结）

services 侧要做到：

- 计算策略需要的 enabled ext 字段集合 `E`（对齐 `DEV-PLAN-083` §5.3）；
- DICT 写入需要读取 `tenant_field_configs.data_source_config` 来解析 dict_code；

因此冻结如下实现策略（不改 DB schema，仅新增读方法）：

- 扩展 `modules/orgunit/domain/ports.OrgUnitWriteStore`（允许其包含“写路径所需的读依赖”，当前接口已包含 `FindEventByEffectiveDate`）：
  - `ListEnabledTenantFieldConfigsAsOf(ctx, tenantID, asOf string) ([]TenantFieldConfig, error)`
- 新增 `modules/orgunit/domain/types.TenantFieldConfig`（或等价命名）最小字段集：
  - `FieldKey, ValueType, DataSourceType, DataSourceConfig`
- 在 `modules/orgunit/infrastructure/persistence/OrgUnitPGStore` 中实现该查询（SQL 以 `orgunit.tenant_field_configs` 为 SSOT；复用现有 day 粒度 enabled-as-of 口径）。

> 说明：本仓库早期阶段允许“写 store 需要少量读依赖”；但必须以单测/契约测试锁住口径，避免 drift。

### 4.4 请求解析与 fail-closed（冻结）

- `POST /org/api/org-units/corrections` 的 `patch` 必须显式支持 `ext`（object）。
- 客户端提交 `ext_labels_snapshot` **必须拒绝**（fail-closed）；服务端生成 label 快照并写入 DB patch（对齐 `DEV-PLAN-100D`）。
- JSON 解码策略（冻结）：
  - handler 侧对 request 采用严格解码（`DisallowUnknownFields`），并**显式声明** `ext` 与 `ext_labels_snapshot` 字段，以便对后者返回稳定错误码（而不是静默忽略）。

### 4.5 错误码口径（冻结，尽量复用既有稳定码）

- 客户端提交 `patch.ext_labels_snapshot`：返回 400 `PATCH_FIELD_NOT_ALLOWED`（把其视为“不允许更正的字段”）。
- `patch.ext` 中的字段不在 `allowed_fields`：返回 400 `PATCH_FIELD_NOT_ALLOWED`。
- DICT 值无法解析为合法 option（无法得到 canonical label）：返回 400 `ORG_INVALID_ARGUMENT`（detail 至少包含 `field_key/value`，便于排障）。
- 其余 ext 相关防线以 Kernel 稳定错误码为准（例如 `ORG_EXT_FIELD_NOT_CONFIGURED/ORG_EXT_FIELD_NOT_ENABLED_AS_OF/ORG_EXT_FIELD_TYPE_MISMATCH/ORG_EXT_LABEL_SNAPSHOT_REQUIRED`；SSOT：`DEV-PLAN-100C/100D`）。

## 5. 实施步骤（Execution Plan）

> 顺序：先“可复用元数据”→ 再“策略单点”→ 再“capabilities 对齐”→ 最后“corrections 支持 ext patch”（闭环）。

1. [x] 共享元数据包落地（对齐 §4.2）
   - [x] 新增共享包（推荐：`modules/orgunit/domain/fieldmeta`），迁移/复用现有 field-definitions + DICT registry + helpers：
     - [x] field-definitions（field_key/value_type/data_source_type/label_i18n_key/allow_filter/allow_sort）
     - [x] `DictCodeFromDataSourceConfig(...)`
     - [x] `LookupDictLabel(...)`（canonical label）
   - [x] 调整 `internal/server` 的 field-definitions/options/details displayValue 引用到共享包，确保行为不变（仅“搬家”，不改语义）。
2. [x] 写入侧元数据读取能力（对齐 §4.3）
   - [x] 扩展 `modules/orgunit/domain/ports.OrgUnitWriteStore` 增加 `ListEnabledTenantFieldConfigsAsOf(...)`。
   - [x] 新增 `modules/orgunit/domain/types.TenantFieldConfig`（最小字段集：field_key/value_type/data_source_type/data_source_config）。
   - [x] `modules/orgunit/infrastructure/persistence/OrgUnitPGStore` 实现该读方法（事务 + tenant 注入；fail-closed）。
   - [x] 单测覆盖：enabled-as-of 边界（`enabled_on/disabled_on` day 粒度半开区间）。
3. [x] mutation policy 单点（最小覆盖 correct_event）
   - [x] `modules/orgunit/services/orgunit_mutation_policy.go`：实现 `ResolvePolicy/AllowedFields/ValidatePatch`（`DEV-PLAN-083` §5.3 core 矩阵 + ext 合并）。
   - [x] 单测覆盖：合法/非法组合、allowed_fields/field_payload_keys 稳定排序、deny_reasons 稳定顺序。
4. [x] capabilities API 复用 policy（消除漂移）
   - [x] `internal/server/orgunit_mutation_capabilities_api.go` 改为通过 policy 计算 `allowed_fields/field_payload_keys`。
   - [x] 行为对齐 `DEV-PLAN-083`：enabled ext 字段集合 `E` 并入 `allowed_fields`（不再按 target=CREATE 特判）。
   - [x] 回归测试：deny reasons 顺序稳定（仍按既有优先级闭集）。
5. [x] corrections 支持 `patch.ext`（服务层为主，presentation 仅做传参）
   - [x] 扩展 corrections 请求 patch 结构以接收 `ext`（object），并显式声明 `ext_labels_snapshot` 字段用于 fail-closed 拒绝（对齐 §4.4/§4.5）。
   - [x] `modules/orgunit/services/orgunit_write_service.go`：
     - [x] 将 ext patch 纳入 patch builder（生成 `patch.ext`；DICT 生成 `patch.ext_labels_snapshot`）。
     - [x]（已被 108 取代）通过 policy 的 `ValidatePatch` 做 fail-closed 校验（含“生效日更正模式”排他规则：除 effective_date 外一律拒绝）。
     - 108 新口径：需移除此排他规则，改为以更正后 effective_date 做统一校验。
     - [x] 通过 store 读取 enabled ext configs 来解析 DICT 的 dict_code，并用共享包 `LookupDictLabel` 生成 canonical label。
   - [x] 测试：
     - [x] DICT：提交 `patch.ext.org_type="DEPARTMENT"` 时，写入 patch JSON 必含对应 label snapshot。
     - [x] clear：提交 `patch.ext.org_type=null` 时，服务端不得生成 label snapshot；Kernel deep-merge 后标签 key 被移除（`DEV-PLAN-100C`）。
     - [x] 客户端提交 `patch.ext_labels_snapshot`：返回 400 `PATCH_FIELD_NOT_ALLOWED`。
     - [x] 不允许：ext 字段不在 allowed_fields 时拒绝（稳定错误码）。
6. [x] 门禁与证据
   - [x] 本地门禁按 `AGENTS.md`（Go/doc/routing/authz 按触发器命中）。
   - [x] 记录到 `docs/dev-records/dev-plan-100e1-execution-log.md`。

## 6. 测试与验收标准（Acceptance Criteria）

- [x] `mutation-capabilities`：
  - [x] 输出结构/字段与 `DEV-PLAN-083` §5.2/§5.3 一致；排序稳定；
  - [x] enabled ext 字段集合 `E` 并入 `allowed_fields`（不再依赖 target=CREATE 的特殊分支）。
- [x] `corrections`：
  - [x] 支持 `patch.ext`（至少 1 个 DICT 字段 + 1 个 PLAIN 字段的正例闭环）；
  - [x] DICT label 快照由服务端生成；客户端提交 `ext_labels_snapshot` 必须被拒绝（fail-closed，稳定错误码）；
  - [x] capabilities 与写入校验一致：allowed_fields 外的字段必拒绝（fail-closed）。
- [x] 质量门禁通过（以 `AGENTS.md`/`DEV-PLAN-012` 为 SSOT；命中项必须有证据记录）。

## 7. 风险与缓解

- 风险：将 ext 字段并入所有 target 的 allowed_fields 可能扩大“可更正范围”。  
  缓解：该策略是 `DEV-PLAN-083` 已冻结选择；若需收紧，必须先更新 `DEV-PLAN-083` 并同步 Kernel/Service/UI 与测试（禁止代码先改）。
- 风险：对 corrections 启用严格 JSON 解码可能造成兼容性问题。  
  缓解：Internal API 的唯一调用方为本仓库 MUI；以契约测试 + E2E 证明后再合入。
- 风险：字段元数据提取导致 import 方向/循环依赖。  
  缓解：将元数据放在非 `internal/` 且无 server 依赖的包中；server/services 只做薄适配。

## 8. 关联文档

- `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- `docs/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/100c-org-metadata-wide-table-phase2-kernel-projection-extension-one-door.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
