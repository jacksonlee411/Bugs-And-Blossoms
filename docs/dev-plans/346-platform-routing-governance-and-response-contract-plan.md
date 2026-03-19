# DEV-PLAN-346：平台路由治理与返回契约子计划（Route Class / Responder / Exposure Gates）

**状态**: 规划中（2026-03-18 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“平台基座先行、边界清晰、fail-closed”的冻结；
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 对“Product Shell 与 Routing & Navigation 一致表达”的要求；
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md) 对 webhook、集成入口和异步回执路径治理的需求；
- 现仓 [DEV-PLAN-017](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/017-routing-strategy.md) 沉淀的“强命名空间 + route_class + 全局 responder + 门禁”经验。

`346` 的职责不是替代 IA 设计，而是冻结平台级路由治理合同：

- 哪些入口是 UI、内部 API、外部 API、Webhook、Ops、Dev/Test；
- 不同入口失败时返回什么契约；
- 生产环境哪些路径默认不可暴露；
- 这些规则如何进入统一门禁并阻断漂移。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结 Greenfield 全局命名空间与 `route_class` 分类口径（含 segment 边界匹配要求）。
- [ ] 冻结按 `route_class` 的成功/失败返回契约（至少覆盖 404/405/500）。
- [ ] 冻结路由 allowlist 与暴露面治理规则（生产默认 fail-closed）。
- [ ] 将路由分类、返回契约、暴露面检查纳入统一质量门禁。
- [ ] 与 `314` 对齐 route contract 与 payload contract 的职责边界，避免路由治理与 API schema 治理再次混写。
- [ ] 为 `340/350/370` 提供共享路由治理输入，避免再次出现模块各自定义路由语义。

### 2.2 非目标

- [ ] 本计划不定义业务模块具体路由清单与 handler 代码实现。
- [ ] 本计划不替代 [DEV-PLAN-351](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/351-product-shell-and-route-information-architecture-detailed-design.md) 的产品 IA 语义与导航结构。
- [ ] 本计划不引入新的 OpenAPI 生成体系或第二套网关体系。
- [ ] 本计划不通过 legacy alias、双链路或环境分叉来兜底路由漂移。

## 3. 范围

- 全局命名空间与 `route_class` 词汇表
- 路径分类规则与 allowlist SSOT
- 全局 responder 返回契约
- 内容协商与默认 content-type 规则
- 生产暴露面规则（dev/test/diagnostic 路径）
- 路由治理门禁接线（本地与 CI）

## 4. 关键设计决策

### 4.1 单点分类：`route_class` 是路由语义权威（选定）

- 路径进入业务前，必须先完成 `path -> route_class` 分类。
- 分类结果用于绑定默认 middleware 与全局错误 responder。
- 禁止在业务模块中重写第二套路径语义分类。

### 4.2 失败返回按类收敛，不按模块自定义（选定）

- UI 类入口默认 HTML 壳层返回。
- JSON API/Webhook 类入口默认 JSON-only 返回。
- 404/405/500 等全局失败由 responder 统一产出，不由业务 handler 各自兜底。

### 4.3 暴露面默认 fail-closed（选定）

- 生产环境默认不暴露 dev/test/playground 入口。
- allowlist 不可用或分类冲突时必须 fail-fast，不允许静默降级。

### 4.4 路由治理以“规则 + 门禁”落地（选定）

- 规则必须体现在机器可校验资产中（allowlist/registry）。
- 门禁必须能阻断：未注册入口、类别漂移、返回契约漂移、非法暴露。

## 5. 建议实施分期

1. [ ] `M1`：命名空间与 `route_class` 合同冻结  
   冻结词汇表、分类边界、entrypoint 约束与 segment 匹配规则。
2. [ ] `M2`：全局 responder 与返回契约冻结  
   冻结 404/405/500 的按类响应语义与最小字段。
3. [ ] `M3`：暴露面与门禁收口  
   将 allowlist、分类、暴露面检查接入 `make check routing` 与 CI required checks。

## 6. 与其他子计划关系

- `340`：拥有平台入口、认证与上下文边界；`346` 提供路由治理合同输入。
- `314`：拥有普通业务 API 的 schema / DTO / compatibility gate；`346` 只拥有 path / `route_class` / responder / exposure contract。
- `350/351`：拥有 IA 与导航；必须消费 `346` 的路由语义与返回契约边界。
- `370`：Webhook/Integration 入口必须按 `346` 的 `route_class` 与暴露面规则落地。
- `390`：Assistant 相关入口同样受 `346` 约束，不可自建旁路入口。

## 7. 验收标准

- [ ] 已形成 Greenfield 统一路由语义词汇表，不再由模块自行解释路径类别。
- [ ] 404/405/500 等全局失败在不同 `route_class` 下有稳定、可预期返回契约。
- [ ] 生产暴露面具有默认 fail-closed 约束，dev/test 路径不能被误暴露。
- [ ] `make check routing` 能阻断路由分类漂移、未登记入口与返回契约漂移。
- [ ] `346` 与 `314` 的职责边界清晰：路径/暴露面 drift 由 `346` 阻断，payload/schema drift 由 `314` 阻断，二者不再互相吞责。
- [ ] `340/350/370` 可以直接引用 `346`，不再重复定义同类规则。

## 8. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-314](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/314-api-contract-governance-compatibility-and-quality-gates-detailed-design.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-351](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/351-product-shell-and-route-information-architecture-detailed-design.md)
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md)
- [DEV-PLAN-017](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/017-routing-strategy.md)
