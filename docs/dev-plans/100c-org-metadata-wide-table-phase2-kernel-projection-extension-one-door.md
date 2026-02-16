# DEV-PLAN-100C：Org 模块宽表元数据落地 Phase 2：Kernel/Projection 扩展（保持 One Door）

**状态**: 已完成（2026-02-13；2026-02-16 文档回填）

**执行记录**：
- `docs/dev-records/dev-plan-100c-execution-log.md`

> 本文从 `DEV-PLAN-100` 的 Phase 2 拆分而来，作为 Phase 2 的 SSOT；`DEV-PLAN-100` 保持为整体路线图。

## 1. 背景与上下文 (Context)

在 `DEV-PLAN-100B`（Phase 1）完成最小数据库骨架后（`orgunit.tenant_field_configs` + `orgunit.org_unit_versions` 扩展槽位列 + `ext_labels_snapshot`），下一步必须把“扩展字段”纳入 OrgUnit 的 **One Door** 写入链路与投射/回放链路，避免出现：

- payload 接受了扩展字段但没有投射到读模型（“写了但读不到”）；
- replay/correction/rescind 路径丢字段（“主字段回放、扩展字段丢失”）；
- 动态列写入绕过 allowlist（潜在注入/越权写）；
- 审计快照缺失扩展字段变化（审计链不完整）。

Phase 2 的交付物是：**Kernel 层对扩展字段的校验 + 投射 + 回放一致性**，并确保所有写入仍遵循 `submit_*_event(...)` 单写入口原则（One Door）。

## 2. 目标与非目标 (Goals & Non-Goals)

- **核心目标**：
  - [x] 扩展 `orgunit.submit_org_event(...)` 与更正链路（`submit_org_event_correction(...)`）的 payload 校验：
    - 仅允许元数据声明字段写入；
    - 字段值类型与槽位类型一致；
    - 字段在 `effective_date` 当天必须处于 enabled 状态（按 `enabled_on/disabled_on` 解释，day 粒度，SSOT：`DEV-PLAN-032`）；
    - 扩展字段 payload 仅允许出现在本计划支持的 mutation 组合中（SSOT：`DEV-PLAN-083`）；其余事件若出现扩展字段 payload 必须拒绝（fail-closed，错误码稳定）。
  - [x] 扩展投射与回放：
    - 扩展 `orgunit.org_events_effective` / `orgunit.org_events_effective_for_replay` / `rebuild_*`，确保扩展值可投射到 `orgunit.org_unit_versions` 的对应 `ext_*` 槽位列；
    - correction/rescind 重放路径保持扩展字段一致性（不得出现“主字段回放、扩展字段丢失”）。
  - [x] DICT 快照投射规则固定：
    - 提交事件时写 `payload.ext_labels_snapshot`；
    - 投射时同步写 `orgunit.org_unit_versions.ext_labels_snapshot`（对齐 `DEV-PLAN-100` D3/D4）。
  - [x] 审计快照包含扩展字段变更：
    - 依赖 `extract_orgunit_snapshot(...)` 的行快照机制，确保扩展列在版本切分/回放中不丢失（对齐 080 系列快照约束）。

- **非目标（本阶段不做）**：
  - 不新增 UI / 不新增 HTTP API（Phase 3/4）；
  - 不新增数据库表（本阶段只改函数/视图/触发器与既有表列）；
  - 不改变事件类型词表（仍只在既有 OrgUnit 事件体系内扩展 payload）。

### 2.1 工具链与门禁（SSOT 引用）

