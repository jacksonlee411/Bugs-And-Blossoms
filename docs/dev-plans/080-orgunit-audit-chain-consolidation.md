# DEV-PLAN-080：OrgUnit 审计链收敛（方向 1：单一审计链）

**状态**: 草拟中（2026-02-09 12:00 UTC）

## 背景
- DEV-PLAN-078 已将 OrgUnit 写模型切换为“状态 SoT + 审计事件”，但当前表结构仍存在 `org_events`、`org_events_audit` 与 `org_event_corrections_*` 多套审计/纠错链路并行。
- 这导致边界不清晰、双写风险与维护负担，违背“Simple > Easy / No Legacy / 单链路”原则（见 `AGENTS.md`）。

## 目标
1. 将 OrgUnit 的“审计事实源”收敛为**单一 append-only 表**。
2. 保持 `org_unit_versions` 作为业务 SoT，不引入第二条写链路。
3. 删除“重复/并行的审计链路与纠错表”，降低心智复杂度与一致性风险。
4. 维持既有业务语义与错误码口径（对齐 075 系列冻结口径）。

## 非目标
- 不改变对外 API 形态与业务语义（除必要的内部实现切换）。
- 不引入 replay / 离线重建工具链。
- 不新增业务表（本计划不新增表，仅收敛/删改）。

## 方案概述（方向 1）
- **保留**：`orgunit.org_unit_versions`（状态 SoT）。
- **保留并扩展**：`orgunit.org_events` 作为**唯一审计链**（append-only）。
- **删除**：`orgunit.org_events_audit`、`orgunit.org_event_corrections_current`、`orgunit.org_event_corrections_history`。

## 表级清单（完全重构视角）
> 说明：本计划以“单一审计链”收敛为目标；除下述“删除/修改”外，其他 OrgUnit 模块表保持不变。

### 保留（不改语义）
**OrgUnit 核心**
- `orgunit.org_unit_versions`
- `orgunit.org_unit_codes`
- `orgunit.org_trees`
- `orgunit.org_id_allocators`

**SetID（租户）**
- `orgunit.setids`
- `orgunit.setid_events`
- `orgunit.setid_binding_events`
- `orgunit.setid_binding_versions`
- `orgunit.setid_scope_packages`
- `orgunit.setid_scope_package_events`
- `orgunit.setid_scope_package_versions`
- `orgunit.setid_scope_subscriptions`
- `orgunit.setid_scope_subscription_events`

**SetID（全局）**
- `orgunit.global_setids`
- `orgunit.global_setid_events`
- `orgunit.global_setid_scope_packages`
- `orgunit.global_setid_scope_package_events`
- `orgunit.global_setid_scope_package_versions`

### 删除
- `orgunit.org_events_audit`
- `orgunit.org_event_corrections_current`
- `orgunit.org_event_corrections_history`

### 修改
- `orgunit.org_events`：作为唯一审计链（append-only）；补齐 `reason/before_snapshot/after_snapshot/tx_time` 等审计字段，并对 `request_code` 施加幂等唯一约束。

## 数据模型与语义
### 1) `orgunit.org_unit_versions`
- 仍为唯一业务读写 SoT，维持 `validity daterange`、`no-overlap`、`gapless` 等不变量。

### 2) `orgunit.org_events`（唯一审计链）
- 承载所有事件类型：CREATE / ENABLE / DISABLE / MOVE / RENAME / SET_BUSINESS_UNIT / CORRECT_* / RESCIND / DELETE_RECORD 等。
- 建议补齐审计字段（与 078 口径对齐）：
  - `request_code`（幂等键，tenant 级唯一）
  - `reason`（修正/撤销说明）
  - `before_snapshot` / `after_snapshot`（jsonb）
  - `tx_time`（timestamptz）


## 写路径与一致性
- 所有写操作在 DB Kernel 中完成区间增量计算，并**仅写入 `org_events` 一次**（append-only）。
- `CORRECT_*` / `RESCIND` 作为事件类型进入同一审计链，不再写入纠错表。
- 失败路径保持 fail-closed：任一审计写失败必须回滚业务写。

## 迁移与重基线策略
- 采用与 078 一致的“早期阶段一次性重基线”策略：
  1. 迁移前冻结变更窗口。
  2. 执行 schema 变更（`org_events` 补字段/约束/索引）。
  3. 删除 `org_events_audit` 与 `org_event_corrections_*`。
  4. 重灌最小可复现 seed（不做在线灰度、无需历史迁移）。

## 影响范围
- 数据库：orgunit schema、DB Kernel 函数、索引与约束。
- 应用层：OrgUnit 写路径与错误码映射（确保对外语义不变）。
- 测试：OrgUnit 写路径与纠错/撤销相关测试回归。

## 风险与缓解
- 审计字段增加导致 `org_events` 体积增长：通过索引与分区策略预留扩展空间（后续可选）。
- 历史审计缺失：因早期阶段采用重基线策略，可接受；若需保留历史，需单独迁移计划。

## 验收标准（概要）
- 审计链唯一性：系统中仅存在 `org_events` 作为审计事实源。
- 无双写：纠错/撤销不再写入 `org_event_corrections_*`。
- 业务不变量：`org_unit_versions` 仍满足 `no-overlap/gapless/upper_inf`。
- 门禁与测试：按 `AGENTS.md` 触发器矩阵执行对应门禁并通过。

## 实施步骤（草案）
1. [ ] 细化 `org_events` 扩展字段与约束（含幂等键策略）。
2. [ ] DB Kernel 写路径调整：纠错/撤销写入 `org_events`。
3. [ ] 移除 `org_events_audit` / `org_event_corrections_*` 相关写入与查询。
4. [ ] Schema 迁移与重基线（按 Atlas+Goose 闭环）。
5. [ ] 回归测试与一致性校验（按 `AGENTS.md` 触发器矩阵）。

## 交付物
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`（本文件）
- 相关迁移与代码变更（后续实施阶段交付）

## 关联与引用
- 质量门禁与触发器：`AGENTS.md`
- 写模型替代方案与前置背景：`docs/dev-plans/078-orgunit-write-model-alternatives-comparison-and-decision.md`
- 业务语义冻结：075 系列
