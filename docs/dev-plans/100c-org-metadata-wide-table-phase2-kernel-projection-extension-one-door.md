# DEV-PLAN-100C：Org 模块宽表元数据落地 Phase 2：Kernel/Projection 扩展（保持 One Door）

**状态**: 草拟中（2026-02-13 07:22 UTC）

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
  - [ ] 扩展 `orgunit.submit_org_event(...)` 与更正链路（`submit_org_event_correction/...`）的 payload 校验：
    - 仅允许元数据声明字段写入；
    - 字段值类型与槽位类型一致；
    - 字段在 `effective_date` 当天必须处于 enabled 状态（按 `enabled_on/disabled_on` 解释，day 粒度，SSOT：`DEV-PLAN-032`）。
    - 扩展字段 payload 仅允许出现在本计划支持的 mutation 动作/事件类型中（SSOT：`DEV-PLAN-083`）；其余事件若出现扩展字段 payload 必须拒绝（fail-closed，错误码稳定）。
  - [ ] 扩展投射与回放：
    - 扩展 `orgunit.apply_*_logic` / `rebuild_*` / `org_events_effective_for_replay`，确保扩展值可投射到 `orgunit.org_unit_versions` 的对应 `ext_*` 槽位列。
    - correction/rescind 重放路径保持扩展字段一致性（不得出现“主字段回放、扩展字段丢失”）。
  - [ ] DICT 快照投射规则固定：
    - 提交事件时写 `payload.ext_labels_snapshot`；
    - 投射时同步写 `orgunit.org_unit_versions.ext_labels_snapshot`（对齐 `DEV-PLAN-100` D3/D4）。
  - [ ] 审计快照包含扩展字段变更：
    - 依赖 `extract_orgunit_snapshot(...)` 的行快照机制，确保扩展列在版本切分/回放中不丢失（对齐 080 系列快照约束）。

- **非目标（本阶段不做）**：
  - 不新增 UI / 不新增 HTTP API（Phase 3/4）。  
  - 不新增数据库表（本阶段预期只改函数/视图/触发器与既有表列）。  
  - 不改变事件类型词表（仍只在既有 OrgUnit 事件体系内扩展 payload）。  

## 2.1 工具链与门禁（SSOT 引用）

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

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图 (Mermaid)

```mermaid
graph TD
  UI[UI/Service (Phase 3/4)] -->|payload.ext| KERNEL[submit_org_event / submit_*_correction]
  KERNEL --> META[(tenant_field_configs)]
  KERNEL --> EVTS[(org_events.payload + ext_labels_snapshot)]
  KERNEL --> PROJ[apply_* / replay]
  PROJ --> VERS[(org_unit_versions.ext_* + ext_labels_snapshot)]
  VERS --> SNAP[extract_orgunit_snapshot -> before/after_snapshot]
```

### 3.2 关键设计决策 (ADR 摘要)

- **ADR-100C-01：扩展字段 payload 命名空间（不污染顶层键）**
  - 选定：扩展字段值写入 `payload.ext`（object；key 为 `field_key`）；DICT label 快照写入 `payload.ext_labels_snapshot`（object；key 同为 `field_key`）。  
  - 目的：避免与既有顶层键（`name/new_name/parent_id/new_parent_id/is_business_unit/...`）冲突（SSOT：`DEV-PLAN-100A`）。

- **ADR-100C-02：更正链路对 `payload.ext` 做“深合并（deep-merge）”**
  - 问题：当前更正合并规则对 payload 是浅合并（`payload || correction_payload`），若 correction 只携带部分 `ext` 键，会覆盖并丢失未提交的 ext 键。  
  - 选定：对 `ext` 与 `ext_labels_snapshot` 执行深合并：`ext = old_ext || patch_ext`，其余顶层键继续浅合并。  
  - 落点：`orgunit.org_events_effective` / `orgunit.org_events_effective_for_replay` 的更正合并逻辑必须同步更新。

- **ADR-100C-03：投射落点：把 `payload.ext` 映射到 `org_unit_versions.ext_*`**
  - 选定：由 Kernel 在写入/回放时读取 `tenant_field_configs`，完成：
    - `field_key -> physical_col` 映射；
    - `value_type` 校验与类型转换；
    - 将值写入 `org_unit_versions.<physical_col>`。  
  - 约束：禁止把列名直接来自用户输入；`physical_col` 必须来自元数据表且满足严格格式校验（fail-closed）。

## 4. 数据模型与约束 (Data Model & Constraints)