> 目的：只声明命中哪些触发器与门禁入口，不在本文复制脚本细节（SSOT：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`）。

- **触发器清单（勾选本计划命中的项）**：
  - [X] 文档（`make check doc`）
  - [X] DB Schema/迁移（`make orgunit plan && make orgunit lint && make orgunit migrate up`；SSOT：`DEV-PLAN-024`）
  - [X] Go 代码（测试与 API/服务映射可能变更：`go fmt ./... && go vet ./... && make check lint && make test`）
  - [X] sqlc（若 schema/函数签名或 sqlc 输入受影响：`make sqlc-generate`，并确保 `git status --short` 为空；SSOT：`DEV-PLAN-025`）

- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - CI 门禁定义：`docs/dev-plans/012-ci-quality-gates.md`
  - Atlas + Goose 模块闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc 规范与门禁：`docs/dev-plans/025-sqlc-guidelines.md`

### 2.2 本次评审拍板（待决项收敛）

> 按 `DEV-PLAN-003`（Simple > Easy）要求，本节把“实现期容易漂移的待决项”先冻结为单口径。

1. **扩展字段写入能力范围（Phase 2）**：
   - 允许：
     - `submit_org_event` 的基础事件（`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`）可携带 `payload.ext`（并按 `effective_date` 投射到 versions）。
     - `submit_org_event_correction`（`CORRECT_EVENT`）可携带 `patch.ext`，并对目标 effective 事件（`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`）生效。
   - 拒绝：`CORRECT_STATUS/RESCIND_*` 上出现 `ext/ext_labels_snapshot`（无论来自 patch 还是 payload），一律 fail-closed。
   - 理由：避免把 `DEV-PLAN-083` 的策略矩阵复制进 Kernel（容易 drift），Kernel 只负责“数据层不变量 + 安全边界”，动作/字段白名单由服务层策略单点控制。
2. **payload 形状严格校验**：
   - `payload.ext`、`payload.ext_labels_snapshot` 若出现，必须是 JSON object；否则 fail-closed。
3. **扩展字段“清空”语义冻结**：
   - `payload.ext.<field_key> = null` 代表清空该字段（投射为对应 `ext_*` 列 `NULL`）；
   - 若该字段为 DICT，`ext_labels_snapshot.<field_key>` 也必须显式为 `null`（或该 key 被删除），否则拒绝。
4. **在线写入与回放共用同一归一化逻辑**：
   - 任何 ext 校验/类型转换/映射逻辑必须复用同一组 Kernel helper，禁止 submit 路径与 replay 路径各写一份判断。

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图 (Mermaid)

```mermaid
graph TD
  UI[UI/Service (Phase 3/4)] -->|payload.ext| KERNEL[submit_org_event / submit_org_event_correction]
  KERNEL --> META[(tenant_field_configs)]
  KERNEL --> EVTS[(org_events.payload + ext_labels_snapshot)]
  KERNEL --> PROJ[effective view + replay + apply/split]
  PROJ --> VERS[(org_unit_versions.ext_* + ext_labels_snapshot)]
  VERS --> SNAP[extract_orgunit_snapshot -> before/after_snapshot]
