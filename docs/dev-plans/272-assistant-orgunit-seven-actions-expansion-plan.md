# DEV-PLAN-272：Assistant OrgUnit 七动作纳管实施计划（1-7 全量纳入）

**状态**: 已完成（2026-03-11 08:21 CST，`PR #481` 已合并；`PR-272-04/05`、`288/290B` 新鲜证据、`make preflight` 与本地 `8080/8081/librechat` 重启复核均已完成）

## 1. 背景与问题
1. [ ] 当前 Assistant 提交主链已完成 `240A~240D` 的结构化收敛，但运行时可稳定提交的业务动作仍以 `create_orgunit` 为主。
2. [ ] 已确认需纳入的七类动作为：
   - [ ] `add_orgunit_version`
   - [ ] `insert_orgunit_version`
   - [ ] `correct_orgunit`
   - [ ] `disable_orgunit`
   - [ ] `enable_orgunit`
   - [ ] `move_orgunit`
   - [ ] `rename_orgunit`
3. [ ] 若继续维持单动作提交，会导致 `240` 主计划“至少 3 类动作闭环”目标长期无法完整收敛。
4. [ ] 本计划作为 `240` 主线的专项实施计划，专门解决“七动作进入同一条 Assistant 正式执行链”的契约、接线、测试与证据问题。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 将以上 1-7 七个 OrgUnit 动作全部纳入 Assistant 正式动作注册表与提交链路。
2. [ ] 保持单链路不变量：`ActionRegistry -> ActionInterceptor -> confirm/commit state machine -> CommitAdapter -> One Door`。
3. [ ] 七个动作统一满足 `No Spec, No Plan`、`No Gate Pass, No Commit`、`No Adapter, No Write`。
4. [ ] 为 `260/271/285` 提供可复核的七动作真实回归证据，不再依赖“仅 create 场景”代表整体能力。

### 2.2 非目标
1. [ ] 不在本计划引入新数据库表或新迁移；若实施中发现必须新增表结构，需先获得用户确认。
2. [ ] 不在本计划新增跨模块业务动作（如 staffing/person）；本期仅聚焦 OrgUnit 七动作。
3. [ ] 不在本计划重写 `260` 业务 FSM 基础语义；仅在既有 FSM 下扩展动作覆盖面。
4. [ ] 不在本计划新增第二提交入口或 legacy 回退路径。

## 3. 范围冻结（七动作清单）
1. [ ] `add_orgunit_version`
   - [ ] 写意图映射：`add_version`
   - [ ] capability：`org.orgunit_add_version.field_policy`
2. [ ] `insert_orgunit_version`
   - [ ] 写意图映射：`insert_version`
   - [ ] capability：`org.orgunit_insert_version.field_policy`
3. [ ] `correct_orgunit`
   - [ ] 写意图映射：`correct`
   - [ ] capability：`org.orgunit_correct.field_policy`
4. [ ] `disable_orgunit`
   - [ ] 写服务入口：`Disable(...)`
   - [ ] capability：沿用 orgunit 既有有效能力键映射，不新增旁路
5. [ ] `enable_orgunit`
   - [ ] 写服务入口：`Enable(...)`
   - [ ] capability：沿用 orgunit 既有有效能力键映射，不新增旁路
6. [ ] `move_orgunit`
   - [ ] 写服务入口：`Move(...)`
   - [ ] capability：沿用 orgunit 既有有效能力键映射，不新增旁路
7. [ ] `rename_orgunit`
   - [ ] 写服务入口：`Rename(...)`
   - [ ] capability：沿用 orgunit 既有有效能力键映射，不新增旁路

## 4. 设计与不变量
1. [ ] `assistantActionSpec.Security` 继续作为 `auth_object/auth_action/risk_tier/required_checks` 单主源，不得新增第二主源。
2. [ ] 七动作均必须显式注册 `CommitAdapterKey`，未注册一律 fail-closed。
3. [ ] `plan/confirm/commit` 三阶段统一走 `assistantEvaluateActionGate(...)`，禁止动作级旁路判断。
4. [ ] 继续复用 `240D` 的 `:commit -> receipt -> task poll -> conversation refresh` 主链，不得回退同步直提。
5. [ ] 动作扩展后，内存路径与 PG 路径的错误码/HTTP/是否落写结果必须一致。

