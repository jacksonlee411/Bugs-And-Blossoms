# DEV-PLAN-380B：CubeBox 后端正式实现面切换

**状态**: 草拟中（2026-04-14 20:52 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `modules/cubebox` 后端正式实现面切换的实施 SSOT。  
> `DEV-PLAN-380A` 负责数据面 contract；本文负责 `modules/cubebox` 的业务实现、模块分层与从 `assistant` 复用链路迁出。

## 1. 背景与定位

1. [ ] 当前 `/internal/cubebox/*` 主链已接线，但正式业务逻辑仍大量复用 `assistantConversationService`。
2. [ ] `CubeBox` 需要从“新命名空间 + 旧实现复用”切换成“新命名空间 + 新模块实现”的正式形态。
3. [ ] 仓库 DDD 分层要求新业务逻辑进入 `modules/cubebox/*`，而不是继续扩散在 `internal/server`。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 在 `modules/cubebox` 落地正式的 `domain / services / infrastructure / presentation` 实现。
2. [ ] 将会话、turn、task、models、runtime-status、files 的正式实现迁出 `assistant` 复用链路。
3. [ ] 收口 `internal/server` 只保留 HTTP 接线、tenant/authz 适配与模块装配职责。

### 2.2 非目标

1. [ ] 不在本文定义数据库 schema 细节。
2. [ ] 不在本文设计前端页面与导航。
3. [ ] 不在本文处理 `third_party/librechat-web`、`deploy/librechat/*` 退役。

## 3. 关键边界

1. [ ] `CubeBox` 继续消费现有 authoritative gate、knowledge runtime、policy/runtime 主线，不新增第二策略解释器。
2. [ ] 新后端实现必须遵循仓库 DDD layering 与 capability/authz/routing 约束。
3. [ ] 模块装配与 HTTP handler 的迁移必须 fail-closed，不保留静默 fallback。

## 4. 实施步骤

1. [ ] 建立 `cubebox` domain model、service contract 与 repository interface。
2. [ ] 将现有 API 所需业务逻辑迁入 `modules/cubebox/services` 与 `infrastructure`。
3. [ ] 收口 `internal/server` 对 `cubebox` 模块的装配、错误映射与流式响应。

## 5. 验收与测试

1. [ ] 为 `modules/cubebox/domain`、`services`、`infrastructure` 补齐直接单测。
2. [ ] 为 handler 与 service 边界补齐 API matrix 回归。
3. [ ] 跑通与 `380A/380C` 联动后的服务端回归。

## 6. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/015-ddd-layering-framework.md`
3. `docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
