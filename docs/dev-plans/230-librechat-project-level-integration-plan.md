# DEV-PLAN-230：LibreChat 项目级集成实施方案（复用优先 + 边界自建）

**状态**: 修订中（2026-03-03 12:40 UTC）

## 1. 背景与问题
- 关联计划：
  - `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
  - `docs/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
  - `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
  - `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- 当前缺口（评审收敛后）：
  1. [ ] 模型配置主源切到 LibreChat 后，224/225 确定性契约映射未冻结，存在异步任务契约漂移风险。
  2. [ ] “官方运行基线复用”粒度偏粗，依赖栈归属/生命周期未固化，PR-230-01 验收会失真。
  3. [ ] `/assistant-ui/*` 会话边界未与现有路由现实完全对齐，存在会话绕过与双身份体系风险。
  4. [ ] 对 MCP/Actions/Domain Allowlist/Agents 的复用边界虽有方向，但未形成“可执行切片 + 门禁 + 验收”闭环。
  5. [ ] 旧接口 `model-providers:apply` 缺少强 stopline 与 CI 阻断，双主源共存风险仍在。

## 2. 目标与成功判定
### 2.1 核心目标
1. [ ] 复用优先：默认采用 LibreChat 官方成熟能力；自建必须给出不可复用证据。
2. [ ] 项目级纳管：LibreChat 版本、来源、运行基线可追溯、可复现。
3. [ ] 单主源配置：模型/Provider 由 LibreChat 主源驱动，本仓仅做只读适配与边界校验。
4. [ ] 边界不变：One Door、RLS/Casbin、AuthN/AuthZ/Tenant 注入保持在本仓。
5. [ ] 契约确定：224/225 版本快照与 hash 在异步链路可继承、可校验、fail-closed。
6. [ ] 退役可执行：旧入口按日期降级/停用/删除，避免双主源长期共存。
7. [ ] 升级可持续：升级/回滚流程与证据模板可执行。

### 2.2 非目标（Out of Scope）
1. [ ] 不将 `confirm/commit/re-auth/One Door` 下放给 LibreChat。
2. [ ] 不替换 `internal/assistant/*` 业务契约与状态机。
3. [ ] 不在本阶段引入 Agents 自动执行业务写动作。
4. [ ] 不引入 legacy 双链路。

### 2.3 目标到验收映射（100% 达成口径）
| 目标 | 完成定义（DoD） | 阻断门禁/证据 |
| --- | --- | --- |
| 复用优先 | 复用/自建矩阵全量覆盖，且每项有系统所有权、主数据源、退出条件 | `docs/dev-records/` 评审记录 + `make check doc` |
| 项目级纳管 | 本地/CI 可启动仓库内 LibreChat，依赖栈归属与生命周期清晰 | e2e 启动证据 + 版本元数据留档 |
| 单主源配置 | 仅 LibreChat 主源可写；本仓无第二写入口 | `make check assistant-config-single-source`（新增） |
| 边界不变 | `/assistant-ui/*` 无会话绕过；无业务写旁路 | `make check routing` + assistant-ui e2e |
| 契约确定 | 224/225 快照 + model_route_snapshot 联合校验通过 | `make e2e` + 契约快照比对报告 |
| 退役可执行 | `model-providers:apply` 按 stopline 410/删除 | CI gate + 路由/handler diff 证据 |
| 升级可持续 | 升级回归脚本固定，失败阻断发布 | 回归脚本输出 + 升级记录 |

## 3. Build vs Buy 与能力边界
### 3.1 反造轮子准则
1. [ ] 先“官方直用”->“薄适配”->“自建替代”。
2. [ ] 自建触发条件仅限：边界合规不满足、仓库契约不满足、关键指标不达标且无法插件化。
3. [ ] 自建项必须具备退出策略（上游具备等价能力后回归上游）。

