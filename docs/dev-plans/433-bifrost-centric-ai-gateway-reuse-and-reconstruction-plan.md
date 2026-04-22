# DEV-PLAN-433：Bifrost 主参考的 AI 网关复用/重构方案

**状态**: 实施中（2026-04-22 16:40 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：承接 `DEV-PLAN-430` 的 Slice 2“AI 网关最小闭环”，首期只做 one OpenAI-compatible provider、active model、health validation、SSE 直通与错误映射；以 Bifrost 作为一方 Go 网关的主参考，尽量复用或重构其 provider、streaming、health/readiness、telemetry 与测试形状，最大化避免在 CubeBox 中重复造车；Codex 仅复用 provider adapter、Responses/OpenAI-compatible 映射和 stream parser 等局部能力。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`modules/cubebox`（候选新模块路径）、`internal/server`、`config`、`migrations`、`sqlc`、`scripts/ci`
- **外部来源**：
  - `https://github.com/maximhq/bifrost`
  - `https://github.com/openai/codex`
  - `https://github.com/BerriAI/litellm`
  - `https://github.com/songquanpeng/one-api`
  - `https://github.com/Portkey-AI/gateway`
- **核心参考文件/目录（优先级顺序）**：
  - Bifrost：`core router`、`provider selection`、`streaming`、`observability`、`health/readiness` 相关目录与测试；`fallback` 仅做后续预留评估，不进入首期验收
  - Codex：`codex-rs/model-provider/**`
  - Codex：`codex-rs/model-provider-info/**`
  - Codex：`codex-rs/codex-api/**`
  - Codex：`codex-rs/responses-api-proxy/**`
  - Codex：`codex-rs/utils/stream-parser/**`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-024`、`DEV-PLAN-025`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-437`

### 0.1 Simple > Easy 三问

1. **边界**：本计划首期只处理 CubeBox 的服务端 AI 网关、provider adapter、active model、SSE 转发、健康检查、配置验证与错误映射；UI 壳、会话持久化、上下文压缩分别由 `431`、`432`、`434` 承接。
2. **不变量**：Bifrost 是首选主参考，能复用或重构其现成能力的地方不重新发明一套平行网关；但 CubeBox 仍必须服从本仓 PostgreSQL、RLS、Authz、路由、错误码与审计要求。
3. **可解释**：reviewer 必须能在 5 分钟内说明哪些 Bifrost 能力被直接复用或重构、哪些因本仓多租户与配置治理约束需要本仓承接、Codex 在网关层仅复用哪些局部能力，以及为什么 fallback/quota/route alias/default model 不属于首期闭环。

## 1. 背景

`DEV-PLAN-430` 已把 Slice 2 定义为 CubeBox 的 AI 网关最小闭环。用户进一步要求：该切片不应按“从零自研网关”推进，而应改成“以 Bifrost 为主参考，尽量复用或重构 Bifrost 的代码或功能，避免重复造车”。

此前横向分析的结论已经明确：

- Codex 适合复用 provider adapter、Responses/OpenAI-compatible 请求映射、SSE passthrough、stream parser 和流式测试样式。
- Codex 不适合作为完整多租户 AI 网关主线基座，因为它不覆盖本仓所需的 provider 配置面、密钥治理、健康检查与治理面。
- Bifrost 是最接近 CubeBox 目标态的一方运行时参考：Go 实现、低开销路由、多 provider 选择、SSE 直通、健康与观测基础设施；其中故障切换能力只进入后续预留评估。
- One API、LiteLLM、Portkey 更适合作为语义与产品能力参考，而不是 CubeBox 的 Go 主网关运行时基座。

因此本计划冻结以下原则：**Bifrost 主参考，Codex 局部复用，One API/LiteLLM/Portkey 作为补充语义参考。**

## 2. 目标

