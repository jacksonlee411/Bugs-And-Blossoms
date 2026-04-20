# DEV-PLAN-360A：LibreChat 功能禁用清单与 Runtime 主链硬切实施计划

**状态**: 已完成（2026-04-13 18:23 CST；Phase 0/1 已完成，Phase 2 已完成，Phase 3/4 的代码与定向测试批次已完成，`tp288b / tp290b` live successor 复验已通过，`make test` 已达 `98.00%` coverage 门槛；`tp288` 已按历史 mock 正式入口证据脚本退役归档）

## 1. 背景

1. [X] `DEV-PLAN-360` 已冻结总体边界：LibreChat 退回聊天 UI 承载层，`LangGraph/LangChain` 承担 proposal / tooling / runtime 编排层，本仓 backend 继续掌握 authoritative gate、会话真相、统一策略消费与 One Door 提交。
2. [X] 当前仓内已经存在与此目标直接相关的落点：
   - 正式入口与静态前缀：`/app/assistant/librechat`、`/assets/librechat-web/**`、`/assistant-ui/*`（见 [handler.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/handler.go)）。
   - LibreChat runtime 基线依赖：`mongodb/meilisearch/rag_api/vectordb`（见 [deploy/librechat/.env.example](/home/lee/Projects/Bugs-And-Blossoms/deploy/librechat/.env.example)、[deploy/librechat/docker-compose.upstream.yaml](/home/lee/Projects/Bugs-And-Blossoms/deploy/librechat/docker-compose.upstream.yaml)）。
   - vendored Web UI 来源：`third_party/librechat-web/source/`，静态产物出口：`internal/server/assets/librechat-web/`（见 [third_party/librechat-web/README.md](/home/lee/Projects/Bugs-And-Blossoms/third_party/librechat-web/README.md)）。
   - 运行态能力信号：`mcp_enabled / actions_enabled / agents_write_enabled`（见 [internal/server/assistant_domain_policy.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/assistant_domain_policy.go)、[internal/server/assistant_runtime_status.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/assistant_runtime_status.go)、[apps/web/src/pages/assistant/AssistantPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/assistant/AssistantPage.tsx)）。
3. [X] 因此，`360A` 的职责不是再讨论“是否降权”，而是把“具体先关什么、改哪些文件、能力信号目标值是什么、依赖何时进入删除批次、哪些测试要改”落成实施清单。
4. [X] 后端统一策略消费主链由 `DEV-PLAN-350/361` 承接，`360A` 不再定义 PDP 实现细节。
5. [X] 为避免继续把过渡期文档当现行入口，`220-293` 系列需纳入退出归档治理；`360A` 负责把“从正式入口、运行态说明与 `AGENTS.md` 文档地图中退场”落成执行步骤。
6. [X] 考虑项目仍处于早期阶段，`360A` 的正式执行口径应从“迁移期兼容”切换为“硬切删除优先”：只要某项能力或依赖不再是 successor 主链唯一必需，就应进入删除队列，而不是继续留在默认主干。

## 2. 目标与非目标

### 2.1 目标

1. [ ] 形成一份按代码路径与配置入口组织的 LibreChat 功能禁用清单。
2. [ ] 冻结 vendored Web UI 中需删除、隐藏或失活的入口、路由、菜单、组件与内容渲染点。
3. [ ] 冻结 runtime 主链切换顺序：`UI -> /internal/assistant/* -> LangGraph/LangChain runtime -> authoritative gate -> task/receipt/DTO`。
4. [ ] 冻结运行态能力目标值与硬切删除口径。
5. [ ] 冻结 Mongo / Meili / RAG / VectorDB 的依赖删除顺序与退役前置条件。
6. [ ] 冻结 runtime 接线边界：本计划只细化 LibreChat 降权、旧 API 切断与 runtime 接线步骤，不新增新的策略旁路。
7. [ ] 将 `220-293` 系列的退出归档动作细化到正式封板步骤：从 `AGENTS.md` 文档地图移除、形成 successor 映射、按批次迁入 `docs/archive/dev-plans/`。
8. [ ] 冻结硬切原则：不保留长期 compat runtime、不保留长期 compat API、不保留长期双依赖栈。
9. [ ] 冻结 compat API 逐端点生死表：每个端点都必须明确“替代者 / 删除批次 / 过渡窗口 / 最终动作”，不允许“先审计、以后再决定”。

### 2.2 非目标

1. [ ] 不在本计划内直接实现 `LangGraph/LangChain` runtime 代码。
2. [ ] 不在本计划内新增数据库表或 checkpoint schema。
3. [ ] 不在本计划内重做已由 successor 主线继承的 authoritative backend 合同（历史来源可追溯至 `223/260/293/350`，但现行收口以 `341/350/360/360A/361` 为准）。
4. [ ] 不在本计划内接管 LibreChat upstream Node backend 为本仓正式实现面。
5. [ ] 不把 LibreChat 旧 API、Assistant runtime adapter 或前端运行态页扩张为新的策略裁决入口。
6. [ ] 不把“为了先跑起来而保留旧链路”当成主干默认策略；任何短期过渡结构都必须绑定删除任务与停止线。

