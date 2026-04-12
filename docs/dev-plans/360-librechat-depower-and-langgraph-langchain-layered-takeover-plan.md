# DEV-PLAN-360：LibreChat 硬切剥离与 LangGraph/LangChain 分层接管方案

**状态**: 修订中（2026-04-12 12:02 CST）

## 1. 背景

1. [X] `DEV-PLAN-220/223/260/266/268/280/293` 代表本仓 Assistant/LibreChat 过渡阶段的历史基线；其中“正式聊天入口由本仓控制的 LibreChat Web UI 承载、前端只消费后端 DTO、外部模型只输出 proposal/semantic state、正式 `Intent / Plan / DryRun / PlanHash` 只能来自 authoritative gate 之后的后端快照”等约束，已被后续主线计划吸收并准备退出归档。
4. [X] `DEV-PLAN-341/350` 已进一步确认：Assistant 当前不是第二写入口，但已经出现“第二策略解释器”倾向；后续收敛目标应是让 Assistant 成为统一策略主链的消费方，而不是继续演化为第二套 Agent/Policy 平台。
5. [X] 在上述边界下，如果继续完整保留 LibreChat 的 Agents / MCP / Memory / Search / RAG / Code Interpreter 等平台能力，同时再引入 `LangGraph/LangChain`，则会形成第二套运行时中心、第二套工具目录、第二套记忆机制与第二套检索入口，违背本仓“单主源、单链路、前端降权”的主线方向。
6. [X] `DEV-PLAN-361` 已进一步明确：`360` 中的 `unified policy consumption` 不应被理解为“Assistant/runtime 可各自保留一套本地策略解释”，而应收敛到 `Context Resolver -> 唯一 PDP -> PrecheckProjection` 主链；即便后续由 OPA 承接唯一 PDP 求值实现，这也只是后端策略消费层的实现收敛，不改变 `360` 的 UI/runtime/authoritative backend 三层分工。
7. [X] 考虑本项目仍处于早期阶段，当前最优策略不是“长期兼容迁移”，而是尽早执行硬切剥离：趁耦合尚浅时移除第二平台、第二依赖栈、第二文档主线与第二策略解释器，避免未来回流成本高于现在一次性切断成本。

## 2. 问题定性

### 2.1 已确认的问题

1. [X] 当前 vendored LibreChat Web UI 是合理的正式聊天承载面，但上游 LibreChat runtime 的平台能力并不天然等于本仓 Assistant 的正式运行时能力。
2. [X] 一旦引入 `LangGraph/LangChain` 作为语义编排与只读工具运行时，再继续保留 LibreChat 自带 Agents / MCP / Memory / Search / RAG / Actions / Agent Chain，会造成重复编排、重复记忆、重复工具注册与重复治理。
3. [X] 本仓的 authoritative backend 已经承担会话真相、任务、审计、risk gate、confirm/commit、One Door 与统一策略消费职责；这些不应迁移给 LibreChat，也不应再由 LibreChat 侧能力间接重建。
4. [X] 若继续以“兼容保留、渐进收口”为默认策略，则很容易让 LibreChat runtime、compat API、旧依赖与历史文档入口在主干长期滞留，形成事实上的第二主线。

### 2.2 正式定性

1. [X] 本问题的本质不是“是否继续使用 LibreChat”。
2. [X] 本问题的本质是“当 `LangGraph/LangChain` 进入主链后，LibreChat 应被收敛成什么角色，哪些能力必须退场，哪些能力可以保留为 UI 壳与交互层”。
3. [X] `LangChain` 在本仓中的正式定位应为：
   - 结构化输出、tool calling、middleware、上下文装配与只读工具编排层。
4. [X] `LangGraph` 在本仓中的正式定位应为：
   - 有状态 proposal runtime、checkpoint / interrupt / durable execution 编排层。
5. [X] LibreChat 在本仓中的正式定位应收敛为：
   - 聊天 UI / 对话交互承载层；
   - 可选消息展示与 artifacts 展示层；
   - 不再作为 Agent 平台、记忆平台、检索平台、工具编排平台或业务运行时平台。