1. 固定 Bifrost 参考 commit SHA，并完成许可证、依赖和代码可搬运性评估。
2. 冻结 CubeBox AI 网关的最小职责边界：
   - provider adapter 抽象
   - 服务端模型配置读取
   - API Key 解密
   - 请求映射
   - SSE 流式转发
   - 错误归一化
   - 健康检查
   - 配置验证
   - active model 选择
3. 以 Bifrost 为主参考，尽量复用或重构以下能力：
   - provider registry
   - 请求路由与 provider 选择
   - SSE 直通
   - 流式错误传播
   - 健康检查 / readiness
   - 观测埋点骨架
4. 以 Codex 为辅，复用或重构以下局部能力：
   - provider capability 元信息
   - Responses/OpenAI-compatible 请求桥接
   - stream parser
   - 流式响应测试样式
5. 将上述能力落到本仓一方 Go 模块，不引入外部网关进程作为运行时依赖。
6. 输出 `430` Slice 2 的明确复用路线，避免“概念上借鉴、实现上重写”的伪复用。
7. fallback/failover、quota、route alias、default model 只做上游评估和后续预留，不作为首期实现与验收。

## 3. 非目标

1. 不直接 vendoring 整个 Bifrost 仓库。
2. 不把外部网关的数据库模型、管理后台、SaaS 计费语义直接搬入本仓。
3. 不使用 LiteLLM 作为 Python 运行时侧车。
4. 不把 Portkey、Helicone 或其他托管网关作为首期生产依赖。
5. 不绕过本仓 PostgreSQL、RLS、Authz、错误码和路由门禁。
6. 不把网关做成 capability governance、PDP 或 legacy 对话栈回流点。
7. 不在未获得用户手工确认前新增数据库表。

## 4. 参考源分工

### 4.1 主参考：Bifrost

优先从 Bifrost 评估能否直接移植小段实现、重构内部模块边界或照搬测试样式：

- Go runtime 与高并发请求路径
- provider route / selection；fallback 仅作为后续预留评估对象
- 低开销 streaming proxy
- SSE passthrough
- health/readiness
- request/response telemetry 钩子

### 4.2 局部复用：Codex

Codex 在网关层只承担局部能力来源，不承担整体网关骨架：

- `model-provider`：provider adapter 接口与 capability 表达
- `codex-api` / `responses-api-proxy`：Responses/OpenAI-compatible 桥接思路
- `utils/stream-parser`：流式事件解析
- app-server / provider tests：流式响应与错误传播测试样式

### 4.3 补充参考：One API / LiteLLM / Portkey

- One API：模型别名、渠道配置、多供应商 OpenAI-compatible 入口语义
- LiteLLM：provider 覆盖矩阵、错误归一化口径、兼容性测试样式
- Portkey：配置层、路由策略层、观测面和虚拟 key 产品语义

这些项目默认只借鉴语义、配置概念和测试矩阵，不作为 CubeBox 的主运行时代码来源。

## 5. 复用优先级矩阵

| 能力 | 主参考 | CubeBox 策略 |
| --- | --- | --- |
| 网关主请求链 | Bifrost | 优先重构其低开销路由与 streaming 结构 |
| provider registry | Bifrost + Codex | 先看 Bifrost 骨架，再借 Codex capability 元信息补足 |
| provider adapter 接口 | Codex + Bifrost | 组合重构，避免本仓第三套命名 |
| active model 选择 | Bifrost + One API | 首期只采纳最小模型选择语义，不做 route alias/default model |
| fallback / failover | Bifrost | 非首期；只做文件级评估和后续预留，不进入 required gate |
| SSE passthrough | Bifrost + Codex | 以 Bifrost 主链为准，Codex stream parser 补局部 |
| Responses/OpenAI-compatible 映射 | Codex + One API | 采纳桥接思路，不自创协议 |
| 错误归一化 | LiteLLM + Bifrost | 借鉴口径，落为本仓错误码映射 |
| 健康检查 / readiness | Bifrost | 优先复用或重构 |
| 配置验证 | Bifrost + 本仓约束 | 路由/模型层参考 Bifrost，密钥/权限层本仓自研 |
| 虚拟 key / 多租户密钥治理 | 本仓自研 | 不直接复用外部项目实现 |
| DB 持久化 / RLS / 审计 | 本仓自研 | 外部项目只作字段参考，不作事实源 |

