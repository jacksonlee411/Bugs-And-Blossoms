# DEV-PLAN-330 PR-3 记录：Schema 双轴化与历史回填

**状态**: 已完成（2026-04-11 15:50 CST）

## 1. 范围

1. [X] 在既有 `orgunit.setid_strategy_registry` 上增量引入 `resolved_setid`，不新建平行事实源。
2. [X] 冻结 `resolved_setid` 的物理 wildcard 表达，并把双轴合法形状写成表级约束。
3. [X] 提供历史记录回填迁移：tenant 作用域显式回填 wildcard，business_unit 作用域按生效日解析 exact `resolved_setid`。
4. [X] 为“无法唯一确定 `resolved_setid` 的历史记录”建立 stopline，不允许用 wildcard 或兼容兜底偷渡。
5. [X] 同步 Registry store、dbtool snapshot/validate 与测试夹具到双轴 schema。
6. [X] 本批不切主查询、不合并唯一 PDP、不宣称 explain/version 主链切换完成；这些仍属于 `PR-4/PR-5`。

## 2. 代码交付

1. [X] 源 schema 增量引入 `resolved_setid`、格式约束、合法形状约束与双轴索引：
   [00020_orgunit_setid_strategy_registry_schema.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql)
2. [X] 新增 `PR-3` 迁移，包含历史回填、stopline 与 downgrade 防坍缩检查：
   [20260411121500_orgunit_setid_strategy_registry_resolved_setid.sql](/home/lee/Projects/Bugs-And-Blossoms/migrations/orgunit/20260411121500_orgunit_setid_strategy_registry_resolved_setid.sql)
3. [X] Registry PG store / runtime key / handler 测试同步到双轴记录键：
   [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go)
   [setid_strategy_registry_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api_test.go)
4. [X] `/internal/rules/evaluate` 测试同步到 `selected_rule_id` 双轴 key 形状：
   [internal_rules_evaluate_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api_test.go)
5. [X] dbtool schema/row 校验与 snapshot import/export/verify 同步到 `resolved_setid`：
   [orgunit_setid_strategy_registry_validate.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_setid_strategy_registry_validate.go)
   [orgunit_setid_strategy_registry_snapshot.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_setid_strategy_registry_snapshot.go)
   [orgunit_setid_strategy_registry_validate_test.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_setid_strategy_registry_validate_test.go)
   [orgunit_setid_strategy_registry_snapshot_test.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_setid_strategy_registry_snapshot_test.go)

## 3. 实施要点

### 3.1 双轴 shape 已冻结

1. [X] `resolved_setid` 的 wildcard 物理表达固定为 `''`，且只能表达 wildcard。
2. [X] 只允许以下三类记录形状：
   - `tenant + resolved_setid='' + business_unit_node_key=''`
   - `business_unit + resolved_setid=exact + business_unit_node_key=exact`
   - 未来可承接 `setid exact + bu wildcard`，但本批不新增新写入口去制造该形状
3. [X] `setid wildcard + bu exact` 被表级约束阻断，不能作为兼容态残留。

### 3.2 历史回填与 stopline 已写入迁移

1. [X] tenant 作用域历史记录统一回填 `resolved_setid=''`。
2. [X] business_unit 作用域历史记录按 `business_unit_node_key + effective_date` 从 SetID 绑定版本解析 exact `resolved_setid`。
3. [X] 若某条 business_unit 历史记录在对应生效日上无法得到唯一 SetID，迁移直接抛出 `SETID_STRATEGY_RESOLVED_SETID_BACKFILL_BLOCKED`，阻断继续迁移。
4. [X] down migration 会检查移除 `resolved_setid` 后是否会造成旧唯一键坍缩；若会坍缩则拒绝降级。

### 3.3 运行时兼容边界

1. [X] 当前 Registry 主写链已能把 business_unit 记录写成 exact `resolved_setid`，tenant 记录写成 wildcard。
2. [X] 当前批次只完成“存储契约 + 回填工具 + 测试夹具”收口，不把单轴 PDP 命中逻辑提前宣称为已切双轴。
3. [X] 运行时 `strategyRegistrySortKey` 与 PG store 的冲突键已纳入 `resolved_setid`，避免存储层唯一键与测试夹具继续停留在单轴。

## 4. 验证

1. [X] `go test ./internal/server`
2. [X] `go test ./cmd/dbtool`

## 5. 对 PR-4 / PR-5 的冻结前提

1. [X] `PR-4` 之前不得声称“唯一 PDP 已完成”，因为当前 `resolveFieldDecisionFromItems(...)` 仍未作为双轴 PDP cutover 的最终形态冻结。
2. [X] `PR-5` 之前不得声称“Registry 主查询已完全切主链”，因为 explain/version/主查询的统一切换仍需后续批次完成。
3. [X] `PR-4/PR-5` 可以直接承接本批的双轴 schema 与回填 stopline，不再需要回头补 `resolved_setid` 存储契约。