### 3.2 能力分界矩阵（可执行）
| 能力域 | 决策 | 系统所有权 | 主数据源 | 迁移退出条件（含停用日期/门禁） |
| --- | --- | --- | --- | --- |
| 聊天 UI 壳层（会话/消息/输入/流式） | Must Reuse | LibreChat | LibreChat Runtime | 仅保留 iframe + proxy；若新增并行 UI，`make check no-legacy` 阻断。 |
| 模型/Provider 配置 | Must Reuse + Adapter-first | LibreChat（配置）+ 本仓（校验） | `librechat.yaml`/`.env` | `model-providers:apply`：2026-03-20 降级迁移入口；2026-04-10 CI/Prod 返回 410；2026-04-24 删除代码与路由。 |
| 官方运行基线（镜像/compose/env） | Must Reuse | LibreChat Upstream + 本仓部署封装 | Upstream tag+digest+compose | 基线必须覆盖 `api/mongodb/meilisearch/rag_api/vectordb` 生命周期；裁剪需证据与审批。 |
| 确定性契约产物（version/hash/snapshot） | Must Build | 本仓 `internal/assistant/*` | 224/225 契约快照 | 永不并入 LibreChat 配置层；来源变更必须先更新契约文档并过 e2e。 |
| 身份与会话边界（AuthN/AuthZ/Tenant） | Must Build | 本仓网关与中间件 | Kratos Session + Casbin + Tenant Resolver | `/assistant-ui/*` 必须纳入会话校验；未登录 302/401；旁路写阻断。 |
| One Door 提交裁决链 | Must Build | 本仓 | 本仓业务库与审计链 | 永不下放；出现直写旁路即阻断合并。 |

### 3.3 开源能力显式取舍表（含落地切片）
| LibreChat 能力 | 判定 | 本阶段动作 | 验收 |
| --- | --- | --- | --- |
| MCP Servers（`mcpServers`） | 复用 | 接入官方注册/调用；本仓仅做租户与能力白名单校验 | MCP 用例通过 + 审计记录可见 |
| MCP 远程域名限制（`mcpSettings.allowedDomains`） | 复用 | 统一使用官方 allowlist；本仓保留出口策略 | SSRF 负测通过 |
| Actions（`actions.allowedDomains`） | 复用 | 复用官方 Actions，不再自建第二配置中心 | Actions 端到端通过 |
| Domain Allowlist | 复用 | 以 LibreChat 配置为主源，本仓只校验与审计 | 非白名单域阻断通过 |
| Agents Builder / 自动执行 | 暂不复用（本阶段） | 禁止可写业务自动执行；保留后续评审入口 | One Door 边界测试通过 |

## 4. 关键架构与契约映射
### 4.1 运行拓扑（保持）
```mermaid
graph TD
    A[/app/assistant/] --> B[iframe /assistant-ui/*]
    B --> C[server reverse proxy]
    C --> D[LibreChat service (repo managed)]
    A --> E[/internal/assistant/*]
    E --> F[Assistant service / One Door chain]
```

### 4.2 配置主源与 224/225 确定性映射（冻结）
1. [ ] `LibreChat provider/model 配置` 仅影响“模型路由输入”，不直接生成 224/225 产物。
2. [ ] 任务创建时必须同时固化：
   - [ ] `model_route_snapshot`（canonical JSON）
   - [ ] `model_route_snapshot_hash`
   - [ ] `contract_snapshot`（224/225 version/hash/snapshot）
3. [ ] workflow 执行前校验：`model_route_snapshot_hash + contract_snapshot` 任一不一致即 fail-closed。
4. [ ] 禁止从 LibreChat 配置层反写 `intent_hash/plan_hash` 等确定性产物。
5. [ ] 与 225 收口：新增 `DEV-PLAN-225A`（或等效修订）补齐任务存储字段与 replay 校验语义。

### 4.3 身份与会话边界（冻结）
1. [ ] 认证：本仓 Kratos Session。
2. [ ] 授权：本仓 Casbin + capability-route-map。
3. [ ] 租户注入：本仓 `withTenantAndSession`。
4. [ ] `/assistant-ui/*` 必须执行会话校验，不得因“非 `/app/**` UI 路径”跳过。

### 4.4 旧接口退役 stopline（冻结）
1. [ ] 阶段 A（截止 2026-03-20）：`POST /internal/assistant/model-providers:apply` 仅作为迁移入口，并返回 `Deprecation`。
2. [ ] 阶段 B（截止 2026-04-10）：CI/Prod 默认禁用，返回 `410 Gone + assistant_model_provider_apply_deprecated`。
3. [ ] 阶段 C（截止 2026-04-24）：删除路由、handler、文档与前端调用残留。
4. [ ] 同步收口相关接口职责：`GET /model-providers`、`GET /models`、`POST ...:validate` 仅保留只读/校验语义，禁止成为第二写入口。

## 5. 实施切片（100% 闭环顺序）
### PR-230-00：前置契约与门禁补齐（新增）
1. [ ] 新增/修订 `DEV-PLAN-225A`：补 `model_route_snapshot(_hash)` 存储与执行前校验。
2. [ ] 新增 `make check assistant-config-single-source` 设计与脚本契约。
3. [ ] 更新 `docs/dev-plans/012-ci-quality-gates.md` 与 CI，纳入新 gate。

