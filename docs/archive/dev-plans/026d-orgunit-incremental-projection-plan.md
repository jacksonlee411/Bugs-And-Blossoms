# [Archived] DEV-PLAN-026D：OrgUnit 增量投射方案（减少全量回放写放大）

**状态**: 已完成（2026-02-03 08:58 UTC）

## 1. 背景与问题
- DEV-PLAN-026B 已完成实施，当前写入采用“事件入库 + 同事务全量 delete+replay”。
- 该机制保证一致性与可重建性，但会随事件规模线性增加写入成本与锁持有时间。
- 需要在不破坏 One Door 与事务一致性的前提下，评估“增量投射”以降低写放大。

## 1.1 现状摘要（证据）
- 事件写入后调用 `replay_org_unit_versions`，回放会删除租户级 `org_unit_versions/org_trees/org_unit_codes` 并全量重建。
- 写入与回放在同一事务内完成（fail-closed）。
- replay 保留为运维修复入口（非应用层 API）。

## 2. 目标与非目标
### 2.1 目标
- 降低写入时的回放成本（避免每次全量 delete+replay）。
- 保持事务一致性、RLS 与 One Door 原则不变。
- 保留 replay 作为运维修复入口（非正常写路径）。

### 2.2 非目标
- 不引入新表或多写入口。
- 不新增缓存、异步投射或双轨写链路。
- 不改变业务语义（Valid Time、同日唯一、gapless/no-overlap）。

### 2.3 前置条件/依赖
- DEV-PLAN-026B 已完成实施，且当前 OrgUnit schema 与函数签名已稳定。
- 明确 `hierarchy_type` 是否已移除；若仍存在，增量实现需沿用当前参数形态；若已移除，需同步调整函数签名与查询条件。
- 026C 中关于校验链路/重放策略/占位策略的修订如影响本计划，应先完成对齐。

## 3. 方案概述
- 将 `submit_org_event` 的正常写路径从“全量回放”改为“增量 apply_*_logic”。
- 事件仍先入库，随后基于事件类型执行对应增量逻辑，仅影响目标 org_id 或子树。
- 仍保留 `replay_org_unit_versions` 作为运维修复入口（非正常写路径）。

## 4. 关键设计
### 4.1 写入口与事件类型映射
- `CREATE` → `apply_create_logic`
- `MOVE` → `apply_move_logic`
- `RENAME` → `apply_rename_logic`
- `DISABLE` → `apply_disable_logic`
- `SET_BUSINESS_UNIT` → `apply_set_business_unit_logic`

> 所有 apply_* 均在同一事务内执行；失败则回滚事件写入（fail-closed）。

### 4.2 full_name_path 增量更新
- 现状：full_name_path 仅在全量 replay 中统一重算。
- 方案：为受影响子树提供“局部重算”逻辑：
  - `CREATE`：为新节点生成 full_name_path（至少包含祖先路径）。
  - `RENAME`：更新该节点及其子树在 `effective_date` 之后的 full_name_path。
  - `MOVE`：更新该节点及其子树的 node_path 与 full_name_path。
- 建议新增内部函数：
  - `rebuild_full_name_path_subtree(p_tenant_uuid, p_root_path, p_from_date)`
  - 仅更新 `node_path <@ p_root_path` 且 `lower(validity) >= p_from_date` 的版本。

### 4.3 局部不变量校验
- 现状：全量 replay 后统一校验 gapless 与末段 infinity。
- 方案：改为“受影响 org_id 子集”校验：
  - `CREATE`：校验新 org_id 的 validity 完整性。
  - `RENAME/DISABLE/SET_BUSINESS_UNIT`：仅校验目标 org_id。
  - `MOVE`：校验目标 org_id 及其子树（涉及 split 与 reparent）。

### 4.4 回放入口保留
- `replay_org_unit_versions` 保留为运维修复入口，仅限非正常写路径调用。
- 应明确权限与使用场景，避免“双链路”长期并存。