## 5A. 上游映射表模板

本计划必须把 Bifrost/Codex 的复用对象冻结成文件级或协议级映射；未填完前不得进入 Slice 2.1 之后的实现。

| 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | CubeBox 对应对象/切片 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 | PR 证据位置 | readiness 证据位置 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `maximhq/bifrost` | `待补` | `目录` | `router/streaming/health/telemetry 相关目录` | `gateway 主请求链 / Slice 2.2-2.5` | `待补` | `待补` | `待补` | `流式集成测试 + lifecycle fixture` | `待补` | `待补` |
| `maximhq/bifrost` | `待补` | `文件` | `provider selection / health readiness / telemetry 代表文件` | `active model 与健康检查 / Slice 2.4-2.5` | `待补` | `待补` | `待补` | `health fixture + start/final 状态测试` | `待补` | `待补` |
| `openai/codex` | `待补` | `目录` | `codex-rs/model-provider/**` | `provider adapter 接口 / Slice 2.1` | `待补` | `待补` | `待补` | `adapter contract test` | `待补` | `待补` |
| `openai/codex` | `待补` | `目录` | `codex-rs/responses-api-proxy/**`、`codex-rs/codex-api/**` | `request mapping / Slice 2.2` | `待补` | `待补` | `待补` | `request mapping golden fixture` | `待补` | `待补` |
| `openai/codex` | `待补` | `目录` | `codex-rs/utils/stream-parser/**` | `SSE parser / Slice 2.2` | `待补` | `待补` | `待补` | `SSE fixture + snapshot` | `待补` | `待补` |
| `songquanpeng/one-api`、`BerriAI/litellm`、`Portkey-AI/gateway` | `待补` | `协议` | `错误归一化/模型别名/渠道语义代表对象` | `错误码口径与配置命名 / Slice 2.3` | `待补` | `待补` | `待补` | `error mapping fixture` | `待补` | `待补` |

填写规则：

- `采用状态` 只允许填写 `直接复用`、`重构复用`、`只借鉴语义`、`明确不引入`。
- 对 `只借鉴语义` 或 `明确不引入` 的对象，必须说明为什么不能更小地复用现成上游实现，例如 `多租户 RLS/Authz`、`密钥治理`、`DDD 分层`、`错误码/i18n 契约`。
- `必备验证` 至少覆盖一个上游形状：请求体、SSE 事件流、provider fail-closed、健康检查输出、错误映射结果；fallback 只作为非首期预留证据。

## 5B. PR-437A 首轮最小冻结

首轮固定参考 commit SHA：

- `maximhq/bifrost`: `de67db28676a8a80ba1e738ebf8f9318d82d16f7`
- `openai/codex`: `ef071cf816950dc416b2a975e7ed023eea639026`

`PR-437A` 只冻结支撑 `PR-437B` 首条竖切所需的最小对象：

| 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | CubeBox 对应对象/切片 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `maximhq/bifrost` | `de67db28676a8a80ba1e738ebf8f9318d82d16f7` | `目录` | `router/streaming/health/telemetry 相关目录` | `gateway 主请求链最小骨架 / Phase B` | `重构复用` | 需服从本仓 route allowlist、RLS/Authz、错误码与审计链 | `仓库约束` | `流式集成测试 + lifecycle fixture` |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `目录` | `codex-rs/responses-api-proxy/**`、`codex-rs/codex-api/**` | `request mapping / deterministic provider bridge` | `重构复用` | 本仓不直接引入 app-server/runtime，需要收敛到一方 Go gateway | `DDD 边界` | `request mapping golden fixture` |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `目录` | `codex-rs/utils/stream-parser/**` | `SSE parser / delta streaming 行为` | `重构复用` | 需与本仓 SSE envelope 和错误映射统一 | `协议不匹配` | `SSE fixture + snapshot` |

