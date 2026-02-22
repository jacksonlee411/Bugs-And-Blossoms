# [Archived] DEV-PLAN-080D：OrgUnit 变更日志“已撤销事件未标识”专项调查与收敛方案

**状态**: 已归档（2026-02-22，审计可读性契约已并入 `DEV-PLAN-080`；本文仅保留专项调查与收敛记录）

## 1. 背景
- 触发页面：`http://localhost:8080/org/nodes?tree_as_of=2026-02-01`。
- 用户反馈：变更日志显示 `RENAME（组织更名）`，字段变更为 `财务部6 -> AI治理办公室`，但组织名称实际并未变成 `AI治理办公室`。
- 目标：确认这是“写入失败/数据错乱”还是“审计展示口径导致误解”，并形成可执行收敛方案。

## 2. 现象复述（用户侧）
- 关注事件：
  - `event_uuid=4aaf4666-c4ef-40ef-bc67-b023d8452d85`
  - `event_type=RENAME`
  - `effective_date=2026-01-04`
  - `request_code=manual:verify:rescind-slot-free:1770861730`
- 页面中显示了完整 diff（含 `name/full_name_path/validity`），但当前业务结果不一致。

## 3. 调查范围与方法
1. 代码链路核查：`变更日志读取 SQL -> 详情渲染`。
2. 数据链路核查：
   - `orgunit.org_events`（原始审计事实）
   - `orgunit.org_events_effective`（业务生效事实）
   - `orgunit.org_unit_versions`（当前版本结果）
3. 关键假设验证：目标 `RENAME` 是否被后续 `RESCIND_*` 事件撤销。

## 4. 证据清单（Evidence）

### E1. 目标 RENAME 事件真实存在，且快照完整
- 数据库记录确认该事件写入成功，`before_snapshot/after_snapshot` 都是有效业务快照：
  - `before_snapshot.name=财务部6`
  - `after_snapshot.name=AI治理办公室`
- 结论：不是“RENAME 写失败”，也不是快照缺失。

### E2. 该 RENAME 已被 RESCIND_EVENT 撤销
- 命中撤销事件：
  - `event_uuid=f0b25c9f-b575-406d-ab8f-78181b9b1b14`
  - `event_type=RESCIND_EVENT`
  - `payload.target_event_uuid=4aaf4666-c4ef-40ef-bc67-b023d8452d85`
- 结论：该 rename 仅是“历史发生过”，不是“当前生效中”。

### E3. 生效视图已过滤该 RENAME（业务口径正确）
- 在 `orgunit.org_events_effective` 中查询该 `event_uuid`，结果为 `0` 行。
- 结论：生效链路已正确执行“撤销优先”，符合 075C/080 约束。

### E4. 当前版本结果与“已撤销”一致
- `orgunit.org_unit_versions` 在 `as_of=2026-02-01` 对应 `org_id=10000004` 的名称为 `财务部5`（非 `AI治理办公室`）。
- 结论：业务结果正确，问题是“变更日志可读性”而非“写模型错误”。

### E5. 变更日志当前读取的是 org_events 全量链路
- `internal/server/orgunit_nodes.go:1455`~`internal/server/orgunit_nodes.go:1474`：`ListNodeAuditEvents` 直接查 `orgunit.org_events`，按 `tx_time DESC` 排序。
- `internal/server/orgunit_nodes.go:3709`~`internal/server/orgunit_nodes.go:3798`：详情渲染仅按 `before/after` 生成 diff，不显示“该事件是否已被撤销”的状态。
- 结论：当前 UI 会展示“已撤销历史事件”，但没有标识，容易被理解为“仍有效”。

