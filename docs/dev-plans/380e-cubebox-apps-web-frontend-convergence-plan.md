# DEV-PLAN-380E：CubeBox `apps/web` 正式前端收口

**状态**: 草拟中（2026-04-14 20:52 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `apps/web` 中 `CubeBox` 正式页面、导航、i18n、状态流与页面测试收口的实施 SSOT。  
> `380C` 负责 API/DTO；本文只负责前端正式产品面。

## 1. 背景与定位

1. [ ] 当前 `CubeBox` 页面已具备最小正式入口，但仍是第一轮最小交付，不代表前端产品面完全收口。
2. [ ] 旧 `assistant/librechat` 前端残留命名、路由与页面仍需系统清理。
3. [ ] `apps/web` 是 `CubeBox` 的唯一正式前端承载面，不再接受 vendored LibreChat Web UI 作为正式入口。
4. [ ] `DEV-PLAN-360A` 已冻结“保留聊天 UI 壳、消息树、输入框、基础展示组件”的 successor UX 边界；`380E` 必须在 `apps/web` 中承接这条契约，而不是只做 API 改名或最小页面占位。
5. [ ] 当前 `apps/web` 的主要残留已从“assistant formal entry 仍在调用”收敛为“assistant 命名页面、死 API client、旧 IA 文案仍待删除”。
6. [ ] 根据 `380C` 新冻结口径，前端本批目标不是继续消费 assistant formal entry，而是删除 assistant 残留前端与死页面，并确保正式入口只剩 `/app/cubebox*`。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 收口 `/app/cubebox`、`/app/cubebox/files`、`/app/cubebox/models` 的 IA、状态流、错误态与空态。
2. [ ] 完成导航、i18n key、测试文案与残留 `LibreChat`/`assistant` 前端命名清理。
3. [ ] 补齐页面级测试与必要的前端回归。
4. [ ] 在 `CubeBox` 品牌下保留“尽量接近 LibreChat”的正式聊天交互骨架，避免正式入口退化成与聊天产品形态无关的通用工作台页面。
5. [ ] 基于 `380C` 已冻结的 successor/gone 方案完成前端收口：
   - 正式请求只保留 `/internal/cubebox/*`
   - 已 gone 的 assistant formal entry / `model-providers*` 不再保留页面或死 API
   - `/app/assistant` 与 `/app/assistant/models` 若继续存在，只作为 redirect alias

### 2.2 非目标

1. [ ] 不在本文定义数据库 schema。
2. [ ] 不在本文设计 `/internal/cubebox/*` API 契约。
3. [ ] 不在本文处理 `third_party/librechat-web`、`deploy/librechat/*` 的仓库级退役。
4. [ ] 不要求继续保留 vendored LibreChat 技术栈、源码目录、runtime、Marketplace、Memory、Search、Agents 等平台能力。

## 3. 关键边界

1. [ ] 正式前端入口冻结为 `apps/web`，不新增第二前端工程。
2. [ ] 不复刻 LibreChat Marketplace、Memory、Search、Agents 侧边栏。
3. [ ] 页面能力只消费 `CubeBox` 正式 API，不再依赖 `assistant/librechat` 桥接页。
4. [ ] “尽量像 LibreChat”只约束正式聊天体验的 IA 与交互骨架，不等于继续依赖 vendored Web UI 或恢复 `/app/assistant/librechat`。
5. [ ] `CubeBox` 正式聊天页至少应继承以下骨架能力：
   - 左侧会话列表 / 新建会话入口
   - 中央消息流 / turn 渲染区
   - 底部输入框 / 发送 / 停止 / 重试或重新生成等主交互区
   - 基础 markdown / code / table / image / file 展示
6. [ ] 若 `files`、`models` 页面采用独立页，它们仍应作为聊天页的配套能力页，而不是让 `/app/cubebox` 主入口退化成与会话交互无关的概览页。
7. [ ] `apps/web` 不允许继续保留 assistant 模型治理页、`/app/assistant/librechat` 桥接页或仅为旧页面服务的 helper/API；前端正式面只保留 `CubeBox` IA。

