# DEV-PLAN-380E：CubeBox `apps/web` 正式前端收口

**状态**: 已完成（2026-04-17 05:46 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `apps/web` 中 `CubeBox` 正式页面、导航、i18n、状态流、错误提示与页面级测试收口的实施 SSOT。  
> `DEV-PLAN-380C` 持有 API/DTO 与 `/internal/assistant/*` 退役 contract；`DEV-PLAN-380D` 持有文件面正式 contract；本文只裁决 `apps/web` 正式产品面什么叫完成、哪些残留必须清掉、哪些页面/测试必须补齐。

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：在不新增第二前端链路、不回退到 vendored LibreChat UI 的前提下，把 `apps/web` 中 `CubeBox` 会话页、文件页、模型页、导航、redirect alias、错误提示、i18n 与页面测试收口到单一正式前端链路。
- **关联模块/目录**：
  - `apps/web/src/pages/cubebox/**`
  - `apps/web/src/api/cubebox.ts`
  - `apps/web/src/router/index.tsx`
  - `apps/web/src/navigation/config.tsx`
  - `apps/web/src/i18n/messages.ts`
  - `apps/web/src/errors/presentApiError.ts`
- **关联计划/标准**：
  - `AGENTS.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/001-technical-design-template.md`
  - `docs/dev-plans/002-ui-design-guidelines.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/020-i18n-en-zh-only.md`
  - `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
  - `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
  - `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
  - `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/dev-plans/380d-cubebox-file-plane-formalization-plan.md`
- **用户入口/触点**：
  - `/app/cubebox`
  - `/app/cubebox/conversations/:conversationId`
  - `/app/cubebox/files`
  - `/app/cubebox/models`
  - `/app/assistant`
  - `/app/assistant/models`

### 0.1 Simple > Easy 三问

1. **边界**：前端正式入口、导航、页面状态流、错误提示与 i18n 由 `380E` 持有；API/DTO shape、canonical 错误码与退役 JSON contract 由 `380C` 持有；文件元数据/删除保护语义由 `380D` 持有。
2. **不变量**：
   - `apps/web` 只允许一条正式聊天前端链路：`/app/cubebox*`
   - `/app/assistant` 与 `/app/assistant/models` 不再承载正式页面组件，只允许 redirect alias
   - `/app/assistant/librechat` 继续保持退役态，不得被重新解释为 redirect alias
   - `/app/cubebox` 首屏必须仍然是聊天壳，不退化成概览面板或占位页
   - 前端不得重建第二套 API client、第二套错误码解释或第二套模型治理/工具平台 UI
3. **可解释**：作者必须能在 5 分钟内说明“用户如何从导航进入 CubeBox、如何进入会话、发送消息、确认/提交、管理文件、查看模型，以及当 API 失败或旧路由访问时前端如何表现”。

### 0.2 现状研究摘要

- **现状实现**：
  - `apps/web` 已存在 `CubeBoxPage`、`CubeBoxFilesPage`、`CubeBoxModelsPage` 与 `apps/web/src/api/cubebox.ts`。
  - 路由层已将 `/app/assistant`、`/app/assistant/models` redirect 到 `/app/cubebox*`，正式页面路由已落到 `/app/cubebox*`。
  - `/app/assistant/librechat` 与 `assistant-ui` 已由服务端退役 handler 承接，完成态是 `410 Gone`，不是 redirect。
  - 导航已注册 `cubebox`、`cubebox-files`、`cubebox-models`，并已收口到 `nav_cubebox*` 专用 i18n key；`cubebox/models` 的导航权限与路由权限已统一为 `orgunit.read`。
  - `CubeBoxPage` 已承接会话列表、消息流、候选确认、提交、回复渲染、会话附件上传等正式聊天骨架。
  - `CubeBoxFilesPage` 与 `CubeBoxModelsPage` 已收口为聊天主入口的配套能力页；模型页补齐了页面级测试与运行态/模型列表分层降级。
- **现状约束**：
  - API/DTO 只允许消费 `/internal/cubebox/*` 正式接口，不能再回写第二份前端 contract。
  - i18n 仅允许 `en/zh`。
  - 错误提示必须走统一 `presentApiError` 映射，不能在页面里散落第二套错误文案策略。
  - `360A` 已冻结“保留聊天 UI 壳、消息树、输入框、基础展示组件”的 successor UX 边界。
- **最容易出错的位置**：
  - 页面把 `/app/cubebox` 继续做成“运行态总览卡片”而不是聊天页。
  - 导航、alias、E2E、i18n、错误提示没有随 `/app/cubebox*` 一起收口。
  - 测试继续沿用 `assistant` 旧断言或 capability 描述，形成语义回流。
  - 文件页、模型页与聊天页之间缺乏明确主次关系，导致主入口退化。
  - `cubebox/models` 出现“用户有路由访问权但看不到导航入口”的可发现性断层。
- **本次不沿用的“容易做法”**：
  - 不恢复 `/app/assistant/librechat` 或 vendored LibreChat Web UI。
  - 不新开一个 `assistant.ts` 或 compat 前端 client 去桥接旧接口。
  - 不通过保留死页面、隐藏菜单但不删代码、或把旧页面改名继续用来糊过收口。

## 1. 背景与上下文（Context）

- **需求来源**：
  - `DEV-PLAN-380` 将 `apps/web` 冻结为 `CubeBox` 唯一正式前端承载面。
  - `DEV-PLAN-380C` 已完成 `/internal/cubebox/*` API/DTO 收口，并将旧 assistant formal entry / `model-providers*` 冻结为稳定退役语义。
  - `DEV-PLAN-360A` 已冻结 successor UX 下必须保留聊天 UI 壳，而不是仅完成品牌或路由替换。
- **当前痛点**：
  - 现有 `380E` 文档仍停留在提纲式 checklist，缺少 owner 边界、失败语义、测试分层与 stopline。
  - `apps/web` 已经有基础页面，但尚未把“哪些内容已完成、哪些只是最小 groundwork、哪些必须继续收口”写成可执行 contract。
  - 代码中仍存在 `assistant` 旧语义残留，例如环境变量命名、错误码映射、测试 fixture 中的 `capability_key` 或旧术语。
- **业务价值**：
  - 让 `CubeBox` 在 `apps/web` 中成为真正可发现、可操作、可验证的正式产品入口。
  - 保证 `380F/380G` 后续只需要消费一个稳定前端面，而不是继续围绕旧入口、旧命名和半活页面收尾。
  - 降低后续页面开发、E2E 与交付验收时的语义漂移成本。
- **仓库级约束**：
  - 单前端链路：`apps/web` 是唯一正式前端。
  - `en/zh` only：所有前端文案必须进入统一 i18n 入口。
  - 用户可见性：前端能力必须可发现、可操作，不能留下“后端可用但 UI 不成形”的僵尸功能。
  - No Legacy：不以 redirect 之外的旧页面/旧 client/旧 alias 作为长期正式路径。
  - 已退役入口保持退役：`/app/assistant/librechat` 不得回流为前端 alias。
  - T2 证据要求：命中的门禁、生成物、一致性与 E2E 入口必须在 readiness 中写清实际执行与结果。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 核心目标

- [X] 把 `/app/cubebox` 明确冻结为正式聊天主入口，具备会话列表、消息流、输入框、发送、确认/提交、附件入口等主链交互。
- [X] 把 `/app/cubebox/files`、`/app/cubebox/models` 收口为聊天主入口的配套能力页，而不是独立产品面或旧助手治理页替身。
- [X] 删除或替换所有不再承载正式运行责任的 assistant 前端残留：页面、client、helper、文案、测试与 alias 语义。
- [X] 将导航、i18n、错误提示、页面测试、E2E 断言统一收口到 `CubeBox` 正式语言和正式路由。
- [X] 冻结 `cubebox/models` 的导航可见性与路由权限口径，消除“能访问但不可发现”的前端权限漂移。
- [X] 为 `380F/380G` 提供稳定前提：前端只依赖 `apps/web + /internal/cubebox/*`，不存在第二 UI 主链。

### 2.2 非目标（Out of Scope）

- 不在本文重新定义 `/internal/cubebox/*` API path、DTO 字段、错误码 canonical 名称；这些以 `380C` 为准。
- 不在本文定义文件元数据 schema、对象存储、引用/删除保护算法；这些以 `380D` 为准。
- 不在本文设计 Prompt Marketplace、Memory、Search、Agents、MCP 管理台或任何超出 `CubeBox v1` 范围的产品能力。
- 不在本文恢复 vendored LibreChat runtime、静态资源或桥接页。
- 不在本文降低页面标准为“只有可打开路由的占位页”。

### 2.3 用户可见性交付

- **用户可见入口**：
  - 左侧导航中的 `CubeBox`
  - `CubeBox` 页面内到 `文件`、`模型` 的入口
  - `/app/assistant`、`/app/assistant/models` redirect alias
- **退役入口约束**：
  - `/app/assistant/librechat` 不属于可见入口规划；其完成态继续是 `410 Gone`
- **最小可操作闭环**：
  - 用户可从导航进入 `/app/cubebox`
  - 用户可查看会话列表、打开会话或创建新会话
  - 用户可输入消息并发起一轮对话
  - 若存在多候选对象，用户可在页面内完成确认
  - 用户可提交任务并看到任务状态反馈
  - 用户可上传并查看会话附件
  - 用户可进入文件页查看最近文件、删除未被引用文件
  - 用户可进入模型页查看当前可用模型与运行态摘要
- **非僵尸功能约束**：
  - `files`、`models` 不允许只在路由上存在但与聊天主入口无任何连接。
  - 若某项能力后端已可用但页面尚未完整承接，必须在本计划中写明当前用户如何触达、如何验收、何时补齐。

## 2.4 工具链与门禁（SSOT 引用）

> 这里只冻结本次命中的门禁边界；实际执行记录写入 readiness。

- **命中触发器（勾选）**：
  - [X] `apps/web/**` / presentation assets / 生成物
  - [X] i18n（仅 `en/zh`）
  - [X] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`error-message`
- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/000-docs-format.md`
  - `docs/dev-plans/002-ui-design-guidelines.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/020-i18n-en-zh-only.md`
  - `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
  - `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/dev-plans/380d-cubebox-file-plane-formalization-plan.md`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `apps/web/src/api/**` | `cubebox` client path、query 参数、task/file/formal entry 调用 | `apps/web/src/api/cubebox.test.ts` | 断言只消费 `/internal/cubebox/*` |
| `apps/web/src/errors/**` | 错误码到用户文案映射 | `apps/web/src/errors/presentApiError.test.ts` | 统一错误入口，不在页面里散落 fallback |
| `apps/web/src/pages/cubebox/**` | 页面状态、主交互、空态/错态、redirect 后主入口可用 | `CubeBoxPage.test.tsx`、`CubeBoxFilesPage.test.tsx`、新增 `CubeBoxModelsPage` 测试 | 先测页面状态机与关键交互，不补洞式堆文件 |
| `apps/web/src/router/**` | `/app/assistant`、`/app/assistant/models` redirect alias、正式路由归属 | 路由级测试或 E2E 断言 | 只验证路由结果与可访问性 |
| `E2E` | 导航发现、主入口、alias redirect、`/app/assistant/librechat` 退役态、主链端到端可操作 | 由 `380G` 统一承接，`380E` 只冻结断言范围 | E2E 是依赖输入，不替代页面测试 |

- **黑盒 / 白盒策略**：
  - API client、错误映射、页面交互默认用黑盒测试。
  - 不为页面内部实现细节写白盒断言；只看用户可见行为和 API 调用结果。
- **并行 / 全局状态策略**：
  - 纯 client/helper 测试可并行。
  - 操作 `react-router-dom` mock、环境变量或全局 `navigator` 的测试，保持串行或局部隔离。
- **fuzz / benchmark 适用性**：
  - 本计划以页面交互和错误映射为主，不额外补 fuzz / benchmark。
- **前端测试原则**：
  - 不新增 `*_coverage_test.go` 式补洞命名。
  - 页面测试优先覆盖“用户能否完成动作”和“错误/空态是否可解释”，而不是只刷渲染覆盖率。

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 5 分钟主流程

```mermaid
flowchart LR
  U[User] --> NAV[apps/web navigation]
  NAV --> PAGE[/app/cubebox* pages]
  PAGE --> API[/internal/cubebox/*]
  API --> SERVICE[modules/cubebox services]
  SERVICE --> DATA[(PostgreSQL / file plane / runtime adapters)]
```

- **主流程叙事**：
  - 用户从 `apps/web` 导航进入 `/app/cubebox`。
  - 页面加载 runtime status、会话列表、目标会话详情与关联文件。
  - 用户在聊天页完成输入、候选确认、提交与回复生成；在文件页完成上传/删除；在模型页查看只读模型列表与运行态摘要。
  - 全部页面统一通过 `apps/web/src/api/cubebox.ts` 调用 `/internal/cubebox/*`。
- **失败路径叙事**：
  - `/app/assistant` 与 `/app/assistant/models` 被访问时，前端只做 redirect alias，不展示旧助手页面。
  - `/app/assistant/librechat` 被访问时，系统继续返回退役语义，而不是 redirect。
  - API 出错时，页面通过统一错误映射展示明确文案，不回退到旧 path 或旧页面。
  - 若 runtime-status 降级，聊天主入口仍需有明确状态显示，不能白屏。
- **恢复叙事**：
  - 会话页可通过刷新列表/重新进入会话恢复局部状态。
  - 文件页和模型页通过重新加载恢复，不引入第二缓存或 fallback client。
  - 遇到 assistant retired/gone 错误，只允许引导用户回到 `CubeBox` 正式入口。

### 3.2 模块归属与职责边界

- **owner module**：`apps/web` 是前端产品面 owner；`modules/cubebox` 与 `internal/server` 只提供被消费的正式 API。
- **交付面**：
  - `apps/web` 页面、导航、i18n、错误提示、前端测试
  - `/app/cubebox*` UI 路由
  - `/app/assistant*` redirect alias
- **跨模块交互方式**：
  - 前端只通过 JSON API 消费 `CubeBox` 正式接口。
  - 不在前端直接拼接 capability 裁决、模型治理写接口或 file-plane 内部语义。
- **组合根落点**：
  - 本计划不把业务逻辑下沉到 router/config 之外的 ad-hoc helper。
  - 统一 client 仍由 `apps/web/src/api/cubebox.ts` 承载；页面只消费稳定函数。

### 3.3 落地形态决策

- **形态选择**：
  - [X] `A. Go DDD`
  - [ ] `B. DB Kernel + Go Facade`
- **选择理由**：
  - `380E` 只消费前端正式交付面，不命中新的写路径形态选择。
  - 本文的核心是前端单链路与页面收口，而不是重新裁决后端写模型。

### 3.4 ADR 摘要

- **决策 1**：`/app/cubebox` 必须保持聊天壳，而不是演化成概览页
  - **备选 A**：把主入口改成运行态/模型/文件卡片总览。缺点：与 `360A` successor UX 边界冲突，用户无法直接开始会话。
  - **备选 B（选定）**：聊天页仍为主入口，文件与模型作为配套页。优点：产品心智清晰，符合“用户一进入就能对话”。

- **决策 2**：`/app/assistant` 与 `/app/assistant/models` 只保留 redirect alias，不保留旧页面组件
  - **备选 A**：旧页面继续存在但菜单隐藏。缺点：会形成半活页面与语义回流。
  - **备选 B（选定）**：路由层直接 redirect 到 `/cubebox*`。优点：入口唯一，可通过 E2E 明确断言。

- **决策 2A**：`/app/assistant/librechat` 保持 `410 Gone`
  - **备选 A**：把退役入口并入 redirect alias。缺点：直接冲突 `380/360A` 已冻结的退役 contract。
  - **备选 B（选定）**：继续保持退役态，只在文档中把它视为负向断言。优点：不回流 legacy。

- **决策 3**：前端错误提示继续统一走 `presentApiError`
  - **备选 A**：各页面各自用 `messageForError` + fallback 文本兜底。缺点：错误语义分叉，无法与 `error-message` 门禁对齐。
  - **备选 B（选定）**：页面文案以统一错误映射为主，页面只补局部上下文 fallback。优点：对外提示稳定、可审计。

- **决策 4**：`cubebox/models` 的导航权限与路由权限统一冻结为 `orgunit.read`
  - **备选 A**：保留“路由可访问但导航不可见”的现状。缺点：违反“可发现、可操作”交付原则。
  - **备选 B（选定）**：导航与路由都统一为 `orgunit.read`。优点：完成定义唯一、可验证，也与当前聊天主入口权限口径保持一致。

- **决策 5**：前端仍允许保留最小 assistant 退役错误码映射，但不再保留正式 assistant 页面/正式 client
  - **备选 A**：删除所有 `assistant_*` 错误语义。缺点：无法解释 redirect 之外的历史退役行为或后续收尾错误。
  - **备选 B（选定）**：保留“退役解释层”，但不保留“正式运行层”。优点：收口更平滑，同时不破坏单主链。

### 3.5 Simple > Easy 自评

- **这次保持简单的关键点**：
  - 前端只认 `/app/cubebox*`
  - API client 只认 `/internal/cubebox/*`
  - 文件页/模型页只是聊天主链的配套页
  - redirect alias 与 retired message 语义分离，不把旧页面继续半活保留
  - 导航权限与路由权限只保留一套可解释口径
- **明确拒绝的“容易做法”**：
  - [X] legacy alias / 双链路 / fallback
  - [X] 页面内自造第二套错误码解释
  - [X] 复制一份 assistant 页面改名继续用
  - [X] 通过只改导航文案掩盖主入口并未真正收口

## 4. 数据模型、状态模型与约束（Data / State Model & Constraints）

### 4.1 数据结构定义

- **页面状态 owner**：
  - `CubeBoxPage`
    - runtime status
    - conversation list
    - selected conversation
    - files scoped to current conversation
    - draft input / busy / error / task feedback
  - `CubeBoxFilesPage`
    - recent files list
    - upload pending / delete pending / error
  - `CubeBoxModelsPage`
    - models list
    - runtime summary
    - load error
- **DTO owner**：
  - `CubeBoxConversation*` / `CubeBoxTurn*` / `CubeBoxTask*` / `CubeBoxSession*` / `CubeBoxRuntimeStatusResponse`：以 `380C` 为准
  - `CubeBoxFile*`：以 `380C + 380D` 组合 contract 为准

### 4.2 时间语义与标识语义

- **页面读取时间**：前端只展示后端返回的 `updated_at`、`created_at`、`uploaded_at`，不在页面内发明额外业务时间字段。
- **路由标识**：
  - 会话详情使用 `conversationId`
  - 文件删除使用 `file_id`
  - 任务反馈使用 `task_id`
- **不变量**：
  - 前端不会把 `assistant` 历史 ID/path 当作新的正式标识。
  - 页面显示的时间字段只做展示，不改变后端 `effective_date/as_of` 语义。

## 5. 路由、UI 与 API 契约（Route / UI / API Contracts）

### 5.1 交付面与路由对齐表

| 交付面 | Canonical Path / Route | `route_class` | owner module | Authz object/action | capability / route-map | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| UI 页面 | `/app/cubebox` | `ui_authn` | `cubebox` | 复用现有 UI 读权限 | N/A | 正式聊天主入口 |
| UI 页面 | `/app/cubebox/conversations/:conversationId` | `ui_authn` | `cubebox` | 同上 | N/A | 会话详情路由 |
| UI 页面 | `/app/cubebox/files` | `ui_authn` | `cubebox` | 同上 | N/A | 文件配套页 |
| UI 页面 | `/app/cubebox/models` | `ui_authn` | `cubebox` | `orgunit.read` | N/A | 模型配套页；导航权限与路由权限统一冻结为 `orgunit.read` |
| UI alias | `/app/assistant` | `ui_authn` | `cubebox` | N/A | N/A | 只允许 redirect 到 `/app/cubebox` |
| UI alias | `/app/assistant/models` | `ui_authn` | `cubebox` | N/A | N/A | 只允许 redirect 到 `/app/cubebox/models` |
| 退役入口 | `/app/assistant/librechat` | `ui_authn` | retired handler | N/A | N/A | 继续返回 `410 Gone`，不是 alias |
| internal API | `/internal/cubebox/*` | `internal_api` | `cubebox` | 以 `380C` 为准 | `380C` 持有 | `apps/web` 唯一消费面 |

- **要求**：
  - 新增前端页面必须挂在 `/app/cubebox*` 下，不新增 `/app/assistant/*` 新页面。
  - `/app/assistant` 与 `/app/assistant/models` 不允许再挂接组件、布局、独立 store 或独立 API prefetch。
  - `/app/assistant/librechat` 不允许重新绑定到 `apps/web` 路由。
  - 导航、搜索入口、路由、测试与页面标题都要一致指向 `CubeBox`。

### 5.2 `apps/web` 交互契约

- **页面/组件入口**：
  - 导航 `CubeBox`
  - 聊天页顶部 `文件` / `模型` 按钮
  - 聊天页内会话列表与会话详情
- **数据来源**：
  - `apps/web/src/api/cubebox.ts`
  - 页面本地 state
  - route param `conversationId`
- **状态要求**：
  - `CubeBoxPage`
    - `loading`：允许初次加载，但最终必须进入会话空态或会话详情态
    - `empty`：无会话时显示“暂无会话”，仍可发送消息创建新会话
    - `error`：页面展示可解释错误，不白屏
    - `busy`：发送、确认、提交、上传附件时按钮禁用
  - `CubeBoxFilesPage`
    - 空态可见“暂无已上传文件”
    - 上传/删除期间禁用相关操作
  - `CubeBoxModelsPage`
    - 模型为空时有明确空态
    - 运行态异常时显示错误，不隐藏整个页面
    - 导航是否可见必须与路由访问权限保持一致，不允许出现隐藏但可达的正式页
- **i18n**：
  - 新增或修改文案必须同步 `en/zh` 到 `apps/web/src/i18n/messages.ts`
  - 不再新增 `assistant` 品牌主文案；如保留历史 key，应明确其服务于 `CubeBox`
- **视觉与交互约束**：
  - 聊天页是主视觉焦点，文件/模型只作为辅助入口
  - 不在页面中散落品牌冲突文案，如 `LibreChat`、`Assistant Model Providers`
  - 保持现有 MUI 页面模式与主题变量，不在页面内引入额外技术栈
- **禁止**：
  - 把 `/app/cubebox` 改成无输入区、无消息流的工作台
  - 为了复用旧代码恢复 `assistant` 页面组件
  - 在页面中硬编码 `/internal/assistant/*` 请求
  - 继续保留导航权限与路由权限不一致的正式页面

### 5.3 JSON API 契约

- `apps/web` 只消费以下正式 client 入口：
  - `createCubeBoxConversation()`
  - `listCubeBoxConversations()`
  - `getCubeBoxConversation()`
  - `createCubeBoxTurn()`
  - `confirmCubeBoxTurn()`
  - `commitCubeBoxTurn()`
  - `renderCubeBoxTurnReply()`
  - `submitCubeBoxTask()`
  - `getCubeBoxTask()`
  - `cancelCubeBoxTask()`
  - `listCubeBoxFiles()`
  - `uploadCubeBoxFile()`
  - `deleteCubeBoxFile()`
  - `getCubeBoxModels()`
  - `getCubeBoxUIBootstrap()`
  - `getCubeBoxSession()`
  - `refreshCubeBoxSession()`
  - `logoutCubeBoxSession()`
  - `getCubeBoxRuntimeStatus()`
- **禁止**：
  - 恢复 `apps/web/src/api/assistant.ts` 为正式运行 client
  - 在页面组件内手写 `/internal/cubebox/*` 路径字符串绕开统一 client
  - 为旧 assistant 接口再做一层 compat 前端 adapter

### 5.4 失败语义消费约束 / stopline

| 失败场景 | owner | 前端正式行为 | 是否允许 fallback | 是否 stopline |
| --- | --- | --- | --- | --- |
| 访问 `/app/assistant` 或 `/app/assistant/models` | router (`380E`) | redirect 到 canonical path | 否 | 否 |
| 访问 `/app/assistant/librechat` | retired handler / `380C` / `360A` | 保持退役态展示，不转成 redirect | 否 | 是 |
| `/internal/cubebox/*` 返回 canonical 错误码 | `380C` / `380D` | 前端只消费并展示，不在本文再冻结第二份 literal | 否 | 是 |
| 页面局部请求失败 | `380E` | 页面展示统一错误映射，不白屏、不跳回旧入口 | 否 | 否 |

- **contract owner 说明**：
  - canonical 错误码、HTTP status、retired JSON envelope 仍由 `380C` 持有。
  - 文件删除阻断等文件面用户可见错误码的最终收口，与 `380D` 剩余项联动；`380E` 只负责消费与展示，不再写第二份错误码主源。
  - 若后续 `380C/380D` 调整 canonical code 或 status，`380E` 只更新消费落点与页面断言，不复制第二份冻结表。
- **错误码约束**：
  - 页面展示优先走统一错误映射。
  - 不允许把 retired/gone 解释成“自动回退到 assistant 旧入口”。
  - `messageForError` 只允许作为最终兜底，不得继续成为页面主错误语义入口。

## 6. 核心流程与算法（Business Flow & Algorithms）

### 6.1 读路径主算法

1. 进入 `/app/cubebox` 或 `/app/cubebox/conversations/:conversationId`
2. 并行加载 runtime status、会话列表、当前会话详情与关联文件
3. 若无 `conversationId`，保持聊天空态并允许用户发送首条消息
4. 若有 `conversationId`，展示消息流、候选区、附件区与主操作按钮
5. 页面只做 view-model 归一化，不改写后端业务语义

### 6.2 写路径主算法

1. 用户在聊天页输入消息并点击发送
2. 若当前没有会话，先创建会话，再创建 turn
3. 返回会话后刷新会话列表与详情
4. 若当前 turn 存在多候选对象，用户在候选面板中确认
5. 用户执行 commit 后显示任务反馈，并拉取任务详情
6. 文件上传和删除分别通过文件 client 完成后刷新页面列表

### 6.3 幂等、回放与恢复

- **幂等键**：前端不新增自造幂等别名，完全消费后端 contract。
- **恢复策略**：
  - 通过刷新会话列表、重新加载会话详情、重新加载文件/模型页实现前向恢复。
  - 不允许通过重开旧助手页、旧 alias 页面或旧 client 恢复。

## 7. 安全、租户、授权与运行保护（Security / Tenancy / Authz / Recovery）

### 7.1 AuthN / Tenancy

- 前端页面全部挂在 `AppShell` 已登录上下文下。
- 未登录或 session 失效时，按现有统一登录流程处理；不新增 `CubeBox` 自己的登录页。
- 前端不自行解释 tenant，只消费后端 fail-closed 结果。

### 7.2 Authz

- 现状中 `CubeBox` 页面路由仍复用既有 UI permission key。
- `380E` 不新增第二套前端 capability 拼装；若后续权限 key 调整，需由相邻计划统一更新路由与导航。
- 页面内禁止因为“当前先能访问”而绕过 `RequirePermission` 自己做临时控制。
- `cubebox/models` 的导航权限与路由权限在本计划中统一冻结为 `orgunit.read`，不得继续分叉。

### 7.3 运行保护

- 不引入功能开关或双入口来保留旧页面。
- 当 runtime-status 降级时，页面需要有明确提示，但仍保留正式入口结构。
- 不以恢复 `/app/assistant/librechat` 作为故障应对。

## 8. 依赖、切片与里程碑（Dependencies & Milestones）

### 8.1 前置依赖

- `380C`：API/DTO、formal entry successor、旧 assistant endpoint 退役 contract 已冻结。
- `380D`：文件 metadata / delete blocked / orphan 可见语义已冻结。
- `360A`：聊天 UI 壳 successor UX 边界已冻结。

### 8.2 建议实施切片

1. [X] **Contract Slice**：将 `380E` 文档细化为可执行 contract，并与 `380C/380D/360A` 对齐。
2. [X] **Delivery Slice**：收口聊天页、文件页、模型页、导航、权限口径、redirect alias、i18n、错误提示。
3. [X] **Test & Gates Slice**：补齐页面/route/error tests，更新 `380G` 依赖的 E2E 断言清单与 `apps/web`/生成物检查。
4. [X] **Readiness Slice**：回写 readiness 证据与剩余 stopline。

### 8.3 每个切片的完成定义

- **Contract Slice**
  - **输入**：`380C/380D/360A` 已可引用
  - **输出**：`380E` 明确 owner、用户闭环、测试分层、失败语义与 stopline
  - **阻断条件**：若相邻计划改变 canonical route 或文件 contract，需先更新本文
- **Delivery Slice**
  - **输入**：正式 API client 已存在
  - **输出**：主入口、配套页、导航、权限口径与残留清理完成
  - **阻断条件**：若实现要求恢复旧页面/旧 client，则必须暂停并更新计划
- **Test & Gates Slice**
  - **输入**：页面与路由实现已收口
  - **输出**：页面/API/error/router 断言完成，生成物一致性已验证，且已为 `380G` 冻结 E2E 断言范围
  - **阻断条件**：若测试只能依赖旧 assistant 路径，视为未完成
- **Readiness Slice**
  - **输入**：实施与验证完成
  - **输出**：形成 readiness 记录与剩余 follow-up
  - **阻断条件**：若尚存在第二前端链路或死页面，则不得宣告完成

## 9. 测试、验收与 Readiness（Acceptance & Evidence）

### 9.1 验收标准

- **边界验收**：
  - [X] `380E` 只持有前端产品面 contract，不重复裁决 `380C/380D`
  - [X] `apps/web` 没有第二聊天前端入口
- **用户可见性验收**：
  - [X] 用户可以从导航发现 `CubeBox`
  - [X] 用户可从 `/app/cubebox` 完成至少一条对话发送闭环
  - [X] 用户可从聊天页进入文件页和模型页
  - [X] `/app/assistant` 与 `/app/assistant/models` 只体现 redirect，不再出现旧页面组件
  - [X] `/app/assistant/librechat` 继续体现退役态，而不是 redirect
- **数据 / 时间 / 租户验收**：
  - [X] 页面不混用旧 assistant 标识与新 cubebox 标识
  - [X] 未登录/无权限/后端 fail-closed 行为不被页面绕开
- **UI / API 验收**：
  - [X] `apps/web` 所有正式请求仅走 `/internal/cubebox/*`
  - [X] 聊天页仍是主入口，不退化为纯状态总览页
  - [X] 文件页与模型页不承接旧助手治理语义
  - [X] `cubebox/models` 的导航权限与路由权限已经一致，且统一为 `orgunit.read`
  - [X] 新增文案已对齐 `en/zh`
- **测试与门禁验收**：
  - [X] `apps/web` API client、页面、错误映射、路由 alias 断言已补齐
  - [X] 命中的 `apps/web` 构建检查、生成物一致性、i18n 与 `error-message` 验证通过
  - [X] `380G` 所需 E2E 断言范围已在 readiness 中冻结并交接
  - [X] 没有通过保留死页面/隐藏入口/临时 fallback 伪装完成态

### 9.2 Readiness 记录

- [X] 新建或更新 `docs/archive/dev-records/DEV-PLAN-380E-READINESS.md`
- [X] readiness 至少记录：
  - `pnpm --dir apps/web test`
  - `pnpm --dir apps/web build`
  - `pnpm --dir apps/web check`
  - `make generate`
  - `make css`
  - `git status --short` 为空
  - 命中的 `make check tr`
  - 命中的 `make check error-message`
  - 命中的 `make e2e`
  - 页面级证据：`/app/cubebox`、`/app/cubebox/files`、`/app/cubebox/models`
  - alias/retired 证据：`/app/assistant` redirect、`/app/assistant/models` redirect、`/app/assistant/librechat` = `410 Gone`
  - 交接给 `380G` 的 E2E 断言清单与覆盖范围
- [X] 本文不复制执行输出，只链接 readiness 证据

### 9.3 例外登记

- **白盒保留理由**：当前不计划新增白盒测试。
- **暂不并行理由**：页面测试共用 router mock 时，串行更稳定。
- **不补 fuzz / benchmark 理由**：本计划不涉及开放输入空间解析器或热点性能函数。
- **暂留页面级测试理由**：聊天主入口的关键价值在用户可见交互，页面级测试是必要边界，不下沉为纯函数即可完全替代。
- **暂不能下沉到更小层的理由**：当前主问题在页面 IA 与交互责任，而不是纯函数复杂度。
- **若删除死分支/旧链路**：必须证明 `/app/assistant` 与 `/app/assistant/models` 已只剩 redirect alias，且 `/app/assistant/librechat` 继续保持退役 contract，不改变对外 canonical path。

## 9.4 当前完成回写与剩余 stopline

### 9.4.1 当前已完成 groundwork

- [X] `apps/web` 已落地 `CubeBoxPage`、`CubeBoxFilesPage`、`CubeBoxModelsPage`。
- [X] `apps/web/src/api/cubebox.ts` 已成为正式前端 client，并消费 `/internal/cubebox/*`。
- [X] `/app/assistant` 与 `/app/assistant/models` 已在路由层改为 redirect alias。
- [X] 导航中已存在 `CubeBox` 入口。
- [X] 聊天页已具备最小聊天骨架：会话列表、消息区、输入区、候选确认、提交、附件上传。
- [X] `/app/assistant/librechat` 已在相邻计划中冻结为 `410 Gone`，不再是前端活入口。

### 9.4.2 剩余 stopline（本文完成前必须清零）

- [X] 将现有“聊天页顶部说明文案仍强调最小版本/当前版本”的临时措辞收口为正式产品文案。
- [X] 收口导航与 i18n key，避免继续以 `nav_ai_assistant` 这类历史 key 作为长期正式语义。
- [X] 将 `cubebox/models` 的导航权限显式改为 `orgunit.read`，与路由权限统一，消除“可访问但不可发现”的状态。
- [X] 审视并清理页面、错误映射、测试中的 `assistant` 历史术语残留，冻结哪些保留用于退役解释，哪些应彻底删除。
- [X] 将 `CubeBoxPage`、`CubeBoxFilesPage`、`CubeBoxModelsPage` 从本地 `messageForError` 主路径收口到统一错误映射。
- [X] 为 `CubeBoxModelsPage` 补齐页面级测试，避免只靠最小渲染存在。
- [X] 增加或更新 alias/retired 断言，证明 `/app/assistant`、`/app/assistant/models` 已不再承接正式页面，且 `/app/assistant/librechat` 继续 `410 Gone`。
- [X] 校对 `CubeBoxPage.test.tsx` 中的历史 capability/fixture 术语，避免把旧 assistant 语言继续固化为前端正式 contract。
- [X] 将 readiness 证据补齐到 T2 口径：`make generate && make css`、`git status --short`、`make check error-message`、`make e2e`。

### 9.4.3 完成证据

- [X] Readiness 记录：`docs/archive/dev-records/DEV-PLAN-380E-READINESS.md`
- [X] `pnpm --dir apps/web build`
- [X] `pnpm --dir apps/web check`
- [X] `make generate`
- [X] `make css`
- [X] `make check tr`
- [X] `make check error-message`
- [X] `tp220-assistant.spec.js` + `tp283-librechat-formal-entry-cutover.spec.js` 定向 E2E 子集通过，覆盖 alias/retired/CubeBox 正式入口
- [X] `tp060-02-master-data.spec.js` 定向 E2E 通过，覆盖主数据链路中的 CubeBox 入口断言
- [ ] 完整 `make e2e` 仍存在非 `380E` 范围残留：备用端口完整套件中 `tp290b-e2e-002` 返回 `ai_plan_schema_constrained_decode_failed`；该问题属于模型/后端链路，已在 readiness 中登记，不阻塞本文前端收口完成。

## 10. 附：作者自检清单（可复制到评审评论）

- [X] 我已经写清 `380E` 与 `380C/380D/360A` 的边界，没有重复冻结别人持有的 contract
- [X] 我已经说明为什么 `/app/cubebox` 必须保持聊天壳，而不是“先做成概览页更容易”
- [X] 我没有把旧 assistant 页面、旧 client 或第二前端链路当 fallback
- [X] 我已经写清命中的前端门禁与 readiness 落点，但没有复制命令矩阵
- [X] 我已经写清用户如何进入 `CubeBox`、如何完成会话/文件/模型三条最小闭环
- [X] 我已经为 reviewer 提供 5 分钟可复述的前端主流程、失败路径与 stopline