`PR-437A` 同时冻结 deterministic provider 口径：required gate 使用 mock SSE / fake provider / fixed transcript fixture，不把真实外部 provider 连通性作为 merge 前置条件。

## 5D. 2026-04-22 首轮真实 provider 竖切冻结

本节记录首条真实 provider 竖切的最初冻结口径。当前实际进度已完成 `Slice 2.1-2.4`，待开工项为 `Slice 2.5`；本节不再代表“当前仅停留在 `2.1-2.3`”。

首轮目标是把 CubeBox 从 `deterministic runtime + settings` 最小闭环推进到“真实 provider 的真实对话主链”，不以 `435` 正式管理面封板为前置。

冻结口径：

- 只支持一个 `OpenAI-compatible provider`。
- CubeBox 内部主链先锁 `Chat Completions`，本轮不先做 `Responses`。
- credential 读取先走现有 `secret_ref` 路径；首轮只要求支持 `env://<ENV_NAME>` 这一类服务端环境变量引用。
- `secret_ref` 解析只存在于服务端请求作用域内，不写前端状态、不写 SSE payload、不写审计/事件明文。
- `turns:stream` 默认先走真实 provider gateway；`deterministic runtime` 只保留为测试夹具与本地受控 fixture，不再作为默认主链。
- `verify` / `health` 本轮不切成真实连通性 owner；显式验证、readiness 与最小 lifecycle telemetry/start-final 留给 `Slice 2.4-2.5`，`outbox` 不在本计划实施范围内。
- required gate 继续使用 fake provider / mock SSE / fixed transcript fixture；真实外部 provider 连通性不作为 merge 阻断。

`2026-04-22` 追加冻结：

- `Slice 2.5` 首轮先只做最小 lifecycle telemetry 与 canonical event 稳定化，不把 `usage_event` 作为本轮实施前置。
- `usage_event` 数据面、token usage 落库、细粒度计量/审计追溯继续后移到后续切片，与 `432` 的 conversation/message/summary 正式数据面一起规划。
- 项目当前尚未建设 `outbox` 能力；`outbox` 从本计划暂停实施，不作为 `433` 当前 owner，也不作为当前 merge gate。
- 涉及 `outbox` 的最终一致、异步重试与事务后补写语义，后移到独立后续计划，不借 `433 Slice 2.5` 首轮落地。

本轮 stopline：

- 不新增数据库表，不把 `cubebox_model_credentials` 改为密文存储模型。
- 不引入 fallback / failover / route alias / quota / default model。
- 不让 `435` 管理面反向定义 `provider / credential / active model / health` 的运行时命名。

## 5C. Phase E 共享对象口径

为避免 `433` 运行时与 `435` 管理面在 `PR-437E` 重新分叉，`Phase E` 统一消费以下共享对象名：

- `provider`：运行时可被选择、验证、启停的上游供应商配置对象；不得再引入 `vendor` / `channel` 作为主名。
- `credential`：附着在 provider 上、仅服务端可见明文的密钥或认证材料；UI 只消费掩码与版本/状态元信息。
- `active model`：当前对话默认选择的模型对象，最小形状为 `provider + model slug + capability summary`；route alias / fallback / default model 暂不并入首期。
- `health`：针对 provider 或 active model 组合的验证 / readiness 快照，最小形状包括 `status`、`validated_at`、`latency_ms`、`error_summary`。

冻结规则：

- `435` 只能消费这些对象名和最小形状，不能先做页面再反推运行时命名。
- `433` 后续新增配置读取、验证、健康输出时，字段命名应优先贴齐 `435/5A` 中冻结的 Bifrost / Codex 参考对象。
- 任何需要后移的 `route alias`、`fallback`、`quota`、`default model` 都必须继续留在非首期，不得借管理面需求提前回流。

## 6. CubeBox 网关目标架构

### 6.1 分层

