# DEV-PLAN-080C：OrgUnit 审计快照 presence 表级强约束（INSERT 即写齐）详细设计

**状态**: 规划中（2026-02-10 15:20 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
  - `docs/dev-plans/080a-orgunit-audit-snapshot-mechanism-and-fix.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
- **当前痛点**:
  1. 080A 已把快照语义收敛到 canonical 业务状态，但写入路径仍存在“INSERT 事件行后 UPDATE 回写 before/after”的两阶段写入。
  2. presence 规则当前主要在 Kernel 的 `assert_org_event_snapshots(...)` 中表达，尚未完全上升为 `orgunit.org_events` 表级强约束。
  3. CORRECT/RESCIND 依赖先把事件落到 `org_events` 再触发 `rebuild_org_unit_versions_for_org(...)`，导致“INSERT 即写齐”改造时存在顺序耦合。
- **业务价值**:
  - 审计事件入表即满足契约（presence + shape），减少中间态与排障歧义。
  - 把关键不变量前移到数据库层 fail-closed，降低 UI/服务层“补救型容错”复杂度。
  - 对齐 One Door / No Legacy 与 DEV-PLAN-003 的“单一权威表达”。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标
- [ ] 所有 `submit_*` 写路径改为**单条 INSERT 一次性写齐** `before_snapshot/after_snapshot`（不再 INSERT 后 UPDATE）。
- [ ] `orgunit.org_events` 新增并启用表级 presence 约束，数据库层拒绝违规快照。
- [ ] 规则“单一权威表达”：presence 规则只维护一份定义，Kernel 与 CHECK 共同复用。
- [ ] CORRECT/RESCIND 在不新增第二重建引擎的前提下支持 pending 事件重建。
- [ ] 保持现有领域错误码口径不退化（`ORG_REQUEST_ID_CONFLICT`、`ORG_IDEMPOTENCY_REUSED`、`ORG_AUDIT_SNAPSHOT_MISSING`、`ORG_AUDIT_SNAPSHOT_INVALID`）。

### 2.2 非目标 (Out of Scope)
- 不引入历史 replay/离线回填工具链。
- 不新增运行时 feature flag、灰度开关、双链路 fallback。
- 不扩展快照 envelope 版本化格式（本轮继续使用 canonical object）。
- 不新增对外 HTTP 路由与 UI 交互协议。

## 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**:
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] `.templ` / Tailwind（本计划默认不命中）
  - [ ] 多语言 JSON（本计划默认不命中）
  - [ ] Authz（本计划默认不命中）
  - [ ] 路由治理（本计划默认不命中）
  - [x] DB 迁移 / Schema（`make orgunit plan && make orgunit lint && make orgunit migrate up`）
  - [x] sqlc（`make sqlc-generate`，并确保生成物无漂移）
  - [x] 文档门禁（`make check doc`）
- **SSOT 链接**:
  - 触发器矩阵：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁：`.github/workflows/quality-gates.yml`
  - Atlas/Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图 (Mermaid)
```mermaid
graph TD
    A[UI/API 调用 submit_*] --> B[DB Kernel 参数/幂等校验]
    B --> C[提取 before_snapshot]
    C --> D[预分配 org_events.id]
    D --> E[apply/rebuild(单重建引擎+可选pending)]
    E --> F[提取 after_snapshot + rescind_outcome]
    F --> G[assert + presence 谓词]
    G --> H[INSERT org_events 一次写齐]
    H --> I[事务提交(含 deferrable FK 校验)]