### 4.1 前置假设（Phase 1 已落地）

- `orgunit.tenant_field_configs` 已存在并具备：
  - `(tenant_uuid, field_key)` 唯一；
  - `(tenant_uuid, physical_col)` 唯一；
  - `enabled_on/disabled_on`（day 粒度）；
  - RLS + FORCE RLS + `tenant_isolation` policy（SSOT：`DEV-PLAN-021`）。
- `orgunit.org_unit_versions` 已增加：
  - 一批 `ext_*` 槽位列（按类型分组）；
  - `ext_labels_snapshot jsonb`。

### 4.2 enabled 判定（day 粒度，半开区间）

字段在 `effective_date` 当天是否可写（enabled）的判定口径冻结为：

- `enabled_on <= effective_date`
- 且 `(disabled_on IS NULL OR effective_date < disabled_on)`

> 说明：采用 `[enabled_on, disabled_on)` 的半开区间模型，避免“同一天既 enabled 又 disabled”的歧义；与仓库 day 粒度 Valid Time 约定保持一致（SSOT：`DEV-PLAN-032`）。

### 4.3 `physical_col` 安全约束（必须 fail-closed）

在 Kernel 内对 `physical_col` 必须同时满足：

- 仅允许扩展槽位命名（示例）：`ext_str_01` / `ext_int_01` / `ext_uuid_01` / `ext_bool_01` / `ext_date_01`；
- 与 `value_type` 一致（例如 `ext_uuid_*` 只能绑定 `value_type='uuid'`）；
- 目标列必须真实存在于 `orgunit.org_unit_versions`（否则拒绝），避免 schema 漂移导致静默写失败。

## 5. 接口契约 (API Contracts)

> 本章描述的是 **Kernel 接口契约**（DB 写入入口的输入/输出/错误码），而非 HTTP API（Phase 3 才实现 HTTP）。

### 5.1 `submit_org_event(...)` payload 扩展

- **新增约定**：
  - `payload.ext`：object，可选。  
  - `payload.ext_labels_snapshot`：object，可选；但当写入 DICT 字段时必须提供相应 key 的 label（否则拒绝）。  

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

### 5.2 更正链路（CORRECT_EVENT）patch 规则

- `correction_payload.ext` 允许只包含“本次更正的 ext 键”，Kernel 必须 deep-merge 到原 payload 的 `ext` 上（避免覆盖丢失）。  
- `correction_payload.ext_labels_snapshot` 同理深合并（或在 DICT 键变更时要求同时提交对应 label）。  

### 5.3 错误码（DB exception MESSAGE）建议集合

> 具体字符串需在实现 PR 内冻结；要求：稳定、可映射、fail-closed。

- `ORG_EXT_FIELD_NOT_ENABLED_AS_OF`：字段在该 `effective_date` 不处于 enabled。  
- `ORG_EXT_FIELD_NOT_CONFIGURED`：payload 出现未配置 `field_key`。  
- `ORG_EXT_FIELD_TYPE_MISMATCH`：值类型与 `value_type/physical_col` 不一致或无法转换。  
- `ORG_EXT_PAYLOAD_NOT_ALLOWED_FOR_EVENT`：在不允许的 event_type/correction 目标上出现扩展字段 payload。  
- `ORG_EXT_LABEL_SNAPSHOT_REQUIRED`：DICT 字段缺少 label 快照。  

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 写入时校验与投射（submit_org_event）

1. **解析扩展字段**：读取 `payload.ext` 与 `payload.ext_labels_snapshot`（若不存在视为 `{}`）。  
2. **事件类型门禁**：
   - 允许写入扩展字段的 event_type 集合以 `DEV-PLAN-083` 的策略矩阵为 SSOT（MVP 默认仅 `CREATE` 与 `CORRECT_EVENT(target=CREATE)`）。  
   - 其他 event_type 若出现 `ext/ext_labels_snapshot`，直接拒绝。  
3. **逐字段校验**（对 `payload.ext` 的每个 `field_key`）：
   - 查 `tenant_field_configs`（同租户）并按 §4.2 判定 enabled；不存在或未生效 -> 拒绝。  
   - 校验 `value_type` 与 `physical_col` 分组匹配；尝试类型转换（失败 -> 拒绝）。  
   - 若 `data_source_type='DICT'`：要求 `ext_labels_snapshot[field_key]` 存在且为非空字符串（否则拒绝）。  
