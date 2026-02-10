# DEV-PLAN-080C：OrgUnit 审计快照 presence 上升为表级强约束（INSERT 即写齐）实施方案

**状态**: 草拟中（2026-02-10 06:35 UTC）

## 1. 背景
- DEV-PLAN-080A 已完成第一阶段收敛：
  - `before_snapshot/after_snapshot` 语义统一为业务状态快照；
  - UI 侧不再回退 payload，缺口显式暴露；
  - DB Kernel 已通过 `assert_org_event_snapshots(...)` 实现 presence fail-closed。
- 当前结构性缺口：presence 仍依赖内核流程，而非数据库表级强约束。
- 本计划针对“方案 1：INSERT 即写齐快照 + CHECK”落地，实现“数据入表即满足快照契约”。

## 2. 原则约束（与仓库不变量对齐）
- One Door：仅允许 DB Kernel `submit_*` 写入口。
- No Legacy：不引入双链路、长期豁免窗口、运行时回退通道。
- Fail-Closed：快照不完整必须写入失败，不能靠 UI 容错掩盖。
- Contract First：先冻结 presence 与错误语义契约，再进入代码改造。

## 3. 目标与非目标

### 3.1 目标
- [ ] 将 OrgUnit 事件写入改造为“**单条 INSERT 即写齐 `before_snapshot/after_snapshot`**”，不再依赖后续 UPDATE 回写。
- [ ] 把 presence 规则提升为 `orgunit.org_events` 的表级 CHECK 约束。
- [ ] 保持 idempotency、纠错/撤销、增量投射行为不回退。
- [ ] 保持 RENAME/CORRECT/RESCIND 在审计 diff 的可读性与可解释性。

### 3.2 非目标
- 不引入第二写入口。
- 不引入快照版本化 envelope（本轮先不做）。
- 不引入历史 replay 回填工具链（历史问题先显式暴露，按单独计划处理）。

## 4. 定稿决策（冻结口径）

### 4.1 写入顺序总原则
统一改为：
1. 参数与租户校验；
2. request_code / event_uuid 幂等冲突预检；
3. 读取 `before_snapshot`；
4. 分配 `v_event_db_id := nextval('orgunit.org_events_id_seq'::regclass)`；
5. 执行 apply/rebuild（按事件族策略）；
6. 读取 `after_snapshot`；
7. `assert_org_event_snapshots(...)`；
8. 单条 INSERT 一次性写入 `id + before_snapshot + after_snapshot`；
9. 事务提交时统一校验 deferrable FK。

### 4.2 presence 语义矩阵（定稿）

| 事件类型 | before_snapshot | after_snapshot | 说明 |
| --- | --- | --- | --- |
| CREATE | 可空 | 必填 | 创建前对象不存在 |
| MOVE / RENAME / DISABLE / ENABLE / SET_BUSINESS_UNIT | 必填 | 必填 | 常规业务变更必须可比对 |
| CORRECT_EVENT / CORRECT_STATUS | 必填 | 必填 | 纠错必须给出前后状态 |
| RESCIND_EVENT / RESCIND_ORG | 必填 | 条件必填 | 见 4.3，按撤销结果判定 |

### 4.3 RESCIND 条件必填（收敛为可验证语义）
为避免“RESCIND after 一律可空”过宽，新增事件元数据列：
- `rescind_outcome text NULL`（仅 RESCIND 使用）
- 枚举：`PRESENT` / `ABSENT`

语义：
- `ABSENT`：撤销后该生效日对象不存在，`after_snapshot` 必须为 `NULL`。
- `PRESENT`：撤销后对象仍存在，`after_snapshot` 必须为非空 object。

约束层强制：
- 非 RESCIND 事件：`rescind_outcome IS NULL`。
- RESCIND 事件：`rescind_outcome IN ('PRESENT','ABSENT')` 且与 `after_snapshot` 空值关系一致。

> 说明：`rescind_outcome` 是事件元数据，不进入快照 diff；它用于让表级约束表达“何时允许 after 为 NULL”。

## 5. 关键难点与对应方案

### 5.1 难点 A：`apply_*` 依赖 `event_db_id`
- 方案：先 `nextval` 预分配 event id，并将该 id 传入 apply。

### 5.2 难点 B：`last_event_id -> org_events(id)` FK 会阻止“先 apply 后 insert"
- 方案：把 `org_unit_versions.last_event_id` FK 改为 `DEFERRABLE INITIALLY DEFERRED`，固定为事务提交时校验。

### 5.3 难点 C：CORRECT/RESCIND 当前依赖“先插入再重建”
现状中 CORRECT/RESCIND 的 `rebuild_org_unit_versions_for_org(...)` 依赖 `org_events_effective` 读到新插入事件；若改为“先重建后插入”会失真。

定稿方案：新增“pending-event 重建”能力，不引入第二写入口：
- 新增内核函数（命名示意）：
  - `rebuild_org_unit_versions_for_org_with_pending_event(...)`