### 4.5 影响面清单（改动点）
- DB 函数：`submit_org_event` 写路径改为事件类型分发；必要时新增内部 helper 函数（full_name_path 子树重算）。  
- `apply_*_logic`：确认在“非空 versions”场景下的幂等与边界；MOVE/RENAME 需同步更新 full_name_path。  
- 运维入口：明确 replay 的使用边界与权限（仅修复，不作为正常写路径）。

### 4.6 具体函数变更清单（草案）
- `orgunit.submit_org_event(...)`  
  - 现状：事件入库 → 全量 `replay_org_unit_versions`。  
  - 目标：事件入库 → 分发到 `apply_*_logic`（增量更新）。  
  - 保留：幂等校验与错误形状不变（`ORG_IDEMPOTENCY_REUSED` 等）。
- `orgunit.replay_org_unit_versions(...)`  
  - 现状：全量 delete+replay。  
  - 目标：保留为维护入口，不再作为正常写路径调用。  
  - 约束：仅 `orgunit_kernel` 可执行（见权限条款）。
- `orgunit.apply_create_logic(...)`  
  - 增量路径需补齐：full_name_path 祖先路径拼接；必要时调用子树重算函数。  
  - 确认：写入 `org_unit_codes` 的时机与约束保持不变。
- `orgunit.apply_move_logic(...)`  
  - 增量路径需补齐：子树 `full_name_path` 与 `node_path` 同步更新（与现有 node_path 更新逻辑对齐）。  
- `orgunit.apply_rename_logic(...)`  
  - 增量路径需补齐：更新节点及子树的 `full_name_path`（`effective_date` 之后）。  
- `orgunit.apply_disable_logic(...)` / `orgunit.apply_set_business_unit_logic(...)`  
  - 维持现有逻辑；如涉及 `full_name_path` 无需额外处理。  
- **新增（建议）**：`orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, p_root_path, p_from_date)`  
  - 仅更新 `node_path <@ p_root_path` 且 `lower(validity) >= p_from_date` 的版本。  

### 4.7 回放权限与使用边界（草案）
- `replay_org_unit_versions` 仅作为维护/修复入口：  
  - 不在正常写路径中调用；  
  - 仅 `orgunit_kernel` 角色拥有执行权限；  
  - 应在文档与门禁中明确“应用层禁止直接调用”。  
- **权限约束（拟新增）**：  
  - `REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM PUBLIC;`  
  - 对 `app/app_runtime/app_nobypassrls/superadmin_runtime` 显式 `REVOKE EXECUTE`；  
  - `GRANT EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) TO orgunit_kernel;`  
  - （如需）设置函数 OWNER 为 `orgunit_kernel` 并固定 `search_path`。  
- 现有 `org_unit_codes` 写入触发器保留：非 `orgunit_kernel` 写入一律拒绝（避免旁路写入口）。  

#### 权限变更 SQL 草案（拟落在 orgunit 权限迁移中）
> 计划位置：优先追加到 `modules/orgunit/infrastructure/persistence/schema/00014_orgunit_org_code_kernel_privileges.sql`，避免新增迁移文件；如需拆分，再创建独立迁移并在计划中注明。

```sql
REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM superadmin_runtime';
  END IF;
END $$;

GRANT EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) TO orgunit_kernel;
ALTER FUNCTION orgunit.replay_org_unit_versions(uuid) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.replay_org_unit_versions(uuid) SET search_path = pg_catalog, orgunit, public;
```

### 4.8 full_name_path 增量算法草案（最小可执行）
- **CREATE**：在插入新版本前，查父节点在 `effective_date` 的 `full_name_path`；新节点 `full_name_path = parent_full_name_path || ' / ' || name`（root 仅为 name）。
- **RENAME**：先执行 `split_org_unit_version_at`，仅对 `lower(validity) >= effective_date` 的目标节点与其子树重算 `full_name_path`（祖先链以当前节点路径为准）。
- **MOVE**：在更新 `node_path` 后，对子树执行 `full_name_path` 重算（以新路径的祖先链拼接）。
- **实现建议**：新增 `rebuild_full_name_path_subtree(p_tenant_uuid, p_root_path, p_from_date)`，内部按 `path_ids` 逐层 join 计算，范围限定 `node_path <@ p_root_path` 且 `lower(validity) >= p_from_date`。