## 3.2 assistant 残留前端收口矩阵（引用 `380C` SSOT）

1. [ ] `GET /internal/assistant/ui-bootstrap`
   - successor path 以 `380C` 冻结的 `GET /internal/cubebox/ui-bootstrap` 为准
   - 当前后端状态：已稳定 `410 Gone`
   - 前端完成态：不再保留 assistant client 或页面对其调用
2. [ ] `GET /internal/assistant/session`
   - successor path 以 `380C` 冻结的 `GET /internal/cubebox/session` 为准
   - 当前后端状态：已稳定 `410 Gone`
   - 前端完成态：不再保留 assistant client 或页面对其调用
3. [ ] `POST /internal/assistant/session/refresh`
   - successor path 以 `380C` 冻结的 `POST /internal/cubebox/session/refresh` 为准
   - 当前后端状态：已稳定 `410 Gone`
   - 前端完成态：刷新逻辑只走 cubebox path 或删除未使用调用
4. [ ] `POST /internal/assistant/session/logout`
   - successor path 以 `380C` 冻结的 `POST /internal/cubebox/session/logout` 为准
   - 当前后端状态：已稳定 `410 Gone`
   - 前端完成态：退出逻辑只走 cubebox path 或删除未使用调用
5. [ ] `GET /internal/assistant/model-providers`
   - 不进入 `CubeBox` 正式前端
   - `apps/web` 若仍存在调用，应在本计划批次删除，而不是寻找 successor
6. [ ] `POST /internal/assistant/model-providers:validate`
   - 不进入 `CubeBox` 正式前端
   - `apps/web` 若仍存在调用，应在本计划批次删除，而不是寻找 successor
7. [ ] 失败语义
   - assistant formal entry 与 `model-providers*` 返回 `410 Gone` 属于预期完成态，不得触发“回退改回 assistant path”
   - 前端不允许为了 convenience 再引入第三条启动链
8. [ ] 权责边界
   - path/method/错误码/final successor 定义由 `380C` 持有；本文只负责前端消费、降级行为与测试
   - 若 `380C` 调整 formal entry path，本文件只更新消费落点与验证步骤，不单独再冻结第二份 API 契约

## 3.1 体验继承责任

1. [ ] `360A` 中“保留聊天 UI 壳”的产品责任，自本计划起由 `380E` 继承为 `CubeBox` 一方前端的正式契约。
2. [ ] 任何把 `/app/cubebox` 主入口改造成非聊天壳形态的实现，都必须先更新本计划并说明为何仍满足 `360A` successor UX 边界。
3. [ ] 若后续希望主动背离 LibreChat 式聊天骨架，必须在 `380E` 中显式改写目标 IA、交互原型与验收标准，不能靠实现默认漂移。

## 4. 实施步骤

1. [ ] 先冻结 `CubeBox` 聊天主入口 IA：会话列表、消息区、输入区、页面级配套入口与主次层级。
2. [ ] 收口会话页、文件页、模型页与正式导航。
3. [ ] 按 `360A` successor UX 边界补齐聊天壳所需交互：新建会话、选中会话、消息渲染、发送、停止、重试/重新生成、附件展示。
4. [ ] 清理前端残留 `LibreChat` 文案、路由、测试与命名。
5. [ ] 补齐页面级测试、状态流回归与 `pnpm check` 验证。

## 4.1 首批实现清单（按文件落点）

1. [ ] `apps/web/src/api/cubebox.ts`
   - 补齐 formal entry API client：
     - `getCubeBoxUIBootstrap()`
     - `getCubeBoxSession()`
     - `refreshCubeBoxSession()`
     - `logoutCubeBoxSession()`
   - 后续若 `380C` 明确 runtime-status/files/models DTO 继续收口，也由此文件承接 cubebox 正式 path