### E6. 设计契约允许“审计全量可见”
- `orgunit.org_events_effective` 明确排除被 `RESCIND_EVENT/RESCIND_ORG` 作用的基础事件：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql:85`~`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql:140`。
- 因此：
  - `org_events` = 审计事实（发生过）
  - `org_events_effective` = 业务事实（当前有效）

### E7. 线上样本存在“撤销事件快照信息不足”
- 本次租户样本中，存在 `RESCIND_EVENT` 记录仅保留 `payload.target_event_uuid`，且 `before_snapshot/after_snapshot` 为 `{}` 空对象：
  - `event_uuid=f0b25c9f-b575-406d-ab8f-78181b9b1b14`
  - `request_code=manual:cleanup:rescind-814:1770862482`
- 与同租户另一条标准 `RESCIND_EVENT` 相比，该样本缺失 `op/reason/target_effective_date` 与完整业务快照。
- 结论：除“已撤销标识”外，还需收敛“撤销/删除事件必须携带完整前后快照”的写入契约与历史补齐策略。

## 5. 根因结论（Root Cause）
1. **不是数据错误**：RENAME 曾经生效过，但随后被撤销。
2. **是展示语义缺口**：变更日志显示全量审计事件，却没有明确标识“该事件已被后续撤销”。
3. **导致误解**：用户将“曾发生的历史变更”误读为“当前仍有效的变更”。

## 6. 影响范围评估
- 所有 OrgUnit 变更日志详情页都会受影响，尤其在存在 `RESCIND_EVENT/RESCIND_ORG` 的组织上。
- 影响类型：
  - 认知误导（高频）
  - 业务数据错误（无）
  - 审计链完整性（无损）

## 7. 收敛方案建议（不引入 legacy/双链路）

### 7.0 已确认产品决策（2026-02-12）
- 保持**全量审计视图**，不引入“仅看生效事件”筛选开关。
- 目标是“完整保留历史事实 + 显式表达生效状态”，而不是裁剪历史事件。

### 7.1 P0（建议优先）：全量审计保留 + “已撤销”显式标识
- 在变更日志项中增加状态标签：`已撤销`。
- 展示信息至少包含：
  - 撤销事件 UUID
  - 撤销事务时间
  - 撤销请求编号（若有）
- 原则：不隐藏历史事实，只消除“是否仍生效”的歧义。

### 7.2 P0（同优先级）：撤销/删除事件完整快照收敛

#### A. 写入契约（新写入必须满足）
1. `RESCIND_EVENT/RESCIND_ORG` 必须写齐：
   - `before_snapshot`：撤销前（或删除前）完整业务快照。
   - `after_snapshot`：撤销后业务快照（`rescind_outcome=PRESENT`）或 `NULL`（`rescind_outcome=ABSENT`）。
2. `payload` 至少包含：`op/reason/target_event_uuid/target_effective_date`。
3. 禁止通过手工 SQL 写“仅 target_event_uuid 的简化撤销事件”。

#### B. 数据库约束（防止再次退化）
1. 在现有 presence 约束基础上增加**快照内容完整性谓词**（示意命名：`orgunit.is_org_event_snapshot_content_valid(...)`）。
2. 对 `RESCIND_*` 强制 `before_snapshot` 至少包含关键字段（建议最小集）：
   - `org_id/name/status/parent_id/node_path/validity/full_name_path/is_business_unit`
3. 对 `payload` 增加键完整性校验：`op/reason/target_event_uuid/target_effective_date` 全部必填。

#### C. UI 展示（满足“可读到完整删除前快照”）
1. 在 `RESCIND_*` 详情右侧新增固定区块：`撤销前完整快照`（JSON 折叠 + 关键字段摘要卡）。
2. `rescind_outcome=PRESENT` 时新增 `撤销后快照` 区块；`ABSENT` 时显示“撤销后已不存在”。
3. 保留现有字段 diff，但不再让用户必须从 diff 推断“删除前到底是什么”。

#### D. 历史数据补齐（一次性治理）
1. 对历史 `RESCIND_*` 中 `before_snapshot={}::jsonb` 或 payload 键缺失的数据做补齐迁移。
2. 补齐优先策略：
   - 优先按 `target_event_uuid + target_effective_date` 通过重建函数回放生成快照；
   - 无法精确重建时，落“可追溯补齐标记”（例如 `snapshot_source=fallback`），并在审计页显式标注。
3. 补齐完成后再启用严格 CHECK，避免线上写入被历史脏数据阻断。

### 7.3 P1（测试补齐）
- 新增用例：
  1) 基础事件被撤销后，变更日志卡片显示“已撤销”。
  2) `RESCIND_*` 详情必须展示“撤销前完整快照”，且字段完整。
  3) 存在 `before_snapshot={}` 的历史样本可被补齐并通过新约束。
  4) 跳转目标事件仍可用，不因“已撤销”状态失效。

## 8. 验收标准（Acceptance）
1. 对已撤销的 `RENAME/MOVE/DISABLE/ENABLE/SET_BUSINESS_UNIT`，日志详情页必须出现“已撤销”标识。
2. 用户在 `tree_as_of` 任意日期查看时，不再把“已撤销历史变更”误认为“当前生效变更”。
3. 对 `RESCIND_EVENT/RESCIND_ORG`，可直接在详情页查看“删除前/撤销前完整快照”，不依赖外部 SQL 排查。
4. 不改变现有审计链完整性：原始事件仍可追溯、可跳转、可对账。
5. 不引入 legacy 回退分支；保持单链路实现。

## 9. 拟落地清单（后续 PR）
1. [x] 增强 `ListNodeAuditEvents`：附带“是否被撤销 + 撤销事件元数据”。
2. [x] 增强 `renderOrgNodeAuditDetailEntry`：渲染“已撤销”状态块与“撤销前完整快照”。
3. [x] DB 侧补齐并收紧 `RESCIND_*` 的 payload/快照内容约束（禁止 `{} + target_event_uuid-only`）。
4. [x] 增补历史数据修复迁移，清理已存在的快照信息不足样本。
5. [x] 补齐 `internal/server/orgunit_nodes_audit_test.go` 与 DB 迁移测试覆盖。
6. [x] 更新 `docs/dev-records/dev-plan-080-execution-log.md` 记录落地证据。

## 10. 关联文档
- `docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
- `docs/dev-plans/080a-orgunit-audit-snapshot-mechanism-and-fix.md`
- `docs/dev-plans/080c-orgunit-audit-snapshot-presence-table-constraint-plan.md`
- `docs/dev-records/dev-plan-080-execution-log.md`
