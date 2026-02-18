# DEV-PLAN-107：OrgUnit 扩展字段槽位扩容（总计 135 槽；按类型合理分布；新增 numeric）

**状态**: 规划中（2026-02-17 22:20 UTC）

## 1. 背景

当前 OrgUnit 扩展字段的物理槽位（`orgunit.org_unit_versions.ext_*`）数量过少，已在本地验证 `DEV-PLAN-106A` 时触发 `ORG_FIELD_CONFIG_SLOT_EXHAUSTED`，导致无法继续启用新的 `d_<dict_code>` 字典字段并验证“启用时自定义描述（label/display_label）”闭环。

本计划将 OrgUnit 扩展字段物理槽位扩容到 **总计 135 个**，并对不同 value_type（text/int/uuid/bool/date/numeric）做合理配比，作为后续“字典字段方式（106A）”与扩展字段管理（100/101/100G 等）的容量基座。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. 扩容 OrgUnit 扩展字段物理槽位到 **135 个**（见 §3 配比）。
2. `orgunit.enable_tenant_field_config(...)` 的槽位分配候选集合与数据库真实列保持一致，避免生成“指向不存在列”的 field-config。
3. `orgunit.org_unit_versions` 的版本切分/移动等逻辑在扩容后**完整拷贝所有 ext_* 列**，避免隐性丢字段值。
4. 保持 One Door：所有 field-config 写入仍通过 DB Kernel（`enable_tenant_field_config(...)` / `rekey_tenant_field_config(...)`）。

### 2.2 非目标（Stopline）

1. 不新增数据库表（不引入 `CREATE TABLE`）。若实施阶段发现必须新增表，需先更新本计划并按仓库红线获得用户手工确认。
2. 不改变 `d_<dict_code>` / `x_...` / built-in 字段的语义与鉴权边界（仅扩容底层“物理槽位容量”）。新增 `numeric` 仅扩展 value_type/物理槽位集合，不改变既有类型的校验与写入语义。
3. 不在本计划引入“按字段动态建索引/删索引”的运维机制；索引策略保持简单（见 §4.3），性能优化另起计划。

## 3. 槽位配比（冻结）

目标：总计 135 个槽位，满足当前“DICT/PLAIN 主要为 text”现状，同时为后续 ENTITY/数值/日期类字段预留空间。

冻结配比（总计 135）：

- text：70（`ext_str_01..ext_str_70`）
- int：15（`ext_int_01..ext_int_15`）
- uuid：10（`ext_uuid_01..ext_uuid_10`）
- bool：15（`ext_bool_01..ext_bool_15`）
- date：15（`ext_date_01..ext_date_15`）
- numeric：10（`ext_num_01..ext_num_10`）

命名规则与校验：

- 继续使用 `ext_<kind>_[0-9]{2}`（与既有 DB check 对齐）。
- 单一 kind 最大 99（两位编号上限）；本计划配比均不超过 99。

## 4. 方案设计（高层）

### 4.1 数据库列扩容（orgunit.org_unit_versions）

在 `orgunit.org_unit_versions` 上新增（`ADD COLUMN IF NOT EXISTS`）：

- `ext_str_06..ext_str_70`
- `ext_int_02..ext_int_15`
- `ext_uuid_02..ext_uuid_10`
- `ext_bool_02..ext_bool_15`
- `ext_date_02..ext_date_15`
- `ext_num_01..ext_num_10`（PostgreSQL `numeric`）

> 说明：已有 `ext_str_01..05` / `ext_int_01` / `ext_uuid_01` / `ext_bool_01` / `ext_date_01` 保持不变（对既有 tenant_field_configs 映射完全兼容）。

### 4.2 Kernel 槽位分配候选集合扩容（enable_tenant_field_config）

扩容 `orgunit.enable_tenant_field_config(...)` 内部 `v_candidate_cols`：

- text：候选数组扩展为 `ext_str_01..ext_str_70`
- int：候选数组扩展为 `ext_int_01..ext_int_15`
- uuid：候选数组扩展为 `ext_uuid_01..ext_uuid_10`
- bool：候选数组扩展为 `ext_bool_01..ext_bool_15`
- date：候选数组扩展为 `ext_date_01..ext_date_15`
- numeric：候选数组扩展为 `ext_num_01..ext_num_10`

并保持既有约束：

- `(tenant_uuid, physical_col)` 唯一；
- 若候选用尽，则稳定抛出 `ORG_FIELD_CONFIG_SLOT_EXHAUSTED`。
- 校验点一致性：数据库 schema 的 check/regex/allowlist（例如 `tenant_field_configs.value_type` 枚举、`physical_col` 格式与分组校验、engine 对 physical_col 的正则检查、Go 侧 `orgUnitExtPhysicalColRe`）必须同步扩展以覆盖 `numeric/ext_num_*`，避免出现“内核分配了新类型槽位，但投射/读写层仍拒绝”的不一致。

