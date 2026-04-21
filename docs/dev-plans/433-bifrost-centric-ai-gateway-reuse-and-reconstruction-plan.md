# DEV-PLAN-433：Bifrost 主参考的 AI 网关复用/重构方案

**状态**: 规划中（2026-04-19 21:56 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：承接 `DEV-PLAN-430` 的 Slice 2“AI 网关最小闭环”，以 Bifrost 作为一方 Go 网关的主参考，要求尽量复用或重构其可用代码、路由思路、provider 选择、故障切换、SSE 直通与流式错误处理能力，最大化避免在 CubeBox 中重复造车；Codex 仅复用 provider adapter、Responses/OpenAI-compatible 映射和 stream parser 等局部能力。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`modules/cubebox`（候选新模块路径）、`internal/server`、`config`、`migrations`、`sqlc`、`scripts/ci`
- **外部来源**：
  - `https://github.com/maximhq/bifrost`
  - `https://github.com/openai/codex`
  - `https://github.com/BerriAI/litellm`
  - `https://github.com/songquanpeng/one-api`
  - `https://github.com/Portkey-AI/gateway`
- **核心参考文件/目录（优先级顺序）**：
  - Bifrost：`core router`、`provider selection`、`fallback`、`streaming`、`observability`、`health/readiness` 相关目录与测试
  - Codex：`codex-rs/model-provider/**`
  - Codex：`codex-rs/model-provider-info/**`
  - Codex：`codex-rs/codex-api/**`
  - Codex：`codex-rs/responses-api-proxy/**`
  - Codex：`codex-rs/utils/stream-parser/**`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-024`、`DEV-PLAN-025`、`DEV-PLAN-300`、`DEV-PLAN-430`

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理 CubeBox 的服务端 AI 网关、provider adapter、模型路由、SSE 转发、健康检查、配置验证与错误映射；UI 壳、会话持久化、上下文压缩分别由 `431`、`432`、`434` 承接。
2. **不变量**：Bifrost 是首选主参考，能复用或重构其现成能力的地方不重新发明一套平行网关；但 CubeBox 仍必须服从本仓 PostgreSQL、RLS、Authz、路由、错误码与审计要求。
3. **可解释**：reviewer 必须能在 5 分钟内说明哪些 Bifrost 能力被直接复用或重构、哪些因本仓多租户与配置治理约束需要自研承接、Codex 在网关层仅复用哪些局部能力，以及 fallback 边界在哪里。

## 1. 背景

`DEV-PLAN-430` 已把 Slice 2 定义为 CubeBox 的 AI 网关最小闭环。用户进一步要求：该切片不应按“从零自研网关”推进，而应改成“以 Bifrost 为主参考，尽量复用或重构 Bifrost 的代码或功能，避免重复造车”。

此前横向分析的结论已经明确：

- Codex 适合复用 provider adapter、Responses/OpenAI-compatible 请求映射、SSE passthrough、stream parser 和流式测试样式。
- Codex 不适合作为完整多租户 AI 网关主线基座，因为它不覆盖本仓所需的 provider 配置面、密钥治理、租户限额、健康检查与故障切换治理面。
- Bifrost 是最接近 CubeBox 目标态的一方运行时参考：Go 实现、低开销路由、多 provider 选择、故障切换、SSE 直通、健康与观测基础设施。
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
   - fallback 路由
3. 以 Bifrost 为主参考，尽量复用或重构以下能力：
   - provider registry
   - 请求路由与 provider 选择
   - fallback/故障切换
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
- provider route / selection / fallback
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
| 模型别名/route 配置 | Bifrost + One API | 采纳语义，落到本仓配置对象 |
| fallback / failover | Bifrost | 强制优先复用或重构 |
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
| `maximhq/bifrost` | `待补` | `目录` | `router/streaming/fallback/health 相关目录` | `gateway 主请求链 / Slice 2.2-2.5` | `待补` | `待补` | `待补` | `流式集成测试 + failover fixture` | `待补` | `待补` |
| `maximhq/bifrost` | `待补` | `文件` | `provider selection / fallback / health readiness 代表文件` | `provider route 与健康检查 / Slice 2.4-2.5` | `待补` | `待补` | `待补` | `route fixture + health test` | `待补` | `待补` |
| `openai/codex` | `待补` | `目录` | `codex-rs/model-provider/**` | `provider adapter 接口 / Slice 2.1` | `待补` | `待补` | `待补` | `adapter contract test` | `待补` | `待补` |
| `openai/codex` | `待补` | `目录` | `codex-rs/responses-api-proxy/**`、`codex-rs/codex-api/**` | `request mapping / Slice 2.2` | `待补` | `待补` | `待补` | `request mapping golden fixture` | `待补` | `待补` |
| `openai/codex` | `待补` | `目录` | `codex-rs/utils/stream-parser/**` | `SSE parser / Slice 2.2` | `待补` | `待补` | `待补` | `SSE fixture + snapshot` | `待补` | `待补` |
| `songquanpeng/one-api`、`BerriAI/litellm`、`Portkey-AI/gateway` | `待补` | `协议` | `错误归一化/模型别名/渠道语义代表对象` | `错误码口径与配置命名 / Slice 2.3` | `待补` | `待补` | `待补` | `error mapping fixture` | `待补` | `待补` |

填写规则：

- `采用状态` 只允许填写 `直接复用`、`重构复用`、`只借鉴语义`、`明确不引入`。
- 对 `只借鉴语义` 或 `明确不引入` 的对象，必须说明为什么不能更小地复用现成上游实现，例如 `多租户 RLS/Authz`、`密钥治理`、`DDD 分层`、`错误码/i18n 契约`。
- `必备验证` 至少覆盖一个上游形状：请求体、SSE 事件流、fallback 路径、健康检查输出、错误映射结果。

## 6. CubeBox 网关目标架构

### 6.1 分层

- `presentation/handler`：统一对内 HTTP API、SSE 输出、错误码映射。
- `services/gateway`：请求验证、模型选择、provider 调度、fallback、流式转发。
- `services/providers`：provider adapter 与 capability registry。
- `services/health`：模型健康检查、配置验证、连通性探测。
- `infrastructure/crypto`：API Key 解密与密钥版本管理。
- `infrastructure/persistence`：模型配置、凭据元数据、健康结果、用量事件。

### 6.2 请求路径

1. 读取租户、用户、会话和模型选择信息。
2. 校验访问权限与模型可用性。
3. 解密 provider API Key。
4. 根据 route 配置选择主 provider。
5. 通过 adapter 构造 OpenAI-compatible 或 Responses 请求。
6. 发起上游流式调用。
7. 对 SSE 进行直通转发；必要时用 Codex stream parser 做局部解析。
8. 主 provider 失败时按 route 规则进行 fallback。
9. 在响应完成后异步记录 usage、latency、错误与健康信号。

### 6.3 本仓必须自研或保留自研 fallback 的部分

下列能力即使前面存在可借鉴实现，也必须由本仓持有最终主导权，必要时保留自研 fallback：

- 多租户模型配置读取与权限裁剪
- API Key 加密存储、轮换与审计
- PostgreSQL/RLS/Authz 对齐
- 本仓错误码和 i18n 映射
- route allowlist / responder / tracing 契约
- 用量事件落库与 readiness 证据
- provider 不可用时的 fail-closed 行为

## 7. Slice 2 细化实施

### Slice 2.0：Bifrost 资产评估

- [ ] 固定 Bifrost 参考 commit SHA。
- [ ] 确认 Apache-2.0 许可证、NOTICE 和复制要求。
- [ ] 盘点 router、provider、streaming、health 相关依赖闭包。
- [ ] 输出“可直接复用 / 可重构 / 仅借鉴语义 / 不引入”清单。
- [ ] 按本计划 `5A` 模板补齐文件级/协议级上游映射表，并为每个对象冻结采用状态与不可复用原因。

### Slice 2.1：provider adapter 最小闭环

- [ ] 先按 Bifrost/Codex 对齐定义 provider adapter 接口。
- [ ] 首期实现一个 OpenAI-compatible provider。
- [ ] 冻结 capability 表达：streaming、responses、health-check、remote-compaction-support 等。
- [ ] 补 adapter 单测。

### Slice 2.2：请求映射与流式转发

- [ ] 以 Bifrost 为主参考重构请求路由与 SSE passthrough。
- [ ] 以 Codex 为辅重构 Responses/OpenAI-compatible bridge 与 stream parser。
- [ ] 补 handler、service、adapter 单元测试和流式响应测试。
- [ ] 验证首字节响应时间与中断传播行为。

### Slice 2.3：配置读取、解密与错误映射

- [ ] 实现服务端模型配置读取。
- [ ] 实现 API Key 解密。
- [ ] 实现错误归一化和本仓错误码映射。
- [ ] provider 原始错误不直接透给前端。

### Slice 2.4：健康检查与配置验证

- [ ] 以 Bifrost 为主参考实现 provider 健康检查。
- [ ] 实现模型 route 配置验证。
- [ ] 实现 provider/base URL/model 组合的启动时检查或显式验证。
- [ ] 补健康检查和配置验证测试。

### Slice 2.5：fallback 与观测

- [ ] 以 Bifrost 为主参考实现 provider fallback。
- [ ] 记录主 provider、fallback provider、错误原因与延迟。
- [ ] 观测字段与 usage event 对齐，不做外部 SaaS 绑定。
- [ ] 补 failover 测试和异常流式测试。

### Slice 2.6：430 回填与封板

- [ ] 更新 `DEV-PLAN-430` Slice 2 回链本计划。
- [ ] readiness 记录 Bifrost/Codex 参考 commit、采纳矩阵、裁剪矩阵、请求映射 golden、SSE fixture、fallback 测试结果。
- [ ] 执行文档、Go、routing、authz、preflight 和反回流门禁验证。

## 8. 验收标准

- [ ] 已固定 Bifrost 参考 commit 与许可证评估结果。
- [ ] `430` Slice 2 已明确以 Bifrost 为主参考，而非从零自研。
- [ ] provider adapter 接口已对齐 Bifrost/Codex 成熟模式。
- [ ] OpenAI-compatible provider 最小闭环可工作。
- [ ] 服务端模型配置读取、密钥解密、请求映射、SSE 转发、错误映射已闭环。
- [ ] 健康检查与配置验证已闭环。
- [ ] fallback 行为可测、可观测。
- [ ] 流式响应测试覆盖成功、失败、中断、fallback。
- [ ] PR 与 readiness 中都能把 handler/service/adapter 改动映射回 `5A` 的具体上游制品。
- [ ] `make check chat-surface-clean` 仍通过。

## 9. Stopline

- 不得在未评估 Bifrost 对应能力前直接自研第三套网关骨架。
- 不得把 Bifrost 仅当“设计灵感”，实现上却完全重写同类模块。
- 不得在 `5A` 映射表缺失 `commit SHA`、文件级对象或采用状态时开始 provider adapter、request mapping、fallback 或 SSE 实现。
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