```

### 3.2 关键设计决策 (ADR 摘要)
- **决策 1：presence 规则单点化（选定）**
  - 选项 A：保留“assert 逻辑 + CHECK 表达式”双份维护。
  - 选项 B（选定）：新增布尔谓词函数 `orgunit.is_org_event_snapshot_presence_valid(...)`，Kernel 与 CHECK 均复用。
  - 理由：避免规则漂移，符合 DEV-PLAN-003 的“同一概念单一权威表达”。

- **决策 2：重建算法单引擎（选定）**
  - 选项 A：新增 `rebuild_org_unit_versions_for_org_with_pending_event(...)` 平行函数。
  - 选项 B（选定）：保留 `rebuild_org_unit_versions_for_org(...)` 为唯一对外重建入口，在其内部支持可选 pending 事件输入。
  - 理由：避免双引擎长期并存，降低维护复杂度。

- **决策 3：RESCIND 的 after 空值可验证化（选定）**
  - 选项 A：沿用“RESCIND after 可空但不区分场景”。
  - 选项 B（选定）：增加 `rescind_outcome` 元数据并绑定 CHECK。
  - 理由：把“何时允许 after 为 NULL”从隐式约定变成数据库可验证契约。

- **决策 4：迁移策略（选定）**
  - 选项 A：长期 cutover 豁免窗口。
  - 选项 B（选定）：无长期豁免；仅允许迁移批次内一次性预检与修复，失败即阻断。
  - 理由：对齐 No Legacy，防止长期并行契约。

## 4. 数据模型与约束 (Data Model & Constraints)

### 4.1 Schema 变更清单（目标）
变更对象：`orgunit.org_events`、`orgunit.org_unit_versions`

1. `orgunit.org_events` 新增列：
```sql
ALTER TABLE orgunit.org_events
  ADD COLUMN rescind_outcome text NULL;
```

2. `orgunit.org_events` 约束：
```sql
ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_snapshot_presence_check,
  DROP CONSTRAINT IF EXISTS org_events_rescind_outcome_check;

ALTER TABLE orgunit.org_events
  ADD CONSTRAINT org_events_rescind_outcome_check CHECK (
    (
      event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      AND rescind_outcome IN ('PRESENT','ABSENT')
    )
    OR
    (
      event_type NOT IN ('RESCIND_EVENT','RESCIND_ORG')
      AND rescind_outcome IS NULL
    )
  );

ALTER TABLE orgunit.org_events
  ADD CONSTRAINT org_events_snapshot_presence_check CHECK (
    orgunit.is_org_event_snapshot_presence_valid(
      event_type,
      before_snapshot,
      after_snapshot,
      rescind_outcome
    )
  );
```

3. `orgunit.org_unit_versions` 外键延迟校验：
```sql
ALTER TABLE orgunit.org_unit_versions
  DROP CONSTRAINT IF EXISTS org_unit_versions_last_event_id_fkey;

ALTER TABLE orgunit.org_unit_versions
  ADD CONSTRAINT org_unit_versions_last_event_id_fkey
  FOREIGN KEY (last_event_id)
  REFERENCES orgunit.org_events(id)
  DEFERRABLE INITIALLY DEFERRED;
