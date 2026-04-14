# DEV-PLAN-380E：CubeBox `apps/web` 正式前端收口

**状态**: 草拟中（2026-04-14 20:52 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `apps/web` 中 `CubeBox` 正式页面、导航、i18n、状态流与页面测试收口的实施 SSOT。  
> `380C` 负责 API/DTO；本文只负责前端正式产品面。

## 1. 背景与定位

1. [ ] 当前 `CubeBox` 页面已具备最小正式入口，但仍是第一轮最小交付，不代表前端产品面完全收口。
2. [ ] 旧 `assistant/librechat` 前端残留命名、路由与页面仍需系统清理。
3. [ ] `apps/web` 是 `CubeBox` 的唯一正式前端承载面，不再接受 vendored LibreChat Web UI 作为正式入口。
4. [ ] `DEV-PLAN-360A` 已冻结“保留聊天 UI 壳、消息树、输入框、基础展示组件”的 successor UX 边界；`380E` 必须在 `apps/web` 中承接这条契约，而不是只做 API 改名或最小页面占位。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 收口 `/app/cubebox`、`/app/cubebox/files`、`/app/cubebox/models` 的 IA、状态流、错误态与空态。
2. [ ] 完成导航、i18n key、测试文案与残留 `LibreChat`/`assistant` 前端命名清理。
3. [ ] 补齐页面级测试与必要的前端回归。
4. [ ] 在 `CubeBox` 品牌下保留“尽量接近 LibreChat”的正式聊天交互骨架，避免正式入口退化成与聊天产品形态无关的通用工作台页面。

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

## 6. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/002-ui-design-guidelines.md`
3. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
