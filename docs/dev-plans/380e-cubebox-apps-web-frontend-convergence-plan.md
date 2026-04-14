# DEV-PLAN-380E：CubeBox `apps/web` 正式前端收口

**状态**: 草拟中（2026-04-14 20:52 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `apps/web` 中 `CubeBox` 正式页面、导航、i18n、状态流与页面测试收口的实施 SSOT。  
> `380C` 负责 API/DTO；本文只负责前端正式产品面。

## 1. 背景与定位

1. [ ] 当前 `CubeBox` 页面已具备最小正式入口，但仍是第一轮最小交付，不代表前端产品面完全收口。
2. [ ] 旧 `assistant/librechat` 前端残留命名、路由与页面仍需系统清理。
3. [ ] `apps/web` 是 `CubeBox` 的唯一正式前端承载面，不再接受 vendored LibreChat Web UI 作为正式入口。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 收口 `/app/cubebox`、`/app/cubebox/files`、`/app/cubebox/models` 的 IA、状态流、错误态与空态。
2. [ ] 完成导航、i18n key、测试文案与残留 `LibreChat`/`assistant` 前端命名清理。
3. [ ] 补齐页面级测试与必要的前端回归。

### 2.2 非目标

1. [ ] 不在本文定义数据库 schema。
2. [ ] 不在本文设计 `/internal/cubebox/*` API 契约。
3. [ ] 不在本文处理 `third_party/librechat-web`、`deploy/librechat/*` 的仓库级退役。

## 3. 关键边界

1. [ ] 正式前端入口冻结为 `apps/web`，不新增第二前端工程。
2. [ ] 不复刻 LibreChat Marketplace、Memory、Search、Agents 侧边栏。
3. [ ] 页面能力只消费 `CubeBox` 正式 API，不再依赖 `assistant/librechat` 桥接页。

## 4. 实施步骤

1. [ ] 收口会话页、文件页、模型页与正式导航。
2. [ ] 清理前端残留 `LibreChat` 文案、路由、测试与命名。
3. [ ] 补齐页面级测试、状态流回归与 `pnpm check` 验证。

## 5. 验收与测试

1. [ ] `pnpm --dir apps/web test`
2. [ ] `pnpm --dir apps/web build`
3. [ ] `pnpm --dir apps/web check`
4. [ ] `/app/cubebox` 系列页面 E2E/手工验收

## 6. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/002-ui-design-guidelines.md`