6. [X] 本计划的正式执行风格应收敛为：
   - 硬切主链；
   - 删除优先于兼容；
   - 退役优先于并存；
   - 防回流优先于“平滑迁移”。

## 3. 目标与非目标

### 3.1 目标

1. [ ] 冻结一份明确的 LibreChat 能力矩阵：哪些保留、哪些禁用、哪些后移到 `LangGraph/LangChain`、哪些后移到本仓 authoritative backend。
2. [ ] 冻结目标架构：`LibreChat Web UI -> 本仓 Assistant API -> LangGraph/LangChain runtime -> 只读工具/统一策略主链 -> authoritative gate -> CommitAdapter -> OrgUnitWriteService -> DB Kernel`。
3. [ ] 让正式聊天入口继续保留官方对话体验与消息壳，但不再保留第二套 Agent/Memory/Search/MCP 平台。
4. [ ] 保证引入 `LangGraph/LangChain` 后，业务真相、策略真相、提交真相仍全部留在本仓。
5. [ ] 将 Mongo / Meilisearch / RAG API / VectorDB 等仅服务 LibreChat 平台能力的依赖纳入优先删除清单，而不是长期兼容清单。
6. [ ] 明确 `360` 对 `361` 的兼容口径：本计划冻结的是产品分层与职责边界；统一策略主链内部是否由 OPA 承接唯一 PDP，属于后端实现收敛问题，不构成对 LibreChat/LangGraph 角色划分的反向修改。
7. [ ] 将 `220-293` 系列历史 Assistant/LibreChat 过渡期文档的退出归档纳入正式收口范围：这些文档不再作为 `AGENTS.md` 文档地图中的现行入口，而仅保留为待归档历史基线。
8. [ ] 冻结硬切原则：正式主链不保留长期 compat runtime、长期双路 API、长期双依赖栈、长期双文档入口。

### 3.2 非目标

1. [ ] 不把本项目改造成通用 AI 平台或开放式 Agent Marketplace。
2. [ ] 不允许 `LangGraph/LangChain` 直接拥有业务写库权、绕过 authoritative gate 或形成第二写入口。
3. [ ] 不在本计划内直接重做正式前端入口；`/app/assistant/librechat` 继续作为正式承载入口。
4. [ ] 不以“长期兼容层”作为主策略；若出现短期开发期过渡开关，其生命周期必须受 stopline 与删除任务约束，不得进入长期主干口径。
5. [ ] 不在本计划内新增数据库表；如后续需要持久化新的 runtime checkpoint / tool audit，再单独立计划获批。

## 4. 冻结原则

### 4.1 角色边界原则

1. [ ] LibreChat 只负责：
   - 聊天 UI；
   - 消息流渲染；
   - 输入交互；
   - 会话壳与基础前端体验。
2. [ ] `LangChain/LangGraph` 只负责：
   - proposal 生成；
   - 只读工具调用；
   - runtime 状态编排；
   - interrupt / resume / checkpoint；
   - 不得直接提交业务写入。
3. [ ] 本仓 backend 继续负责：
   - conversation / turn / task / audit SoT；
   - authoritative gate；
   - unified policy consumption；
   - `Context Resolver -> 唯一 PDP -> PrecheckProjection`；
   - confirm / commit；
   - One Door 与最终写链。
4. [ ] 若后续采纳 `DEV-PLAN-361`，`LangGraph/LangChain` 与 LibreChat 仍只允许消费统一 `PrecheckProjection`、dry-run 结果与 explain 视图，不得各自直接解释底层策略 store、字段配置表或租户策略表。

### 4.2 单主源原则

1. [ ] 会话真相只允许存在于本仓 `conversation/turn/phase/transition/task` 主链。
2. [ ] 记忆真相不得同时存在于 LibreChat memory 与本仓 conversation snapshot 两套体系。
3. [ ] 工具目录真相不得同时存在于 LibreChat Agents/MCP UI 与本仓 `Readonly Tool Registry` / `ActionSchema` 两套体系。
4. [ ] 检索与知识装配真相不得同时存在于 LibreChat file search / RAG API 与本仓知识/runtime/tooling 两套主链。

### 4.3 降权原则

