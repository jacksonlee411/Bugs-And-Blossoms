# DEV-PLAN-236：LibreChat 旧入口退役与单主源封板实施计划（详细设计）

**状态**: 进行中（2026-03-07 23:35 CST，已按 280/235/237 的激进 cutover 口径修订；实施记录见 `docs/archive/dev-records/dev-plan-236-execution-log.md`）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/230-librechat-project-level-integration-plan.md`（PR-230-05）
  - `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
  - `docs/dev-plans/233-librechat-single-source-config-convergence-plan.md`
- **当前痛点**:
  1. [ ] `POST /internal/assistant/model-providers:apply` 仍存在，可能导致“单主源名义化”。
  2. [ ] 旧入口相关资产分散在路由、capability 映射、前端 API 与测试，易发生删改不同步。
  3. [ ] 退役 stopline 尚未落实为可执行的删改顺序与门禁证据。
4. [ ] 旧入口退役若继续按温和过渡思路推进，容易与 `280` 的 cutover-first 方向冲突。
- **业务价值**:
  - 将模型配置写入口彻底收敛到 LibreChat 主源，消除第二写入口，保障 224/225 确定性链路与 No-Legacy 不变量。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
1. [ ] 按激进 stopline 完成 `model-providers:apply` 与相关旧入口资产的一次性退役：
   - [ ] 不再保留长期 A/B/C 温和过渡窗口作为正式策略。
   - [ ] 一旦替代链路与门禁就绪，直接删除路由、实现与全部引用。
   - [ ] 若因现实原因临时保留过渡响应，其职责仅限短期阻断提示，不得继续承担迁移入口价值。
2. [X] 同步清理路由、handler、capability 映射、allowlist、前端调用、测试与文档残留。
3. [X] 保证 `GET /model-providers`、`GET /models`、`POST ...:validate` 仅保留只读/校验语义，不回流为第二写入口。
4. [X] 形成封板：`assistant-config-single-source + no-legacy` 持续阻断旧入口回流。

### 2.2 非目标 (Out of Scope)
1. [ ] 不新增任何替代写入口（含脚本旁路、临时管理口）。
2. [ ] 不引入功能开关、灰度旁路或“测试环境保留旧链路”。
3. [ ] 不通过 legacy 分支/双链路进行回滚。
4. [ ] 不为旧入口保留长期兼容窗口、双入口共存期或“先不删代码、后面再收”的技术债缓冲区。

## 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [X] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] `.templ` / Tailwind
  - [ ] 多语言 JSON（仅命中错误码文案时执行 `make check tr`）
  - [ ] Authz 策略包
  - [X] 路由治理（`make check routing`）
  - [ ] DB 迁移 / Schema
  - [ ] sqlc
  - [X] E2E（`make e2e`）
  - [X] 文档门禁（`make check doc`）
- **本计划强制门禁**：
  1. [X] `make check assistant-config-single-source`
  2. [X] `make check no-legacy`
  3. [X] `make check routing`
  4. [X] `make check capability-route-map`
  5. [X] `make check error-message`
  6. [X] `make check assistant-domain-allowlist`
  7. [X] `make e2e`
  8. [X] `make check doc`
- **SSOT 链接**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `docs/dev-plans/012-ci-quality-gates.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 退役架构图（cutover-first）
```mermaid
graph TD
    A[Web Console / Internal Client] --> B[/internal/assistant/model-providers:apply]
    B --> C{阶段}
    C -->|A: 2026-03-20 前| D[迁移过渡语义 + Deprecation]
    C -->|B: 2026-04-10 前| E[410 Gone + 稳定错误码]
    C -->|C: 2026-04-24 前| F[路由物理删除]

    G[config/routing/allowlist.yaml] --> H[Routing Gate]
    I[config/capability/route-capability-map.v1.json] --> J[Capability Gate]
    K[scripts/ci/check-assistant-config-single-source.sh] --> L[Single Source Gate]
    H --> M[CI Fail-Closed]
    J --> M
    L --> M
