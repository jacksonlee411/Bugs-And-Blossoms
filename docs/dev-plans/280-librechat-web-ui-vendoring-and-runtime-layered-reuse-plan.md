# DEV-PLAN-280：LibreChat Web UI 源码纳管与 Runtime 分层复用实施方案

**状态**: 规划中（2026-03-07 22:25 CST）

## 1. 背景与重开原因
- `DEV-PLAN-230` 将 LibreChat 集成冻结为“官方运行基线复用 + 本仓边界适配”，这一决策对运行态落地是正确起点；但随着 `DEV-PLAN-260` 将目标提升为“真实业务对话闭环”，现有模式的能力边界已出现结构性错位。
- 当前项目主要复用了 **LibreChat 官方已编译 runtime**：`deploy/librechat/` 负责上游镜像与 compose 基线；`/assistant-ui/*` 通过反向代理接入上游页面；`/app/assistant/librechat` 再通过 iframe + bridge 与本仓业务链路拼接。
- 这种“黑盒 runtime + 外层桥接编排”的方式，在以下方面已成为 `260` 的主要阻力：
  1. [ ] 发送链路依赖 DOM 拦截与 `postMessage`，不是消息管线内生控制。
  2. [ ] 业务回执依赖注入容器/外挂流，难以严格等同于官方 assistant 气泡体系。
  3. [ ] 业务 FSM 主要停留在本仓前端 helper，未真正进入官方 UI 的消息生命周期。
  4. [ ] 一旦上游 DOM、按钮、表单、消息列表结构变化，`260/266` 很容易回归退化。
- 因此本计划提出新的分层路线：
  - **保留上游 runtime 镜像复用**；
  - **仅将 LibreChat Web UI 源码 vendoring/patch 到本仓编译**，拿到发送、消息渲染、会话 UI 的源码级控制；
  - **后端 runtime、MCP、Actions、Allowlist、模型配置等继续尽量复用上游能力**。

## 2. 目标与非目标

### 2.1 核心目标
1. [ ] 保持 `DEV-PLAN-232/234/235/237` 的上游 runtime 复用原则：LibreChat API、MongoDB、Meilisearch、RAG API、VectorDB 仍以上游镜像/compose 为运行事实源。
2. [ ] 将 LibreChat **Web UI 源码**纳入本仓，并由本仓统一构建、打包、发布与回归验证。
3. [ ] 将当前依赖 DOM 拦截/注入的发送与回写逻辑，替换为 **源码级发送管线接管 + 源码级消息渲染接入**。
4. [ ] 让 `260` 所需的缺字段补全、多候选确认、提交确认、成功/失败回执，都落到 **官方消息列表/官方 assistant 气泡体系** 内，而非外挂容器。
5. [ ] 将 `/app/assistant/librechat` 收敛为单一真实入口，不再依赖 iframe 套壳作为正式交互承载面。
6. [ ] 保持 One Door：任何业务写入仍只允许经本仓 `/internal/assistant/*` 与业务提交链路完成，绝不把可写业务能力下放到上游 runtime。
7. [ ] 明确业务事实源：业务真相以本仓 `conversation_id/turn_id/request_id/trace_id` 与其状态流转为准；官方消息树只是唯一用户可见渲染面，不得反客为主成为业务事实源。
8. [ ] 明确前端降权：vendored UI 只消费后端返回的 `phase/candidates/draft/commit-reply` 等 DTO，不得在页面 helper / adapter 内重算业务 FSM、候选裁决或提交约束。

### 2.2 非目标
1. [ ] **不** vendoring LibreChat 后端 Node 服务，不在本计划中接管上游 API/runtime 实现。
2. [ ] **不** 自建第二套 MCP/Actions/模型配置中心；继续遵循 `DEV-PLAN-233/234` 的“上游主源 + 本仓校验”原则。
3. [ ] **不** 在本计划中改写 `260` 的业务语义本身；`280` 只解决“承载面与控制点层级错位”，不替代 `260` 主计划。
4. [ ] **不** 引入 legacy 双链路；迁移期间允许受控灰度，但正式入口只能有一条用户可见交互链路。

