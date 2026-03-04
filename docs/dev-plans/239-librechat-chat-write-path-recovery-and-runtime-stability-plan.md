# DEV-PLAN-239：LibreChat 聊天可写链路恢复与运行态稳定性收口详细设计

**状态**: 已实施（待人工验收真实模型回包）（2026-03-04 03:42 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/230-librechat-project-level-integration-plan.md`
  - `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
  - `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
  - `docs/dev-plans/238-librechat-mongodb-runtime-failure-hardening-plan.md`
- **当前痛点**（2026-03-04 现场复现）:
  1. `assistant-ui` 代理当前仅允许 `GET`，聊天发送请求被拦截，导致 iframe 可加载但不可交互。
  2. LibreChat 运行态在部分环境会退化为 `unavailable`（典型表现为 `mongodb` 挂载/权限异常与 `rag_api` 未运行）。
  3. 本地清理与重建链路在容器写入文件权限不一致场景下不稳定，放大恢复成本与停机窗口。
- **业务价值**:
  - 让 `/app/assistant` 页面“聊天壳层（LibreChat）”恢复可对话，同时保持本仓 `AuthN/AuthZ/Tenant/One Door` 边界不变。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标（量化）
1. [ ] **可对话闭环（真实模型）**：登录用户在 `/app/assistant` 的 LibreChat iframe 内可完成“发送消息 -> 由真实外网模型返回响应”最小闭环。
2. [X] **边界不退化**：`/assistant-ui/*` 继续受会话与租户校验，未登录仍 `302 /app/login`，跨租户会话仍拒绝。
3. [X] **代理最小放开**：仅放开聊天必需 HTTP 方法，维持路径边界与敏感头剥离，fail-closed 不变。
4. [X] **运行态可恢复**：`make assistant-runtime-down && make assistant-runtime-clean && make assistant-runtime-up && make assistant-runtime-status` 连续 3 轮通过，状态为 `healthy`。
5. [X] **证据可审计**：完成单测/E2E/runtime 回归并在 `docs/dev-records/dev-plan-239-execution-log.md` 留档。

> 当前未闭环项：真实外网模型回包（需人工在具备有效密钥的环境中完成 iframe 实聊验收并补证据）。

### 2.2 非目标 (Out of Scope)
1. [ ] 不引入 LibreChat 自管身份，不改变 Kratos/Casbin/Tenant 注入归属。
2. [ ] 不替代“当前回合操作”业务提交链（Confirm/Commit/Task 仍走本仓 One Door）。
3. [ ] 不新增数据库 schema/迁移/sqlc 变更。
4. [ ] 不恢复 legacy 双链路，不引入临时绕过开关。
5. [ ] 不在本计划推进 `model-providers:apply` 三阶段退役（由 `DEV-PLAN-236` 承接）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [X] Go 代码（server/proxy/scripts/tests）
  - [ ] `.templ` / CSS 生成
  - [ ] 多语言 JSON
  - [X] Routing
  - [X] E2E
  - [X] 文档门禁
- **本地必跑（命中项）**：
  1. [X] `go fmt ./... && go vet ./... && make check lint && make test`
  2. [X] `make check routing`
  3. [X] `make check capability-route-map`（命中映射改动时）
  4. [X] `make check error-message`（命中错误码/提示改动时）
  5. [X] `make assistant-runtime-down && make assistant-runtime-clean && make assistant-runtime-up && make assistant-runtime-status`
  6. [X] `docker compose -p ${LIBRECHAT_COMPOSE_PROJECT:-bugs-and-blossoms-librechat} --env-file deploy/librechat/.env -f deploy/librechat/docker-compose.upstream.yaml -f deploy/librechat/docker-compose.overlay.yaml exec -T api sh -lc 'test -n "${OPENAI_API_KEY:-}"'`
  7. [X] `make e2e`
  8. [X] `make check doc`
- **SSOT 链接**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `docs/dev-plans/012-ci-quality-gates.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 目标拓扑
```mermaid
graph TD
    A[/app/assistant/] --> B[iframe /assistant-ui/*]
    B --> C[withTenantAndSession]
    C --> D[assistant_ui_proxy]
    D --> E[LibreChat upstream]

    A --> F[当前回合操作面板]
    F --> G[/internal/assistant/*]
    G --> H[One Door chain]
```

### 3.2 关键决策（ADR 摘要）
- **ADR-239-01：代理方法从只读收敛到聊天最小可写集**（选定）
  - 选项 A：继续仅 `GET`；缺点：聊天无法发送。
  - 选项 B（选定）：允许 `GET/HEAD/POST/OPTIONS`，其余继续 `405`；`OPTIONS` 保留用于浏览器预检稳定性，避免写链路回归。
- **ADR-239-02：可用性修复不得突破边界**（选定）
  - 即便放开 `POST`，仍必须保持 `/assistant-ui/*` 前缀、会话校验、敏感头剥离、禁止旁路本仓 `/internal/**`。
- **ADR-239-03：运行态故障优先脚本化恢复**（选定）
  - 把目录存在性/可写性/挂载一致性检查前置到脚本，失败直接中止并输出可操作诊断。

### 3.3 职责边界（显式冻结）
1. [X] LibreChat iframe：负责聊天 UI 交互与通用对话体验。
2. [X] 当前回合操作面板：负责业务裁决动作（Generate/Confirm/Commit/Submit Task）。
3. [X] 两者通过受控消息桥协同，不互相替代，不合并状态机。

## 4. 数据模型与约束 (Data Model & Constraints)
> 本计划不新增数据库表，仅收敛运行时契约。

### 4.1 assistant-ui 代理边界契约（冻结）
```yaml
assistant_ui_proxy:
  methods_allowlist: [GET, HEAD, POST, OPTIONS]
  path_prefix: /assistant-ui
  request_header_allowlist:
    - Accept
    - Accept-Encoding
    - Accept-Language
    - Cache-Control
    - Content-Type
    - Origin
    - Referer
    - User-Agent
  request_header_strip:
    - Cookie
    - Authorization
  response_header_strip:
    - Set-Cookie
```
约束：
1. [X] 非 allowlist 方法返回 `405`。
2. [X] 非 `/assistant-ui/*` 路径返回 `400`。
3. [X] 请求进入 proxy 前必须通过 tenant/session/principal 校验。
4. [X] 禁止透传本仓登录态凭据到上游。

### 4.2 真实模型密钥与配置入口契约（冻结）
1. [X] `OPENAI_API_KEY` 必须通过运行时环境注入 `api` 容器（必要时同步注入 `rag_api`），且容器内值必须非空。
2. [X] `apps/web/src/pages/assistant/AssistantModelProvidersPage.tsx` 仅做“只读展示 + validate”，不作为配置写入口；模型与密钥最终以运行时环境为准。
3. [X] 密钥仅允许放在本机私有环境文件（如 `.env.local`、`deploy/librechat/.env`），禁止提交到仓库。

### 4.3 运行态状态文件契约（沿用并补齐）
`deploy/librechat/runtime-status.json` 字段保持：
- `status`: `healthy | unavailable`
- `checked_at`: UTC 时间戳
- `upstream.url`
- `services[]`: `{name, required, healthy, reason}`

失败原因枚举至少包含：
- `mount_source_missing`
- `container_not_running`
- `upstream_unreachable`

## 5. 接口契约 (API / HTTP Contracts)
### 5.1 `/assistant-ui/*` 代理行为（目标态）
| 场景 | 输入 | 期望 |
| --- | --- | --- |
| 未登录访问 | `GET /assistant-ui` | `302 /app/login` |
| 正常聊天发送 | `POST /assistant-ui/*` | 转发上游，响应透传（剥离 `Set-Cookie`） |
| 预检请求 | `OPTIONS /assistant-ui/*` | 允许通过（用于浏览器预检），其余边界约束不变 |
| 方法越界 | `PUT/DELETE/PATCH ...` | `405` + 稳定错误码 |
| 路径越界 | 非 `/assistant-ui/*` | `400` + 稳定错误码 |
| 上游不可达 | 代理转发失败 | `502` + 稳定错误码 |

### 5.2 错误码与提示（拟收敛）
1. [X] `assistant_ui_method_not_allowed`
2. [X] `assistant_ui_path_invalid`
3. [X] `assistant_ui_upstream_unavailable`

要求：
- [X] `en/zh` 文案明确，不输出泛化失败提示。
- [X] 与 `make check error-message` 口径一致。

### 5.3 runtime 命令接口契约
1. [X] `assistant-runtime-clean`：在权限冲突场景给出明确提示与恢复建议。
2. [X] `assistant-runtime-up`：挂载源异常时 fail-fast，并输出具体 service/路径。
3. [X] `assistant-runtime-status`：输出可机读 JSON，非健康时返回非零退出码。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 代理入口判定算法
```text
if method not in allowlist: return 405
if path not starts_with /assistant-ui: return 400
if tenant/session/principal invalid: return 302(/app/login) or 401 by route class
sanitize request headers
proxy to librechat upstream
strip response Set-Cookie
on proxy error: return 502
```

### 6.2 消息桥协同算法（保持）
```text
receive postMessage from iframe
validate origin + schema + channel + nonce
if type == assistant.prompt.sync: sync payload.input to "输入需求"文本框
if type == assistant.turn.refresh: refresh current conversation
otherwise: drop
```

### 6.3 runtime 恢复闭环算法
```text
down -> clean -> up -> status
if status != healthy:
  inspect reason per service
  fix mount/permission/container issue
  rerun status until healthy
```

## 7. 安全与鉴权 (Security & Authz)
1. [X] 身份体系保持本仓 Kratos Session，不引入 LibreChat 自管登录。
2. [X] `/assistant-ui/*` 必须纳入受保护 UI 前缀，与 `/app/**` 同级校验。
3. [X] 保持 One Door 边界：assistant-ui 仅为聊天壳层，不直接触发业务写路由。
4. [X] 敏感头剥离与路径约束默认启用，不提供 warning-only 模式。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：
  - `DEV-PLAN-231`（单主源门禁）已完成
  - `DEV-PLAN-232`（运行基线）已实施
  - `DEV-PLAN-235`（会话边界）已完成
  - `DEV-PLAN-238`（Mongo 运行异常修复）已完成
- **里程碑**：
  1. [X] M1：冻结代理方法矩阵、错误码契约与测试清单。
  2. [X] M2：完成 proxy 改造与单测（方法/路径/头部/会话边界）。
  3. [X] M3：完成 runtime 脚本稳定性改造、容器内密钥前置校验与 3 轮恢复闭环证据。
  4. [ ] M4：完成真实模型 e2e 回归、门禁全绿、readiness 留档。

## 9. 测试与验收标准 (Acceptance Criteria)
### 9.1 单元/集成测试
1. [X] `assistant_ui_proxy_test` 覆盖 `GET/HEAD/POST/OPTIONS` 正向与非法方法负向。
2. [X] 覆盖路径越界（非 `/assistant-ui/*`）返回 `400`。
3. [X] 覆盖敏感头剥离（`Cookie`/`Authorization`）与 `Set-Cookie` 响应剥离。
4. [X] 覆盖未登录与跨租户会话拒绝。

### 9.2 E2E 必测
1. [ ] 登录后在 iframe 内发送消息并收到真实外网模型响应（记录 provider/model 与响应片段）。
2. [ ] 未登录访问 `/assistant-ui` 被重定向。
3. [ ] 跨租户会话复用被拒绝。
4. [X] assistant-ui 无法旁路调用本仓业务写接口。
5. [ ] 本计划验收禁止使用本地假服务替代真实模型响应。

### 9.3 runtime 回归
1. [X] 连续 3 轮 `down -> clean -> up -> status` 均为 `healthy`。
2. [X] 人工构造挂载缺失场景时，脚本能输出稳定且可操作的失败原因。
3. [X] `api` 容器内 `OPENAI_API_KEY` 非空（以容器内检查结果为准，不以页面展示为准）。

### 9.4 验收命令
1. [X] `go fmt ./... && go vet ./... && make check lint && make test`
2. [X] `make check routing`
3. [X] `make check capability-route-map`（命中时）
4. [X] `make check error-message`（命中时）
5. [X] `make assistant-runtime-down && make assistant-runtime-clean && make assistant-runtime-up && make assistant-runtime-status`
6. [X] `docker compose -p ${LIBRECHAT_COMPOSE_PROJECT:-bugs-and-blossoms-librechat} --env-file deploy/librechat/.env -f deploy/librechat/docker-compose.upstream.yaml -f deploy/librechat/docker-compose.overlay.yaml exec -T api sh -lc 'test -n "${OPENAI_API_KEY:-}"'`
7. [X] `make e2e`
8. [X] `make check doc`

### 9.5 完成定义（DoD）
1. [ ] 用户可在“聊天壳层（LibreChat）”中通过真实外网模型完成交互。
2. [X] 安全边界与 One Door 约束不回退。
3. [X] 运行态恢复流程在本地可重复通过并具备故障可诊断性。
4. [X] `docs/dev-records/dev-plan-239-execution-log.md` 证据完整。

## 10. 运维与监控 (Ops & Monitoring)
1. [X] 保持早期阶段最小运维原则，不新增外部监控栈与开关系统。
2. [X] 运行异常处置顺序固定：阻断发布 -> 修复运行态 -> 重跑门禁 -> 恢复。
3. [X] 关键日志需包含 `tenant_id/request_id/trace_id/path/method/reason`。
4. [X] 回滚策略仅允许版本回滚或前向修复，禁止恢复 legacy 双链路。

## 11. Readiness 记录要求
1. [X] 新建 `docs/dev-records/dev-plan-239-execution-log.md`。
2. [X] 每条证据记录：UTC 时间、命令、退出码、关键响应片段、关联 commit/PR。
3. [ ] 至少包含 1 条正向证据（真实模型聊天可发送并回包，含 provider/model）与 3 条负向证据（未登录/跨租户/越界）。
4. [ ] 全部验收项勾选完成后再将状态更新为 `已完成`。

## 12. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `internal/server/assistant_ui_proxy.go`
- `internal/server/handler.go`
- `internal/server/assistant_ui_proxy_test.go`
- `internal/server/tenancy_middleware_test.go`
- `apps/web/src/pages/assistant/AssistantPage.tsx`
- `apps/web/src/pages/assistant/assistantMessageBridge.ts`
- `scripts/librechat/up.sh`
- `scripts/librechat/clean.sh`
- `deploy/librechat/healthcheck.sh`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/238-librechat-mongodb-runtime-failure-hardening-plan.md`
