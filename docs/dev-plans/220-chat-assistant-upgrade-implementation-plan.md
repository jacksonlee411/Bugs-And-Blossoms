# DEV-PLAN-220：聊天框式 AI 助手升级总纲（历史基线修订）

**状态**: 进行中（2026-03-07 22:05 CST）— 自 2026-03-07 起，`DEV-PLAN-220` 不再作为 AI 助手 UI 承载面、交互模式、路由拓扑与提交交互的主约束。当前架构 SSOT 以 `DEV-PLAN-280` 为准，业务需求与验收 SSOT 以 `DEV-PLAN-260/266` 为准，持久化与审计以 `DEV-PLAN-223` 为准，模型与意图治理以 `DEV-PLAN-224/224A` 为准。本文仅保留“为何引入聊天式助手、哪些业务边界必须保留在本仓、哪些旧假设已失效”的历史总纲作用。

## 1. 背景与适用性冻结
- `220` 是项目早期把 AI 助手从“分散治理页 + 内部接口”升级为“聊天框式工作台”的总起点文档。
- 它当时做出的关键判断有两类：
  1. [X] **仍然正确的判断**：引入 LibreChat 作为聊天承载层，但不下放业务提交裁决；One Door、AuthZ、租户边界、业务审计仍保留在本仓。
  2. [X] **已不再适用的判断**：把 LibreChat 视为“左侧 iframe 聊天壳”，把确认/提交按钮放在右侧事务面板，并以 `postMessage + proxy + iframe` 作为正式交互结构。
- 在 `220` 编写时，这是一条合理的探索路线；但随着后续计划推进，以下新事实已经冻结：
  1. [ ] `DEV-PLAN-260` 已将目标提升为“真实业务对话闭环”，要求缺字段补全、多候选确认、提交确认、成功/失败回执都在对话内完成。
  2. [ ] `DEV-PLAN-266` 已将“单通道、官方气泡内回写、无外挂容器、无官方原始错误体验”冻结为 UI 前置 stopline。
  3. [ ] `DEV-PLAN-280` 已将主架构切换为“保留上游 runtime 镜像复用，但将 LibreChat Web UI 源码 vendoring/patch 到本仓编译，拿回发送、消息渲染、会话 UI 的源码级控制权”。
- 因此自本次修订起，`220` 不再约束：
  - [X] `iframe + /assistant-ui/* + postMessage` 必须存在；
  - [X] “左聊天、右提交控制台”必须是正式交互形态；
  - [X] `Confirm/Commit` 必须由页面外按钮完成；
  - [X] `/app/assistant` 必须是唯一正式交互入口。

## 2. 文档优先级（自本次修订起冻结）
| 主题 | 当前 SSOT | `220` 的角色 |
| --- | --- | --- |
| LibreChat UI 主架构、发送链路、消息渲染、承载面 | `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md` | 历史背景，不再约束 |
| AI 对话真实业务闭环（Case 1~4、FSM、对话确认） | `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md` | 历史背景，不再约束 |
| 官方 UI 单通道、气泡内回写、无外挂容器 | `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md` | 历史背景，不再约束 |
| 会话持久化与审计闭环 | `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md` | 仍有效 |
| 多模型与意图治理 | `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md` | 仍有效 |
| 真实 Codex API / 多轮工作台 | `docs/dev-plans/224a-assistant-codex-live-api-and-multi-turn-workspace-plan.md` | 仍有效 |
| LibreChat runtime 基线 | `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md` | 仍有效 |
| LibreChat UI 分层复用 | `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md` | 当前主架构 |

## 3. 经修订后仍有效的核心判断
1. [ ] **聊天式交互方向仍成立**：AI 助手应当是可发现、可操作、可回放的对话式工作台，而不是散落在各治理页的点状能力。
2. [ ] **业务裁决边界仍成立**：LibreChat 或其 UI 承载层不直接拥有业务提交裁决权；真正的 `confirm/commit`、租户校验、AuthZ、One Door 始终留在本仓。
3. [ ] **会话事务化仍成立**：对话不是一次性 prompt，而是有 `conversation / turn / request / trace / audit` 的事务语义。
4. [ ] **可审计与可回放仍成立**：用户看见的交互结果必须能回溯到稳定的会话与审计记录。
5. [ ] **聊天 UI ≠ 业务真相** 仍成立：UI 只是承载面，业务事实源仍在本仓会话、任务、审计与提交链路中。

## 4. 明确失效/不再适用的旧约束

### 4.1 以下内容自本次修订起失效
1. [X] “P0 = 左侧 LibreChat iframe，右侧事务控制面板”的正式形态约束。
2. [X] “`/app/assistant` 双面板工作台”必须作为正式验收入口。
3. [X] “`Confirm/Commit` 仅在右侧按钮出现”的交互约束。
4. [X] “`/assistant-ui/*` 反向代理 + iframe 嵌入”作为正式承载结构。
5. [X] “通过 `postMessage` 把计划摘要/会话定位回传给宿主页”的长期正式方案。
6. [X] “高风险信息必须通过页面右侧卡片/控制面板表达”的强制性要求。