> 复杂度取舍（冻结）：`enable_tenant_field_config(...)` 仍采用当前“按候选顺序逐个探测”的实现方式。该路径仅发生在管理员启用字段配置时（低频），本计划优先保证正确性与可读性，不引入 set-based/动态 SQL 抽象。若后续出现批量启用性能证据，再另起 dev-plan 优化。

### 4.3 索引策略（冻结）

原则：正确性优先，性能其次；避免一次性创建过多索引导致写放大与迁移时间膨胀。

- 保留现有 `ext_str_01..05` 索引不变。
- 新增索引仅覆盖 **ext_str_06..ext_str_20**（先把“可查询的字典字段”常见规模覆盖到 20 个 text 槽位）。
- 其余新槽位默认不建索引（可正确过滤/排序，但可能慢；性能优化另起 dev-plan）。

> 注：此策略与 `DEV-PLAN-100G` “字典字段 allow_filter/allow_sort=true”不冲突：API 语义不承诺索引存在，只承诺行为正确。

### 4.4 版本切分/移动时的 ext_* 拷贝完整性

扩容后，必须保证涉及 `INSERT INTO orgunit.org_unit_versions (...) VALUES/SELECT ...` 的逻辑完整覆盖全部 ext_* 列。

本计划要求（冻结，采用 **A 方案：显式列出全部 ext_* 列**）：

1. 必须覆盖至少两条“复制旧版本 -> 新版本”路径：
   - 版本切分路径：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` 中 `INSERT INTO orgunit.org_unit_versions`（split）。
   - 组织移动路径：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` 中 `INSERT INTO orgunit.org_unit_versions`（move）。
2. 上述两条路径中的列清单必须显式包含完整 ext 槽位：
   - `ext_str_01..ext_str_70`
   - `ext_int_01..ext_int_15`
   - `ext_uuid_01..ext_uuid_10`
   - `ext_bool_01..ext_bool_15`
   - `ext_date_01..ext_date_15`
   - `ext_num_01..ext_num_10`
   - `ext_labels_snapshot`
3. 本计划不引入动态列生成函数/片段（B 方案不在本次范围），以降低抽象复杂度并提升可审计性（对齐 `DEV-PLAN-003`）。

## 5. 实施步骤（草案）

1. [ ] 契约对齐：检查并更新可能显式假设“槽位数量”的历史文档（如存在），避免口径漂移（引用 SSOT，不复制细节）。
2. [ ] 新增 orgunit 模块迁移（Goose）：
   - `ALTER TABLE orgunit.org_unit_versions ADD COLUMN IF NOT EXISTS ...`（按 §4.1）
   - `CREATE INDEX IF NOT EXISTS ...`（按 §4.3）
3. [ ] 更新 orgunit kernel/schema：
   - 扩容 `enable_tenant_field_config` 的候选列集合（按 §4.2）
   - 扩展 schema 校验点以支持 `numeric/ext_num_*`（value_type 枚举、physical_col format/group check、engine allowlist 正则等）
4. [ ] 同步 schema 汇总与生成物：
   - 更新 `internal/sqlc/schema.sql`（确保与模块 schema 一致）
   - 执行 `make sqlc-generate`，并确认 `git status --short` 为空（若有生成物变更则提交）
5. [ ] 更新 orgunit engine（投射写路径）：
   - 在 split/move 两处 `INSERT INTO orgunit.org_unit_versions` 显式列出并拷贝全部 ext_* 列（按 §4.4 A 方案）
6. [ ] 补充最小防回归测试：
   - 至少覆盖“尾部槽位” token：`ext_str_70`、`ext_int_15`、`ext_uuid_10`、`ext_bool_15`、`ext_date_15`、`ext_num_10`
   - 覆盖 split 与 move 两条复制路径，防止后续改动遗漏尾部列
7. [ ] 本地验证与门禁对齐（入口与触发器见 `AGENTS.md` 与 `DEV-PLAN-012`）：
   - 模块级闭环：`make orgunit plan && make orgunit lint && make orgunit migrate up`
   - 生成物闭环：`make sqlc-generate` 后 `git status --short` 为空
   - 跑通 106A 关键用例：能启用至少 20 个 `d_<dict_code>` 并不再触发 `ORG_FIELD_CONFIG_SLOT_EXHAUSTED`

## 6. 验收标准（DoD）

1. 在单租户下至少可连续启用 **20 个 `d_<dict_code>` text 字段**，不再出现 `ORG_FIELD_CONFIG_SLOT_EXHAUSTED`（覆盖 106A 的当前核心验证场景）。
2. `orgunit.enable_tenant_field_config(...)` 分配的 `physical_col` 必须是数据库中真实存在的列。
3. 扩容后执行典型写路径（版本切分与移动）不会丢失 ext_* 列数据；并有测试覆盖尾部槽位（`ext_str_70`、`ext_int_15`、`ext_uuid_10`、`ext_bool_15`、`ext_date_15`、`ext_num_10`）。
4. `make orgunit plan && make orgunit lint && make orgunit migrate up` 在本地可重复执行且幂等（无 drift/无异常）。
5. `make sqlc-generate` 后 `git status --short` 为空（或仅包含本次应提交的生成物变更）。