```

> 说明：`org_events_snapshot_shape_check` 保留（非空必须是 JSON object）。

### 4.2 新增谓词函数（单一规则源）
```sql
CREATE OR REPLACE FUNCTION orgunit.is_org_event_snapshot_presence_valid(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb,
  p_rescind_outcome text
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT CASE
    WHEN p_event_type = 'CREATE'
      THEN p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS')
      THEN p_before_snapshot IS NOT NULL AND p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      THEN p_before_snapshot IS NOT NULL
           AND (
             (p_rescind_outcome = 'ABSENT' AND p_after_snapshot IS NULL)
             OR
             (p_rescind_outcome = 'PRESENT' AND p_after_snapshot IS NOT NULL)
           )

    ELSE true
  END;
$$;
```

### 4.3 迁移策略
- **Up**:
  1. 创建/更新谓词函数；
  2. 加列 `rescind_outcome`；
  3. 调整 `last_event_id` 为 deferrable FK；
  4. 落地 `rescind_outcome` + `presence` 约束；
  5. 重建相关函数 owner/security/search_path（防 `CREATE OR REPLACE FUNCTION` 回归）。
- **Down**:
  - 仅做最小可逆（移除新增约束/列/函数），不承诺恢复旧逻辑数据语义。
  - 生产环境默认不执行破坏性 down。

## 5. 接口契约 (API / Kernel Contracts)

### 5.1 外部 HTTP 契约
- 本计划**不新增/不修改**对外 HTTP 路由。
- 现有 API 错误码语义保持不变（重点是 DB 侧保证稳定 message，服务层维持映射）。

### 5.2 DB Kernel 函数契约（需要实现）
1. `orgunit.assert_org_event_snapshots(...)`
   - 输入：`event_type/before/after/rescind_outcome`
   - 行为：
     - shape 非法 -> `ORG_AUDIT_SNAPSHOT_INVALID`
     - presence 非法 -> `ORG_AUDIT_SNAPSHOT_MISSING`
   - 实现要求：presence 判断调用 `is_org_event_snapshot_presence_valid(...)`。

2. `orgunit.rebuild_org_unit_versions_for_org(...)`
   - 仍作为唯一重建引擎；新增“可选 pending 事件输入”。
   - 禁止新增长期并存的第二套重建算法。

3. `orgunit.submit_org_event` / `submit_org_event_correction` / `submit_org_status_correction` / `submit_org_event_rescind` / `submit_org_rescind`
   - 统一改造为 INSERT 即写齐 before/after/rescind_outcome。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 通用写入算法（所有 submit_*）
1. 开启事务并设置租户上下文。
2. 参数校验 + 幂等冲突预检（event_uuid/request_code）。
3. 读取 `before_snapshot := extract_orgunit_snapshot(...)`。
4. 分配 `v_event_db_id := nextval('orgunit.org_events_id_seq'::regclass)`。
5. 执行 apply/rebuild：
   - 基础事件：直接 apply（使用 `v_event_db_id`）。
   - CORRECT/RESCIND：调用同一重建引擎，传入 pending 事件输入。
6. 读取 `after_snapshot := extract_orgunit_snapshot(...)`。
7. 计算 `rescind_outcome`：
   - 仅 RESCIND：`after_snapshot IS NULL -> ABSENT`，否则 `PRESENT`。
   - 非 RESCIND：`NULL`。
8. 调用 `assert_org_event_snapshots(...)`（内部复用谓词函数）。
9. 单条 INSERT 写入完整事件行（显式写入 `id/before/after/rescind_outcome`）。
10. 提交事务，由 deferrable FK 在 commit 时统一校验。

### 6.2 CORRECT/RESCIND 的 pending 输入模式
- 构造单条 pending 事件结构：`id/event_type/effective_date/payload/event_uuid`。
- 重建时使用“persisted effective events + pending event”排序流：`ORDER BY effective_date, id`。
- 重建算法本体仅一份，禁止复制粘贴出第二实现。

### 6.3 错误语义与映射
- Kernel 内统一抛稳定 message：
  - `ORG_AUDIT_SNAPSHOT_INVALID`
  - `ORG_AUDIT_SNAPSHOT_MISSING`
- 服务/API 层禁止将上述错误退化为默认泛化码。

## 7. 安全与鉴权 (Security & Authz)
- 继续遵循 No Tx, No RLS：所有函数在事务内执行并依赖 `assert_current_tenant(...)`。
- 函数权限要求（防回归）：
  - owner: `orgunit_kernel`
  - `SECURITY DEFINER`
  - `search_path = pg_catalog, orgunit, public`
- 不引入新的授权模型与权限点。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**:
  - 080A 已上线并稳定（当前已满足）。
  - 080B 的函数权限修复规则必须继续保持。

- **里程碑**:
  1. [x] M1：谓词函数 + 约束 DDL + deferrable FK 迁移草案。
  2. [x] M2：重建引擎支持 pending 输入（单引擎实现）。
  3. [x] M3：全部 submit_* 改为 INSERT 即写齐。
  4. [x] M4：测试契约升级（从“禁止 INSERT 快照列”改为“禁止后置 UPDATE 快照”）。
  5. [x] M5：门禁全通过并更新执行记录。

## 9. 测试与验收标准 (Acceptance Criteria)

### 9.1 单元/集成测试要求
- [x] presence 谓词矩阵测试（事件类型 x before/after/rescind_outcome）完整覆盖。
- [x] `submit_org_event*` / `submit_org_*rescind*` 无“INSERT 后 UPDATE 快照”语句路径。
- [ ] RENAME diff 显示 `name old->new`，不出现 `new_name` 漂移。
- [x] CORRECT 缺 before/after 任一必须失败并返回稳定错误码。
- [x] RESCIND：
  - `ABSENT -> after_snapshot IS NULL`
  - `PRESENT -> after_snapshot IS NOT NULL`
- [ ] 幂等/冲突语义回归：不退化为裸 SQL 错误。

### 9.2 门禁执行清单
- [x] `make orgunit plan && make orgunit lint && make orgunit migrate up`
- [x] `make sqlc-generate` 且 `git status --short` 无额外生成物漂移
- [x] `go fmt ./... && go vet ./... && make check lint && make test`
- [x] `make check doc`

### 9.3 验收结论条件
- [x] 任一 presence 违规由数据库层阻断。
- [x] `org_events` 入表即完整（before/after/rescind_outcome 已齐）。
- [x] 项目内只有一个重建算法实现。
- [x] 无长期豁免窗口、无 legacy 分支。

## 10. 运维与监控 (Ops & Monitoring)
- 按 `AGENTS.md` 3.6：当前阶段不新增运维开关或监控开关。
- 排障日志最小字段：`event_uuid`, `request_code`, `org_id`, `tenant_uuid`。
- 回滚策略：仅代码与迁移回滚（不引入运行时 dual-path fallback）。

## 11. 实施任务清单（可直接执行）

### 11.1 DDL/函数改造任务
1. [x] 在 `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` 新增 `is_org_event_snapshot_presence_valid(...)`。
2. [x] 更新 `assert_org_event_snapshots(...)` 参数与逻辑，调用新谓词。
3. [x] 在 `modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql` 增加 `rescind_outcome` 列及两个 CHECK。
4. [x] 在同 schema 文件把 `org_unit_versions.last_event_id` FK 调整为 `DEFERRABLE INITIALLY DEFERRED`。
5. [x] 在 `migrations/orgunit/*` 新增对应迁移并更新 `migrations/orgunit/atlas.sum`。

### 11.2 Kernel 写路径任务
6. [x] 改造 `submit_org_event`（含 allocator 场景）为单条 INSERT 写齐。
7. [x] 改造 `submit_org_event_correction` / `submit_org_status_correction` 为 pending 输入模式 + 单引擎重建。
8. [x] 改造 `submit_org_event_rescind` / `submit_org_rescind` 同步写入 `rescind_outcome`。
9. [x] 删除所有 `UPDATE orgunit.org_events SET before_snapshot=..., after_snapshot=...` 回写路径。

### 11.3 测试与证据任务
10. [x] 更新 `internal/server/orgunit_audit_snapshot_schema_test.go` 的防回归断言。
11. [x] 新增/补齐 presence 谓词矩阵测试与 rescind_outcome 场景测试。
12. [x] 在 `docs/dev-records/dev-plan-080-execution-log.md` 增加 080C 证据块（命中触发器、命令、结果）。

## 12. 关联文档
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
- `docs/dev-plans/080a-orgunit-audit-snapshot-mechanism-and-fix.md`
- `docs/archive/dev-plans/080b-orgunit-correction-failure-investigation-and-remediation.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- `docs/dev-plans/025-sqlc-guidelines.md`
- `AGENTS.md`