### 4.2 失效原因（冻结）
1. [ ] `260` 已要求所有关键业务步骤都通过对话完成，不能再依赖页面外按钮、面板或隐藏状态提示完成业务确认。
2. [ ] `266` 已要求所有业务回执位于官方聊天流内，不再允许外挂容器、外置控制面板承担正式业务回执职责。
3. [ ] `280` 已决定拿回 Web UI 源码级控制权，旧的 iframe/proxy/postMessage 方案只能视为阶段性过渡，不再是目标态。
4. [ ] 因此，`220` 中关于“双面板结构、按钮提交、iframe 路由拓扑”的内容，只能保留为历史设计痕迹，不能继续约束实现。

## 5. 经修订后的能力边界
| 能力域 | 修订后决策 | 说明 |
| --- | --- | --- |
| 聊天 UI 承载面 | 由 `280` 主导 | 使用 vendored LibreChat Web UI，而非强制 iframe 套壳 |
| 业务 FSM / 对话确认 / 补全 / 候选 | 由 `260` 主导 | 所有正式业务闭环通过对话完成 |
| 官方 UI 单通道 / 气泡内回写 | 由 `266` 主导 | 不再以外挂面板/外挂卡片作为正式回执承载 |
| 会话 / 回合 / 审计 / 回放 | 由 `223` 主导 | `220` 只保留方向性背景 |
| 模型 / 意图治理 | 由 `224/224A` 主导 | `220` 不再定义详细协议 |
| 提交裁决 / One Door / AuthZ / 租户边界 | 仍由本仓主导 | 这是 `220` 仍然有效的底层边界 |

## 6. 经修订后的目标态理解
```mermaid
graph TD
    A[/app/assistant/librechat] --> B[本仓编译的 LibreChat Web UI]
    B --> C[/internal/assistant/*]
    C --> D[会话/回合/审计/任务]
    C --> E[One Door 提交链]
    B --> F[LibreChat Upstream Runtime]
```

### 6.1 解释
1. [ ] 正式用户入口以用户真实对话入口为准，不再由 `220` 强制固定为 `/app/assistant` 双面板。
2. [ ] 会话信息、审计信息、错误码、确认语义可以在官方对话流中表达，不再强制依赖右侧控制面板或外置卡片。
3. [ ] 若仍保留 `/app/assistant`，其角色应服从后续主计划：可作为日志/审计/运行态页，但不再自动等同于正式业务交互入口。

## 7. 仍然有效的安全与业务不变量
1. [ ] LibreChat UI 层不得直连业务数据库。
2. [ ] LibreChat UI 层不得直接拥有业务写库凭据。
3. [ ] 任何业务写入都必须经过本仓 `Assistant` 编排接口与 One Door 提交链。
4. [ ] 租户、身份、授权校验必须在本仓统一完成，且 fail-closed。
5. [ ] 会话与回合必须绑定稳定的 `tenant_id + actor_id + conversation_id` 语义，不得在生命周期内漂移。
6. [ ] 所有拒绝、失败、阻断都必须具有稳定错误码与可审计记录。

## 8. 对原阶段划分的修订

### 8.1 仍保留价值的阶段目标
1. [ ] **P0 的“聊天承载面 + 会话只读闭环”目标仍成立**，但承载方式不再受 `220` 原始 UI 结构约束。
2. [ ] **P1 的“受控提交”目标仍成立**，但确认与提交不再要求通过页面外按钮完成，而应服从 `260` 的对话式确认语义。
3. [ ] **P2 的“编排增强”目标仍成立**，但其前提是 `280/260/266` 提供稳定承载面与单通道。

### 8.2 不再由 `220` 主导的阶段约束
1. [X] `P0 = iframe + reverse proxy + postMessage` 的具体实现路径。
2. [X] `P1 = 右侧事务控制面板按钮提交` 的具体交互形态。
3. [X] `P0/P1` 的 UI 布局、按钮位置、消息回填方式。

## 9. 门禁与验证清单（修订后口径）
1. [ ] `make check doc`
2. [ ] `make e2e`
3. [ ] `make check no-legacy`
4. [ ] 与会话/审计有关的验证，以 `223` 为 SSOT
5. [ ] 与模型/意图治理有关的验证，以 `224/224A` 为 SSOT
6. [ ] 与 UI 主架构、承载面、单通道有关的验证，以 `280/266` 为 SSOT
7. [ ] 与真实业务闭环 Case 1~4 有关的验证，以 `260` 为 SSOT

## 10. 风险与缓解（修订后）
1. [ ] 风险：团队继续按旧文档实现“双面板 + 外部按钮提交”。  
   缓解：本次修订已冻结优先级；所有交互形态问题一律引用 `260/266/280`。
2. [ ] 风险：误把 `220` 整体作废，连带丢失“业务裁决不下放、会话事务化、可审计可回放”的底层原则。  
   缓解：本次修订已显式保留这些原则为仍有效约束。
3. [ ] 风险：旧测试与截图仍以 `/app/assistant` 双面板为通过依据。  
   缓解：后续验收口径应迁移到 `260/266/280` 的真实入口与 stopline。

## 11. 关联文档
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/224a-assistant-codex-live-api-and-multi-turn-workspace-plan.md`
- `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `AGENTS.md`