## 2.1 工具链与门禁（SSOT 引用）
- **本计划命中触发器**：
  - [X] 文档变更（`make check doc`）
  - [ ] Go 代码（网关/静态资源服务/代理收口）
  - [ ] MUI / Web UI / presentation assets
  - [ ] E2E（官方 UI 真实回归）
  - [ ] Routing（入口切换、旧路由退役）
  - [ ] Assistant 配置单主源 / No Legacy / 错误提示门禁
- **SSOT 入口**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`、`docs/dev-plans/012-ci-quality-gates.md`

## 3. 问题陈述：为什么 230 路线不足以支撑 260

### 3.1 当前模式的本质
```mermaid
graph TD
    A[浏览器 /app/assistant/librechat] --> B[本仓页面 iframe]
    B --> C[/assistant-ui/* 反向代理]
    C --> D[LibreChat Upstream Runtime]
    A --> E[本仓 helper / bridge / FSM]
    E --> F[/internal/assistant/*]
    E --> B
```

当前结构的问题不在于“能否跑起来”，而在于：
1. [ ] **消息发送控制点过晚**：要到 DOM 事件层才能拦截“官方发送”。
2. [ ] **消息落点非官方内生**：即使视觉上在聊天区附近，也可能仍是注入式容器。
3. [ ] **业务状态与 UI 状态割裂**：`conversation_id/turn_id/request_id` 与官方前端消息实体不是同一套 store。
4. [ ] **回归成本高**：上游 UI 结构漂移时，本仓要先修 DOM 兼容，再谈业务闭环。

### 3.2 对 `260` 的直接影响
1. [ ] 无法从架构层保证“同轮唯一 assistant 回复”；更多依赖 E2E 证明“暂时没坏”。
2. [ ] 无法从源码级保证“官方原始发送未发出”；更多依赖桥接统计与外围证据。
3. [ ] `Case 2~4` 的业务 FSM 更像“外挂 orchestrator 驱动的结果贴回”，而不是官方聊天消息生命周期的一部分。
4. [ ] `266` 的“气泡内回写”难以硬化为长期稳定契约，容易在上游升级时回退为外挂流。

## 4. 新分层原则（280 目标态）

### 4.1 总体分层
```mermaid
graph TD
    A[浏览器 /app/assistant/librechat] --> B[本仓编译的 LibreChat Web UI]
    B --> C[本仓 BFF / 受控代理层]
    C --> D[LibreChat Upstream Runtime 镜像]
    B --> E[本仓 Assistant 对话编排接口]
    E --> F[One Door 业务提交链]
```

### 4.2 分层冻结
| 层 | 所有权 | 复用策略 | 说明 |
| --- | --- | --- | --- |
| LibreChat runtime（api/mongo/meili/rag/vectordb） | Upstream + 本仓部署封装 | 继续复用 | 仍由 `deploy/librechat/` 管理 |
| LibreChat Web UI 源码 | 本仓纳管 | 新增纳管 | 进入本仓构建链，允许 patch |
| 发送动作与消息渲染 | 本仓 patch | 源码级接管 | 禁止继续依赖 DOM 劫持 |
| 业务 FSM 与确认语义 | 本仓 | 与 `260` 对齐 | 进入正式 message pipeline，而非外挂 helper |
| 业务事实源（conversation/turn/request/trace） | 本仓持久化与审计层 | `223 + 260` | 持久化会话/回合/状态转移审计 | 官方消息树只负责渲染，不得成为业务真相 |
| MCP / Actions / Allowlist / 模型配置 | Upstream 主源 + 本仓校验 | 继续复用 | 不建设第二配置中心 |

## 5. 关键设计决策（ADR 摘要）

### ADR-280-01：保留上游 runtime 镜像复用（选定）
- 选项 A：runtime 与 Web UI 全部 vendoring。缺点：fork 面过大，升级成本过高。
- 选项 B（选定）：runtime 继续复用官方镜像；只在 Web UI 层拿回源码控制权。

### ADR-280-02：只纳管 LibreChat Web UI，而非整个上游仓库（选定）
- 选项 A：完整 vendoring LibreChat monorepo。缺点：Node backend、worker、infra 一并进入本仓，责任面失控。
- 选项 B（选定）：只纳管 Web UI 必需源码/构建资产，并保留对上游 runtime 的 API 兼容。

### ADR-280-03：`/app/assistant/librechat` 直接承载官方 UI，不再以 iframe 作为正式入口（选定）
- 选项 A：继续 iframe，仅把桥逻辑改成更深 patch。缺点：消息流仍跨窗口，状态仍然分裂。
- 选项 B（选定）：直接以本仓编译的官方 UI 页面作为正式承载面；若需兼容 `/assistant-ui/*`，仅保留短期灰度别名。

### ADR-280-04：发送与回执必须进入源码级消息管线（选定）
- 选项 A：继续 DOM 事件 `preventDefault` + `postMessage`。缺点：脆弱、不可维护。
- 选项 B（选定）：在 vendored UI 的发送 action / store / renderer 内接入本仓业务语义，确保消息实体是第一类对象。

### ADR-280-05：业务回执必须渲染为“官方 assistant message”，禁止外挂容器（选定）
- 选项 A：继续向 DOM 追加 bridge stream。缺点：违反 `260/266` 的长期目标。
- 选项 B（选定）：本仓生成的草案、缺字段、多候选、成功/失败回执都进入官方消息列表的数据模型与组件树。

### ADR-280-06：patch 必须显式化与可升级（选定）
- 选项 A：直接在 vendored 源码里散改。缺点：升级不可审计。
- 选项 B（选定）：保留上游来源信息、patch 清单、版本锁与回归清单，形成“可重复升级”的 patch stack。

### ADR-280-07：业务事实源以后端会话/回合为准（选定）
- 选项 A：以官方前端消息树为事实源。缺点：会把 UI 状态与业务状态混同，无法保证 `223/260` 的事务与审计语义。
- 选项 B（选定）：以本仓 `conversation/turn/request/trace` 为唯一业务真相；官方消息树只承担唯一用户可见渲染职责。

### ADR-280-08：前端降权，只消费 DTO（选定）
- 选项 A：继续让页面 helper / adapter 承担候选判断、确认词语义与提交前置校验。缺点：会把旧的“DOM hack”升级成“源码 patch hack”。
- 选项 B（选定）：业务 FSM、候选裁决、确认约束以后端为 SSOT；vendored UI 只消费后端返回的 `phase/candidates/draft/commit-reply` DTO 并负责渲染。

## 6. 仓库布局与资产模型（目标态）

### 6.1 目录建议
1. [ ] `third_party/librechat-web/`：上游 Web UI 源码快照（只纳管必要部分）。
2. [ ] `third_party/librechat-web/UPSTREAM.yaml`：记录来源仓库、commit/tag、导入时间、回滚基线。
3. [ ] `third_party/librechat-web/patches/`：本仓 patch 清单，按主题拆分（send-pipeline / message-render / auth-shell / assistant-adapter）。
4. [ ] `scripts/librechat-web/`：同步、校验、构建、升级辅助脚本。
5. [ ] `internal/server/assets/librechat-ui/` 或等价目录：构建产物归档路径。

### 6.2 资产约束
1. [ ] vendored UI 必须有单一来源元数据，不得出现“手抄文件 + 无来源”的隐式纳管。
2. [ ] patch 必须可枚举、可审计、可在升级时逐个回放与冲突处理。
3. [ ] 本仓不直接编辑上游构建产物；所有变更都应回到 vendored 源码或 patch 层。

## 7. 核心技术方案

### 7.0 业务事实源与前端职责冻结
1. [ ] 业务真相固定为本仓持久化的 `conversation_id/turn_id/request_id/trace_id + phase + 审计状态转移`；官方消息树不是业务真相，只是唯一用户可见渲染面。
2. [ ] vendored UI 只能消费后端 DTO（如 `phase/candidates/draft/commit_reply/error_code`），不得在前端 helper 中重新计算候选解析、确认判定、提交约束或状态推进规则。
3. [ ] 若前端需要临时 adapter，只能做展示层归一、事件分发与协议适配，不得承载业务判定。
4. [ ] `223/260` 是业务事实源与业务 FSM 的 SSOT；`280` 负责承载面与控制点收口，不得与其冲突。

### 7.1 UI 承载面收口
1. [ ] `/app/assistant/librechat` 改为直接加载本仓构建的 vendored LibreChat Web UI。
2. [ ] `/assistant-ui/*` 若保留，只能作为迁移别名或调试入口，不再作为 iframe 套壳的正式承载面。
3. [ ] 迁移完成后，`apps/web/src/pages/assistant/LibreChatPage.tsx` 不再承担业务桥接 orchestrator 角色，只保留必要入口外壳或直接退役。

### 7.2 发送链路接管
1. [ ] 在 vendored UI 的发送 action / composer 提交路径中加入本仓 adapter：
   - 识别用户输入；
   - 将业务相关输入转发至本仓 `Assistant` 对话接口；
   - 禁止官方原始发送与本仓业务发送并行发出。
2. [ ] “是否进入本仓业务链路”必须在源码级决策，不再依赖按钮文本、表单 DOM 或跨窗口事件拦截。
3. [ ] 若该轮属于普通聊天而非业务闭环请求，可按明确规则转发至上游原生模型聊天；但该规则必须受 `260/263/264/265` 与单链路策略约束。

### 7.3 消息渲染接管
1. [ ] 缺字段提示、多候选列表、候选确认、提交确认、成功/失败回执，都必须构造为官方消息列表中的 assistant message 实体。
2. [ ] 这些消息实体所承载的业务语义必须来自后端 DTO 与持久化状态，不得由前端根据文本或局部上下文自行推断。
2. [ ] 官方 UI 组件应直接消费这些消息实体，不得再通过 `document.createElement(...)` 方式注入额外流。
3. [ ] 每条业务回执都必须带上 `conversation_id/turn_id/request_id/trace_id` 的可追溯元数据，并能与唯一 assistant message 一一对应。

### 7.4 BFF / 代理边界
1. [ ] 本仓保留受控代理/BFF，用于：会话衔接、cookie/headers 归一、运行态健康探测、上游 API 转发边界。
2. [ ] 但 BFF 不再承担“通过 HTML 注入 bridge.js 篡改消息流”的职责。
3. [ ] 任何为 `260` 服务的业务编排都必须进入正式 API/消息模型，不得继续藏在 HTML rewrite/注入脚本中。

### 7.5 与 `260` 的关系
1. [ ] `280` 是 `260` 的承载面改造前置计划，不替代 `260` 主计划。
2. [ ] `280` 完成后，`260` 的 Case 2~4 才真正具备“源码级可落地空间”：
   - 缺字段补全；
   - 多候选选择；
   - 提交确认；
   - 成功/失败回执。
3. [ ] 若 `280` 未完成，即使短期 E2E 通过，也不视为 `260` 已获得稳定、可升级的实现底座。

## 8. 迁移策略与停止线

### 8.1 迁移阶段
1. [ ] **M1：资产纳管与来源冻结**
   - 引入 vendored Web UI 源码、来源元数据、patch 清单与构建脚本。
2. [ ] **M2：本仓构建与静态发布闭环**
   - 本地/CI 能从 vendored 源码稳定构建官方 UI 产物并由本仓服务。
3. [ ] **M3：源码级发送链路接管**
   - 正式移除 DOM 级原始发送拦截与跨窗口桥接发送。
   - readiness：`235` 中新的 LibreChat UI 入口会话/租户边界已补齐；否则不得切正式入口。
4. [ ] **M4：源码级消息渲染接管**
   - 正式移除外挂 dialog stream / DOM 注入式回执。
   - readiness：`223` 已明确业务事实源字段与审计回放口径，`260` 已冻结 phase / candidate / draft / commit-reply DTO 契约。
5. [ ] **M5：260 Case 1~4 回归闭环**
   - 在新承载面上重跑 `260/266/263/264/265` 真实回归集。
   - readiness：`237` 已纳入 vendored UI source + patch stack + runtime compatibility 回归项。
6. [ ] **M6：旧桥接退役与封板**
   - 下线 iframe、bridge.js 注入、外挂回执容器与相关 legacy 测试口径。
   - 明确退役对象最少包括：`iframe`、`bridge.js`、HTML 注入、`data-assistant-dialog-stream`、`assistantDialogFlow`、`assistantAutoRun` 以及等价的页面外桥接业务职责。

### 8.2 停止线（Fail-Closed）
1. [ ] 若 vendored UI 无法稳定构建，不允许回退到“临时继续堆 bridge.js 逻辑”作为正式解法。
2. [ ] 若消息仍通过外挂容器显示，则 `280` 不得宣称通过。
3. [ ] 若同轮仍存在双发送或双回复，则 `260/266` 均视为未满足前置条件。
4. [ ] 若迁移后引入第二业务写入口或绕开 One Door，立即阻断。
5. [ ] 若页面 helper / adapter 仍承担业务 phase 推进、候选裁决或提交约束，则 `280` 不得宣称“前端降权”完成。
6. [ ] 若 `assistantDialogFlow`、`assistantAutoRun` 或等价逻辑仍承担正式用户可见业务职责，则 `M6` 视为未完成。

## 9. 风险与应对
1. [ ] **风险：前端源码纳管后升级成本上升**。
   - 处置：限制纳管范围为 Web UI；维护来源元数据 + patch stack + 回归基线。
2. [ ] **风险：上游 Web UI 架构变化导致 patch 失效**。
   - 处置：将 patch 聚焦在发送/store/render 三个明确控制点，避免散改全局。
3. [ ] **风险：本仓与上游 runtime API 版本漂移**。
   - 处置：在 `237` 升级闭环中新增 UI source/runtime compatibility 回归项。
4. [ ] **风险：迁移期间出现双入口**。
   - 处置：以 `no-legacy` 为硬门槛，正式入口只保留一个；别名入口仅限短期灰度，并有明确下线日期。

## 10. 测试与验收标准

### 10.1 覆盖率与统计范围
- 覆盖率口径与仓库级门禁以 `AGENTS.md`、`Makefile`、CI workflow 为 SSOT。
- 本计划重点不在扩大排除项，而在把“难以测试的 DOM hack 逻辑”替换成“可单测的源码级 action/store/render adapter”。
- 若未来新增 vendored UI patch 代码，必须优先通过更小职责拆分与可测试 adapter 提升可测性，不得以扩大排除范围替代设计修正。

### 10.2 验收标准（硬门槛）
1. [ ] `/app/assistant/librechat` 不再依赖 iframe 作为正式聊天承载面。
2. [ ] 不再依赖运行时注入 `bridge.js` 才能阻断原始发送或显示业务回执。
3. [ ] 不再存在 `data-assistant-dialog-stream` 或等价外挂消息流承担用户可见业务回执职责。
4. [ ] `260` Case 1~4 中，所有业务回执都由官方消息列表组件树渲染，且每轮仅有唯一 assistant 回复实体。
5. [ ] 前端只消费后端 `phase/candidates/draft/commit-reply` 等 DTO；业务事实源仍以本仓 `conversation/turn/request/trace` 与审计状态转移为准。
6. [ ] 发送、缺字段、多候选、确认、提交成功/失败的关键路径，都能通过源码级单测/组件测 + 真实 E2E 双重验证。
7. [ ] 上游 runtime 镜像基线仍可独立启动、健康检查、升级与回滚，不因 UI 源码纳管而退化。

## 11. 实施里程碑
1. [ ] **280A**：LibreChat Web UI 源码纳管与来源锁定。
2. [ ] **280B**：本仓 UI 构建链与静态发布接线。
3. [ ] **280C**：发送 action / store 级单通道接管。
4. [ ] **280D**：消息渲染模型收口（官方气泡内回写）。
5. [ ] **280E**：`260` Case 1~4 真实回归闭环与旧桥退役。

## 12. 交付物
1. [ ] `DEV-PLAN-280` 主计划文档。
2. [ ] vendored Web UI 来源元数据与 patch 清单。
3. [ ] 构建/同步/升级脚本与回归清单。
4. [ ] 与 `260/266/237` 对齐的测试证据与执行日志。

## 13. 关联文档
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- `docs/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/270-project-container-deployment-review-and-layered-convergence-plan.md`
- `AGENTS.md`