1. [ ] 若某项 LibreChat 能力与 `LangGraph/LangChain` 或本仓 backend 的正式职责重叠，则默认以“禁用/后移”为优先，而不是“双保留”。
2. [ ] 如果某项 LibreChat 能力短期不得不保留，也只能作为开发期临时门控，不得进入正式入口、文档地图、运行态主口径或长期主干默认配置。
3. [ ] 任何用户可见结果若无法回溯到本仓 SoT，则不得作为正式验收依据。

### 4.4 硬切原则

1. [ ] 正式入口只允许存在一条业务主链，不保留双运行时并存。
2. [ ] 旧入口、旧依赖、旧运行态字段、旧文档入口一旦失去唯一必需职责，应优先删除而不是标记“以后再收”。
3. [ ] 任何为“平滑迁移”而新增的 compat 结构，都必须同时带删除条件、删除时点与停止线，否则不应进入主干。

### 4.5 文档职责边界

1. [ ] `360` 是母法：只冻结分层角色、不变量、失败原则、停止线与防回流约束。
2. [ ] `360A` 是执行面 SSOT：冻结 successor DTO/API 契约、compat API 生死表、删除批次、runtime-status 语义与可执行拆除清单。
3. [ ] 同一主题若在两份文档同时出现：
   - 以 `360` 为架构边界权威；
   - 以 `360A` 为可执行契约权威。
4. [ ] `360` 不再逐端点复制 successor DTO、错误码和删除批次细节，避免形成双文档权威表达。

## 5. 能力矩阵（冻结版）

### 5.1 保留：LibreChat 继续承担的能力

1. [ ] 聊天 UI 壳：
   - 消息列表；
   - Composer 输入框；
   - 流式 token 渲染；
   - 停止生成 / 重试 / 重发等基础交互。
2. [ ] 会话壳：
   - 会话列表；
   - 会话切换；
   - 临时会话 / 新建会话入口；
   - 基础消息级操作（复制、展开、折叠、引用）。
3. [ ] 纯展示能力：
   - Markdown / code block / table / list 渲染；
   - 附件展示与预览；
   - 可选 artifact 展示壳（若未来本仓需要在对话中展示 HTML / Mermaid / 结构化结果）。
4. [ ] UI 主题与国际化展示能力：
   - 仅作为承载层消费本仓返回结果，不作为业务事实源。

### 5.2 禁用/下线：应尽量从 LibreChat 侧移除的能力

1. [ ] `Agents / Agent Builder`
2. [ ] `Agents API`
3. [ ] `MCP UI / MCP server 管理 / 聊天下拉选择 MCP`
4. [ ] `User Memory`
5. [ ] `Web Search`
6. [ ] `File Search / File Context / RAG API` 作为正式业务主链能力
7. [ ] `Code Interpreter`
8. [ ] `OpenAPI Actions`
9. [ ] `Agent Chain`
10. [ ] 任何由 LibreChat 直接持有“业务工具目录”“业务执行编排”“用户级策略选择”的能力

### 5.3 后移到 LangChain / LangGraph 的能力

1. [ ] 结构化 proposal 生成
2. [ ] Tool calling / middleware
3. [ ] 只读工具编排
4. [ ] interrupt / human-in-the-loop / pause-resume
5. [ ] proposal runtime checkpoint / durable execution
6. [ ] 多步语义流转与状态图编排
7. [ ] 统一联网检索、文件检索、知识检索的受控工具层（如后续确有必要）

### 5.4 后移到本仓 backend 的能力

1. [ ] conversation / turn / task / audit 真相
2. [ ] authoritative gate
3. [ ] unified policy consumption / precheck projection
4. [ ] confirm / commit
5. [ ] receipt / poll / cancel / refresh
6. [ ] capability / action registry
7. [ ] error code / risk gate / tenant/session/authz 边界

### 5.5 可选保留但必须降权的能力

1. [ ] 模型选择器：
   - 仅当最终模型路由仍允许前端选择时保留；
   - 若模型路由由本仓或 `LangGraph/LangChain` runtime 统一控制，则应隐藏或只读显示。
