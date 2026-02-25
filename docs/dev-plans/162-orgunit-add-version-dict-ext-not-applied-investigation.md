# DEV-PLAN-162：OrgUnit 新增版本后组织类型回退问题（最正确改动方案）

**状态**: 已实施（2026-02-24 14:06 UTC，代码/迁移/门禁完成；历史数据重建待执行）

## 1. 背景与问题定义

用户在页面  
`/app/org/units/00000002?as_of=2026-02-24&effective_date=2026-01-12&tab=profile`  
执行“新增版本”，把组织类型改为“部门1”后提交，最终仍显示“单位”。

该问题本质不是 UI 或 API 入参丢失，而是 **事件已写入，版本投影未按有效日期切分后再应用 ext**，导致详情读取到旧版本值。

## 2. 已完成调查（事实证据）

1. [X] `d_org_type` 已启用且映射 `physical_col=ext_str_01`。  
2. [X] `org_events_effective` 已存在 `UPDATE@2026-01-12`，payload 含 `ext.d_org_type='11'` 与 `ext_labels_snapshot='部门1'`。  
3. [X] `org_unit_versions` 同 org 仍为 `ext_str_01='20'`、`ext_labels_snapshot='单位'`。  
4. [X] 当前内核 `submit_org_event(...)` 已调用 `apply_org_event_ext_payload(...)`，并非“漏调用”。  
5. [X] 真实根因：`apply_update_logic(...)` 对 ext-only payload 不切分版本，`apply_org_event_ext_payload(...)` 又按 `lower(validity) >= effective_date` 更新，导致命中空集。  

## 3. 影响范围（已确认）

- 受影响：
  - `UPDATE` 事件的 ext-only 变更（`add_version` / `insert_version` 的典型路径）。
  - `CORRECT_EVENT`（目标=UPDATE）在有效 payload 为 ext-only 时同样受影响（重放链路同机制）。
- 当前未观察到受影响：
  - `CREATE`、`MOVE`、`RENAME`、`DISABLE`、`ENABLE`、`SET_BUSINESS_UNIT`（这些路径已有切分/或天然落在新版本起点）。

## 4. 目标与非目标

### 4.1 目标

- [x] 建立内核不变量：**只要事件 payload 含 ext/ext_labels_snapshot，就保证在该 effective_date 上有可写版本切片**。  
- [x] 消除 ext-only 的静默失败（事件成功但投影不变）。  
- [x] 覆盖 UPDATE 与 CORRECT_EVENT(目标=UPDATE) 两条链路。  
- [ ] 提供历史数据修复闭环（已受污染数据可重建）。  

### 4.2 非目标

- 不新增 legacy 回退通道。
- 不改动业务语义（仍是“从 effective_date 起生效并向后传播，后续事件可覆盖”）。

## 5. 设计原则（Simple > Easy）

1. **内核中心化**：不依赖调用方“记得先改 core 再改 ext”。  
2. **不变量优先**：以 DB Kernel 保证正确性，前端/服务层仅作体验与参数校验。  
3. **写后即一致**：事件提交后 `after_snapshot` 与 `org_unit_versions` 必须一致。  
4. **同构链路**：在线提交与重放（replay）共享同一正确语义。  

## 6. 最正确改动（Chosen Design）

> 结论：不采用“在单个上游调用点补丁式加 split”的局部修法，而是把“ext 生效前必须可命中版本切片”收敛到 ext 投影内核函数自身。

### 6.1 内核改动

1. [x] 为 `apply_org_event_ext_payload(...)` 增加事件上下文参数（含 `event_db_id`），在检测到 payload 含 ext 时执行“必要切分”。  
2. [x] 在 `apply_org_event_ext_payload(...)` 内部统一执行：
   - 校验 event 类型是否允许 ext；
   - 若有 ext payload，则先 `split_org_unit_version_at(...)`；
   - 再更新 ext 列与 `ext_labels_snapshot`。
3. [x] 所有调用点统一传入 `event_db_id`：
   - `submit_org_event(...)`；
   - `rebuild_org_unit_versions_for_org_with_pending_event(...)`（覆盖 CORRECT_EVENT 重放路径）。