4. **投射写入**：
   - 将 `field_key -> physical_col` 映射为对 `org_unit_versions.<physical_col>` 的写入；
   - 同步写 `org_unit_versions.ext_labels_snapshot`（DICT 快照；只允许 object；键集合限定在启用字段之内）。  

### 6.2 回放（rebuild_* / org_events_effective_for_replay）

回放链路必须与在线写入链路保持一致：

- `orgunit.org_events_effective_for_replay` 产出的 `payload` 必须包含“更正后的有效 ext 值”（deep-merge）。  
- 回放 loop 中，在每个事件 `apply_*_logic` 后：
  - 若事件 payload 含 `ext/ext_labels_snapshot`：执行与在线写入同口径的校验与投射；  
  - 若不含：依赖版本切分逻辑复制 ext 列，保证值不丢失。  

### 6.3 版本切分复制（split_org_unit_version_at）

`split_org_unit_version_at` 当前插入新版本行使用显式列清单；Phase 1 引入 ext 列后，必须把：

- 所有新增 `ext_*` 列  
- `ext_labels_snapshot`

纳入 insert 的复制列清单，否则在 `RENAME/MOVE/DISABLE/...` 触发 split 时会把扩展字段置空，导致审计与回放不一致。

## 7. 安全与鉴权 (Security & Authz)

- **RLS**：Kernel 读取 `tenant_field_configs` 必须遵循 tenant 注入口径（`assert_current_tenant` + RLS policy），fail-closed（SSOT：`DEV-PLAN-021`）。  
- **动态 SQL 安全**：若实现中需要对 `physical_col` 做动态列写入，必须：
  - 只允许来自 `tenant_field_configs` 的 `physical_col`；
  - 通过严格格式校验 + `quote_ident/%I` 绑定；
  - 发现异常（非法列名/列不存在）直接拒绝。  

## 8. 依赖与里程碑 (Dependencies & Milestones)

- **依赖**：
  - `DEV-PLAN-100A`（Phase 0：契约冻结，ext 命名空间与能力边界）
  - `DEV-PLAN-100B`（Phase 1：schema 骨架）
  - `DEV-PLAN-083`（策略矩阵与 capabilities SSOT）
  - `DEV-PLAN-080*`（审计快照约束）

- **实施步骤（Phase 2）**：
  1. [ ] 扩展 `orgunit.submit_org_event(...)` 与更正链路：增加扩展字段校验（enabled/type/event gating）与稳定错误码。  
  2. [ ] 扩展 `orgunit.org_events_effective` 与 `orgunit.org_events_effective_for_replay`：对 `ext/ext_labels_snapshot` 做 deep-merge。  
  3. [ ] 扩展 replay：在 `rebuild_org_unit_versions_for_org_with_pending_event(...)` 中对 ext 做同口径投射。  
  4. [ ] 修复版本切分复制：更新 `split_org_unit_version_at(...)` 的 insert 列清单，确保 ext 列与 `ext_labels_snapshot` 被复制。  
  5. [ ] 验证审计快照：确保 `extract_orgunit_snapshot(...)` 在扩展列存在时可稳定产出并被 080 系列约束接受。  

## 9. 测试与验收标准 (Acceptance Criteria)

### 9.1 必测用例（新增/回归）

- [ ] 正例：CREATE 写入 1~2 个扩展字段（DICT/非 DICT）后，详情快照/versions 行包含对应 `ext_*` 值与 `ext_labels_snapshot`。  
- [ ] 负例：payload.ext 出现未配置 field_key -> 拒绝（稳定错误码）。  
- [ ] 负例：field_key 配置存在但在 effective_date 未 enabled -> 拒绝。  
- [ ] 负例：value_type 与列类型不一致或无法转换 -> 拒绝。  
- [ ] 负例：不允许的事件类型携带 ext -> 拒绝。  
- [ ] 更正：CORRECT_EVENT 对 ext 做局部 patch，不应覆盖丢失未更正的 ext 键（deep-merge 断言）。  
- [ ] 回放：replay/correction/rescind 后，扩展字段在 versions 中可复现且与预期一致。  
- [ ] 版本切分：RENAME/MOVE/DISABLE 等导致 split 时，ext 列值必须被复制（不变）。  

### 9.2 出口条件（与路线图一致）

- [ ] `internal/server/orgunit_audit_snapshot_schema_test.go` 相关测试保持通过。  
- [ ] 新增扩展字段重放回归测试通过（覆盖 correction/rescind + replay）。  

## 10. 运维与监控 (Ops & Monitoring)

本阶段不引入运维/监控开关；遵循 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。

