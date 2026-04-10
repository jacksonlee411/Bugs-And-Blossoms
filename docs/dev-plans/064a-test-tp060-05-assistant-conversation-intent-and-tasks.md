# DEV-PLAN-064A：全链路业务测试子计划 TP-060-05——Assistant（会话 + 意图 + 提交 + 任务编排）

**状态**: 已完成（2026-03-03 02:06 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`（租户/登录/隔离基线）。  
> 本文按 `docs/dev-plans/000-docs-format.md` 组织，聚焦 Assistant 端到端业务验收，不复制工具链细节（SSOT：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`）。

## 1. 背景与上下文（Context）

- `DEV-PLAN-221` 与 `DEV-PLAN-222` 已完成 Assistant P1 收口与 FE/E2E 证据闭环，但 `DEV-PLAN-060` 之前未纳入 Assistant 子套件。  
- `DEV-PLAN-224` 正在实施多模型与意图治理，`DEV-PLAN-225` 规划 Tasks API + Temporal（P2）；需要统一纳入 TP-060 主套件，避免“功能已交付但主套件缺失”的漂移。  
- 为避免与既有 `tp060-04`（OrgUnit 双栏回归）编号冲突，本子计划编号固定为 `TP-060-05`。  
- 对齐 TG-004（`docs/dev-plans/226-test-guide-tg004-gate-caliber-change-approval.md`）：功能交付后必须同步更新 `DEV-PLAN-060` 套件。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

1. [X] 页面入口可发现：`/app/assistant` 可进入，未登录/跨租户访问仍 fail-closed。  
2. [X] 会话主链路闭环：会话创建、详情查询、turn 创建、confirm、commit 可复现。  
3. [X] 状态机闭环：`validated -> confirmed -> committed` 成功，非法迁移稳定 fail-closed。  
4. [X] 意图治理闭环：候选确认后不可静默改写；版本漂移可回退且返回稳定错误码。  
5. [X] 多模型治理闭环：provider validate/apply 正负例稳定，错误码与用户提示可审计。  
6. [X] 任务编排闭环按“启用门槛”执行（见 §2.3）：启用后 Tasks/Temporal 成为强制验收项。

### 2.2 非目标

- 不做性能/压测与容量评估（对齐 `AGENTS.md` §3.6）。
- 不扩展业务契约：若遇到“有实现无契约”，登记 `CONTRACT_MISSING` 后回到对应 dev-plan 处理。

### 2.3 Tasks/Temporal 启用门槛（强制）

1. [X] **未启用阶段（可暂不阻断 TP-060-05 完成）**  
   满足以下全部条件时，可把 Tasks/Temporal 作为观察项，仅记录问题不阻断：
   - `config/routing/allowlist.yaml` 尚未注册 `/internal/assistant/tasks*`；
   - `DEV-PLAN-225` 仍处于规划态，且仓内无 `internal/server/assistant_tasks_api.go`。
2. [ ] **启用触发（任一满足即转为强制）**
   - allowlist 注册了 `/internal/assistant/tasks`、`/internal/assistant/tasks/{task_id}`、`/internal/assistant/tasks/{task_id}:cancel`；
   - 或 `config/capability/route-capability-map.v1.json` 出现上述 tasks 路由映射；
   - 或 `internal/server/assistant_tasks_api.go` 已落地。
3. [ ] **启用后强制要求**  
   触发后，§5.5 与 §6.2 必须通过；未通过时 `TP-060-05` 不得标记完成。

### 2.5 当前判定（2026-03-03 01:20 UTC）

- [X] 当前处于“未启用阶段”：仓内未发现 `/internal/assistant/tasks*` allowlist/capability 映射，且 `internal/server/assistant_tasks_api.go` 不存在。
- [X] 启用触发条件未满足，§5.5/§6.2 暂作为观察项，不阻断本轮 TP-060-05。

### 2.4 工具链与门禁（SSOT 引用）

- 触发器清单（执行记录在 dev-record 或 PR 留证）：
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] 路由治理（`make check routing`；涉及 allowlist/route_class/responder 时）
  - [X] E2E（`make e2e`；重点用例 `e2e/tests/tp220-assistant.spec.js`；2026-03-03 13/13 全通过）
  - [X] 文档（`make check doc`）
  - [ ] 覆盖率口径（如需改 `config/coverage/policy.yaml`，必须先获你确认，遵循 `docs/dev-plans/226-test-guide-tg004-gate-caliber-change-approval.md`）

