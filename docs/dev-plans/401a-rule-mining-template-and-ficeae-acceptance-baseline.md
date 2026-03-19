# DEV-PLAN-401A：规则矿模板与 Ficeae 验收对照基线（首批 20 条）

**状态**: 进行中（2026-03-19 15:35 CST）

## 1. 背景与定位

本计划承接 [DEV-PLAN-401](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/401-phase0-ficeae-bootstrap-execution-plan.md) 的关键决策：

- 旧线（Go）只作为“规则矿”；
- `Ficeae` 只迁移“行为契约与验收样例”，不迁移实现代码。

`401A` 的职责是把“规则矿”做成可执行标准：

- 统一规则卡片模板；
- 统一证据分级与追溯方式；
- 冻结首批 20 条 `Ficeae` 必达规则基线。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 定义统一的“规则卡片（Rule Card）”模板。
- [ ] 定义“旧线证据 -> `Ficeae` 验收”映射流程与质量门槛。
- [ ] 输出首批 20 条可直接执行的规则基线。
- [ ] 为 `401` 的 Phase 0 验收提供可追溯输入。

### 2.2 非目标

- [ ] 本计划不迁移旧线业务实现代码或数据库对象。
- [ ] 本计划不替代 `300/310/320/330` 的上层架构决策。
- [ ] 本计划不定义 `Ficeae` 的最终技术细节（ORM、字段命名、表结构）。

## 3. Rule Card 模板（冻结）

每条规则必须有一张卡片，字段如下：

| 字段 | 说明 |
| --- | --- |
| `rule_id` | 规则唯一 ID（`RM-001` 起） |
| `rule_name` | 规则名称（业务语言） |
| `business_statement` | 一句话业务语义（用户可读） |
| `trigger` | 触发条件（输入/场景） |
| `expected` | 预期行为（成功或拒绝） |
| `error_contract` | 失败时错误码/错误语义要求 |
| `time_semantics` | 是否要求 `effective_date / as_of / history` |
| `security_boundary` | 租户/权限/审批边界 |
| `oldline_evidence` | 旧线证据（测试文件/文档） |
| `ficeae_test_plan` | `Ficeae` 落地测试类型（unit/integration/e2e） |
| `owner` | 责任模块/责任人 |
| `status` | `draft / verified / implemented / gated` |

示例：

```yaml
rule_id: RM-001
rule_name: 读取时间锚点显式化
business_statement: 任意业务读取必须显式给出时间锚点，不允许隐式 today
trigger: 查询 Org/Person/JobCatalog/Staffing 任一对象
expected: 成功返回 current/as_of/history 之一，且语义一致
error_contract: 缺失时间锚点时返回稳定错误码
time_semantics: required
security_boundary: tenant-scoped + authz enforced
oldline_evidence:
  - docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md
  - apps/web/src/pages/org/orgUnitFieldPolicyAsOf.test.ts
ficeae_test_plan: integration
owner: ficeae-core-hr
status: draft
```

## 4. 规则抽取与验收映射流程

1. [ ] 发现规则：从旧线 `dev-plan + test` 识别行为契约。
2. [ ] 规则归一：转写为 Rule Card，去除实现细节与技术名词噪声。
3. [ ] 证据分级：标记 `A/B/C` 证据等级（见第 5 节）。
4. [ ] `Ficeae` 对照：为每条规则指定 `Ficeae` 测试类型与 owner。
5. [ ] 自动化落地：规则至少对应 1 条自动化测试。
6. [ ] 门禁接线：规则从 `implemented` 升级到 `gated` 前，必须进入 CI 阻断链路。

## 5. 旧线证据分级

- [ ] `A 级`：有稳定自动化测试 + 明确文档契约（优先迁入 `Ficeae`）。
- [ ] `B 级`：有文档契约但测试样例不足（需先补样例再迁入 `Ficeae`）。
- [ ] `C 级`：只有实现痕迹无契约语义（默认不迁，需重新业务评审）。

## 6. 首批 20 条规则基线（Ficeae 必达）