2. [ ] 会话搜索 / 分享 / 导入导出：
   - 可以保留为 UI 体验能力；
   - 但其底层数据必须逐步切到本仓正式会话真相，而不是继续依赖 LibreChat 自有会话存储。
3. [ ] 附件上传：
   - 可以保留前端交互；
   - 但后端存储、解析、权限与知识接入必须服从本仓或 `LangGraph/LangChain` 主链。

## 6. 目标架构

### 6.1 目标结构

```mermaid
graph TD
    A[/app/assistant/librechat] --> B[Vendored LibreChat Web UI]
    B --> C[/internal/assistant/*]
    C --> D[LangGraph/LangChain Runtime]
    D --> E[Readonly Tool Registry]
    D --> F[Proposal / Checkpoint / Interrupt]
    C --> G[Authoritative Gate]
    G --> H[Unified Policy Consumption]
    G --> I[CommitAdapter]
    I --> J[OrgUnitWriteService]
    J --> K[DB Kernel]
    C --> L[Conversation / Turn / Task / Audit SoT]
```

### 6.2 架构解释

1. [ ] 正式用户入口继续是 `/app/assistant/librechat`。
2. [ ] `Vendored LibreChat Web UI` 是唯一正式聊天承载面。
3. [ ] `LangGraph/LangChain Runtime` 只能提供 proposal、只读工具与 runtime 编排，不得形成业务真值。
4. [ ] authoritative gate 与统一策略主链继续由本仓后端持有。
5. [ ] 任何业务写入仍只能经 `CommitAdapter -> OrgUnitWriteService -> DB Kernel` 完成。
6. [ ] 若 `DEV-PLAN-361` 落地，图中的 `Unified Policy Consumption` 应按 `DEV-PLAN-350/361` 解释为：
   - `Context Resolver -> 唯一 PDP -> PrecheckProjection`；
   - OPA 仅可作为唯一 PDP 的候选求值引擎；
   - 不得因此把策略真相迁移到 LibreChat 或 `LangGraph/LangChain` runtime。
7. [ ] successor `ui-bootstrap/session` DTO、compat API 生死表、runtime-status 枚举扩展与失败语义的可执行合同，以 `DEV-PLAN-360A` 为唯一事实源；`360` 只冻结 fail-closed 主原则。

## 7. 迁移阶段

### Phase 0：冻结能力矩阵、硬切边界与停止线

1. [ ] 冻结本计划中的“保留 / 下线 / 后移”矩阵。
2. [ ] 盘点当前正式入口中是否仍暴露：
   - Agents；
   - MCP UI；
   - Memory；
   - Search；
   - Code Interpreter；
   - File Search / RAG。
3. [ ] 将这些能力标记为：
   - 正式保留；
   - 立即隐藏并删除；
   - 仅开发期临时门控；
   - 立即归档退出。

### Phase 1：前端可见层降权

1. [ ] 在正式入口隐藏或禁用 LibreChat 的 Agents / MCP / Memory / Web Search / Code Interpreter / File Search 等用户可见入口。
2. [ ] 保证用户在正式入口上只能看到“聊天 UI 能力”，而看不到第二套 Agent 平台配置面板。
3. [ ] 若 `/assistant-ui/*` 或上游调试直链仍存在，其角色只允许是调试/排障，不得作为正式验收入口。

### Phase 2：runtime 主链切换

1. [ ] 将发送主链正式收口为：
   - `LibreChat UI action/store/render`
   - `-> /internal/assistant/*`
   - `-> LangGraph/LangChain proposal runtime`
   - `-> backend 统一策略主链（Context Resolver / 唯一 PDP / PrecheckProjection）`
   - `-> authoritative gate`
   - `-> DTO / task / receipt 回写`
2. [ ] 保证聊天流中的正式助手回复全部来自本仓主链，不再依赖 LibreChat Agents runtime 或 Memory runtime。
3. [ ] 上游 LibreChat runtime 若仍存在，也只能作为待删除遗留实现，不得再承担正式业务智能职责，也不得作为默认回退路径。
4. [ ] 若 `361` 已采纳，Phase 2 不得把 runtime 切换实现成新的 Assistant 专用策略分支；tool 输出、precheck、dry-run 与正式写链前置解释必须共同消费统一 PDP/Projection 主链。

