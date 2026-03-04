# DEV-PLAN-239：LibreChat 聊天可写链路恢复与运行态稳定性收口计划

**状态**: 进行中（2026-03-04 01:20 UTC）

## 1. 背景与问题
- 承接计划：
  - `docs/dev-plans/230-librechat-project-level-integration-plan.md`
  - `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
  - `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
  - `docs/dev-plans/238-librechat-mongodb-runtime-failure-hardening-plan.md`
- 当前用户可见问题（2026-03-04 现场）：在 `/app/assistant` 页面中，LibreChat iframe 可加载但无法在聊天框正常发起对话。
- 已确认的主要缺口：
  1. [x] `assistant-ui` 代理当前仅允许 `GET`，聊天发送所需写请求被 `405` 拒绝（“只读壳层”与“可对话壳层”语义不一致）。
  2. [x] LibreChat runtime 在部分环境进入 `unavailable`，常见表现为 `mongodb mount_source_missing`、`rag_api container_not_running`。
  3. [x] 本地清理链路对容器写入的 root/非当前用户文件处理不稳定，导致 `assistant-runtime-clean/up` 偶发失败，进一步放大不可用窗口。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 恢复“可对话”最小闭环：登录后可在 LibreChat 聊天框提交消息并获得响应。
2. [ ] 维持边界不变：`/assistant-ui/*` 仍受本仓 `AuthN/AuthZ/Tenant` 保护，禁止业务写旁路。
3. [ ] 收敛代理契约：方法放开仅覆盖聊天必需范围，路径/头部继续最小透传与 fail-closed。
4. [ ] 收敛运行稳定性：`make assistant-runtime-up/status` 在干净环境可重复恢复为 `healthy`。
5. [ ] 补齐回归证据：单测 + e2e + runtime 闭环证据进入 `docs/dev-records/`。

### 2.2 非目标（Out of Scope）
1. [ ] 不引入 LibreChat 自管身份体系，不改变本仓 Kratos/Casbin/Tenant 归属。
2. [ ] 不引入新数据库/新中间件，不新增业务能力。
3. [ ] 不恢复 legacy 双链路或临时旁路开关。
4. [ ] 不在本计划处理 `model-providers:apply` 退役阶段推进（由 `DEV-PLAN-236` 承接）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [X] Go 代码（代理/中间件/测试）
  - [ ] `.templ` / CSS 生成
  - [ ] 多语言 JSON
  - [X] Routing / capability 映射
  - [X] E2E
  - [X] 文档门禁
- **本地必跑（命中项）**：
  1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
  2. [ ] `make check routing`
  3. [ ] `make check capability-route-map`（命中映射改动时）
  4. [ ] `make check error-message`（命中错误码/提示改动时）
  5. [ ] `make assistant-runtime-down && make assistant-runtime-clean && make assistant-runtime-up && make assistant-runtime-status`
  6. [ ] `make e2e`
  7. [ ] `make check doc`
- **SSOT 链接**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `docs/dev-plans/012-ci-quality-gates.md`

## 3. 架构与关键决策
### 3.1 目标拓扑（保持 One Door 边界）
```mermaid
graph TD
    A[/app/assistant/] --> B[iframe /assistant-ui/*]
    B --> C[withTenantAndSession]
    C --> D[assistant_ui_proxy]
    D --> E[LibreChat upstream]

    F[/internal/assistant/*] --> G[One Door chain]
```

### 3.2 ADR 摘要
- **ADR-239-01：代理方法从“只读”收敛到“聊天最小可写集”**（选定）
  - 选项 A：继续仅 `GET`；缺点：无法在聊天框发送消息。
  - 选项 B（选定）：按证据放开聊天必需方法（默认 `GET/HEAD/POST`，必要时补 `OPTIONS`），其余仍拒绝。
- **ADR-239-02：边界优先于可用性**（选定）
  - 即便放开 `POST`，仍保持 `/assistant-ui/*` 前缀约束、敏感头剥离、禁止本仓内部路由旁路。
- **ADR-239-03：运行目录治理脚本化**（选定）
  - 启动/清理流程需对目录存在性、权限与挂载源一致性做前置检查与自动修复路径，避免人工临时命令依赖。

## 4. 输入/输出契约
### 4.1 assistant-ui 代理契约（拟修订）
```yaml
assistant_ui_proxy:
  methods_allowlist: [GET, HEAD, POST]
  path_prefix: /assistant-ui
  request_header_strip: [Cookie, Authorization]
  response_header_strip: [Set-Cookie]
```
约束：
1. [ ] 非 allowlist 方法返回 `405`。
2. [ ] 非 `/assistant-ui/*` 路径返回 `400`。
3. [ ] 仍需通过会话与租户校验；未登录保持 `302 /app/login`。
4. [ ] assistant-ui 不得触达本仓 `/internal/**` 业务写路径。

### 4.2 runtime 脚本契约（拟修订）
1. [ ] `assistant-runtime-up`：启动前校验各数据目录“存在 + 可写 + 可删除探针”并输出可操作错误。
2. [ ] `assistant-runtime-clean`：清理时兼容容器写入文件权限差异，避免因权限导致半清理状态。
3. [ ] `assistant-runtime-status`：失败原因保持稳定枚举（如 `mount_source_missing`、`container_not_running`、`upstream_unreachable`）。

## 5. 实施切片（建议顺序）
1. [ ] **PR-239-01：契约冻结与测试先行**
   - [ ] 冻结代理方法矩阵与错误码口径。
   - [ ] 先补失败用例（聊天请求被拒绝、路径越界、旁路写）。
2. [ ] **PR-239-02：assistant-ui 聊天可写链路恢复**
   - [x] 在代理中放开聊天必需方法。
   - [x] 保持路径/头部边界与 fail-closed。
3. [ ] **PR-239-03：runtime 目录与挂载稳定性加固**
   - [x] 强化 `up/clean/status` 脚本前置检查与错误提示。
   - [x] 固化最小恢复流程（down -> clean -> up -> status）。
4. [ ] **PR-239-04：回归与证据封板**
   - [ ] 补齐 e2e：iframe 内聊天发送正向 + 未登录/跨租户/旁路写负向。
   - [ ] 输出 `docs/dev-records/dev-plan-239-execution-log.md`。

## 6. 验收标准（DoD）
1. [ ] 登录用户在 `/app/assistant` 内可完成一次真实聊天发送与响应接收。
2. [ ] 未登录访问 `/assistant-ui` 仍返回 `302 /app/login`，跨租户会话仍被拒绝。
3. [ ] 非法方法与越界路径仍被稳定阻断（`405`/`400`），无旁路写。
4. [ ] `assistant-runtime-down -> clean -> up -> status` 连续 3 轮通过，状态为 `healthy`。
5. [ ] 本计划门禁命令全绿，且证据入档。

## 7. 风险与缓解
1. [ ] 风险：放开 `POST` 引入代理攻击面。  
   缓解：仅放开最小方法集合 + 路径/头部双层约束 + 负测门禁。
2. [ ] 风险：Docker/WSL 路径语义差异导致偶发挂载失败。  
   缓解：脚本前置校验绝对路径与目录探针，失败即中止并输出修复建议。
3. [ ] 风险：为追求可用性放松身份边界。  
   缓解：复用 `DEV-PLAN-235` 边界用例，任一回归失败即阻断发布。

## 8. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `internal/server/assistant_ui_proxy.go`
- `internal/server/handler.go`
- `scripts/librechat/up.sh`
- `scripts/librechat/clean.sh`
- `deploy/librechat/healthcheck.sh`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/238-librechat-mongodb-runtime-failure-hardening-plan.md`