```

### 3.2 关键设计决策（ADR 摘要）
- **ADR-236-01：阶段切换通过代码发布实现，不做运行时开关**（选定）
  - 选项 A：运行时 flag 切换 A/B/C。缺点：违反“早期不引入开关切换”。
  - 选项 B（选定）：按 stopline 合并阶段 PR，行为由代码版本决定。
- **ADR-236-02：B 阶段固定 410 + 稳定错误码**（选定）
  - 选项 A：继续 200 + warning。缺点：无法形成强阻断。
  - 选项 B（选定）：`410 Gone + assistant_model_provider_apply_deprecated`。
- **ADR-236-03：C 阶段必须“全链路物理删除”**（选定）
  - 选项 A：仅删 handler。缺点：映射/前端残留导致漂移。
  - 选项 B（选定）：路由、映射、前端、测试、文档一次收口。

## 4. 数据模型与约束 (Data Model & Constraints)
> 本计划不新增 DB Schema；数据约束聚焦“接口资产与门禁一致性”。

### 4.1 退役资产清单（控制面模型）
| 资产 | 阶段 A | 阶段 B | 阶段 C |
| --- | --- | --- | --- |
| `internal/server/handler.go` 中 `:apply` 路由 | 保留 | 保留（返回 410） | 删除 |
| `config/routing/allowlist.yaml` 的 `:apply` 项 | 保留 | 保留 | 删除 |
| `internal/server/capability_route_registry.go` 的 `:apply` 映射 | 保留 | 保留 | 删除 |
| `config/capability/route-capability-map.v1.json` 的 `:apply` 项 | 保留 | 保留 | 删除 |
| `apps/web/src/api/assistant.ts` 的 `:apply` 调用 | 可保留但标注弃用 | 禁止业务入口继续调用 | 删除 |
| 相关测试 | 覆盖 Deprecation | 覆盖 410 错误码 | 覆盖路由不存在 |

### 4.2 约束规则
1. [ ] 阶段行为由代码版本决定，不允许按环境/开关切换行为。
2. [ ] 任何阶段都不得新增第二写入口。
3. [ ] C 阶段后仓库中不得存在对 `model-providers:apply` 的生产路径引用（历史文档归档说明除外）。
4. [ ] 读/校验接口禁止写入确定性产物（`intent_hash/plan_hash/contract_snapshot`）。

## 5. 接口契约 (API Contracts)
### 5.1 `POST /internal/assistant/model-providers:apply`（阶段 A）
- **语义**：迁移过渡入口（非长期可写 API）。
- **响应**：
  - Status: `200`
  - Headers:
    - `Deprecation: true`
    - `Sunset: Thu, 10 Apr 2026 00:00:00 GMT`（可选）
- **约束**：必须输出可审计的弃用日志，提示迁移到 LibreChat 主源。

### 5.2 `POST /internal/assistant/model-providers:apply`（阶段 B）
- **响应**：
  - Status: `410 Gone`
  - Body（示例）:
    ```json
    {
      "error_code": "assistant_model_provider_apply_deprecated",
      "message": "Model provider apply endpoint has been retired. Use LibreChat as the single source of truth."
    }
    ```
- **错误提示**：必须通过 `make check error-message`（en/zh 明确提示）。

### 5.3 `POST /internal/assistant/model-providers:apply`（阶段 C）
- **语义**：路由删除后不再可达。
- **期望行为**：请求命中 `404`，仓库无任何活动写入口。

### 5.4 相关只读/校验接口职责冻结
1. [ ] `GET /internal/assistant/model-providers`：只读展示。
2. [ ] `GET /internal/assistant/models`：只读枚举。
3. [ ] `POST /internal/assistant/model-providers:validate`：只做校验，不落库、不改写配置。
4. [ ] 读/校验接口执行前后，配置版本号（或等价 route hash）与审计写入计数不发生变化（无写副作用）。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 cutover 发布算法（无开关、先删后切）
```text
freeze replacement path and gates
delete old route + handler + allowlist + capability mapping
delete frontend API callsites, tests, docs, scripts and residual references
run gates: routing/capability/single-source/no-legacy/error-message/domain-allowlist/doc/e2e
if any gate fails: block merge (fail-closed)
merge only when old entry no longer carries formal responsibility
```

### 6.2 旧入口残留检查算法
```text
search repo for apply/write-entry references
if any active route, frontend callsite, test path or doc still treats it as formal entry: fail
if any fallback/compatibility code remains for the old entry: fail
```

## 7. 安全与鉴权 (Security & Authz)
1. [ ] 保持 AuthN/AuthZ/Tenant 边界：退役不改变 Kratos/Casbin/tenant 注入责任归属。
2. [ ] 任何写行为仍需通过 One Door 链路，`apply` 不得作为旁路写入口。
3. [ ] 清理 `:apply` capability 映射后，防止“无路由有权限”或“有路由无映射”的漂移。
4. [ ] 结构化日志至少包含：`request_id`、`trace_id`、`tenant_id`、`path`、`error_code`。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：
  - `DEV-PLAN-231` 已完成（single-source gate）。
  - `DEV-PLAN-233` 已完成（主源收口）。
  - `DEV-PLAN-235` 必须补齐新正式入口边界，防止旧入口退役后再被临时放行回流。
  - `DEV-PLAN-280/237` 的 cutover 与封板口径优先于本计划的旧三阶段表述。
- **里程碑**：
  1. [ ] M1：冻结替代链路与门禁，不再继续投资旧入口。
  2. [ ] M2：物理删除旧入口路由、实现、前端调用、测试与文档残留。
  3. [ ] M3：封板验证——仓库内不存在旧入口正式职责、无双入口并存。

## 9. 测试与验收标准 (Acceptance Criteria)
### 9.1 必测场景
1. [ ] 目标态下 `:apply` 不再作为活动写入口，默认应为 `404` 或等价的彻底不可用状态。
2. [ ] `GET /model-providers`、`GET /models`、`POST ...:validate` 仅保留只读/校验语义。
3. [ ] 对 2) 的三类接口执行 before/after 断言：配置版本号（或 route hash）不变、无新增审计写事件。
4. [ ] 旧入口相关映射/前端/测试/文档残留清零。
5. [ ] 任意尝试恢复旧入口的改动，被 single-source + no-legacy 阻断。
6. [ ] 不存在“虽然用户不用了，但代码/测试/文案还留着”的旧入口正式职责残留。

### 9.2 验收命令
1. [X] `go fmt ./... && go vet ./... && make check lint && make test`
2. [X] `make check assistant-config-single-source`
3. [X] `make check no-legacy`
4. [X] `make check routing`
5. [X] `make check capability-route-map`
6. [X] `make check error-message`
7. [X] `make check assistant-domain-allowlist`
8. [X] `make e2e`
9. [X] `make check doc`
10. [X] `make check tr`（仅命中多语言 JSON/错误提示文案变更时）

## 10. 运维与监控 (Ops & Monitoring)
1. [ ] 遵循早期最小运维原则：不新增运行时开关，不新增双链路监控体系。
2. [ ] 故障处置顺序固定：环境级保护（只读/停写）→ 修复主源配置/代码 → 重跑门禁 → 恢复。
3. [ ] 回滚策略仅允许“版本回滚或前向修复”，禁止恢复 legacy 入口；回滚也不得把旧入口重新设为正式职责。
4. [ ] 每次阶段推进必须产出可审计日志与 `dev-records` 证据。

## 11. Readiness 与执行记录要求
1. [X] 记录文件：`docs/archive/dev-records/dev-plan-236-execution-log.md`。
2. [ ] 每次 cutover 步骤至少记录：UTC 时间、命令、退出码、关键响应片段、PR/commit。
3. [ ] 至少 1 条负向证据：旧入口请求被阻断或命中 `404`。
4. [ ] 至少 1 条残留清零证据：搜索/门禁证明旧入口正式职责已清空。
5. [ ] 删除与封板证据齐全后，方可将状态更新为 `已完成`。

## 12. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `docs/dev-plans/001-technical-design-template.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- `docs/dev-plans/233-librechat-single-source-config-convergence-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `internal/server/handler.go`
- `internal/server/capability_route_registry.go`
- `config/routing/allowlist.yaml`
- `config/capability/route-capability-map.v1.json`
- `apps/web/src/api/assistant.ts`
- `scripts/ci/check-assistant-config-single-source.sh`
