# DEV-PLAN-077：OrgUnit Replay 写放大评估与收敛方案

**状态**: 进行中（2026-02-08 12:32 UTC）

## 背景
- 075E 上线后，OrgUnit 的 correction/rescind/status-correction 路径稳定可用，但均会触发 `replay_org_unit_versions(...)`。
- 现有 replay 为“按租户全量重建投影”（删除再重放），存在显著写放大风险。
- 用户关切场景：若把顶节点生效日修正为 `2026-02-01`，系统会发生什么，写放大会有多大。

## 目标
1. 给出当前 replay 写放大基线（可复现实测，而非主观估计）。
2. 评估“顶节点生效日改到 2026-02-01”的行为与风险。
3. 给出可落地的收敛路径（短期护栏 + 中期增量化）。
4. 与 One Door / No Legacy / Fail-Closed 不变量保持一致。

## 范围与非目标
- 范围：OrgUnit 写路径（correction、status-correction、rescind）触发 replay 的放大评估与改进路线。
- 非目标：
  - 本文不直接改 SQL 内核逻辑；
  - 不引入 legacy 双链路；
  - 不变更 Valid Time 日粒度口径。

## 现状机制（SSOT）
- correction/status-correction/rescind 最终都会调用 `orgunit.replay_org_unit_versions(...)`。
- replay 入口即全量清空并重建：
  - `DELETE org_unit_versions`；
  - `DELETE org_trees`；
  - `DELETE org_unit_codes`；
  - 遍历 `org_events_effective` 逐条重放；
  - 最后全量更新 `full_name_path`。
- 同日唯一约束仍生效：`(tenant_uuid, org_id, effective_date)`。

## 面向用户的解释（本计划口径）

### 什么是“重放（replay）”
- 对用户可理解为：**把组织事件流水重新过一遍，重算组织树结果**。
- 事件表（`org_events`）是“原始流水/事实账本”，投影表（`org_unit_versions`/`org_trees`/`org_unit_codes`）是“可读结果/报表”。
- replay 的作用是：当 correction/rescind 这类操作改变了历史事件语义后，系统用同一套规则重算当前正确状态。

### 哪些表会受 replay 直接影响
- **会被清空并重建（按租户）**：
  - `orgunit.org_unit_versions`
  - `orgunit.org_trees`
  - `orgunit.org_unit_codes`
- 这三张表是投影层，属于“可由事件重建”的结果数据。

### 哪些表不受 replay 直接清空影响
- **不会被 replay 删除/重建**：
  - `orgunit.org_events`（事件 SoT，仅被读取用于重算）
  - `orgunit.org_id_allocators`（org_id 分配器）
  - `orgunit.org_event_corrections_current`
  - `orgunit.org_event_corrections_history`
- 说明：上述 correction 表不会被 replay 清空，但 correction/rescind 操作会触发 replay。

### 与 026D 的关系（避免口径混淆）
- 026D 已将 `submit_org_event` 的常规写路径收敛为增量投影（不再每次全量 replay）。
- 本计划 077 聚焦的是 correction/status-correction/rescind 这几条路径，它们当前仍会触发 replay，因此写放大问题依然成立。

> 参考：
> - `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`
> - `migrations/orgunit/20260202160000_orgunit_remove_hierarchy_type.sql`

## 基线测量（P0）

### 样本规模（tenant=00000000-0000-0000-0000-000000000001）
- `org_events_total=20`
- `org_events_effective=19`
- `org_unit_versions=18`
- `org_unit_codes=15`
- `org_trees=1`

### 实测场景 A（可成功）
- 操作：对 `org_id=10000011` 执行 `effective_date -> 2026-02-01` correction（事务内回滚）。
- xact 写计数：
  - `org_event_corrections_history`: `ins=1`
  - `org_event_corrections_current`: `upd=1`
  - `org_unit_versions`: `ins=19, upd=27, del=18`
  - `org_unit_codes`: `ins=15, del=15`
  - `org_trees`: `ins=1, del=1`
- 结论：单次业务修正可触发近百次行写（本样本约 98 行级写）。

### 实测场景 B（顶节点改 2026-02-01，失败）
- 操作：对租户根节点（`org_id=10000000`）执行 `effective_date -> 2026-02-01` correction（事务内回滚）。
- 结果：`ORG_TREE_NOT_INITIALIZED`（重放顺序下，子节点在根节点创建前出现）。
- 失败前已发生尝试写：
  - `org_unit_versions del=18`
  - `org_unit_codes del=15`
  - `org_trees del=1`
- 结论：即便 fail-closed 最终回滚，仍会产生显著计算与写开销。

## 对“顶节点生效日改为 2026-02-01”的影响评估
1. 业务语义层：属于“早期关键事件重排”，会改变大量后续事件的可重放上下文。
2. 数据一致性层：系统会 fail-closed，不会留下半成品脏数据。
3. 性能层：是高风险高放大操作，容易放大锁持有时间与抖动。
4. 用户体验层：若无前置可行性校验，用户会看到提交失败，但系统已消耗大量资源。

## 写放大模型（当前实现）
- 粗略下界（每次 replay）：
  - 固定删：`V + C + 1`
  - 固定重建：与有效事件规模近似同阶
  - 额外更新：由 split/rename/move/path rebuild 决定，通常不可忽略
- 因此总体复杂度可视为：`O(E_effective + V + C + subtree_updates)`，且常数项较大。

## 收敛策略（分阶段）

### P0：基线与告警口径（已完成）
1. [X] 完成样本 tenant 的 replay 写放大实测。
2. [X] 明确“顶节点改早期日期”失败模式与错误码。
3. [X] 形成证据记录并纳入 `docs/dev-records/`。

### P1：短期护栏（待实施）
- 已落地：新增稳定错误码 `ORG_HIGH_RISK_REORDER_FORBIDDEN`（DB fail-fast + API/页面映射）。
1. [X] 在 correction 前增加“高风险重排预校验”（根节点/祖先节点早移直接 fail-fast）。
2. [X] 在 API 层补充稳定错误语义，避免用户触发高成本失败重放。
3. [ ] 增加 replay 耗时与行写统计日志（最小字段：tenant/org_id/request_id/op）。

### P2：中期收敛（待实施）
1. [ ] 对齐 DEV-PLAN-026D，推进 OrgUnit 增量投影回放（按受影响 org/subtree 范围重建）。
2. [ ] 评估 `org_unit_codes/org_trees` 局部重建可行性，减少全量 delete/insert。
3. [ ] 为 correction/status-correction/rescind 建立“影响域计算”并复用。

### P3：验收与门禁（待实施）
1. [ ] 建立回归压测样例（小/中/大租户）。
2. [ ] 定义并固化 SLO：单次写操作的可接受重放耗时与写行数上限。
3. [ ] 将基线对比结果写入执行日志并完成状态收口。

## 最小验收标准
- 能稳定复现并量化当前写放大。
- 对“顶节点改早期日期”提供前置可解释失败，不进入高成本重放。
- 增量化方案明确落地到可执行任务，并与 026D 对齐。

## 证据与记录
- 执行记录：`docs/dev-records/dev-plan-077-write-amplification-baseline.md`

## 触发器与门禁（引用 SSOT）
- 以 `AGENTS.md` 的触发器矩阵与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
- 本文档变更本地最小校验：`make check doc`。

## 关联文档
- `docs/archive/dev-plans/026d-orgunit-incremental-projection-plan.md`
- `docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- `docs/archive/dev-plans/075d-orgunit-status-field-active-inactive-selector.md`
- `docs/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