```

### 3.2 关键设计决策 (ADR 摘要)

- **ADR-100C-01：扩展字段 payload 命名空间（不污染顶层键）**
  - 选定：扩展字段值写入 `payload.ext`（object；key 为 `field_key`）；DICT label 快照写入 `payload.ext_labels_snapshot`（object；key 同为 `field_key`）。
  - 目的：避免与既有顶层键（`name/new_name/parent_id/new_parent_id/is_business_unit/...`）冲突（SSOT：`DEV-PLAN-100A`）。

- **ADR-100C-02：更正链路对 `ext/ext_labels_snapshot` 做 deep-merge**
  - 问题：当前更正合并规则是浅合并（`payload || correction_payload`），若 correction 只携带部分 `ext` 键，会覆盖并丢失未提交的 ext 键。
  - 选定：对 `ext` 与 `ext_labels_snapshot` 执行深合并：`new.ext = old.ext || patch.ext`，`new.ext_labels_snapshot = old.ext_labels_snapshot || patch.ext_labels_snapshot`，其余顶层键继续浅合并。
  - 落点：`orgunit.org_events_effective` / `orgunit.org_events_effective_for_replay` 同步更新。

- **ADR-100C-03：投射落点固定为 `org_unit_versions.ext_*`**
  - 选定：由 Kernel 在写入/回放时读取 `tenant_field_configs`，完成 `field_key -> physical_col` 映射、`value_type` 校验/转换，并写入 `org_unit_versions.<physical_col>`。
  - 约束：禁止把列名直接来自用户输入；`physical_col` 必须来自元数据表且满足严格格式校验（fail-closed）。

- **ADR-100C-04：单实现原则（Simple > Easy）**
  - 选定：新增共享 helper（命名可在实现 PR 中细化），统一承担：
    - payload 形状校验；
    - as_of enabled 判定；
    - value 类型转换；
    - DICT label 快照一致性；
    - `field_key -> physical_col` 归一化映射。
  - 约束：submit 与 replay 必须复用该 helper，禁止复制粘贴两份规则。

## 4. 数据模型与约束 (Data Model & Constraints)

### 4.1 前置假设（Phase 1 已落地）

- `orgunit.tenant_field_configs` 已存在并具备：
  - `(tenant_uuid, field_key)` 唯一；
  - `(tenant_uuid, physical_col)` 唯一；
  - `enabled_on/disabled_on`（day 粒度）；
  - RLS + FORCE RLS + `tenant_isolation` policy（SSOT：`DEV-PLAN-021`）。
- `orgunit.org_unit_versions` 已增加：
  - `ext_str_01..05`、`ext_int_01`、`ext_uuid_01`、`ext_bool_01`、`ext_date_01`；
  - `ext_labels_snapshot jsonb`。

### 4.2 enabled 判定（day 粒度，半开区间）

字段在 `effective_date` 当天是否可写（enabled）的判定口径冻结为：

- `enabled_on <= effective_date`
- 且 `(disabled_on IS NULL OR effective_date < disabled_on)`

> 说明：采用 `[enabled_on, disabled_on)` 的半开区间模型，避免“同一天既 enabled 又 disabled”的歧义；与仓库 day 粒度 Valid Time 约定一致（SSOT：`DEV-PLAN-032`）。

### 4.3 payload 形状与 key 约束（必须 fail-closed）

- `payload.ext` / `payload.ext_labels_snapshot` 若出现，必须是 object。
- `payload.ext` 的 key 必须满足 `field_key` 形状约束（`^[a-z][a-z0-9_]{0,62}$`，对齐 `DEV-PLAN-100B`）。
- `payload.ext_labels_snapshot` 的 key 只能来自 `payload.ext` 中配置为 DICT 的字段集合；非 DICT 字段出现 label 快照必须拒绝。

### 4.4 `physical_col` 安全约束（必须 fail-closed）

Kernel 内对 `physical_col` 必须同时满足：

- 仅允许扩展槽位命名：`ext_(str|int|uuid|bool|date)_NN`；
- 与 `value_type` 一致（如 `ext_uuid_*` 只能绑定 `value_type=uuid`）；
- 目标列真实存在于 `orgunit.org_unit_versions`（否则拒绝，避免 schema 漂移导致静默写失败）。

## 5. 接口契约 (Kernel Contracts)

> 本章描述的是 **Kernel 写入口契约**（DB function 输入/校验/错误码），不是 HTTP API（Phase 3 才实现 HTTP）。

### 5.1 `submit_org_event(...)` / `submit_org_event_correction(...)` 扩展约定

- `payload.ext`：object，可选。
- `payload.ext_labels_snapshot`：object，可选；但写入 DICT 字段且 value 非 `null` 时必须提供相应 label。
- 禁止信任 UI 传入 label：`ext_labels_snapshot` 必须由服务层按 `effective_date` 解析并写入（SSOT：`DEV-PLAN-100D`）。

**示例（CREATE）**：

```json
{
  "org_code": "R&D",
  "name": "R&D",
  "parent_id": "10000001",
  "ext": {
    "short_name": "R&D",
    "org_type": "DEPARTMENT"
  },
  "ext_labels_snapshot": {
    "org_type": "Department"
  }
}
```

### 5.2 允许/拒绝矩阵（Phase 2 冻结）

| 写入口 | event_type | 目标 effective 事件类型 | `ext` 是否允许 |
| --- | --- | --- | --- |
| `submit_org_event` | `CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT` | - | 允许 |
| `submit_org_event_correction` | `CORRECT_EVENT` | `CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT` | 允许 |
| `submit_org_status_correction` | `CORRECT_STATUS` | `ENABLE/DISABLE` | 拒绝 |
| `submit_org_event_rescind` / `submit_org_rescind` | `RESCIND_*` | 任意 | 拒绝 |

> 后续若要扩大允许范围，必须先更新 `DEV-PLAN-083` 与本表，再进入实现。

### 5.3 错误码（DB exception MESSAGE）冻结集合

- `ORG_EXT_PAYLOAD_INVALID_SHAPE`：`ext` 或 `ext_labels_snapshot` 不是 object。
- `ORG_EXT_PAYLOAD_NOT_ALLOWED_FOR_EVENT`：在不允许的 action/event/target 组合上出现扩展 payload。
- `ORG_EXT_FIELD_NOT_CONFIGURED`：payload 出现未配置 `field_key`。
- `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`：字段在该 `effective_date` 不处于 enabled。
- `ORG_EXT_FIELD_TYPE_MISMATCH`：值类型与 `value_type/physical_col` 不一致或无法转换。
- `ORG_EXT_LABEL_SNAPSHOT_REQUIRED`：DICT 字段（value 非 `null`）缺少 label 快照。
- `ORG_EXT_LABEL_SNAPSHOT_NOT_ALLOWED`：为非 DICT 字段提供了 label 快照。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 单次写入（submit）

1. 解析 `payload.ext` 与 `payload.ext_labels_snapshot`（缺失按 `{}`；非 object 直接拒绝）。
2. 根据 §5.2 矩阵判定当前写入口是否允许 ext payload。
3. 逐字段校验（同租户）：
   - `tenant_field_configs` 存在且在 `effective_date` enabled；
   - `value_type` 与 `physical_col` 分组匹配，完成类型转换；
   - DICT 字段：value 非 `null` 必须有非空 string label；value 为 `null` 时 label 必须为 `null` 或不存在；
   - 非 DICT 字段：必须无 label；
   - `value=null` 时按“清空语义”处理。
4. 投射写入：
   - `field_key -> physical_col` 写入 `org_unit_versions.<physical_col>`；
   - DICT label 快照同步写入 `org_unit_versions.ext_labels_snapshot`。

### 6.2 更正合并（effective view + replay view）

- `CORRECT_EVENT` 生成 effective payload 时：
  - 顶层仍保持浅合并；
  - `ext/ext_labels_snapshot` 走 deep-merge；
  - 合并后仍需经过 §6.1 的同口径校验，防止“更正绕过校验”。

### 6.3 回放（rebuild）

- `orgunit.org_events_effective_for_replay(...)` 必须产出“深合并后的有效 payload”。
- replay loop 对含 ext payload 的事件执行与在线写入一致的 helper。
- 若事件不含 ext payload，则依赖版本切分复制 ext 列，保证历史值不丢失。

### 6.4 版本切分复制（split）

`split_org_unit_version_at(...)` 当前使用显式列清单；Phase 2 必须将以下列纳入复制：

- `ext_str_01..05`、`ext_int_01`、`ext_uuid_01`、`ext_bool_01`、`ext_date_01`
- `ext_labels_snapshot`

否则在 `RENAME/MOVE/DISABLE/...` 触发 split 时会把扩展字段置空，导致审计与回放不一致。

## 7. 安全与鉴权 (Security & Authz)

- **RLS**：Kernel 读取 `tenant_field_configs` 必须遵循 tenant 注入口径（`assert_current_tenant` + RLS policy），fail-closed（SSOT：`DEV-PLAN-021`）。
- **动态 SQL 安全**：若实现需按 `physical_col` 动态写列，必须仅使用元数据 allowlist + 严格格式校验 + `quote_ident/%I`；异常即拒绝。
- **单写入口**：OrgUnit 业务写入仍只走 `submit_*` 内核函数；禁止新增直接写 `org_unit_versions` 的旁路入口（SSOT：`DEV-PLAN-026`）。

## 8. 依赖与里程碑 (Dependencies & Milestones)

- **依赖**：
  - `DEV-PLAN-100A`（Phase 0：契约冻结）
  - `DEV-PLAN-100B`（Phase 1：schema 骨架）
  - `DEV-PLAN-083`（策略矩阵与 capabilities SSOT）
  - `DEV-PLAN-080*`（审计快照约束）
  - `DEV-PLAN-003`（Simple > Easy 评审约束）

- **实施步骤（Phase 2）**：
  1. [x] 新增共享 helper：统一 ext payload 形状校验、enabled 判定、类型转换、DICT label 一致性与映射解析。
  2. [x] 扩展 `submit_org_event(...)` / `submit_org_event_correction(...)`：接入 helper，落地 §5.2 矩阵与稳定错误码。
  3. [x] 扩展 `orgunit.org_events_effective` / `orgunit.org_events_effective_for_replay`：对 `ext/ext_labels_snapshot` 做 deep-merge。
  4. [x] 扩展 replay：`rebuild_org_unit_versions_for_org_with_pending_event(...)` 使用与 submit 同口径 helper。
  5. [x] 修复 `split_org_unit_version_at(...)`：复制全部 ext 列与 `ext_labels_snapshot`。
  6. [x] 验证审计快照：`extract_orgunit_snapshot(...)` 在扩展列存在时可稳定产出并满足 080 系列约束。

## 9. 测试与验收标准 (Acceptance Criteria)

### 9.1 必测用例（新增/回归）

- [x] 正例：CREATE 写入 1~2 个扩展字段（DICT/非 DICT）后，versions 与快照包含对应 `ext_*` 值及 `ext_labels_snapshot`。
- [x] 正例：`payload.ext.<field_key>=null` 时，目标 `ext_*` 列被清空，DICT label 快照同步清空。
- [x] 负例：`ext` 或 `ext_labels_snapshot` 非 object -> `ORG_EXT_PAYLOAD_INVALID_SHAPE`。
- [x] 负例：payload.ext 出现未配置 field_key -> `ORG_EXT_FIELD_NOT_CONFIGURED`。
- [x] 负例：field_key 在 `effective_date` 未 enabled -> `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`。
- [x] 负例：value_type 与列类型不一致或无法转换 -> `ORG_EXT_FIELD_TYPE_MISMATCH`。
- [x] 负例：不允许的 action/event/target 组合携带 ext -> `ORG_EXT_PAYLOAD_NOT_ALLOWED_FOR_EVENT`。
- [x] 更正：CORRECT_EVENT 对 ext 做局部 patch 时，不覆盖未更正键（deep-merge 断言）。
- [x] 回放：replay/correction/rescind 后，扩展字段在 versions 中可复现且与预期一致。
- [x] 版本切分：RENAME/MOVE/DISABLE 等触发 split 时，ext 列值保持不变。

### 9.2 出口条件（与路线图一致）

- [x] `internal/server/orgunit_audit_snapshot_schema_test.go` 保持通过。
- [x] 新增扩展字段重放回归测试通过（覆盖 correction/rescind + replay）。
- [x] 无第二写入口；所有扩展字段写入可追溯到 `submit_*` 内核函数。

## 10. 运维与监控 (Ops & Monitoring)

本阶段不引入运维/监控开关；遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。

## 11. 关联文档

- `docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- `docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`
- `docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