## 3. 契约引用（SSOT）

- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
- `docs/archive/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- `docs/dev-plans/226-test-guide-tg004-gate-caliber-change-approval.md`

## 4. 前置条件与数据准备

- 环境口径承接 `DEV-PLAN-060/012/226`：若本子计划复用 TP-060 数据 seed、联动全量 `make e2e`，或需要数据库直连/校验，必须默认 PostgreSQL 运行于 Docker / compose 并优先使用容器内工具链；不得把宿主机 `psql` 缺失误判为 Assistant 业务失败。

1. [ ] 复用 TP-060-01 基线：`T060` 可登录、RLS/Authz enforce、跨租户 fail-closed 已验证。  
2. [ ] 测试账号：`tenant-admin@example.invalid`（至少 1 个可写账号）。  
3. [ ] 固定业务日期：`as_of=2026-01-01`（避免执行期漂移）。  
4. [ ] Assistant 最小数据：
   - 1 条新建会话（conversation）；
   - 2 条 turns（至少 1 条需要 confirm 的候选）；
   - 1 条非法状态迁移负例；
   - 1 条版本漂移负例（contract version mismatch）。

## 5. 路由与接口契约（可执行口径）

### 5.1 当前已注册入口（依据 allowlist）

| 场景 | Method | Path | 期望（最小） |
| --- | --- | --- | --- |
| Assistant 页面入口 | GET | `/app/assistant?as_of=2026-01-01` | 已登录 200；未登录 302 到 `/app/login` |
| 创建会话 | POST | `/internal/assistant/conversations` | 200；失败码含 `bad_json`/`unauthorized`/`assistant_conversation_create_failed` |
| 会话详情 | GET | `/internal/assistant/conversations/{conversation_id}` | 200；不存在 404 `conversation_not_found` |
| 创建 turn | POST | `/internal/assistant/conversations/{conversation_id}/turns` | 200；空输入 422 `invalid_request`；schema/边界违约 422 `ai_plan_*` |
| turn 动作 | POST | `/internal/assistant/conversations/{conversation_id}/turns/{turn_id}:{turn_action}` | `confirm/commit` 成功 200；非法迁移 409 `conversation_state_invalid` |
| 模型提供方查询 | GET | `/internal/assistant/model-providers` | 200 |
| 模型配置校验 | POST | `/internal/assistant/model-providers:validate` | 200（`valid=true/false`） |
| 模型配置应用 | POST | `/internal/assistant/model-providers:apply` | 200；校验失败 422 |
| 模型列表 | GET | `/internal/assistant/models` | 200 |

补充用户可见契约：

- Assistant 任一涉及组织引用的提示词、候选展示、确认文案、提交日志与页面回显，只允许出现 `org_code`。
- 不得在用户可见文本、页面 DOM、调试日志或对外响应中泄露 `org_id` / `org_unit_id` / `org_node_key`。

### 5.2 状态机与错误码（最小闭集）

1. [X] 成功链路：`draft/proposed/validated -> confirmed -> committed`。  
2. [X] 负例链路：终态或逆向迁移返回 `409 conversation_state_invalid`。  
3. [X] 候选缺失返回 `422 assistant_candidate_not_found`。  
4. [X] 版本漂移返回 `409 ai_plan_contract_version_mismatch`。  
5. [X] 授权漂移返回 `403 ai_actor_role_drift_detected`（或同语义稳定码）。

### 5.3 FE E2E 合同（现有自动化映射）

1. [X] `e2e/tests/tp220-assistant.spec.js` 至少覆盖：
   - 正常生成/确认/提交；
   - 高风险阻断；
   - 候选确认后提交；
   - 角色漂移导致提交失败。
2. [X] `data-testid` 锚点稳定：`assistant-generate-button`、`assistant-confirm-button`、`assistant-commit-button`、`assistant-turn-state`、`assistant-error-alert`。

### 5.4 多模型治理合同

1. [X] `model-providers:validate` 正负例可复现（`valid=true/false`、`errors[]` 可审计）。  
2. [X] `model-providers:apply` 失败 422；成功后 `applied_at/applied_by/normalized` 可见。  
3. [X] `GET /internal/assistant/models` 与 apply 后配置一致。

### 5.5 Tasks/Temporal 合同（启用后强制）

> 本节仅在 §2.3 触发条件满足后转为强制。

1. [ ] `POST /internal/assistant/tasks`：同 `(tenant, conversation, turn, request_id)` 同 payload 幂等返回同 `task_id`。  
2. [ ] `GET /internal/assistant/tasks/{task_id}`：可查询 `status/dispatch_status/workflow_id/request_id/trace_id`。  
3. [ ] `POST /internal/assistant/tasks/{task_id}:cancel`：`pending` 可取消；已终态不回写覆盖。  
4. [ ] 异常稳定码（最小）：`assistant_task_workflow_unavailable`、`assistant_task_dispatch_failed`。

## 6. 验收证据（最小集）

### 6.1 必需证据（当前阶段）

1. [X] `make e2e` 通过，且包含 `e2e/tests/tp220-assistant.spec.js` 执行记录。  
2. [X] API 证据：create/turn/confirm/commit 各 1 条成功样例 + 1 条失败样例（含状态码与错误码）。  
3. [X] UI 证据：`/app/assistant` 入口可见、会话详情可见、状态变化可见，且组织引用仅展示 `org_code`。  
4. [X] 多模型证据：validate/apply 正负例各 1 条。

### 6.2 启用后追加证据（Tasks/Temporal）

1. [ ] tasks submit/get/cancel 三条 API 证据（含 request_id/trace_id）。  
2. [ ] workflow/dispatch 证据：`workflow_id` 可追踪，失败可回放。  
3. [ ] 幂等证据：同键同载荷不重复派发。

## 7. 问题记录（执行时必填）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-03-03 01:20 | localhost / 2026-01-01 / `make e2e` | 执行全量 E2E（含 `tp220-assistant`） | `make e2e` 通过且 TP-060-05 证据可留存 | `tp220-assistant` 全部通过（5/5），但全套件因 `tp060-02-orgunit-ext-query` 的 `row bounding box missing` 失败（12/13） | P1 | BUG | 先修复/稳定 `tp060-02-orgunit-ext-query` 后复跑 `make e2e`；Assistant API/多模型实链路证据继续补齐 | 待指派 | `docs/archive/dev-records/dev-plan-064a-execution-log.md` |
| 2026-03-03 01:39 | localhost / 2026-01-01 / `make e2e` | 修复 `tp060-02-orgunit-ext-query` 后复跑全量 E2E | 全量套件通过，TP-060-05 证据可持续留存 | 13/13 通过，`tp220-assistant` 维持 5/5 通过 | P2 | BUG | 继续补齐 Assistant API 实链路与多模型证据后再标记 TP-060-05 完成 | 待指派 | `docs/archive/dev-records/dev-plan-064a-execution-log.md` |
| 2026-03-03 01:58 | localhost / unit-test / `go test -v ./internal/server -run ...` | 执行 Assistant API + 多模型分支覆盖测试 | create/turn/confirm/commit 与 validate/apply 至少各 1 正/负例并可审计 | 目标用例全部通过；状态码/错误码覆盖含 `invalid_request`/`assistant_candidate_not_found`/`conversation_state_invalid`/`unauthorized`/422 validate 错误 | P2 | BUG | 保持该证据基线；后续补齐 `ai_plan_contract_version_mismatch` 专项样例 | 待指派 | `docs/archive/dev-records/dev-plan-064a-execution-log.md` |
| 2026-03-03 02:05 | localhost / unit-test / `go test -v ./internal/server -run ...` | 新增并验证 `ai_plan_contract_version_mismatch` 专项分支 | 提交阶段契约版本漂移返回 409 `ai_plan_contract_version_mismatch` 且状态回退 validated | 用例通过，返回码与状态回退符合预期 | P2 | BUG | TP-060-05 证据闭环完成，后续进入维护期 | 待指派 | `docs/archive/dev-records/dev-plan-064a-execution-log.md` |