## 5. 分批实施（PR 切片）

### 5.1 PR-272-01：契约与注册表扩展（无行为切换）
1. [x] 扩展 `assistantActionRegistry` 默认注册，纳入七动作完整 spec。
2. [x] 每个动作补齐 `PlanTitle/PlanSummary/CapabilityKey/Security/Handler`。
3. [x] 保持现有 create 行为不变，先确保新增 spec 不影响既有链路。

### 5.2 PR-272-02：CommitAdapter 全量接线
1. [x] 新增并注册七动作对应 adapter（复用 `orgunitservices.OrgUnitWriteService` 既有入口）。
2. [x] 适配各动作最小输入载荷与 request_id/initiator 透传。
3. [x] 明确 unsupported/invalid payload 的稳定错误码映射。

### 5.3 PR-272-03：Intent 编译与字段校验扩展
1. [x] 扩展 intent decode/normalize 与 compile 逻辑，覆盖七动作最小必填字段。
2. [x] 按动作输出 `skill_execution_plan` 与 `config_delta_plan`，不再隐式回落到 create-only 分支。
3. [x] 缺字段、候选冲突、日期格式错误等失败路径统一落入现有错误契约。

### 5.4 PR-272-04：Gate 规则按动作收敛
1. [x] 对 `required_checks` 做动作级最小化配置（如 create/move 可能需要候选确认，其余动作按需启用）。
2. [x] 删除新增过程中的散点判断，确保只剩 interceptor 主源。
3. [x] 校准 confirm/commit 阶段拒绝时的 `reason_code` 与 `turn.error_code` 一致性。

### 5.5 PR-272-05：回归与证据封板
1. [x] 补齐七动作 API/PG/任务执行回归测试与 E2E 证据。
2. [x] 按 `271-S5` 新鲜度规则刷新 `288/290` 受影响证据。
3. [x] 在 `docs/dev-records/` 新增或更新 `dev-plan-272-execution-log.md`。

### 5.6 下一步实施策略（2026-03-10 评估回写）
1. [x] 实施顺序冻结为：**先关 `PR-272-04`，再关 `PR-272-05`，最后统一刷新跨计划 live 证据**。
2. [x] `PR-272-04` 当前主目标不是继续扩动作，而是把“动作级 gate 规则已接入”固化为**可复核测试矩阵**：
   - [x] `assistant_action_interceptor_test.go`：按七动作补齐 `plan/confirm/commit` 允许与拒绝分支，覆盖 `required_checks` 最小化配置。
   - [x] `assistant_api_test.go` / `assistant_api_coverage_test.go`：补齐 `reason_code / turn.error_code / HTTP` 映射断言，防止语义漂移。
   - [x] `assistant_persistence_gap_test.go`：补齐 PG 路径下 gate 拒绝不写门、不建 task、不误推进状态的断言。
3. [x] `PR-272-05` 执行顺序冻结为“**后端主证据优先，live 证据后置**”：
   - [x] 先补七动作 API/PG 集成回归：每个动作至少 1 条成功样例 + 1 条关键拒绝路径（权限/校验/状态漂移之一）。
   - [x] 再补 `:commit -> receipt -> task poll -> conversation refresh` 的异步终态一致性断言，避免只证明 task 成功、不证明会话终态。
   - [x] 仅在服务端回归冻结后，统一重跑 `288 + 290B` 关键用例并刷新索引，满足 `271-S5` 证据新鲜度。
4. [x] 跨计划证据策略冻结为：`272` 若继续影响运行时 gate、错误码语义、Resolver 行为或 fail-closed 行为，则 `288/290B` 历史证据视为待刷新，不得提前用于 `271-S5/285` 封板判定。
5. [x] 本计划当前完成度判断更新为：**七动作正式链路、后端主证据、`288/290B` 新鲜证据、`make preflight` 与重启后 live 复核均已收口，并已通过 `PR #481` 合并入 `main`**。

