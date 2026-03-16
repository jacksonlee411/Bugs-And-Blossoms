# DEV-PLAN-230：LibreChat 项目级集成总纲（历史基线修订）

**状态**: 长期有效（2026-03-17 02:48 CST 修订）— 自 2026-03-07 起，`DEV-PLAN-230` 不再作为 LibreChat UI 承载面与交互链路的主约束；截至 2026-03-09，`DEV-PLAN-280/260/266/285` 已分别完成主架构、业务闭环、单通道 stopline 与封板回归，因此本文继续只保留仍有效的“项目级分层原则、runtime 复用、配置主源、边界不变量与子计划关系”。

> 口径说明：本文未勾选条目默认表示持续有效的项目级约束/不变量，不表示当前主线仍存在一轮待封板的单次实施缺口。

## 1. 背景与适用性冻结
- `230` 的原始目标，是把 LibreChat 纳入本仓项目级治理，明确“哪些能力复用上游、哪些边界必须由本仓自建”。
- 这一目标本身仍然成立，但原文中把“聊天 UI 壳层 = 仅保留 iframe + proxy”写成硬约束，已经与后续需求演进发生冲突：
  1. [ ] `DEV-PLAN-260` 已将目标升级为“真实业务对话闭环”，要求缺字段补全、多候选确认、提交确认、成功/失败回执都在同一官方聊天流中完成。
  2. [ ] `DEV-PLAN-266` 已将“单通道、官方气泡内回写、无外挂容器、无官方原始错误体验”冻结为前置 stopline。
  3. [ ] `DEV-PLAN-280` 已明确新的主架构方向：**保留上游 runtime 镜像复用，但将 LibreChat Web UI 源码 vendoring/patch 到本仓编译，拿回发送、消息渲染、会话 UI 的源码级控制权**。
- 因此，自本次修订起：
  - [X] `230` **不再约束**“必须 iframe + proxy 承载官方 UI”。
  - [X] `230` **不再约束**“`/assistant-ui/*` 必须是正式用户入口”。
  - [X] `230` **不再接受** bridge.js / DOM 注入 / 外挂消息流作为长期正式解法。
  - [X] `230` 继续保留并约束：runtime 复用、配置主源、MCP/Actions 复用、AuthN/AuthZ/Tenant 边界、One Door、升级闭环。

### 1.1 当前角色（2026-03-17 冻结）
1. [X] `230` 当前是 LibreChat 项目级治理基线文档，而不是待单次封板的实施主计划。
2. [X] UI 承载面、消息控制点、渲染链路与正式入口相关实现已经由 `280/266/285` 收口；真实业务闭环由 `260` 收口。
3. [X] 后续若继续做 LibreChat 相关演进，仍需回到 `230` 复核 runtime 复用、配置单主源、边界不变量与升级闭环是否被破坏。

## 2. 文档优先级（自本次修订起冻结）
| 主题 | 当前 SSOT | `230` 的角色 |
| --- | --- | --- |
| LibreChat UI 主架构、承载面、发送/渲染控制点 | `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md` | 历史背景，不再约束 |
| AI 对话真实业务闭环（Case 1~4、FSM、确认语义） | `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md` | 提供背景，不再主导 |
| 官方 UI 单通道、气泡内回写、无外挂回执 | `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md` | 前置 stopline 来源 |
| LibreChat runtime 运行基线（镜像/compose/env） | `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md` | 仍有效 |
| 模型配置单主源 | `docs/dev-plans/233-librechat-single-source-config-convergence-plan.md` | 仍有效 |
| MCP / Actions / Allowlist 复用 | `docs/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md` | 仍有效 |
| Auth / Session / Tenant 边界 | `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md` | 仍有效，但路径口径需服从 280 新承载面 |
| 升级与回归闭环 | `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md` | 仍有效，且需纳入 vendored UI patch 回归 |