## 3. 当前代码落点与正式职责

### 3.1 正式入口与别名

1. [X] `/app/assistant/librechat` 是正式聊天入口。
2. [X] `/assets/librechat-web/**` 是正式静态资源前缀。
3. [X] `/assistant-ui/*` 仍存在历史别名/代理入口，当前由 `internal/server/handler.go` 与 `internal/server/assistant_ui_proxy.go` 维持。
4. [X] `/app/assistant` 仍保留运行态与日志页，页面代码位于：
   - [AssistantPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/assistant/AssistantPage.tsx)
   - [LibreChatPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/assistant/LibreChatPage.tsx)

补充冻结：
1. [ ] `/assistant-ui/*` 不再允许作为“长期调试别名”存在。
2. [ ] `360A` 目标态要求：
   - `Phase 0/1` 实施批次中保持现状：继续由 `internal/server/assistant_ui_proxy.go` 做 `302 -> /app/assistant/librechat`；
   - `Phase 4` 才执行 `410 Gone -> 路由删除` 收口；
   - 若 `Phase 4` 之后仍保留 `/assistant-ui/*`，按 `DEV-PLAN-004M1` 视为 legacy 回流。

### 3.2 vendored Web UI 来源

1. [X] 来源根目录：`third_party/librechat-web/source/client/src/`
2. [X] patch stack 入口：`third_party/librechat-web/patches/series`
3. [X] 构建脚本：
   - `scripts/librechat-web/build.sh`
   - `scripts/librechat-web/verify.sh`
4. [X] 当前已接管 send/store/render 主链的历史实现基线可追溯至 `DEV-PLAN-284`；后续功能禁用仍必须以 patch 方式在 vendored 源码层完成，而不是在产物层做 DOM hack。

### 3.3 运行态与部署配置

1. [X] runtime env 模板：`deploy/librechat/.env.example`
2. [X] 上游 compose：`deploy/librechat/docker-compose.upstream.yaml`
3. [X] overlay compose：`deploy/librechat/docker-compose.overlay.yaml`
4. [X] 状态探针与快照：
   - `scripts/librechat/status.sh`
   - `deploy/librechat/runtime-status.json`
   - `GET /internal/assistant/runtime-status`

## 4. 运行态能力目标值

### 4.1 现有能力信号

当前运行态页与 API 已暴露：

1. [X] `capabilities.mcp_enabled`
2. [X] `capabilities.actions_enabled`
3. [X] `capabilities.agents_write_enabled`

### 4.2 目标值冻结

在 `360A` 目标态下，运行态能力应收敛为：

| 能力信号 | 目标值 | 说明 |
| --- | --- | --- |
| `mcp_enabled` | `false` | 正式入口不再允许 LibreChat 侧 MCP 成为用户可见正式工具平台 |
| `actions_enabled` | `false` | LibreChat/OpenAPI actions 不再作为正式执行主链 |
| `agents_write_enabled` | `false` | 不允许 LibreChat agents 拥有写动作或第二执行入口 |

补充要求：
1. [ ] `AssistantPage.tsx` 仍可展示这些信号，但应把“关闭”解释为目标态，而不是异常态。
2. [ ] 若开发期短期保留某项能力，则必须标记为“临时门控”，并在本计划后续任务中注明删除时间点；不得作为正式运行默认值。

### 4.3 建议新增运行态字段（契约草案）

为避免后续只靠三个位判断，建议在 `assistantRuntimeCapabilities` 增补：

1. [ ] `agents_ui_enabled`
2. [ ] `memory_enabled`
3. [ ] `web_search_enabled`
4. [ ] `file_search_enabled`
5. [ ] `code_interpreter_enabled`
6. [ ] `artifacts_enabled`
7. [ ] `runtime_cutover_mode`：`cutover-prep | ui-shell-only`

说明：
1. [ ] 这些字段仅用于诊断与切换观测，不改变 authoritative backend 合同，也不意味着正式支持长期 compat 模式。
2. [ ] 若不新增字段，也必须至少在本计划实施记录中维护同等粒度的矩阵。
3. [ ] 当前实施批次完成后，默认值应从 `cutover-prep` 切到 `ui-shell-only`；若保留环境覆盖，仅允许在这两个枚举之间切换。

### 4.4 依赖退役的运行态语义

为避免“删除旧依赖 = 运行时报红 = 团队回滚”这一回流路径，运行态状态模型必须冻结为：

1. [ ] 对已计划删除且已完成主链切换的依赖，状态不再记为 `unavailable`，而应记为：
   - `required=false`
   - `healthy=retired`
   - `reason=retired_by_design`
2. [ ] `assistant_runtime_status.go` 的聚合规则必须忽略 `retired_by_design` 服务，不能因为已退役依赖而把整体状态降为 `degraded/unavailable`。
3. [ ] `mongodb/meilisearch/rag_api/vectordb` 一旦进入删除批次，就必须同步从“required service”语义迁出，而不是先删 compose 再让运行态探针报故障。
4. [ ] 正式运行态页必须能明确区分：
   - successor 主链依赖异常；
   - 能力被硬切关闭；
   - 依赖已按设计退役。