| rule_id | 规则基线（业务语言） | Ficeae 最低验收 | 旧线证据（规则矿） |
| --- | --- | --- | --- |
| `RM-001` | 读取必须显式时间锚点（`current/as_of/history`），禁止隐式 today | integration | `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`、`apps/web/src/pages/org/orgUnitFieldPolicyAsOf.test.ts` |
| `RM-002` | 写入必须显式 `effective_date`，缺失即拒绝 | integration | `apps/web/src/pages/org/orgUnitRecordDateRules.test.ts`、`docs/dev-plans/032-effective-date-day-granularity.md` |
| `RM-003` | Assignment 同主体同时间不得重叠激活 | integration | `modules/staffing/infrastructure/persistence/assignment_pg_store_test.go`、`docs/dev-plans/364-staffing-position-assignment-business-rules-and-detailed-design.md` |
| `RM-004` | 跨租户访问必须 fail-closed，不能靠默认过滤“碰运气” | integration | `internal/server/tenancy_middleware_test.go`、`docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md` |
| `RM-005` | 授权拒绝必须返回稳定错误语义（403 + 可诊断） | unit + integration | `internal/server/authz_middleware_test.go`、`docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md` |
| `RM-006` | 路由到 capability 映射必须单点注册、无重复、无缺失 | unit | `internal/server/capability_route_registry_test.go`、`scripts/ci/check-capability-route-map.sh` |
| `RM-007` | 幂等请求必须可重试且不产生重复副作用 | integration | `docs/dev-plans/109a-request-code-total-convergence-and-anti-drift.md`、`internal/server/assistant_tasks_api_test.go` |
| `RM-008` | 错误返回必须“稳定错误码 + 明确提示”，禁止泛化失败文案直出 | unit + e2e | `internal/server/test_errors_test.go`、`apps/web/src/errors/presentApiError.test.ts`、`docs/dev-plans/140-error-message-clarity-and-gates.md` |
| `RM-009` | OrgUnit 更正链路必须保持时间语义一致，不允许隐式改历史 | integration | `internal/server/orgunit_effective_date_sticky_sql_test.go`、`apps/web/src/pages/org/orgUnitCorrectionIntent.test.ts` |
| `RM-010` | OrgUnit 字段策略决议必须可解释且支持 `as_of` 一致读取 | integration | `internal/server/orgunit_field_policy_api_test.go`、`apps/web/src/pages/org/orgUnitFieldPolicyAsOf.test.ts` |
| `RM-011` | Job Catalog 是组合分类体系，不得退化成单树语义 | integration + e2e | `internal/server/jobcatalog_test.go`、`docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md` |
| `RM-012` | JobProfile 必须满足“至少一个 Family 且恰好一个主 Family” | integration | `docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md`、`internal/server/jobcatalog_api_test.go` |
| `RM-013` | 未知字段/非法 payload 必须 fail-closed | unit + integration | `internal/server/orgunit_ext_payload_schema_test.go`、`internal/server/orgunit_field_metadata_validation_test.go` |
| `RM-014` | UI 字段可编辑性由后端决议驱动，前端不得自行猜测 | e2e | `internal/server/orgunit_mutation_capabilities_api_test.go`、`apps/web/src/pages/org/orgUnitWritePatch.test.ts`、`docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md` |
| `RM-015` | 关键写操作必须可审计并回显 before/after 证据 | integration | `internal/server/orgunit_audit_snapshot_schema_test.go`、`docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md` |
| `RM-016` | Assistant 写动作必须经过 `dry-run -> confirm -> commit`，禁止旁路提交 | integration + e2e | `internal/server/assistant_action_interceptor_test.go`、`internal/server/assistant_api_243_gap_test.go`、`docs/dev-plans/390-chat-assistant-capability-plan.md` |
| `RM-017` | Assistant 必须复用操作者同源授权，不得拥有并行权限体系 | integration | `internal/server/assistant_240c_coverage_test.go`、`docs/dev-plans/390-chat-assistant-capability-plan.md` |
| `RM-018` | Assistant 长任务必须有可轮询状态票据与回执 | integration | `internal/server/assistant_tasks_api_test.go`、`docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md` |
| `RM-019` | 路由治理必须按 allowlist + route_class + responder 合同执行 | unit | `internal/routing/allowlist_test.go`、`internal/routing/gates_test.go`、`docs/dev-plans/346-platform-routing-governance-and-response-contract-plan.md` |
| `RM-020` | 页面必须维持“列表/详情/历史 + 上下文锚点”统一交互语义 | e2e | `apps/web/src/pages/org/orgUnitVersionSelection.test.ts`、`docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md` |

## 7. 实施步骤

1. [ ] 在 `Ficeae` 建立 `rule-cards/` 目录，按 `rule_id` 一卡一文件管理。
2. [ ] 把本计划第 6 节 20 条规则转为结构化卡片。
3. [ ] 为每条规则指定 `Ficeae` owner 与落地测试类型。
4. [ ] 将规则状态推进到 `implemented`，并接入 CI。
5. [ ] 在 `402` 启动前完成至少 20 条中的 12 条 `gated`。

## 8. 验收标准

- [ ] 规则卡片模板已冻结，团队统一使用。
- [ ] 首批 20 条规则均可追溯到旧线证据路径。
- [ ] 每条规则都有 `Ficeae` 对应测试计划与 owner。
- [ ] 至少 12 条规则进入 `Ficeae` CI 门禁，失败可阻断合并。

## 9. 关联文档

- [DEV-PLAN-401](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/401-phase0-ficeae-bootstrap-execution-plan.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-400](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/400-implementation-roadmap-and-vertical-slice-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
- [DEV-PLAN-395](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/395-assistant-surface-registry-and-enforcement-gates-detailed-design.md)