### Phase 3：数据与依赖收口

1. [ ] 将会话列表、会话搜索、历史回放、任务刷新直接切到本仓会话 SoT，不保留长期双存储读取。
2. [ ] 直接删除仅用于 LibreChat 平台能力的基础设施依赖：
   - MongoDB（若仅用于 LibreChat 会话/agent state）
   - Meilisearch（若仅用于 LibreChat 会话/文件搜索）
   - RAG API
   - VectorDB
3. [ ] 若某项依赖仍被正式产品能力使用，必须在删除前先完成职责重定义与单主源迁移；否则默认执行退役，而不是继续挂在“LibreChat 默认能力”名下。

### Phase 4：正式封板

1. [ ] 正式宣告 LibreChat 在本仓中的角色为“聊天 UI 壳与交互层”。
2. [ ] 宣告正式 Agent/runtime/tooling 主链为：
   - `LangGraph/LangChain + 本仓 authoritative backend`
3. [ ] 完成 stopline 搜索，确认仓内不再把 LibreChat Agents / MCP / Memory / Search / RAG 作为正式能力来源引用。
4. [ ] 对 `220-293` 系列文档执行退出归档审计：
   - 从 `AGENTS.md` 文档地图移除现行入口；
   - 将仍需保留的历史事实迁移或回写到 `341/350/360/360A/361`；
   - 将仅服务过渡阶段的计划文档转入 `docs/archive/dev-plans/`。
5. [ ] 删除已失去正式职责的 compat runtime、旧依赖接线、旧入口与遗留说明，不以“未来可能还要用”作为保留理由。

## 8. 风险与处置

1. [ ] 风险：只引入 `LangGraph/LangChain`，但不下线 LibreChat 平台能力，导致双运行时长期并存。  
   处置：本计划将“可见入口下线 + 旧依赖删除 + stopline 搜索 + 文档地图移除”作为强制交付，而不是建议项。
2. [ ] 风险：过早退役上游 runtime 依赖，导致正式 UI 某些基础体验回退。  
   处置：项目早期优先选择回退后补，而不是长期保留第二依赖栈；如确有缺口，应补齐 successor 主链能力，而不是恢复旧平台。
3. [ ] 风险：把 `LangGraph/LangChain` 当作新的 authoritative backend。  
   处置：沿用 `223/268/293/350` 边界，要求 proposal/runtime 仅为建议与只读编排层。
4. [ ] 风险：Mongo / Search / RAG 退役时遗漏隐性依赖。  
   处置：在 Phase 3 先完成依赖盘点与删除清单，再一次性切断，不以“继续保留运行观察”代替删除动作。
5. [ ] 风险：`360` 完成 UI/runtime 切换后，Assistant 仍保留一套独立于写链的策略解释实现，导致“第二策略解释器”从 LibreChat 平台能力迁移到 `LangGraph/LangChain` 或 compat API。  
   处置：将 `DEV-PLAN-350/361` 视为 Phase 2 后端接线约束，要求 runtime 只消费统一 `PrecheckProjection` 与 explain 视图，不直接访问底层策略 store。
6. [ ] 风险：为了求稳而保留过多 compat 开关、compat API 与旧测试口径，导致几个月后形成新的回流基础。  
   处置：把“兼容”从产品策略降级为短期开发期例外，并要求每个例外都绑定删除任务与失败停止线。
7. [ ] 风险：successor runtime 不可用时，团队临场发明 bootstrap/session 旁路、错误码或回退语义。  
   处置：在 `360A` 先冻结 successor DTO 与 fail-closed 失败合同；`360` 只允许“显式拒绝/只读浏览/终止任务”，不允许回退旧平台。

## 9. 验收标准

