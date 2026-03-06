# DEV-PLAN-235：LibreChat 身份/会话/租户边界硬化详细设计

**状态**: 已完成（2026-03-03 15:10 UTC，实施与验证见 `docs/archive/dev-records/dev-plan-235-execution-log.md`）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/230-librechat-project-level-integration-plan.md`（PR-230-04）
  - `docs/dev-plans/019-tenant-and-authn.md`
  - `docs/dev-plans/017-routing-strategy.md`
  - `docs/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md`
- **当前痛点**:
  1. 现有 `withTenantAndSession` 对 UI 路径采用“非 `/app/**` 直通”策略，`/assistant-ui/*` 可绕过会话校验。
  2. `assistant-ui` 反向代理默认透传请求头，缺少“最小透传 + 敏感头剥离”契约。
  3. 代理路径/方法边界未冻结，存在越界访问、跨租户会话复用与身份混淆风险。
  4. 缺少覆盖 `/assistant-ui/*` 的端到端负测（未登录、跨租户、旁路写）作为 CI 阻断证据。
- **业务价值**:
  - 将 `assistant-ui` 纳入与 `/app/**` 同级的会话与租户边界，确保 LibreChat UI 复用不破坏本仓 AuthN/AuthZ/Tenant 不变量。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
1. [ ] `/assistant-ui` 与 `/assistant-ui/*` 强制会话校验，禁止绕过。
2. [ ] 固化 UI 会话行为矩阵（未登录、会话失效、租户不匹配）并保持 fail-closed。
3. [ ] 收敛代理边界：方法白名单、路径规范化、请求/响应头最小透传。
4. [ ] 明确并保持 AuthN/AuthZ/Tenant 注入归属在本仓，不引入 LibreChat 自管身份旁路。
5. [ ] 补齐单测 + E2E 负测，并接入现有门禁链路。

### 2.2 非目标 (Out of Scope)
1. [ ] 不引入新的身份系统、SSO 流程或 LibreChat 本地用户体系。
2. [ ] 不变更 One Door 提交边界，不允许 assistant-ui 直接触发业务写路由。
3. [ ] 不新增数据库表、迁移或 sqlc 变更。
4. [ ] 不通过 feature flag/legacy 双链路做灰度绕过。

## 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [X] Go 代码（中间件/代理/测试）
  - [ ] `.templ` / Tailwind
  - [ ] 多语言 JSON
  - [ ] Authz 策略包（策略本身不变）
  - [X] 路由治理（allowlist 与分类一致性）
  - [ ] DB 迁移 / Schema
  - [ ] sqlc
  - [X] E2E
  - [X] 文档门禁
- **本地必跑（命中项）**：
  1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
  2. [ ] `make check routing`
  3. [ ] `make check capability-route-map`（命中映射调整时）
  4. [ ] `make check error-message`（命中错误码/提示调整时）
  5. [ ] `make e2e`
  6. [ ] `make check doc`
- **SSOT 链接**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `docs/dev-plans/012-ci-quality-gates.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 目标拓扑
```mermaid
graph TD
    A[/app/assistant/] --> B[/assistant-ui/*]
    B --> C[withTenantAndSession]
    C --> D{session + tenant + principal valid?}
    D -->|No| E[302 /app/login + clear cookie]
    D -->|Yes| F[assistant UI reverse proxy]
    F --> G[LibreChat upstream]

    H[/internal/assistant/*] --> I[withAuthz + capability-route-map]
```

### 3.2 请求时序（assistant-ui）
```mermaid
sequenceDiagram
    participant Browser as Browser
    participant MW as withTenantAndSession
    participant PX as assistant_ui_proxy
    participant LC as LibreChat

    Browser->>MW: GET /assistant-ui
    MW->>MW: Resolve tenant + lookup SID + principal
    alt 会话无效/跨租户/主体失效
      MW-->>Browser: 302 /app/login (清理 SID)
    else 校验通过
      MW->>PX: forward request
      PX->>PX: method/path/header gate
      PX->>LC: proxy request
      LC-->>PX: response
      PX-->>Browser: filtered response
    end
```

### 3.3 ADR 摘要
- **ADR-235-01：`assistant-ui` 作为“受保护 UI 前缀”显式纳管**（选定）
  - 选项 A：保留“仅 `/app/**` 校验”；缺点：`assistant-ui` 继续绕过会话。
  - 选项 B（选定）：受保护 UI 前缀集合最少包含 `/app` 与 `/assistant-ui`。
- **ADR-235-02：代理方法白名单 fail-closed**（选定）
  - 选项 A：透传任意方法；缺点：扩大攻击面。
  - 选项 B（选定）：仅允许 `GET/HEAD`，其余返回 `405`。
- **ADR-235-03：代理头部“最小透传 + 敏感剥离”**（选定）
  - 选项 A：透传全部头；缺点：SID/Authorization 泄露与双身份风险。
  - 选项 B（选定）：白名单透传，并剥离 `Cookie/Authorization/Set-Cookie` 等敏感头。

## 4. 数据模型与约束 (Data Model & Constraints)
> 本计划不新增数据库 schema；冻结运行时边界契约。

### 4.1 受保护 UI 路径契约
```yaml
protected_ui_prefixes:
  - /app
  - /assistant-ui
```
约束：
1. [ ] 受保护前缀必须经过完整 `tenant -> session -> principal` 校验。
2. [ ] 历史 UI 路径（如 `/login`）仍保持“无别名跳转、由路由层返回 404”的既有行为。

### 4.2 assistant-ui 代理请求边界契约
```yaml
assistant_ui_proxy:
  methods: [GET, HEAD]
  path_prefix: /assistant-ui
  request_header_allowlist:
    - Accept
    - Accept-Encoding
    - Accept-Language
    - Cache-Control
    - Content-Type
    - User-Agent
    - Referer
    - Origin
  request_header_strip:
    - Cookie
    - Authorization
  response_header_strip:
    - Set-Cookie
```
约束：
1. [ ] 非白名单方法拒绝（405）。
2. [ ] 路径必须保持在 `/assistant-ui/*` 语义域内，不得旁路到本仓 `/internal/**`。
3. [ ] 代理禁止把本仓登录态 cookie 透传给上游。

### 4.3 审计日志字段约束（结构化日志）
`assistant_ui_proxy_denied` / `assistant_ui_auth_denied` 事件最少字段：
- `tenant_id`
- `request_id`
- `trace_id`
- `path`
- `method`
- `reason`

## 5. 接口契约 (API / HTTP Contracts)
### 5.1 `/assistant-ui/*` 会话行为矩阵
| 场景 | 输入 | 期望结果 |
| --- | --- | --- |
| 未登录 | 无 SID | `302 -> /app/login` |
| SID 无效 | SID 查无记录 | 清理 SID + `302 -> /app/login` |
| 跨租户 SID | `sess.tenant_id != tenant.id` | 清理 SID + `302 -> /app/login` |
| 主体失效 | principal 缺失/非 active | 清理 SID + `302 -> /app/login` |
| 已登录同租户 | SID 有效且主体 active | 允许进入 proxy |

### 5.2 assistant-ui 代理行为契约
1. [ ] 仅允许 `GET/HEAD /assistant-ui/*`。
2. [ ] 上游不可达返回 `502`，错误码 `assistant_ui_upstream_unavailable`（消息明确、可审计）。
3. [ ] 方法不允许返回 `405`，错误码 `assistant_ui_method_not_allowed`。
4. [ ] 路径越界返回 `400`，错误码 `assistant_ui_path_invalid`。

### 5.3 错误码与提示契约
新增或复用错误码需进入统一错误目录并通过门禁：
- `assistant_ui_method_not_allowed`
- `assistant_ui_path_invalid`
- `assistant_ui_upstream_unavailable`

要求：
1. [ ] `en/zh` 提示明确，不使用泛化失败文案。
2. [ ] 与 `make check error-message` 口径一致。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 受保护 UI 判定算法
```text
rc = classifier.Classify(path)
if path in {health, healthz, assets/*}: bypass
resolve tenant
if rc == ui and path under protected_ui_prefixes:
  require session/principal
else if rc == ui and path not protected:
  passthrough (保持旧路径 404 语义)
```

### 6.2 assistant-ui 会话校验算法
```text
if no sid: redirect /app/login
sess = sessions.Lookup(sid)
if lookup failed or tenant mismatch: clear sid + redirect
principal = principals.GetByID(sess.principal_id)
if principal missing or inactive: clear sid + redirect
forward to proxy
```

### 6.3 代理边界算法
```text
if method not in {GET, HEAD}: deny(405)
if path not prefixed by /assistant-ui: deny(400)
proxy_path = trim_prefix_and_join(base_path)
strip request headers: Cookie, Authorization
forward only allowlisted headers + x-forwarded-prefix
on response: strip Set-Cookie
```

### 6.4 失败策略
- 任一边界校验失败均 fail-closed，不提供 warning-only 或临时旁路开关。

## 7. 安全与鉴权 (Security & Authz)
1. [ ] 身份与会话仍由本仓 Kratos Session 驱动，不引入 LibreChat 自管身份。
2. [ ] 租户与主体校验在进入 proxy 之前执行，避免未鉴权流量触达上游。
3. [ ] 保持 `RLS/Casbin/One Door` 既有边界：assistant-ui 仅作为 UI 读入口，不得成为写旁路。
4. [ ] 敏感头与 cookie 不透传，避免跨系统会话污染与凭据泄露。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**:
  - `DEV-PLAN-230`（总体切片）
  - `DEV-PLAN-232`（运行基线）
  - `DEV-PLAN-233`（单主源配置）
  - `DEV-PLAN-234`（域名策略与 OSS 能力复用）
- **里程碑**:
  1. [ ] M1：冻结会话行为矩阵与代理边界契约。
  2. [ ] M2：完成 `withTenantAndSession` 与 `assistant_ui_proxy` 代码改造。
  3. [ ] M3：补齐单测（中间件/代理）与 E2E 三类负测。
  4. [ ] M4：完成门禁验证并产出 `dev-records` 证据。

## 9. 测试与验收标准 (Acceptance Criteria)
### 9.1 必测场景
1. [ ] **未登录访问**：`GET /assistant-ui` 返回 `302 /app/login`。
2. [ ] **跨租户 cookie 复用**：tenant A 的 SID 访问 tenant B 域名时被拒绝并清理 cookie。
3. [ ] **主体失效**：禁用用户访问 assistant-ui 被拒绝。
4. [ ] **方法越界**：`POST /assistant-ui/*` 返回 `405`。
5. [ ] **旁路写防护**：assistant-ui 无法触达本仓 `/internal/assistant/*` 写路由。
6. [ ] **敏感头剥离**：请求不向上游透传本仓 `Cookie/Authorization`，响应不回写 `Set-Cookie`。

### 9.2 验收命令
1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] `make check routing`
3. [ ] `make check capability-route-map`（命中映射时）
4. [ ] `make check error-message`（命中错误码时）
5. [ ] `make e2e`
6. [ ] `make check doc`

### 9.3 完成定义（DoD）
1. [ ] `/assistant-ui/*` 与 `/app/**` 在会话与租户边界上一致。
2. [ ] 代理默认最小透传，敏感头与 cookie 剥离可被测试稳定验证。
3. [ ] 三类负测（未登录/跨租户/旁路写）在 CI 中可重复通过。
4. [ ] 无 legacy 旁路或临时开关残留。

## 10. 运维与监控 (Ops & Monitoring)
1. [ ] 本阶段不引入新运维开关或外部监控栈，遵循早期最小运维原则。
2. [ ] 当 assistant-ui 出现边界告警时，处置顺序固定：阻断发布 -> 修复边界 -> 重跑门禁 -> 恢复。
3. [ ] 日志需可追踪到 `tenant_id/request_id/trace_id`，用于审计与故障复盘。
4. [ ] 回滚只允许前向修复或版本回滚，禁止恢复 legacy 双链路。

## 11. Readiness 记录要求
1. [ ] 在 `docs/dev-records/` 新建 `dev-plan-235-execution-log.md`。
2. [ ] 记录至少 1 次正向样例与 3 次负向样例（未登录、跨租户、方法越界/旁路写）。
3. [ ] 每条证据需包含：时间、命令、结果、关键日志/响应片段。
4. [ ] 全部验收项勾选完成后再更新状态为 `准备就绪` 或 `已完成`。

## 12. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `config/routing/allowlist.yaml`
- `internal/server/handler.go`
- `internal/server/assistant_ui_proxy.go`
- `internal/server/tenancy_middleware_test.go`
- `internal/server/assistant_ui_proxy_test.go`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/236-librechat-legacy-endpoint-retirement-and-single-source-closure-plan.md`