- `presentation/handler`：统一对内 HTTP API、SSE 输出、错误码映射。
- `services/gateway`：请求验证、active model 选择、provider 调度、流式转发。
- `services/providers`：provider adapter 与 capability registry。
- `services/health`：模型健康检查、配置验证、连通性探测。
- `infrastructure/crypto`：API Key 解密与密钥版本管理。
- `infrastructure/persistence`：模型配置、凭据元数据、健康结果；`usage event` 数据面后移。

### 6.2 请求路径

1. 读取租户、用户、会话和模型选择信息。
2. 校验访问权限与模型可用性。
3. 解密 provider API Key。
4. 根据 active model 配置选择 provider。
5. 通过 adapter 构造 OpenAI-compatible 或 Responses 请求。
6. 长期目标为同事务写入 `request-start`、`usage-intent` 与 `audit-start`；首轮 `Slice 2.5` 先冻结 canonical event 内的 lifecycle 字段与 final 语义。
7. 发起上游流式调用。
8. 对 SSE 进行直通转发；必要时用 Codex stream parser 做局部解析。
9. 流式完成后更新 final 状态；异步补写与 `usage_event` 作为后续增强项，`outbox` 不在本计划实施范围内。

### 6.3 本仓必须主导并优先复用上游形状的部分

下列能力即使前面存在可借鉴实现，也必须由本仓持有最终主导权；主导权不等于扩大自研，需优先复用上游 telemetry、stream lifecycle、health/readiness 与测试形状：

- 多租户模型配置读取与权限裁剪
- API Key 加密存储、轮换与审计
- PostgreSQL/RLS/Authz 对齐
- 本仓错误码和 i18n 映射
- route allowlist / responder / tracing 契约
- request-start / usage-intent / audit-start / final 的长期状态推进契约，以及首轮最小 lifecycle telemetry/readiness 证据；`outbox` 后移到独立后续计划
- provider 不可用时的 fail-closed 行为

## 7. Slice 2 细化实施

### Slice 2.0：Bifrost 资产评估

- [ ] 固定 Bifrost 参考 commit SHA。
- [ ] 确认 Apache-2.0 许可证、NOTICE 和复制要求。
- [ ] 盘点 router、provider、streaming、health 相关依赖闭包。
- [ ] 输出“可直接复用 / 可重构 / 仅借鉴语义 / 不引入”清单。
- [ ] 按本计划 `5A` 模板补齐文件级/协议级上游映射表，并为每个对象冻结采用状态与不可复用原因。
- [ ] `PR-437A` 先以 `5B` 的最小冻结集满足 deterministic provider、request mapping 与 SSE passthrough 的开工条件。

### Slice 2.1：provider adapter 最小闭环

- [x] 先按 Bifrost/Codex 对齐定义 provider adapter 接口。
- [x] 首期实现一个 OpenAI-compatible provider。
- [ ] 冻结 capability 表达：streaming、responses、health-check、remote-compaction-support 等。
- [x] 补 adapter 单测。

### Slice 2.2：请求映射与流式转发

- [x] 以 Bifrost 为主参考重构请求路由与 SSE passthrough。
- [x] 以 Codex 为辅重构 `Chat Completions` OpenAI-compatible bridge 与 stream parser；`Responses` 暂缓。
- [x] 补 handler、service、adapter 单元测试和流式响应测试。
- [ ] 验证首字节响应时间与中断传播行为。

### Slice 2.3：配置读取、解密与错误映射

- [x] 实现服务端模型配置读取。
- [x] 实现 API Key 解密。
- [x] 实现错误归一化和本仓错误码映射。
- [x] provider 原始错误不直接透给前端。

### Slice 2.4：健康检查与配置验证

- [x] 以 Bifrost 为主参考实现 provider 健康检查。
- [x] 实现 active model 与 provider/base URL/model 组合验证。
- [x] 实现 provider/base URL/model 组合的显式验证；启动时检查继续后移。
- [x] 补健康检查和配置验证测试。

