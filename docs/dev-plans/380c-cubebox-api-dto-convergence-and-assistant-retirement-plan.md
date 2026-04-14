# DEV-PLAN-380C：CubeBox API/DTO 收口与 `/internal/assistant/*` 退役

**状态**: 草拟中（2026-04-14 20:52 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `CubeBox` API/DTO 收口与旧 `/internal/assistant/*` 正式退役的实施 SSOT。  
> `DEV-PLAN-380` 负责总范围；本文只负责命名空间、DTO、错误码、runtime-status 契约与旧 API 退役策略。

## 1. 背景与定位

1. [ ] 当前 `/internal/cubebox/*` 与 `/internal/assistant/*` 仍并存，旧命名空间尚未进入正式退役阶段。
2. [ ] 现有部分 DTO 与运行态语义仍带 `assistant` 或 `LibreChat` 历史命名。
3. [ ] `CubeBox` 要成为正式 API 面，必须把命名、错误码、runtime-status 与鉴权/能力映射同步收口。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 收口 `/internal/cubebox/*` 为唯一正式 API 命名空间。
2. [ ] 收口 `cubeboxConversation / cubeboxTurn / cubeboxTask / cubeboxFile / cubeboxFileLink` 等 DTO 口径。
3. [ ] 明确 `/internal/assistant/*` 的删除、`410 Gone` 或迁移窗口策略，并实现配套断言。

### 2.2 非目标

1. [ ] 不在本文设计数据库表结构。
2. [ ] 不在本文实现前端页面视觉或 IA。
3. [ ] 不在本文处理 `370` 知识 runtime 的命名裁决。

## 3. 关键边界

1. [ ] `CubeBox` 产品 DTO 不得与 `370` 的知识 prompt / clarification / reply guidance 命名冲突。
2. [ ] 路由、capability、authz、error-message 门禁必须与 API 命名空间收口同步更新。
3. [ ] 旧命名空间退役必须显式断言，不允许隐式桥接长期存在。

## 4. 实施步骤

1. [ ] 盘点并收口 `/internal/cubebox/*` DTO、错误码、runtime-status 字段与客户端消费面。
2. [ ] 设计并实施 `/internal/assistant/*` 正式退役批次。
3. [ ] 补齐 routing/capability/authz/error-message/compat 断言。

## 5. 验收与测试

1. [ ] API matrix 覆盖 `cubebox` 主链与旧 `/internal/assistant/*` 退役断言。
2. [ ] `make check routing`
3. [ ] capability-route-map / authz / error-message 相关门禁回归

## 6. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/017-routing-strategy.md`
3. `docs/dev-plans/022-authz-casbin-toolchain.md`