### PR-230-01：官方运行基线落地
1. [ ] 引入官方镜像与环境模板，默认上游切到仓库内 LibreChat。
2. [ ] 固化依赖栈 `api/mongodb/meilisearch/rag_api/vectordb` 的归属、版本 pin、生命周期。
3. [ ] `/app/assistant` 展示不可用诊断（错误码 + 版本标识）。

### PR-230-02：单主源配置收口
1. [ ] 模型/Provider 主写源收敛到 LibreChat。
2. [ ] `assistantModelGateway` 改为只读适配 + 规范化 + 边界校验。
3. [ ] 增加配置迁移与一致性校验脚本（短期只读比对，长期单主源）。

### PR-230-03：开源能力复用落地（MCP/Actions/Allowlist）
1. [ ] 接入并验证 MCP Servers / mcp allowedDomains。
2. [ ] 接入并验证 Actions / actions allowedDomains。
3. [ ] 明确 Agents 暂不复用的边界与触发复评条件。

### PR-230-04：身份边界硬化
1. [ ] 修复 `/assistant-ui/*` 会话校验绕过。
2. [ ] 强化 proxy 最小透传与路径/方法约束。
3. [ ] 增补 e2e：未登录访问、跨租户访问、业务写旁路三类负测。

### PR-230-05：旧入口退役与单主源封板
1. [ ] 执行 `model-providers:apply` A/B/C 三阶段。
2. [ ] 清理双主源残留（路由、handler、前端入口、文档）。
3. [ ] 新 gate 与 no-legacy 联动阻断回流。

### PR-230-06：升级与回归闭环
1. [ ] 固化升级回归脚本（含协议兼容、边界、契约快照）。
2. [ ] 输出首轮 `docs/dev-records/` 证据模板并留档。
3. [ ] 冻结回滚策略：只允许版本回滚，禁止临时 legacy 分支。

## 6. 门禁与验证清单
1. [ ] `make check routing`
2. [ ] `make check capability-route-map`
3. [ ] `make check no-legacy`
4. [ ] `make check error-message`
5. [ ] `make check assistant-config-single-source`（新增）
6. [ ] `make e2e`（含 assistant-ui 负测 + 224/225 契约快照回归）
7. [ ] `make check doc`

## 7. 剪切/缩水策略（允许项与禁止项）
### 7.1 允许的策略性剪切（不冲突）
1. [ ] 暂不复用 Agents 自动执行（保护 One Door 边界）。
2. [ ] 不复用 LibreChat 自身身份体系（保持本仓统一 AuthN/AuthZ/Tenant）。

### 7.2 禁止的缩水（命中即阻断）
1. [ ] 只“口头复用”MCP/Actions/Allowlist，未进入切片与验收。
2. [ ] 保留 `model-providers:apply` 作为长期可写入口。
3. [ ] 未补 224/225 契约映射字段却上线主源切换。
4. [ ] `/assistant-ui/*` 仍可绕过会话校验。
5. [ ] 未将新 gate 纳入 CI 就宣称“单主源完成”。

## 8. 风险与缓解
1. [ ] 风险：上游配置理解不足导致迁移错配。  
   缓解：迁移脚本 + 双向比对 + 回滚演练。
2. [ ] 风险：上游升级引入协议漂移。  
   缓解：升级前跑协议与边界回归，失败阻断。
3. [ ] 风险：为了交付速度回退为自建扩张。  
   缓解：Build-vs-Buy 审批 + 退出策略 + no-legacy/gate 联动。

## 9. 参考资料（开源能力核对）
- `https://www.librechat.ai/docs/quick_start/local_setup`
- `https://www.librechat.ai/docs/quick_start/docker_compose_install`（当前 404，待上游路径确认）
- `https://raw.githubusercontent.com/danny-avila/LibreChat/main/librechat.example.yaml`
- `https://raw.githubusercontent.com/danny-avila/LibreChat/main/README.md`
- `https://raw.githubusercontent.com/danny-avila/LibreChat/main/docker-compose.yml`

## 10. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`

## 11. 子计划拆分（直接落地）
- [ ] DEV-PLAN-231（前置契约与门禁）：`docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- [ ] DEV-PLAN-232（官方运行基线）：`docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- [ ] DEV-PLAN-233（单主源配置收口）：`docs/dev-plans/233-librechat-single-source-config-convergence-plan.md`
- [ ] DEV-PLAN-234（MCP/Actions/Allowlist 复用落地）：`docs/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md`
- [ ] DEV-PLAN-235（身份/会话/租户边界硬化）：`docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- [ ] DEV-PLAN-236（旧入口退役与单主源封板）：`docs/dev-plans/236-librechat-legacy-endpoint-retirement-and-single-source-closure-plan.md`
- [ ] DEV-PLAN-237（升级与回归闭环）：`docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