- 函数内部用 CTE 构造 `effective_events = persisted_events UNION pending_event_overlay`，重建时显式包含“尚未入表的当前事件”。
- `submit_org_event_correction/submit_org_status_correction/submit_org_event_rescind/submit_org_rescind` 全部改走该能力，再执行单条 INSERT 写齐快照。

## 6. 表级约束设计（目标 DDL）

### 6.1 shape 约束
- `org_events_snapshot_shape_check`：`before/after` 只要非空必须是 JSON object。

### 6.2 presence 约束
新增或收紧 `org_events_snapshot_presence_check`（伪码）：
- CREATE：`after_snapshot IS NOT NULL`
- MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT/CORRECT_EVENT/CORRECT_STATUS：`before_snapshot IS NOT NULL AND after_snapshot IS NOT NULL`
- RESCIND_EVENT/RESCIND_ORG：
  - `before_snapshot IS NOT NULL`
  - `rescind_outcome=ABSENT -> after_snapshot IS NULL`
  - `rescind_outcome=PRESENT -> after_snapshot IS NOT NULL`

### 6.3 rescind_outcome 一致性约束
新增 `org_events_rescind_outcome_check`：
- 非 RESCIND：`rescind_outcome IS NULL`
- RESCIND：`rescind_outcome IN ('PRESENT','ABSENT')`

## 7. No-Legacy 落地（替代 cutover 豁免）
本计划不采用“可配置历史窗口豁免”。改为：
1. 迁移前先执行违约扫描 SQL（presence/shape/rescind_outcome）并留证；
2. 若扫描非 0，迁移阻断，不上线约束；
3. 通过后直接启用正式约束；
4. 不保留运行时回退分支。

> 允许一次性“预检失败 -> 修复 -> 再迁移”的工程流程；不允许长期并行契约。

## 8. 幂等与错误语义契约（防行为回归）
- 保持现有领域错误语义不退化：
  - `ORG_REQUEST_ID_CONFLICT`
  - `ORG_IDEMPOTENCY_REUSED`
  - `ORG_AUDIT_SNAPSHOT_MISSING`
  - `ORG_AUDIT_SNAPSHOT_INVALID`
- 即使底层触发唯一约束/检查约束，也要在 kernel 层转换为既有领域错误码（或在计划中明确新增错误码并统一回归）。

## 9. 实施拆解（WP）

### WP-0：契约冻结
1. [ ] 冻结 4.2/4.3 矩阵与 RESCIND 条件必填规则。
2. [ ] 在 080/080A/080C 执行记录中留痕“为何取消 cutover 豁免”。

### WP-1：DDL 改造
3. [ ] `last_event_id` FK 改为 `DEFERRABLE INITIALLY DEFERRED`。
4. [ ] `org_events` 增加 `rescind_outcome` 列与一致性约束。
5. [ ] 落地 `shape + presence + rescind_outcome` 三类 CHECK 约束。

### WP-2：Kernel 改造
6. [ ] `submit_org_event`（含 allocator）改造为 INSERT 即写齐。
7. [ ] 增加 pending-event 重建函数，并让 CORRECT/RESCIND 全量改用。
8. [ ] 删除所有“INSERT 后 UPDATE 快照”路径。

### WP-3：回归验证
9. [ ] RENAME：断言 `name old->new`，禁止 `new_name` 漂移到业务 diff。
10. [ ] CORRECT：缺 before/after 任一即失败。
11. [ ] RESCIND：
   - `rescind_outcome=ABSENT` 时 `after_snapshot` 必须为 NULL；
   - `rescind_outcome=PRESENT` 时 `after_snapshot` 必须非空。
12. [ ] 幂等/冲突错误码回归，避免退化为裸 SQL 异常。

### WP-4：门禁与证据
13. [ ] `make orgunit plan && make orgunit lint && make orgunit migrate up`
14. [ ] `make sqlc-generate`（生成物无漂移）
15. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
16. [ ] 更新 `docs/dev-records/dev-plan-080-execution-log.md`（必要时新增 080C 执行记录）

## 10. 验收标准
- [ ] 任一应填快照缺失，数据库层直接拒绝。
- [ ] 全部 `submit_*` 不再依赖 INSERT 后 UPDATE 快照。
- [ ] CORRECT/RESCIND 在“pending-event 重建”下仍与现行业务语义一致。
- [ ] RESCIND 的 after 空值不再是宽松放行，而是由 `rescind_outcome` 强约束。
- [ ] 无长期豁免窗口、无 legacy 回退路径。

## 11. 风险与缓解
- 风险 1：pending-event 重建实现复杂，易引入 replay 偏差。  
  - 缓解：先引入双轨对照测试（新旧重建结果比对）再切换。
- 风险 2：deferrable FK 失败点后移，排障复杂度上升。  
  - 缓解：统一异常映射 + 失败日志中输出 event_uuid/request_code/org_id。
- 风险 3：历史数据存量不满足新约束导致迁移阻断。  
  - 缓解：迁移前扫描脚本 + 明确修复工单，修完再上约束。

## 12. 关联文档
- `docs/dev-plans/080a-orgunit-audit-snapshot-mechanism-and-fix.md`
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- `docs/dev-plans/025-sqlc-guidelines.md`
- `AGENTS.md`