## 3. 经修订后仍有效的核心目标
1. [ ] **运行时复用优先**：LibreChat runtime（API、MongoDB、Meilisearch、RAG API、VectorDB）仍以上游镜像与 `deploy/librechat/` 运行为事实源。
2. [ ] **配置单主源**：模型/Provider 配置仍由 LibreChat 主源驱动，本仓只做只读适配、校验与审计；不得回流第二写入口。
3. [ ] **开源能力复用优先**：MCP / Actions / Domain Allowlist 等继续遵循“上游主源 + 本仓 fail-closed 校验”原则。
4. [ ] **边界不下放**：One Door、RLS、Casbin、AuthN/AuthZ、Tenant 注入始终留在本仓。
5. [ ] **项目级可升级**：LibreChat 集成必须具备可追溯来源、可回归验证、可升级、可回滚的闭环。

## 4. 明确失效/不再适用的旧约束

### 4.1 以下内容自本次修订起失效
1. [X] “聊天 UI 壳层（会话/消息/输入/流式） = Must Reuse，且仅保留 iframe + proxy”。
2. [X] “`/assistant-ui/*` 必须作为正式聊天承载面”。
3. [X] 通过 HTML rewrite 注入 bridge.js、拦截 DOM 发送、外挂回执流等方式，作为长期正式方案。
4. [X] 将“视觉上看似在聊天区内”视作“官方气泡内回写”成立依据。

### 4.2 失效原因（冻结）
1. [ ] 这些约束只能保证“复用已编译 runtime”，不能保证 `260` 所要求的源码级发送控制、官方消息模型内回写与稳定 FSM 落地。
2. [ ] 它们把关键控制点放在 DOM/iframe/bridge 层，而不是官方 UI 的 action/store/render 层，升级脆弱性过高。
3. [ ] 它们与 `280` 已选定的“runtime 继续复用、Web UI 源码纳管”路线直接冲突。

## 5. 经修订后的能力分界矩阵
| 能力域 | 修订后决策 | 系统所有权 | 主数据源 / 事实源 | 说明 |
| --- | --- | --- | --- | --- |
| LibreChat Web UI（会话/消息/输入/流式） | Must Build on Vendored Source | 本仓 UI 层 | vendored UI 源码 + patch 清单 | 以 `280` 为准，不再受“iframe + proxy only”约束 |
| LibreChat runtime（api/mongodb/meilisearch/rag_api/vectordb） | Must Reuse | Upstream + 本仓部署封装 | upstream 镜像 / compose / env | 以 `232/270` 为准 |
| 模型/Provider 配置 | Must Reuse + Adapter-first | LibreChat（配置）+ 本仓（校验） | `librechat.yaml` / `.env` | 以 `233` 为准 |
| MCP / Actions / Allowlist | Must Reuse + Fail-Closed Validate | LibreChat + 本仓 | upstream config + 本仓 allowlist 校验 | 以 `234` 为准 |
| 身份 / 会话 / 租户边界 | Must Build | 本仓网关与中间件 | Kratos Session + Casbin + Tenant Resolver | 适用于所有 LibreChat UI 暴露路径，而非仅 `/assistant-ui/*` |
| 业务 FSM / 提交裁决链 | Must Build | 本仓 | `/internal/assistant/*` + One Door | 以 `260` 为准，永不下放到上游 runtime |
| 升级 / 回归闭环 | Must Build | 本仓 | 回归脚本、来源元数据、patch stack | `237` 需扩展纳入 vendored UI patch 回归 |

## 6. 经修订后的目标态拓扑
```mermaid
graph TD
    A[/app/assistant/librechat] --> B[本仓编译的 LibreChat Web UI]
    B --> C[本仓 BFF / 受控代理层]
    C --> D[LibreChat Upstream Runtime]
    B --> E[/internal/assistant/*]
    E --> F[One Door 业务提交链]
```

