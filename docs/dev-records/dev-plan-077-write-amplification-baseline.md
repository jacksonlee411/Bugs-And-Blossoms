# DEV-PLAN-077 基线记录：OrgUnit Replay 写放大测量

## 基本信息
- 对应计划：`docs/dev-plans/077-orgunit-replay-write-amplification-assessment-and-mitigation.md`
- 记录时间：2026-02-08（UTC）
- 执行环境：本地开发库，tenant=`00000000-0000-0000-0000-000000000001`
- 说明：以下操作均在事务内执行并 `ROLLBACK`，不落库。

## 样本规模
- `org_events_total=20`
- `org_events_effective=19`
- `org_unit_versions=18`
- `org_unit_codes=15`
- `org_trees=1`
- `org_ids_distinct=16`

## 场景 A：非根节点生效日修正（成功）
- 输入：`submit_org_event_correction(org_id=10000011, target_effective_date=2026-01-01, patch.effective_date=2026-02-01)`
- 结果：返回 `correction_uuid`，可成功完成 replay。
- `pg_stat_xact_user_tables`：
  - `org_event_corrections_history`: `ins=1`
  - `org_event_corrections_current`: `upd=1`
  - `org_events`: `ins=0 upd=0 del=0`
  - `org_trees`: `ins=1 del=1`
  - `org_unit_codes`: `ins=15 del=15`
  - `org_unit_versions`: `ins=19 upd=27 del=18`

## 场景 B：租户根节点生效日修正为 2026-02-01（失败）
- 输入：`submit_org_event_correction(org_id=10000000, target_effective_date=2026-01-01, patch.effective_date=2026-02-01)`
- 结果：`ORG_TREE_NOT_INITIALIZED`（fail-closed，事务回滚）。
- 捕获到失败前写尝试：
  - `org_trees`: `del=1`
  - `org_unit_codes`: `del=15`
  - `org_unit_versions`: `del=18`
  - corrections 表仍有当次事务内写尝试（回滚后不落库）

## 结论
1. 当前 correction/replay 的写放大显著，单次业务修正可触发近百次行写（在本样本规模下）。
2. 顶节点早移属于高风险场景，即使最终回滚也会产生较大运行时开销。
3. 后续应优先推进“前置可行性校验 + 增量投影回放”以降低写放大。

## P1 进展（2026-02-08）
- 新增 DB fail-fast 护栏：当 `CREATE` 事件被 correction 后移且会早于其子树最早 `CREATE` 事件时，直接返回 `ORG_HIGH_RISK_REORDER_FORBIDDEN`。
- 新增迁移：`migrations/orgunit/20260208170500_orgunit_correction_reorder_guard.sql`。
- API/UI 错误语义已接入：`ORG_HIGH_RISK_REORDER_FORBIDDEN` 统一映射为 409 与可操作提示，避免进入高成本失败重放路径。
- 验证（事务内回滚）：根节点早移触发 `ORG_HIGH_RISK_REORDER_FORBIDDEN` 后，`org_unit_versions/org_unit_codes/org_trees` 的 xact 写计数均为 0。