### 4.9 局部不变量校验草案（SQL 模板）
- **gapless（局部）**：
  - 输入：`p_org_ids int[]`
  - 模板：
    ```sql
    WITH ordered AS (
      SELECT org_id, validity,
        lag(validity) OVER (PARTITION BY org_id ORDER BY lower(validity)) AS prev_validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = $1 AND org_id = ANY($2)
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL AND lower(validity) <> upper(prev_validity)
    LIMIT 1;
    ```
- **末段 infinity（局部）**：
  - 模板：
    ```sql
    SELECT 1
    FROM (
      SELECT DISTINCT ON (org_id) org_id, validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = $1 AND org_id = ANY($2)
      ORDER BY org_id, lower(validity) DESC
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1;
    ```
- **MOVE 子树范围**：以 `node_path <@ p_root_path` 选出 org_id 列表，再做局部校验。

### 4.10 锁策略与收益说明
- 方案默认**保持租户级锁**（现有 `org:write-lock:<tenant>`），收益来自“写放大减少”，不是并发度提升。
- 如需细化为子树/节点锁，需新增锁键策略与死锁规约，并补充一致性验证。

## 5. 风险与缓解
- **复杂度提升**：增量逻辑必须覆盖 full_name_path 与局部不变量，否则易产生漂移。
  - 缓解：在测试中进行“增量写入后全量 replay 对比”。
- **MOVE/RENAME 影响范围广**：子树更新开销较大，需评估实际树规模。
  - 缓解：以子树为单位更新，避免全租户扫描。
- **回放入口滥用**：若应用层可调用 replay，可能形成双链路。
  - 缓解：权限隔离、文档约束与门禁检查。
- **校验覆盖不足**：局部校验可能遗漏跨节点的不变量漂移。
  - 缓解：在验收中保留“增量写入后全量 replay 对照”与基线校验。

## 6. 验收标准
- [ ] 增量写入与全量 replay 结果一致（结构、validity、full_name_path）。
- [ ] 写入时不再执行租户级 delete+replay。
- [ ] 回放入口仅用于运维修复；应用层不依赖 replay。
- [ ] 关键错误码与 fail-closed 行为不变。
- [ ] `org_unit_codes` 仍由事件投射产生，不引入旁路写入口。

### 6.1 测试矩阵（最小集）
- **对照测试**：同一组事件分别走“增量写入路径”与“全量 replay”，比较 `org_unit_versions/org_trees/org_unit_codes` 的一致性。
- **场景覆盖**：CREATE / RENAME / MOVE / DISABLE / SET_BUSINESS_UNIT。
- **断言口径**：结构（node_path/parent_id）、validity（gapless/no-overlap/末段 infinity）、full_name_path。

## 7. 实施步骤
1. [ ] 明确前置依赖（`hierarchy_type` 状态、026C 修订对齐）。
2. [ ] 设计增量投射的 full_name_path 局部更新函数与适用范围。
3. [ ] 调整 `submit_org_event`：以事件类型分发到 `apply_*_logic`。
4. [ ] 为 MOVE/RENAME/CREATE 补齐子树 full_name_path 重算逻辑。
5. [ ] 增加“增量写入 vs 全量 replay”一致性测试（对照结构/validity/full_name_path）。
6. [ ] 保留 replay 维护入口，并补充权限与文档说明。
7. [ ] 记录验证结果到 `docs/dev-records/`（按 SSOT 规则）。

## 8. 门禁对齐
- 触发器与门禁入口以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为 SSOT。

## 9. 交付物
- DEV-PLAN-026D（本文件）。
- 增量投射实现与测试用例。
- 相关 dev-records 证据记录（如涉及执行/验证）。