### Slice 2.5：观测、start/final 与后续 fallback 预留

`Slice 2.5` 在暂缓 `usage_event / outbox` 后，收敛为三个可执行子切片：

#### Slice 2.5A：最小 lifecycle telemetry

- [ ] 以 Bifrost/Codex 为主参考冻结首轮最小 telemetry、stream lifecycle 和测试样式。
- [ ] 把 `turn.started` / `turn.error` / `turn.interrupted` / `turn.completed` 的 lifecycle 字段稳定到 canonical event payload，不以 `usage_event` 落库为前置。
- [ ] 为每个 turn 生成稳定 `trace_id`，并在同一轮所有 terminal path 中透传。
- [ ] 在成功、失败、中断三条 terminal path 中统一写出 `provider_id`、`provider_type`、`model_slug`、`runtime`、`latency_ms`、`status`。
- [ ] 首轮只要求这些字段存在于 canonical event payload 与受控日志中，不要求额外持久化数据面。

#### Slice 2.5B：start/final 长期目标占位

- [ ] `request-start` / `usage-intent` / `audit-start` 的正式持久化继续保留为长期目标，但不作为本轮 required gate。
- [ ] 流式完成后的 final 语义必须在请求内稳定收口；当前只要求 `turn.completed` 与 `turn.error` 的 terminal event 一致，不依赖事务后补写。
- [ ] 本轮不新增数据库表，不为 start/final 语义单独引入持久化对象。

#### Slice 2.5C：非首期预留与后移项

- [ ] fallback/failover 只做 Bifrost 文件级映射和后续预留，不进入首期代码实现。
- [ ] 首轮记录 provider、错误原因、延迟、trace_id 与 final status；token usage 与 `usage_event` 落库暂缓。
- [ ] 观测字段保持与未来 `usage_event` 兼容的 shape，不做外部 SaaS 绑定。
- [ ] 补最小 start/final lifecycle、异常流式和 provider fail-closed 测试；`outbox` 不属于本计划测试范围。

#### Slice 2.5A 字段冻结

首轮必须冻结以下 canonical event payload 字段：

- `turn.started`
  - `user_message_id`
  - `trace_id`
  - `provider_id`
  - `provider_type`
  - `model_slug`
  - `runtime`
- `turn.error`
  - `code`
  - `message`
  - `retryable`
  - `trace_id`
  - `provider_id`
  - `provider_type`
  - `model_slug`
  - `runtime`
  - `latency_ms`
- `turn.interrupted`
  - `reason`
  - `trace_id`
  - `provider_id`
  - `provider_type`
  - `model_slug`
  - `runtime`
  - `latency_ms`
- `turn.completed`
  - `status`
  - `trace_id`
  - `provider_id`
  - `provider_type`
  - `model_slug`
  - `runtime`
  - `latency_ms`

其中：

- `runtime` 首轮只允许 `openai-chat-completions` 或 `deterministic-fixture`。
- `latency_ms` 以服务端请求起点到 terminal event 写出前的壁钟时间计算。
- `trace_id` 必须在单轮内稳定，且前端无须理解其生成方式，只消费字段存在性和可关联性。

#### Slice 2.5 暂缓影响评估

`usage_event` 暂缓的直接影响：

- 本轮不具备正式的 token 用量计量、会话级 usage 审计追溯和后续配额/计费数据面。
- provider/model/latency/error 的最小观测仍可通过 canonical event lifecycle 字段与服务端受控日志承接，不阻塞真实对话主链。

`outbox` 从本计划暂停实施的前提：

- `final` 状态、错误状态与最小 health 语义必须能在同一请求处理路径内稳定收口。
- 当前不要求跨进程、跨重试窗口去保证 `final` 补写、usage 补写或异步审计对象的最终一致。
- CI/readiness 暂不把“事务内登记 + 异步重试”作为 merge 前置，且项目当前没有现成 `outbox` 能力可复用。

没有 `outbox` 的影响：