2. [ ] `apps/web/src/api/assistant.ts`
   - 删除该文件或将其完全替换为中性 shared types，不再保留正式运行 client
3. [ ] `apps/web/src/api/cubebox.test.ts`
   - 承接 conversations / turns / tasks / models / runtime-status / formal entry 的正式 API 测试
4. [ ] `apps/web/src/pages/assistant/*`
   - 删除 `AssistantModelProvidersPage.tsx`
   - 删除 `LibreChatPage.tsx`
   - 删除 `AssistantPage.tsx`
   - 删除仅为上述页面服务的测试与 helper
5. [ ] `apps/web` 路由与启动链
   - 确认 `/app/cubebox` 成为唯一正式聊天主入口
   - `/app/assistant`、`/app/assistant/models` 若保留，仅作为 redirect alias
6. [ ] `internal/server` 配套改动清单（供前后端联动，不在本文实现）
   - `internal/server/handler.go`：新增 cubebox formal entry path，并为 assistant formal entry 进入兼容窗口/`410 Gone` 做准备
   - `internal/server/capability_route_registry.go`：新增 cubebox formal entry path，后续删除 assistant formal entry path
   - `internal/server/assistant_formal_entry_api.go`：迁移或下沉为 cubebox formal entry 承载
7. [ ] 测试联动清单
   - `internal/server/handler_test.go`
   - `internal/server/capability_route_registry_test.go`
   - `internal/server/authz_middleware_test.go`
   - `internal/server/tenancy_middleware_test.go`
   - `internal/server/assistant_formal_entry_api_test.go`
   - 上述文件需随着 successor/gone 方案同步更新，不能只改前端 API client

## 4.2 分批切换策略

1. [ ] Phase E0：文档冻结
   - 读取并落实 `380C` formal entry successor/gone 决策
   - 不在本文重复冻结 `/internal/cubebox/ui-bootstrap`、`/internal/cubebox/session`、`/internal/cubebox/session/refresh`、`/internal/cubebox/session/logout`
2. [ ] Phase E1：API client 切换
   - `cubebox.ts` 成为唯一正式 client
   - `assistant.ts` 不再保留正式运行 client
3. [ ] Phase E2：页面与状态流切换
   - `/app/cubebox` 首屏、登录后恢复、刷新会话、退出登录全部不再依赖 assistant 页面或 assistant API client
4. [ ] Phase E3：测试与文案清理
   - 更新前端 API tests、页面测试、E2E 文案、开发文档
5. [ ] Phase E4：进入 `380C C3/C4`
   - 仅在前端与 E2E 全量切走后，compat window only 的旧 assistant 主业务 API 才可进入最终删除批次

## 5. 验收与测试

1. [ ] `pnpm --dir apps/web test`
2. [ ] `pnpm --dir apps/web build`
3. [ ] `pnpm --dir apps/web check`
4. [ ] `/app/cubebox` 系列页面 E2E/手工验收
5. [ ] `/app/cubebox` 首屏必须是聊天主入口，而不是通用工作台或模块占位页。
6. [ ] 手工或 E2E 验证中，必须能在 `/app/cubebox` 明确看到：
   - 会话列表或会话创建入口
   - 消息流区域
   - 输入框与发送主操作
7. [ ] 若页面视觉不再使用 vendored LibreChat 资产，也必须在 IA 与交互层证明其仍是“LibreChat successor chat shell”，而不是仅保留品牌名。
8. [ ] `/app/assistant` 与 `/app/assistant/models` 仅验证 redirect 到 `/app/cubebox*`，不再对应任何 assistant 页面组件。
9. [ ] 当 assistant formal entry 维持 `410 Gone` 时，前端仍能以 `CubeBox` 正式页正常启动，不出现“首屏白屏”或“点进死页面”。
10. [ ] 若 `model-providers*` 在 `apps/web` 中已无正式入口，则相关调用、测试和文案必须一起删除；不得只在后端标记 gone 而前端继续发请求。

## 6. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/002-ui-design-guidelines.md`
3. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
4. `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