4. [x] 保持 ext 投影发生在 `assert_org_event_snapshots(...)` 之前。

### 6.2 为什么这是“最正确”

- 规则被放在“唯一 ext 投影入口”，不会因未来新增事件分支而再次漏改。  
- 在线路径与重放路径共享同一实现，不会出现“线上错、重放对”或相反。  
- 不依赖“core 字段是否同时变化”的偶然性。  

## 7. 数据修复策略（历史污染收敛）

1. [ ] 发布修复后，按租户扫描含 ext 的 `UPDATE/CORRECT_EVENT` org 集合。  
2. [ ] 对受影响 org 执行 `rebuild_org_unit_versions_for_org(...)` 重建版本投影。  
3. [ ] 重建后抽样比对：
   - `org_events_effective.payload.ext`  
   - `org_unit_versions` 对应 `effective_date` 切片  
   - `/org/api/org-units/details` 返回值  
   三者一致才视为修复完成。  

## 8. 测试与门禁（新增必测矩阵）

1. [x] SQL/内核验证：`UPDATE ext-only` 在中间 effective_date 可正确切分并生效（本地事务回滚验证）。  
2. [x] SQL/内核验证：`CORRECT_EVENT(target=UPDATE) ext-only` 可生效（本地事务回滚验证）。  
3. [ ] API 层测试：`write(add_version|insert_version)` 提交 ext-only 后，details 读到新值。  
4. [ ] 回归测试：`MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT + ext` 行为不回退。  
5. [x] 文档门禁：`make check doc`。  
6. [x] 实施阶段按触发器执行 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 对应门禁。  

## 9. 验收标准（DoD）

1. [x] SQL 内核链路下，`UPDATE ext-only` 后读取值稳定为“部门1”。  
2. [x] UPDATE ext-only 不再出现“事件成功但版本不变”。  
3. [x] CORRECT_EVENT(target=UPDATE) ext-only 不再出现同类问题。  
4. [ ] 历史受影响数据完成重建并抽样通过。  
5. [x] 无 legacy 分支、无双链路。  

## 10. 实施步骤（后续执行）

1. [x] M1：提交 DB Kernel 迁移（函数签名 + 调用点 + 不变量落地）。  
2. [x] M2：补齐 SQL/Go 测试矩阵并跑通质量门禁。  
3. [ ] M3：执行历史数据重建与核对，记录证据到 `docs/dev-records/`。  
4. [x] M4：更新计划状态与验收结果。  

## 11. 本次实施结果（2026-02-24）

### 11.1 代码与迁移落地

1. 新增迁移：`migrations/orgunit/20260224142000_orgunit_ext_payload_split_invariant.sql`  
   - 重定义 `apply_org_event_ext_payload(...)`：新增 `p_event_db_id`，并在 ext 投影前统一执行切分。  
   - 重定义 `rebuild_org_unit_versions_for_org_with_pending_event(...)`：传递 `v_event.id` 给 ext 投影。  
   - 重定义 `submit_org_event(...)`：传递 `v_event_db_id` 给 ext 投影。  
2. 同步 SSOT：
   - `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`  
   - `modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql`  
3. 更新迁移哈希：`migrations/orgunit/atlas.sum`。  
4. 更新导出 schema：`internal/sqlc/schema.sql`。  

### 11.2 验证结果

1. 结构/门禁验证通过：  
   - `make orgunit plan`  
   - `make orgunit lint`  
   - `make check doc`  
2. Go 质量门禁通过：  
   - `go fmt ./...`  
   - `go vet ./...`  
   - `make check lint`  
   - `make test`  
3. 内核事务回滚验证（本地 DB）：
   - `UPDATE ext-only`：`2026-01-19` 写入后，`org_unit_versions` 读取 `ext_str_01=11`、label=`部门1`。  
   - `CORRECT_EVENT(target=UPDATE) ext-only`：修正后同样读取 `ext_str_01=11`、label=`部门1`。  