## 5. 功能禁用清单（按代码落点）

### 5.1 立即隐藏/下线的用户可见入口

1. [ ] Agent Marketplace / Agents 路由与导航入口
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/Nav/AgentMarketplaceButton.tsx`
     - `third_party/librechat-web/source/client/src/components/Nav/Nav.tsx`
     - `third_party/librechat-web/source/client/src/routes/index.tsx`
     - `third_party/librechat-web/source/client/src/routes/Root.tsx`
2. [ ] MCP 管理 UI / MCP 配置弹窗 / MCP 状态图标
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/MCP/**`
     - `third_party/librechat-web/source/client/src/Providers/AgentPanelContext.tsx`
     - `third_party/librechat-web/source/client/src/store/mcp.ts`
3. [ ] Web Search 用户开关与显示入口
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/WebSearch.tsx`
     - `third_party/librechat-web/source/client/src/Providers/BadgeRowContext.tsx`
     - `third_party/librechat-web/source/client/src/routes/Search.tsx`
4. [ ] Code Interpreter / Execute Code 展示入口
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/CodeAnalyze.tsx`
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/Parts/ExecuteCode.tsx`
5. [ ] Memory 展示与 Memory artifact UI
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/MemoryInfo.tsx`
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/MemoryArtifacts.tsx`
     - `third_party/librechat-web/source/client/src/utils/memory.ts`
6. [ ] Retrieval / File Search / RAG 作为正式入口能力
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/RetrievalCall.tsx`
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/SearchContent.tsx`
     - `third_party/librechat-web/source/client/src/components/Chat/Input/Files/**`

### 5.2 可保留但必须改为“只展示，不编排”

1. [ ] Model Selector
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/Chat/Menus/Endpoints/ModelSelector.tsx`
     - `.../ModelSelectorContext.tsx`
     - `.../ModelSelectorChatContext.tsx`
   - 目标：
     - 若模型路由归后端，前端只读显示当前模型；
     - 不允许在前端直接决定正式业务 runtime 选路。
2. [ ] Artifacts 展示壳
   - 候选落点：
     - `third_party/librechat-web/source/client/src/utils/artifacts.ts`
     - `third_party/librechat-web/source/client/src/store/artifacts.ts`
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/UIResourceCarousel.tsx`
   - 目标：
     - 可保留展示；
     - 但 artifact 生成必须来自本仓或未来 runtime 主链，而非 LibreChat agent runtime。
3. [ ] ToolCall 显示壳
   - 候选落点：
     - `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/ToolCall.tsx`
     - `.../ToolCallInfo.tsx`
   - 目标：
     - 可保留只读展示；
     - 不允许继续承接 LibreChat 原生 MCP / action / memory / agent chain 语义。

### 5.3 必须保留的聊天 UI 能力

1. [ ] 消息列表、消息渲染、流式更新
2. [ ] 输入框、发送、停止、重试、重新生成
3. [ ] 会话列表、新建会话、临时会话
4. [ ] 基础 markdown/code/table/image/file 展示
5. [ ] `/app/assistant/librechat` 正式入口与 `/assets/librechat-web/**` 静态前缀

## 6. 配置与开关收口

### 6.1 `deploy/librechat/.env.example` 收口要求

当前模板中存在：

1. [X] `MONGO_URI`
2. [X] `MEILI_HOST`
3. [X] `RAG_API_URL`
4. [X] `VECTOR_DB_PROVIDER`
5. [X] `QDRANT_URL`

收口规则：

1. [ ] 将这些变量分组为：
   - 核心 UI 壳仍需
   - 开发期临时门控
   - 仅历史平台能力、待删除
2. [ ] 若某变量只服务已计划下线的 LibreChat 平台能力，模板中必须标记“delete soon”，而不是默认为长期 compat-only。
3. [ ] 进入 `ui-shell-only` 目标态后，这些变量必须允许为空，并在删除批次中被完整移除。

### 6.2 短命切流开关（唯一例外）

为避免在主干重新发明一套 legacy，本计划明确拒绝成组 `LIBRECHAT_COMPAT_*` 开关矩阵。若确需保留切流开关，最多只允许一个短命总开关：

1. [ ] 允许的唯一候选：
   - `LIBRECHAT_VENDORED_BOOTSTRAP_COMPAT=true|false`
2. [ ] 作用范围冻结为：
   - 仅保护 vendored UI 启动期最小 bootstrap 兼容面；
   - 不覆盖 Agents/MCP/Memory/Search/RAG/Code Interpreter；
   - 不改变正式业务主链，不提供回退到旧平台能力的入口。
3. [ ] 删除约束必须与开关同时冻结：
   - 删除批次：`360A Phase 4`
   - 删除条件：vendored UI 已完成 successor bootstrap DTO 切换，`/assistant-ui/*` 已退场，compat API 生死表中的“暂留端点”已清零
   - 门禁：若默认部署仍启用该开关，`360A` 失败
   - 文档要求：不得再新增第二个细分 compat 开关
4. [ ] 若实现证明该总开关也不是必需，则优先选择“不引入任何切流开关”。

## 7. 部署依赖收口矩阵

### 7.1 当前依赖

1. [X] `mongodb`
2. [X] `meilisearch`
3. [X] `rag_api`
4. [X] `vectordb`

### 7.2 目标判定

| 依赖 | 当前用途 | `360A` 判定 | 退役前置条件 |
| --- | --- | --- | --- |
| `mongodb` | LibreChat runtime 数据与部分 RAG | `优先删除候选` | 正式会话/agent state 不再依赖 LibreChat runtime 存储 |
| `meilisearch` | LibreChat 搜索 | `优先删除候选` | 会话搜索切到本仓 SoT 或正式取消该能力 |
| `rag_api` | File Search / RAG | `立即退役候选` | 正式知识检索迁移到 LangChain/后端工具层 |
| `vectordb` | RAG 向量库 | `立即退役候选` | `rag_api` 不再是正式主链能力 |

补充要求：
1. [ ] `deploy/librechat/docker-compose.upstream.yaml` 与 `overlay.yaml` 中，退役候选依赖应优先进入删除批次，而不是长期标注为 compat-only。
2. [ ] 在正式宣告退役前，`assistant-runtime-status` 需要能区分“必需依赖未就绪”和“待删除依赖已关闭”。

### 7.3 compat API 逐端点生死表（冻结版）

以下端点同时挂载于：
- `/app/assistant/librechat/api/*`
- `/assets/librechat-web/api/*`

两条前缀共用同一 handler，因此必须共生共死，不允许只删一侧。

| 端点 suffix | 当前职责 | successor | 删除批次 | 过渡窗口 | 最终动作 |
| --- | --- | --- | --- | --- | --- |
| `/auth/refresh` | vendored UI 会话续期 | `/internal/assistant/session/refresh` 或等价 successor session DTO | Phase 2 | 仅限 cutover PR 到 cleanup PR 之间 | 先 `410 Gone`，后删 handler 分支 |
| `/auth/logout` | vendored UI 退出登录 | `/internal/assistant/session/logout` 或主站统一退出链 | Phase 2 | 同上 | 先 `410 Gone`，后删 handler 分支 |
| `/user` | 当前用户 bootstrap | `/internal/assistant/session` | Phase 2 | 同上 | 先 `410 Gone`，后删 handler 分支 |
| `/roles/user` | UI 权限占位 | 合并进 `/internal/assistant/session` | Phase 2 | 同上 | 删除，不保留 alias |
| `/roles/admin` | UI 权限占位 | 合并进 `/internal/assistant/session` | Phase 2 | 同上 | 删除，不保留 alias |
| `/config` | UI 配置 bootstrap | `/internal/assistant/ui-bootstrap` | Phase 1 | 无长期窗口；切到 successor DTO 的同一批次完成 | 删除 |
| `/endpoints` | provider 列表 bootstrap | `/internal/assistant/ui-bootstrap` | Phase 1 | 无长期窗口；切到 successor DTO 的同一批次完成 | 删除 |
| `/models` | model 列表 bootstrap | `/internal/assistant/ui-bootstrap` | Phase 1 | 无长期窗口；切到 successor DTO 的同一批次完成 | 删除 |

补充冻结：
1. [ ] `Phase 1` 必须先定义 successor bootstrap DTO，避免 `config/endpoints/models` 因“还没替代者”而继续滞留。
2. [X] `Phase 2` 只允许保留认证/会话相关最小端点，且过渡窗口最多一个清理批次。
3. [ ] [librechat_vendored_compat_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/librechat_vendored_compat_api.go#L69) 中未列入上表的 suffix，一律视为不允许新增。

### 7.4 successor DTO 契约冻结（`ui-bootstrap` / `session`）

为满足 `DEV-PLAN-003` 的 T2 要求，生死表中出现的 successor 端点必须同步冻结 DTO、错误码、鉴权边界与失败行为。以下为本计划的最小稳定合同：

#### 7.4.1 `GET /internal/assistant/ui-bootstrap`

1. [ ] 正式职责：
   - 一次性替代旧 `/config`、`/endpoints`、`/models` 三个 compat 端点；
   - 只返回正式入口启动所需的最小 bootstrap 事实，不回传任何 LibreChat 专属 legacy 结构。
2. [ ] 鉴权边界：
   - 必须要求当前 SID 会话有效且 tenant/principal 已解析；
   - 不允许匿名访问；
   - 不允许通过 query/header 切换到旧 bootstrap 路径。
3. [ ] `200 OK` 最小 DTO：
   ```json
   {
     "contract_version": "v1",
     "viewer": {
       "id": "user_123",
       "username": "alice",
       "email": "alice@example.com",
       "name": "Alice",
       "role": "USER"
     },
     "ui": {
       "model_select": true,
       "artifacts_enabled": true,
       "agents_ui_enabled": false,
       "memory_enabled": false,
       "web_search_enabled": false,
       "file_search_enabled": false,
       "code_interpreter_enabled": false
     },
     "models": [
       {
         "endpoint_key": "openai",
         "endpoint_type": "openai",
         "provider": "openai",
         "model": "gpt-5.4",
         "label": "OpenAI / gpt-5.4"
       }
     ],
     "runtime": {
       "status": "healthy",
       "runtime_cutover_mode": "ui-shell-only",
       "domain_policy_version": "v1"
     }
   }
   ```
4. [ ] DTO 冻结要求：
   - `contract_version` 必填，当前冻结为 `v1`；
   - `models` 为数组，不再返回旧 `/endpoints` map 与旧 `/models` map；
   - 不返回 `token`、`plugins`、`personalization.memories`、`sharedLinksEnabled` 一类仅服务旧平台能力的字段；
   - `ui.*_enabled` 必须显式声明关闭能力，避免前端通过“字段缺失”自行推导。
5. [ ] 失败语义：
   - `401 assistant_session_invalid`：SID 缺失或已失效；
   - `401 assistant_principal_invalid`：principal/tenant 解析失败；
   - `503 assistant_ui_bootstrap_unavailable`：successor bootstrap 最小事实缺失，如模型列表、runtime bootstrap 或必要 viewer 信息不可得；
   - 失败时不得拆分返回部分成功字段，不得回退调用 compat `/config`、`/endpoints`、`/models`。

#### 7.4.2 `GET /internal/assistant/session`

1. [ ] 正式职责：
   - 替代旧 `/user` 与 `/roles/*` 的最小会话/身份摘要；
   - 不再保留独立 `roles/user`、`roles/admin` successor 端点。
2. [ ] `200 OK` 最小 DTO：
   ```json
   {
     "contract_version": "v1",
     "authenticated": true,
     "viewer": {
       "id": "user_123",
       "username": "alice",
       "email": "alice@example.com",
       "name": "Alice",
       "role": "USER"
     }
   }
   ```
3. [ ] 失败语义：
   - `401 assistant_session_invalid`
   - `401 assistant_principal_invalid`
   - 不返回旧 `token + user` 结构。

#### 7.4.3 `POST /internal/assistant/session/refresh`

1. [ ] 正式职责：
   - 校验当前 SID 会话并返回最新 session 摘要；
   - `Phase 1` 不新增 SID 轮换、续期写入或 refresh token payload。
2. [ ] `200 OK` 最小 DTO：
   ```json
   {
     "contract_version": "v1",
     "authenticated": true,
     "viewer": {
       "id": "user_123",
       "username": "alice",
       "email": "alice@example.com",
       "name": "Alice",
       "role": "USER"
     },
     "refreshed_at": "2026-04-12T12:30:00Z"
   }
   ```
3. [ ] 失败语义：
   - `401 assistant_session_invalid`
   - `401 assistant_principal_invalid`
   - 不返回旧 `token` 字段。

#### 7.4.4 `POST /internal/assistant/session/logout`

1. [ ] 正式职责：
   - 撤销 SID 并清理正式入口登录态。
2. [ ] `204 No Content`
3. [ ] 不再返回旧平台 logout payload。

#### 7.4.5 successor 契约总要求

1. [ ] 以上四个 successor 端点必须在同一文档合同下实现，不允许不同实现者各自发明 bootstrap/session 结构。
2. [ ] [apps/web/src/api/assistant.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/api/assistant.ts#L243) 必须新增对应 TS 接口，并与后端 DTO 同步变更，同一 patch 完成。
3. [ ] [apps/web/src/errors/presentApiError.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/errors/presentApiError.ts) 必须为新增错误码提供正式用户提示，不允许沿用 vendored compat 错误码。

## 8. Runtime 主链接线

### 8.1 目标发送链

正式发送链冻结为：

```text
Vendored LibreChat UI
-> /internal/assistant/*
-> LangGraph/LangChain proposal runtime
-> backend 统一策略消费主链
-> authoritative gate
-> conversation/turn/task/audit SoT
-> DTO / receipt / poll / refresh
-> official message tree render
```

### 8.2 禁止路径

1. [ ] `Vendored UI -> LibreChat Agents runtime -> tool/action/memory/search -> assistant reply`
2. [ ] `Vendored UI -> LibreChat MCP UI -> user-managed tool registry`
3. [ ] `Vendored UI -> LibreChat search/memory state -> 作为正式 conversation 真相`
4. [ ] `LangGraph/LangChain -> direct commit / direct DB write`
5. [ ] `LangGraph/LangChain / compat API / assistant precheck -> direct policy store access -> 形成第二策略解释器`

### 8.3 代码边界

1. [ ] UI 入口与兼容 API 边界：
   - [handler.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/handler.go)
   - `internal/server/librechat_web_ui.go`
   - `internal/server/assistant_ui_proxy.go`
2. [ ] 运行态展示与能力信号：
   - [assistant_runtime_status.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/assistant_runtime_status.go)
   - [assistant_domain_policy.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/assistant_domain_policy.go)
   - [AssistantPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/assistant/AssistantPage.tsx)
   - [apps/web/src/api/assistant.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/api/assistant.ts)
3. [ ] future runtime adapter 边界：
   - `internal/server/assistant_model_gateway.go`
   - `internal/server/assistant_semantic_orchestrator.go`
   - `internal/server/assistant_task_store.go`
   - 后续新增 `LangChain/LangGraph` adapter 文件
4. [ ] 以上 runtime adapter 边界只允许消费后端统一策略视图，不得各自再解释 `tenant_field_configs`、字段策略 registry 或租户策略表。

### 8.4 successor 失败语义（冻结版）

`360A` 目标态下，系统必须 fail-closed，不允许“successor 主链失败 -> 回退旧平台/旧 API/旧依赖”。

1. [ ] 读写分离原则：
   - 若 conversation SoT 与 task store 仍可用，则 `list/get/poll` 等只读能力继续可用；
   - `create turn`、`confirm`、任何会进入 proposal/runtime/gate/commit 的动作必须显式拒绝，不得静默降级。
2. [ ] bootstrap/session 失败：
   - `/internal/assistant/ui-bootstrap` 不可用时，正式入口显示阻断态与显式错误，不再回退 compat bootstrap；
   - `/internal/assistant/session*` 不可用时，前端清理本地会话感知并引导重新登录，不回退 `/auth/refresh` 或 `/user` compat 端点。
3. [ ] successor runtime/gate 不可用时的冻结错误码：
   - `503 assistant_ui_bootstrap_unavailable`
   - `503 assistant_runtime_unavailable`
   - `503 assistant_gate_unavailable`
   - `401 assistant_session_invalid`
   - `401 assistant_principal_invalid`
4. [ ] `create turn` 失败语义：
   - 若在 authoritative turn/task 创建前发现 runtime 不可用，直接 `503 assistant_runtime_unavailable`，不创建新 turn/task；
   - 若在 task 创建后失败，task 必须进入 terminal failed，保留 `last_error_code`，不得 reroute 到旧链路。
5. [ ] `confirm/commit` 失败语义：
   - authoritative gate、policy snapshot 或 commit prerequisite 不可用时，显式 `503 assistant_gate_unavailable`；
   - 不进入写链副作用，不切到旧平台兜底。
6. [ ] 页面与消息提示：
   - 正式入口必须用 `presentApiError` 对应的正式错误码渲染提示；
   - 用户看到的是“当前主链失败，请稍后重试/重新登录/等待依赖恢复”，而不是“系统已自动切到旧实现”。
7. [ ] 恢复语义：
   - 仅允许“修复 successor 主链后重试”；
   - 不允许通过恢复 `/assistant-ui/*`、compat API 或旧依赖来临时解锁写路径。

### 8.5 runtime-status 契约修订（冻结版）

1. [ ] [assistant_runtime_status.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/assistant_runtime_status.go#L48) 中 `assistantRuntimeService.Healthy` 的枚举必须从：
   - `healthy | degraded | unavailable`
   扩展为：
   - `healthy | degraded | unavailable | retired`
2. [ ] `reason=retired_by_design` 仅允许与：
   - `required=false`
   - `healthy=retired`
   同时出现。
3. [ ] 顶层 `assistantRuntimeStatusResponse.status` 继续保持：
   - `healthy | degraded | unavailable`
   不新增第四种顶层状态；已退役依赖通过 service-level `retired` 表达，且不参与降级聚合。
4. [ ] [apps/web/src/api/assistant.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/api/assistant.ts#L243) 中 `AssistantRuntimeService.healthy` 的 TS 联合类型必须与后端同步扩展，同一 patch 完成。
5. [ ] 若服务被标记为 `retired_by_design`，运行态页应显示“按设计退役”，而不是“故障”或“暂不可用”。

## 9. 实施步骤

### Phase 0：建立观测、端点生死表与删除批次

1. [X] 扩展 `assistantRuntimeCapabilities`，补齐 `agents_ui_enabled / memory_enabled / web_search_enabled / file_search_enabled / code_interpreter_enabled / artifacts_enabled / runtime_cutover_mode`。
2. [X] 扩展 `AssistantRuntimeStatusResponse` 与 `AssistantPage.tsx`，让运行态页能区分“正式关闭”“依赖异常”“retired_by_design”。
3. [X] 冻结 compat API 逐端点生死表，并回写到实现任务单。
4. [X] 本批次未引入任何新的 compat 开关；successor DTO 直接切主链。
5. [X] 冻结 successor `ui-bootstrap/session` DTO、错误码、鉴权边界与前端 TS 接口，禁止实现阶段临场发明 bootstrap/session 结构。

### Phase 1：正式入口下线第二平台入口

1. [X] 从 vendored Web UI 导航与路由移除或隐藏 Agents、MCP、Search、Memory、Code Interpreter 相关入口。
2. [X] 保留聊天 UI 壳、消息树、输入框、基础展示组件。
3. [X] 新增针对正式入口的前端 smoke 断言：
   - 不出现 Agent Marketplace；
   - 不出现 MCP 配置入口；
   - 不出现 Web Search / Memory / Code Interpreter 开关。
4. [X] 在同一批次引入 `/internal/assistant/ui-bootstrap`，接管 `/config`、`/endpoints`、`/models` 的职责，并删除这三个 compat 端点。
5. [X] 在同一批次引入 `/internal/assistant/session*` successor 端点，并让正式入口不再调用 `/user`、`/roles/*` 与 `/auth/refresh`；这些 compat 端点的 `410 Gone -> 删除` 仍留到 `Phase 2`。
6. [X] `Phase 1` 完成后，将 `runtime_cutover_mode` 从 `cutover-prep` 切到 `ui-shell-only`。

### Phase 2：旧 API 切断与 runtime 主链硬切

1. [X] 按生死表切断 `/app/assistant/librechat/api/*` 与 `/assets/librechat-web/api/*` 中的旧会话端点，不再做开放式审计后再决定。
2. [X] 将正式业务链只保留到 `/internal/assistant/*` 所需的最小 successor 适配面，不再保留长期 compat API。
3. [X] 会话相关旧端点在 cutover PR 中已先返回 `410 Gone`，并统一返回错误码 `assistant_vendored_api_retired`。
4. [X] cleanup PR 删除 compat handler 分支与路由绑定。
5. [X] retired compat path 的短路已前移到 `withTenantAndSession`，确保缺 SID、tenant mismatch、principal invalid 不再暴露 vendored `401` 语义。
6. [X] 本批次明确不提前处理 `/assistant-ui/*`；该别名仍按 `Phase 4` 保持 `302 -> /app/assistant/librechat`。
7. [X] runtime fail-closed 错误码与任务终止语义（如 `assistant_runtime_unavailable / assistant_gate_unavailable`）已在 `Phase 2` 收口批次完成。
8. [X] 当前 compat session API cutover 的实现与文档证据已回写并提交到 `bb5a8568`，执行记录见 `docs/archive/dev-records/dev-plan-360a-execution-log.md`。

### Phase 3：依赖去平台化

1. [X] 盘点 `mongodb/meilisearch/rag_api/vectordb` 与正式产品能力的实际绑定关系。
2. [X] 将只服务平台能力的依赖直接纳入删除批次，而不是下调为 compat-only。
3. [X] 在 successor 主链完成接管后，已从 compose 默认主链与默认 health probe 中移除这些依赖。
4. [X] 删除依赖的同一批次已同步完成 runtime-status `retired_by_design` 语义切换，未出现“先删依赖、后改探针”漂移。

### Phase 4：封板

1. [X] 更新 `360` 与相关执行记录，标注哪些 LibreChat 能力已正式退场。
2. [X] 完成 stopline 搜索，确保仓内不再把 LibreChat Agents/MCP/Memory/Search 视为正式能力来源。
3. [X] 同步完成文档退场收口：
   - `AGENTS.md` 文档地图不再列出 `220-293` 系列现行入口；
   - 形成 `220-293 -> 341/350/360/360A/361` 的 successor 映射；
   - 将确认退出的过渡期计划文档迁入 `docs/archive/dev-plans/`。
4. [X] 删除已失去正式职责的旧依赖、旧 redirect 语义与默认双栈接线，不保留“以后可能还会用”的主干残留。
5. [X] `/assistant-ui/*` 在本收口批次中已统一返回 `410 Gone`，并已从 protected tenant UI 口径中移除；后续若再做物理删路由，只允许作为零行为差异清理，不得恢复 redirect/alias 语义。

## 10. 测试与验收

### 10.1 必须补的验证

1. [X] 运行态页能区分“功能已硬切关闭”“依赖异常”“retired_by_design”。
2. [X] 正式入口 smoke：用户不可见 Agents / MCP / Memory / Search / Code Interpreter 入口。
3. [X] 正式聊天闭环已通过 `tp288b / tp290b` live successor 主链 E2E。
4. [X] `tp288` 旧 mock 正式入口证据脚本已确认退役归档，并保留为默认跳过的历史测试文件，仅用于老文档路径引用。
5. [X] `Phase 0/1` 实施批次中，`/assistant-ui/*` 仍为 `302` alias/redirect，且不能旁路正式业务写接口；`410 Gone -> 删除` 验收留到 `Phase 4`。
6. [X] compat session API 在 `/app/assistant/librechat/api/*` 与 `/assets/librechat-web/api/*` 下统一返回 `410 Gone`，且 retired path 在 session middleware 前已短路，不再泄露 vendored `401` 错误语义。
7. [X] `AGENTS.md` 文档地图已移除 `220-293` 系列现行入口，正式入口说明只保留 successor 计划链路。
8. [X] 默认部署不再依赖 `mongodb/meilisearch/rag_api/vectordb` 提供正式主链能力；退役依赖仅在 `runtime-status` 中以 `retired_by_design` 暴露。
9. [X] compat API 生死表中的所有端点都已进入 successor 或删除态，不存在“待审计、待决定”的灰区端点。
10. [X] 若进入 `Phase 4` 收口批次，`/assistant-ui/*` 已按计划返回 `410 Gone`，不再作为历史别名长期存活。
11. [X] `/internal/assistant/ui-bootstrap` 与 `/internal/assistant/session*` 已按冻结契约返回最小 DTO、错误码与鉴权行为，不存在实现者自定义字段漂移。
12. [X] successor runtime 不可用时，系统只表现为显式拒绝/只读浏览/任务失败终止，不出现旧平台回退、隐式降级或 bootstrap 旁路。

### 10.2 需要更新的现有测试

1. [X] `apps/web/src/pages/assistant/AssistantPage.test.tsx`
   - 当前断言仍包含 `assistant-ui` 依赖展示语义；
   - 后续需改为“待删除依赖/硬切关闭能力”口径。
2. [X] `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`
   - 增加“正式入口不暴露第二平台能力”断言。
3. [X] `internal/server/librechat_vendored_compat_api_test.go`
   - 会话 compat 端点统一改断言为 `410 Gone + assistant_vendored_api_retired`；
   - `/config`、`/endpoints`、`/models` 继续保持删除态断言。
4. [X] `internal/server/handler_test.go` 与 `internal/server/tenancy_middleware_test.go`
   - 增加 `/assistant-ui/*` 退役入口的 `410 Gone` 断言；
   - 确认 `/assistant-ui/*` 已从 protected tenant UI 口径移除，不再触发登录跳转别名。
5. [X] `apps/web/src/errors/presentApiError.test.ts`
   - 补齐 `assistant_vendored_api_retired` 的显式错误提示断言。
6. [X] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`
   - 已确认为历史 mock 正式入口证据脚本；
   - 保留文件路径仅为兼容旧文档引用；
   - 已退役为默认跳过，不再属于现行 E2E gate。
7. [X] `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
   - 已完成 live successor 复验并通过。
8. [X] `e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`
   - 已完成 live successor 复验并通过。
9. [X] 新增运行态断言：
   - 已退役依赖显示为 `retired_by_design`
   - 不因退役依赖把整体 runtime 标成故障
10. [ ] 新增 successor 契约断言：
   - `ui-bootstrap/session` DTO 字段最小集与 `contract_version=v1`
   - `assistant_session_invalid / assistant_principal_invalid / assistant_ui_bootstrap_unavailable / assistant_runtime_unavailable / assistant_gate_unavailable` 的错误码与 HTTP 状态一致
11. [X] 截至 2026-04-13 18:23 CST，`make test` 已通过，coverage `98.00% >= 98.00%`；仓库级 coverage 已不再阻塞 `360A` 封板。

### 10.3 停止线

1. [ ] 若正式入口仍可直接访问或配置 LibreChat Agents / MCP / Search / Memory，则 `360A` 失败。
2. [ ] 若运行态页无法区分“能力关闭”与“依赖异常”，导致运维语义混淆，则 `360A` 失败。
3. [ ] 若旧 API 继续隐式承接平台能力，但仓内已宣称“LibreChat 仅为 UI 壳”，则 `360A` 失败。
4. [ ] 若 `mongodb/meilisearch/rag_api/vectordb` 继续长期作为正式能力依赖存在，却没有对应 successor 主链职责说明，则 `360A` 失败。
6. [ ] 若 `AGENTS.md` 仍把 `220-293` 系列作为现行主线文档暴露，导致新旧入口并存，则 `360A` 失败。
7. [ ] 若为了“平滑迁移”继续在默认部署中保留长期 compat 开关、长期双 API 或长期双依赖栈，则 `360A` 失败。
8. [ ] 若 compat API 仍存在未冻结生死表的灰区端点，则 `360A` 失败。
9. [ ] 若 `/assistant-ui/*` 在 Phase 4 后仍作为可访问调试别名存在，则 `360A` 失败。
10. [ ] 若删除 Mongo/Meili/RAG/Vector 后，runtime-status 仍把它们计为必需服务并报整体故障，则 `360A` 失败。
11. [ ] 若 successor `ui-bootstrap/session` 端点已命名却未冻结 DTO、错误码、鉴权边界与失败行为，则 `360A` 失败。
12. [ ] 若 successor 主链失败时仍需要实现者在代码阶段临时决定“只读/停写/终止任务/页面提示”语义，则 `360A` 失败。

## 11. 交付物

1. [ ] 计划文档：`docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
2. [ ] 文档地图更新：`AGENTS.md`
3. [ ] 后续实施输入：
   - vendored UI 需收口文件清单
    - runtime status 字段扩展清单
   - compose / env 待删除变量清单
   - compat API 逐端点生死表
   - successor `ui-bootstrap/session` DTO 合同
   - fail-closed 错误码与任务终止语义合同
   - E2E 断言补充清单
4. [ ] 文档治理输入：
   - `220-293` 系列退出归档候选表
   - successor 映射与归档批次说明

## 12. 关联文档

1. `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
2. `docs/dev-plans/360-librechat-depower-and-langgraph-langchain-layered-takeover-plan.md`
3. `docs/dev-plans/361-opa-pdp-adoption-boundary-and-migration-plan.md`
4. `AGENTS.md`
5. `docs/archive/dev-records/dev-plan-360a-execution-log.md`