### 5.7 运行时补充收口（2026-03-11）
1. [x] `create_orgunit` 的 dry-run 不能只停留在基础意图字段校验；当父组织已唯一解析后，必须前置校验创建字段策略与租户字段启用状态。
2. [x] 若 `org_code` / `d_org_type` 在创建策略决议后仍无法得到可提交值，`createTurn` 阶段必须直接回填 `FIELD_REQUIRED_VALUE_MISSING`，不得延迟到 commit 才失败。
3. [x] 若 `d_org_type` 依赖默认值落写但租户未启用对应字段配置，`createTurn` 或“候选确认后再次 dry-run”阶段必须直接回填 `PATCH_FIELD_NOT_ALLOWED`。
4. [x] 上述错误必须复用现有 `dry_run.validation_errors -> await_missing_fields -> confirm/commit blocker` 主链，不新增第二套阻断机制。

## 6. 测试与覆盖率
1. [x] 覆盖率口径：沿用仓库 CI 既有口径与阈值，不新增排除项规避。
2. [x] 单测最小集：
   - [x] `assistant_action_registry_test.go`：七动作注册完整性
   - [x] `assistant_action_interceptor_test.go`：七动作 gate 分支
   - [x] `assistant_api_test.go` / `assistant_api_coverage_test.go`：plan/confirm/commit 错误码映射
   - [x] `assistant_persistence_gap_test.go`：PG 路径下 gate 拒绝不写门
3. [x] 集成/E2E 最小集：
   - [x] 七动作至少各 1 条 `plan -> confirm -> commit` 成功样例
   - [x] 七动作至少各 1 条关键拒绝路径（权限/校验/状态漂移之一）
   - [x] `:commit` 异步 receipt 链路下任务终态与 conversation 刷新一致
4. [x] 运行时补充回归：
   - [x] `create_orgunit` 在 `createTurn` 阶段命中“必填字段无默认值”时，直接返回 `FIELD_REQUIRED_VALUE_MISSING`
   - [x] `create_orgunit` 在候选确认后命中“`d_org_type` 未启用”时，直接返回 `PATCH_FIELD_NOT_ALLOWED`，且 turn 不推进到 confirmed

## 7. 验收标准（DoD）
1. [x] 七动作均可在正式入口完成受控提交，不再出现“只有 create 能提交”。
2. [x] 七动作全部通过统一拦截链，且不存在 bypass `ActionInterceptor` 的路径。
3. [x] 任一动作 `spec/capability/authz/risk/required_checks` 失败时均 fail-closed，不触发写门。
4. [x] 七动作在内存与 PG 路径的错误码/HTTP 语义一致。
5. [x] `240C/240D` 已有不变量无回退，`271-S5` 证据新鲜度要求满足。

## 8. 停止线（Fail-Closed）
1. [ ] 若任一动作存在“无 spec 仍可 plan”或“无 adapter 仍可 commit”，计划失败。
2. [ ] 若新增动作引入第二主源（risk/required_checks/capability 散点判断回流），计划失败。
3. [ ] 若任务执行链绕过统一 gate 或绕过 One Door，计划失败。
4. [ ] 若新增动作后出现内存/PG 结果漂移（错误码、HTTP、写入结果不一致），计划失败。
5. [ ] 若影响性合入后未刷新 `288/290` 关键证据却推进封板，计划失败。

## 9. 交付物
1. [x] 计划文档：`docs/dev-plans/272-assistant-orgunit-seven-actions-expansion-plan.md`
2. [ ] 代码改动：
   - [x] `internal/server/assistant_action_registry.go`
   - [x] `internal/server/assistant_action_interceptor.go`
   - [x] `internal/server/assistant_intent_pipeline.go`
   - [x] `internal/server/assistant_api.go`
   - [x] `internal/server/assistant_persistence.go`
3. [x] 测试改动：`internal/server/*assistant*_test.go` 邻近文件
4. [x] 执行证据：`docs/dev-records/dev-plan-272-execution-log.md`

## 10. 门禁与命令（SSOT 引用）
1. [x] 基础门禁：`go fmt ./... && go vet ./... && make check lint && make test`
2. [x] 质量门禁：`make check routing && make check capability-route-map && make check error-message`
3. [x] 权限门禁：`make authz-pack && make authz-test && make authz-lint`
4. [x] 文档门禁：`make check doc`
5. [x] 合并前建议：`make preflight`

## 11. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/240c-assistant-action-interceptor-and-risk-gate-plan.md`
- `docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md`
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `AGENTS.md`