### 6.1 解释
1. [ ] 正式聊天承载面由本仓编译的 vendored LibreChat Web UI 提供。
2. [ ] runtime 仍由上游镜像提供，不将 Node backend vendoring 进本仓。
3. [ ] 本仓仍保留 BFF/代理层处理会话、边界、防旁路与运行诊断，但不再以 DOM 注入篡改消息流作为正式方案。
4. [ ] 业务相关消息通过正式消息模型与组件树落入官方 UI，而不是外挂容器。

## 7. 经修订后仍然有效的边界约束
1. [ ] **One Door 不变**：所有业务写入必须走本仓 `Assistant` 对话与提交链路。
2. [ ] **No Legacy 不变**：不得因 UI 架构切换而引入第二正式入口或长期双链路。
3. [ ] **单主源配置不变**：不得恢复 `model-providers:apply` 之类第二写入口。
4. [ ] **Auth / Session / Tenant 边界不变**：所有 LibreChat UI 入口都必须服从本仓统一会话与租户边界；不得因路径调整出现绕过。
5. [ ] **开源能力复用边界不变**：MCP / Actions / Allowlist 仍优先复用上游，不建设第二配置中心。
6. [ ] **升级与回归不变**：任何 upstream runtime 或 vendored UI patch 升级都必须过回归闭环。

## 8. 对原实施切片的修订

### 8.1 保留有效的切片
1. [X] `231`：前置契约与门禁补齐（已完成）。
2. [X] `232`：官方 runtime 基线落地（已实施）。
3. [X] `233`：配置单主源收口。
4. [X] `234`：MCP / Actions / Allowlist 复用落地。
5. [X] `235`：身份 / 会话 / 租户边界硬化（已完成）。
6. [X] `237`：升级与回归闭环（长期契约保留；本轮封板前置执行由 `291` 完成）。

### 8.2 不再由 `230` 主导的切片
1. [X] “聊天 UI 壳层仅保留 iframe + proxy”的要求从 `230` 删除，改由 `280` 统一设计与实施。
2. [X] `260` 的业务 FSM、Case 1~4、确认语义、补全语义不再由 `230` 定义，改由 `260` 作为主计划。
3. [X] `266` 的单通道、气泡内回写、无外挂容器 stopline 不再由 `230` 解释，改由 `266` 直接约束。

### 8.3 新的承接关系
1. [ ] `280` 作为 LibreChat UI 主架构计划，承接并替代 `230` 中所有与 UI 承载面、消息控制点、渲染管线有关的旧约束。
2. [ ] `260` 作为业务主计划，定义真实业务闭环目标。
3. [ ] `266` 作为 UI / 通道前置子计划，为 `260` 提供单通道与官方消息落点前提。

## 9. 门禁与验证清单（修订后口径）
1. [ ] `make check assistant-config-single-source`
2. [ ] `make check no-legacy`
3. [ ] `make check routing`
4. [ ] `make check capability-route-map`
5. [ ] `make check error-message`
6. [ ] `make e2e`（包含 `260/266/237` 关联回归集）
7. [ ] `make check doc`

> 说明：凡涉及 UI 承载面、消息发送、消息渲染、iframe/bridge 退役、vendored UI 构建等验证，均应以 `280` 的测试与验收标准为 SSOT；`230` 不再单独定义该类验收口径。

## 10. 风险与缓解（修订后）
1. [ ] 风险：团队继续按旧印象把 `230` 视为“iframe + proxy 必须保留”的约束。  
   缓解：本次修订已冻结优先级；遇到 UI 架构问题一律引用 `280`。
2. [ ] 风险：迁移期间出现“runtime 继续复用”与“UI 源码纳管”之间的责任混淆。  
   缓解：明确 runtime 看 `232/270`，UI 看 `280`，业务闭环看 `260`。
3. [ ] 风险：以“230 失效”为由，连带放松单主源、边界、升级闭环等仍有效约束。  
   缓解：本次修订已明确这些约束继续有效，不得借机回退。

## 11. 关联文档
- `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- `docs/dev-plans/233-librechat-single-source-config-convergence-plan.md`
- `docs/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/270-project-container-deployment-review-and-layered-convergence-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `AGENTS.md`