1. [ ] 正式入口 `/app/assistant/librechat` 不再暴露 LibreChat Agents / MCP / Memory / Web Search / Code Interpreter / File Search 等第二平台能力入口。
2. [ ] 正式助手回复与任务状态回写均来自本仓 `/internal/assistant/*` 主链。
3. [ ] `LangGraph/LangChain` 不直接生成 authoritative turn state，不直接触发业务提交。
4. [ ] 本仓 conversation / turn / task / audit 仍是唯一业务事实源。
5. [ ] stopline 搜索中，不再把 LibreChat Agents / Memory / Search / MCP 视为正式功能依赖。
6. [ ] 若上游 LibreChat runtime 尚未删除，也已明确处于待退役状态，不再拥有默认接线、默认部署或平台级产品中心地位。
7. [ ] 若已采纳 `DEV-PLAN-361`，则 Assistant runtime、tooling、precheck 与正式写链前置解释对同一 `PolicyContext` 输出一致结论，不存在第二策略解释器旁路。
8. [ ] `AGENTS.md` 文档地图不再将 `220-293` 系列列为现行 Assistant/LibreChat 主线入口。
9. [ ] Mongo / Meilisearch / RAG API / VectorDB 若不再承担 successor 主链职责，则已从正式部署与默认运行链路中删除，而不是仅被标记为 compat-only。
10. [ ] 正式主干不再以 compat runtime、compat API 或双依赖栈作为默认运行方式。
11. [ ] successor runtime 不可用时，系统按 `360A` 冻结的 fail-closed 契约显式拒绝或终止，不回退到旧平台、旧 API 或 `/assistant-ui/*`。

## 10. 停止线

1. [ ] 若正式入口仍允许用户通过 LibreChat Agents / MCP / Memory / Search 构建第二套业务能力主链，则本计划失败。
2. [ ] 若引入 `LangGraph/LangChain` 后，仓内仍长期同时维护两套正式工具目录与记忆机制，则本计划失败。
3. [ ] 若 `LangGraph/LangChain` 直接持有业务写入权限、绕过 authoritative gate 或形成第二写入口，则本计划失败。
4. [ ] 若 Mongo / Search / RAG 等依赖继续仅为“LibreChat 平台默认能力”服务，却又没有正式产品职责说明，则本计划失败。
5. [ ] 若 `360` 落地后，仓内仍允许 runtime/tooling 直接解释底层策略表，未收敛到 `Context Resolver -> 唯一 PDP -> PrecheckProjection` 主链，则本计划失败。
6. [ ] 若 `220-293` 系列仍继续以 `AGENTS.md` 文档地图现行入口身份存在，导致新旧主线并列指路，则本计划失败。
7. [ ] 若为了“平滑迁移”继续在正式主干保留长期 compat runtime、长期 compat API 或长期双依赖栈，则本计划失败。

## 11. 测试与覆盖率

1. [ ] 覆盖率口径、统计范围、目标阈值与证据记录继续以 `AGENTS.md`、`Makefile` 与 CI workflow 为 SSOT；本计划不复制脚本细节。
2. [ ] 文档与配置收口阶段至少执行：
   - `make check doc`
3. [ ] 正式切换实施时，至少补齐以下验证：
   - 正式入口不可见第二平台能力入口；
   - 正式对话流仍能完成 successor 主线要求的真实业务闭环；
   - 会话列表、回执、任务刷新仍由本仓 SoT 驱动；
   - `LangGraph/LangChain` runtime 失败时不会绕回 LibreChat Agents / Memory 主链。
4. [ ] E2E 与 Assistant 主链验证继续以 `341/350/360/360A/361` 对齐后的正式入口、统一策略消费与 stopline 为准。

## 12. 交付物

1. [ ] 计划文档：`docs/dev-plans/360-librechat-depower-and-langgraph-langchain-layered-takeover-plan.md`
2. [ ] 文档地图更新：`AGENTS.md`
3. [ ] 后续实施输入：
   - LibreChat 功能盘点与禁用清单
   - `LangGraph/LangChain` runtime 接线方案
   - 会话/搜索/依赖删除清单
4. [ ] 文档治理输入：
   - `220-293` 系列退出归档候选清单
   - successor 文档承接映射（`341/350/360/360A/361`）
   - `docs/archive/dev-plans/` 迁移批次计划

## 13. 关联文档

1. `docs/dev-plans/341-assistant-mainline-evolution-and-340-350-correlation-investigation.md`
2. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
3. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
4. `docs/dev-plans/361-opa-pdp-adoption-boundary-and-migration-plan.md`