- 若上游流式已完成、但请求尾部在写 `final` 或补写观测字段时失败，将缺少“事务内登记后重试”的修复通道，只能依赖请求内错误处理或后续人工/脚本修复。
- 无法保证关键补写在进程重启、连接中断或短暂数据库故障后的最终一致性。
- 后续若引入正式 `request-start/final` 落库、usage 补写、审计通知或计费流水，没有 `outbox` 将放大双写不一致风险。

因此，本计划冻结为：`outbox` 从 `DEV-PLAN-433` 暂停实施；`433 Slice 2.5` 只负责在单请求路径内收口最小 lifecycle telemetry 与 final 语义，不承接事务后重试、最终一致补写或异步 outbox 数据面。

### Slice 2.6：430 回填与封板

- [ ] 更新 `DEV-PLAN-430` Slice 2 回链本计划。
- [ ] readiness 记录 Bifrost/Codex 参考 commit、采纳矩阵、裁剪矩阵、请求映射 golden、SSE fixture、health fixture、最小 lifecycle 测试结果，以及 `usage_event` 暂缓与 `outbox` 暂停实施证据。
- [ ] 执行文档、Go、routing、authz、preflight 和反回流门禁验证。

## 8. 验收标准

- [ ] 已固定 Bifrost 参考 commit 与许可证评估结果。
- [ ] `430` Slice 2 已明确以 Bifrost 为主参考，而非从零自研。
- [ ] provider adapter 接口已对齐 Bifrost/Codex 成熟模式。
- [x] OpenAI-compatible provider 最小闭环可工作。
- [x] 服务端模型配置读取、密钥解密、请求映射、SSE 转发、错误映射已闭环。
- [x] 健康检查与配置验证已闭环。
- [ ] 首轮最小 lifecycle telemetry 与 final 语义已可测、可观测；`usage_event` 暂缓与 `outbox` 暂停实施证据已回填。
- [ ] 流式响应测试覆盖成功、失败、中断和 provider fail-closed。
- [ ] fallback/failover、quota、route alias、default model 已明确列为非首期并暂缓。
- [ ] PR 与 readiness 中都能把 handler/service/adapter 改动映射回 `5A` 的具体上游制品。
- [ ] `make check chat-surface-clean` 仍通过。

## 9. Stopline

- 不得在未评估 Bifrost 对应能力前直接自研第三套网关骨架。
- 不得把 Bifrost 仅当“设计灵感”，实现上却完全重写同类模块。
- 不得在 `5A` 映射表缺失 `commit SHA`、文件级对象或采用状态时开始 provider adapter、request mapping、telemetry lifecycle 或 SSE 实现。
- 不得把 fallback/failover、quota、route alias、default model 加回首期 required gate。
- 不得只说“结合本仓情况适配 Bifrost/Codex”而没有请求映射、SSE 路径或测试样例的对应关系。
- 不得把 Codex 当作完整多租户网关主基座。
- 不得直接采纳外部项目的数据库模型作为本仓事实源。
- 不得引入 Python 网关进程作为首期默认运行时。
- 不得绕过本仓 RLS/Authz/错误码/路由门禁。
- 不得在未获用户手工确认前新增数据库表。

## 10. 本地必跑与门禁

- 文档变更：`make check doc && make markdownlint`
- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- Routing/Authz/API 变更：`make check routing && make authz-pack && make authz-test && make authz-lint`
- DB/sqlc 变更：按模块执行 schema/migration/sqlc 闭环，新增表前必须获得用户手工确认
- 旧栈反回流：`make check chat-surface-clean`
- PR 前：`make preflight`

## 11. 参考链接

- Bifrost：`https://github.com/maximhq/bifrost`
- OpenAI Codex：`https://github.com/openai/codex`
- LiteLLM：`https://github.com/BerriAI/litellm`
- One API：`https://github.com/songquanpeng/one-api`
- Portkey Gateway：`https://github.com/Portkey-AI/gateway`
- DEV-PLAN-430：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
