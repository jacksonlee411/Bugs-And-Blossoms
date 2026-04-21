# DEV-PLAN-435：Bifrost 主参考的模型配置 UI 与管理权限复用/重构方案

**状态**: 规划中（2026-04-19 22:12 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：承接 `DEV-PLAN-430` 的 Slice 5“模型配置 UI 与管理权限”，统一沿用 `DEV-PLAN-433` 的 Bifrost 主参考路线；首期管理面只做 provider、credential、active model 与 health validation，尽量复用或重构 Bifrost 的配置、健康状态、provider 能力与验证交互；`One API` 仅作为渠道、模型映射、令牌/渠道信息架构的补充参考；本仓继续主导多租户权限、密钥生命周期、审计、错误码、i18n 与 E2E，但必须优先复用开源对象命名、页面 IA 和验证交互，避免扩大自研。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`、`apps/web`、`internal/server`、`modules/cubebox`（候选新模块路径）、`config/access`、`config/routing`、`config/errors`、`migrations`、`sqlc`
- **外部来源**：
  - `https://github.com/maximhq/bifrost`
  - `https://github.com/songquanpeng/one-api`
  - `https://github.com/openai/codex`
- **核心参考方向**：
  - Bifrost：Web UI、provider config、health/readiness、capability、observability；route/fallback 仅做后续预留评估，不进入首期验收
  - One API：channels、tokens、model mapping、group/额度、启停与验证交互
  - Codex：仅 provider capability / model metadata 展示语义，不承担管理面主线
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-024`、`DEV-PLAN-025`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-433`

### 0.1 Simple > Easy 三问

1. **边界**：本计划首期只处理模型配置 UI、管理权限、密钥生命周期、active model、健康验证与管理面交互；不重写网关运行时内核，运行时主线由 `DEV-PLAN-433` 承接。
2. **不变量**：管理面也统一由 Bifrost 主参考，不再为 Slice 5 切换另一套主参考；`One API` 只补充渠道与模型映射的信息架构语义；多租户权限、密钥治理、审计和错误码必须由本仓主导，但不得以此为理由跳过开源 IA、对象命名和验证交互复用。
3. **可解释**：reviewer 必须能在 5 分钟内说明为什么 Slice 2 和 Slice 5 统一由 Bifrost 主参考、为什么 One API 只是补充样板、首期为什么不做 route/fallback/quota/default model，以及权限矩阵如何落到 subject/domain/object/action。

## 1. 背景

`DEV-PLAN-430` 已把 Slice 5 定义为模型配置 UI 与管理权限。用户进一步确认：与其为管理面另起一条 `One API` 主参考路线，不如统一由 `Bifrost` 主参考，从而减少概念分叉、命名漂移和重复映射。

此前已经确定：

- Slice 2 的 AI 网关运行时由 `DEV-PLAN-433` 冻结为 `Bifrost` 主参考。
- `One API` 虽然在 channels、tokens、模型映射与管理面信息架构上很有参考价值，但如果把它升级为 Slice 5 的主参考，会造成 Slice 2/5 的双主参考分裂。
- `Codex` 不适合作为管理面的主参考，只适合作为 provider capability / metadata 层面的局部参考。
- 用户已冻结首期范围：one OpenAI-compatible provider + active model + health validation；fallback/quota/route/default model 暂缓。

因此本计划冻结以下统一口径：

- 主参考：`Bifrost`
- 补充参考：`One API`
- 局部参考：`Codex`
- 本仓主导：Authz、RLS、密钥加密、审计、错误码、i18n、E2E；主导权不等于从零自研，必须优先复用上游页面 IA、对象命名和验证交互。

## 2. 目标

1. 固定 Bifrost 参考 commit SHA，并完成管理面相关许可证、依赖和可搬运性评估。
2. 冻结 CubeBox 模型配置管理面的核心对象语义：
   - provider
   - credential
   - health status
   - active model
   - route/alias/fallback/timeout/quota/default model 的后续预留边界
3. 以 Bifrost 为主参考，尽量复用或重构以下能力：
   - provider 配置模型
   - health/readiness 状态展示
   - capability 显示
   - 配置验证动作
   - active model 与启停语义
4. 以 One API 为补充参考，补强以下管理面交互：
   - 渠道/模型映射表格
   - 令牌与渠道的信息架构
   - 启停状态的页面组织
   - 连通性验证与状态展示
5. 明确本仓必须主导且优先复用开源形状的部分：
   - 多租户权限矩阵
   - API Key 加密存储、轮换和掩码展示
   - 审计日志
   - 路由与错误码/i18n
   - E2E 与 readiness
6. 输出 `430` Slice 5 的明确复用/重构路线，避免把管理面当成纯自研页面。

## 3. 非目标

1. 不直接 vendoring Bifrost 或 One API 的整个前端或后台。
2. 不直接采用外部项目的用户系统、角色系统、数据库模型或默认安全策略。
3. 不绕过本仓 Authz、RLS、routing、错误码、i18n、E2E 门禁。
4. 不在前端存储 API Key 明文。
5. 不让普通业务用户读取、验证或轮换未授权的模型密钥。
6. 不在未获用户手工确认前新增数据库表。

## 4. 参考源分工

### 4.1 主参考：Bifrost

`Bifrost` 负责提供统一的管理面主语义：

- provider config
- active model 与 provider selection；route/fallback 仅作为后续预留评估对象
- health / readiness
- capability / status
- 统一网关和管理面的命名口径

### 4.2 补充参考：One API

`One API` 只作为管理面信息架构与对象组织的补充来源：

- channel 列表
- token / channel 关系
- 模型映射表格
- 启停、分组、额度、验证交互

### 4.3 局部参考：Codex

`Codex` 在 Slice 5 中只保留以下局部参考角色：

- provider capability 命名
- model metadata 展示语义
- 不同模型能力差异的呈现方式

## 5. 采纳矩阵

| 能力 | 主参考 | CubeBox 策略 |
| --- | --- | --- |
| provider 配置语义 | Bifrost | 优先复用或重构 |
| active model 选择 | Bifrost + One API | 首期只做 active model，表格/信息架构借鉴 One API |
| model route / alias / fallback / quota / default model | Bifrost + One API | 非首期；只做后续预留评估，不进入 required gate |
| 健康状态 / readiness | Bifrost | 强制优先复用或重构 |
| 配置验证动作 | Bifrost + One API | 运行态语义对齐 Bifrost，交互组织借鉴 One API |
| 启用 / 停用 | Bifrost + One API | 主语义用 Bifrost，补充页面组织 |
| API Key 轮换 | 本仓主导 + Bifrost/One API 交互参考 | 不复用外部密钥存储，但复用录入/验证/掩码/轮换 IA |
| 权限矩阵 | 本仓主导 + 开源角色语义参考 | 不直接复用外部角色系统，必须落到 subject/domain/object/action |
| 审计与错误码 | 本仓主导 + Bifrost telemetry 参考 | 不外包给外部项目，但复用 telemetry/health 状态形状 |
| i18n / E2E | 本仓主导 | 必须纳入仓库门禁 |

## 5A. 上游映射表模板

本计划必须把 Bifrost/One API/Codex 在管理面上的复用对象冻结成可审计制品；未填完前不得进入 Slice 5.1 之后的实现。

| 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | CubeBox 对应对象/切片 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 | PR 证据位置 | readiness 证据位置 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `maximhq/bifrost` | `待补` | `页面信息架构` | `provider config / health 相关页面或目录` | `provider/active model/health 管理面 / Slice 5.2-5.5` | `待补` | `待补` | `待补` | `IA snapshot + E2E` | `待补` | `待补` |
| `maximhq/bifrost` | `待补` | `文件` | `health/readiness/validation 代表文件` | `健康验证动作与状态展示 / Slice 5.5` | `待补` | `待补` | `待补` | `validation fixture` | `待补` | `待补` |
| `songquanpeng/one-api` | `待补` | `页面信息架构` | `channels/tokens/model mapping 代表页面` | `表格组织与信息架构 / Slice 5.3` | `待补` | `待补` | `待补` | `IA snapshot` | `待补` | `待补` |
| `openai/codex` | `待补` | `协议` | `provider capability / model metadata 代表对象` | `capability 命名与元信息展示 / Slice 5.2-5.3` | `待补` | `待补` | `待补` | `metadata snapshot` | `待补` | `待补` |
| `本仓主导` | `N/A` | `协议` | `Authz/RLS/密钥治理/错误码/i18n 契约` | `权限矩阵、密钥生命周期、审计 / Slice 5.1-5.4` | `重构复用` | `外部角色/DB 不可直接采用，但 IA/命名/验证交互需优先复用` | `仓库约束` | `Authz test + E2E` | `待补` | `待补` |

填写规则：

- `采用状态` 只允许填写 `直接复用`、`重构复用`、`只借鉴语义`、`明确不引入`。
- 若 `One API` 或 `Codex` 只承担局部参考，必须明确到页面 IA、命名对象或 metadata 级别，不能写“后台风格类似”。
- `必备验证` 必须锁住命名、页面 IA、权限行为或验证动作；不能只靠人工截图说明“看起来差不多”。

## 6. CubeBox 管理面目标架构

### 6.1 用户可见页面

- `模型供应商列表页`：展示 provider、base URL、状态、健康状态、启停状态。
- `active model 配置面板`：展示当前启用模型、所属 provider、模型能力、最近验证结果。
- `密钥管理面板`：新增、验证、启用、停用、轮换 API Key；永远只显示掩码。
- `健康与验证面板`：展示最近一次验证结果、错误摘要、延迟、状态时间戳。
- `模型路由配置页`、fallback provider、timeout、quota、default model 不进入首期用户可见面；只允许在上游映射中作为后续能力预留。

### 6.2 权限分层

| 角色 | subject | domain | object | action | 首期允许能力 |
| --- | --- | --- | --- | --- | --- |
| 平台 admin | `platform_admin` | `platform` | `cubebox.model_provider` / `cubebox.model_credential` / `cubebox.model_selection` | `create` / `read` / `update` / `rotate` / `verify` / `activate` / `deactivate` | 新增/编辑 provider，录入与轮换密钥，验证连通性，启停 provider，设置 active model |
| 平台 operator | `platform_operator` | `platform` | `cubebox.model_provider` / `cubebox.model_selection` | `read` / `verify` / `activate` / `deactivate` | 查看 provider 与健康状态，执行验证，启停 provider，切换 active model；不可读取或轮换密钥 |
| 租户 admin | `tenant_admin` | `tenant:{tenant_id}` | `cubebox.model_selection` | `read` / `select` | 在平台授权范围内选择可用 active model；不可管理 provider、base URL 或密钥 |
| 普通用户 | `user` | `tenant:{tenant_id}` | `cubebox.model_selection` / `cubebox.conversation` | `read` / `use` | 查看可用模型展示名和健康状态，使用已授权模型发起对话；不可验证、启停、轮换或读取密钥 |

具体权限对象和动作仍由本仓 Authz 冻结，不采用外部项目原生角色模型；但页面 IA、provider/config 命名与验证交互应优先复用 Bifrost/One API 的可审计对象。

### 6.3 数据原则

- 密钥明文只在服务端输入与解密瞬间存在。
- 配置页返回值不包含明文密钥。
- 轮换必须生成新密钥版本，并让旧版本失效或退出活跃使用。
- 健康验证与配置验证要有明确审计记录。
- 配置变更失败时必须 fail-closed，不保留半生效状态。

## 7. 实施切片

### Slice 5.0：Bifrost 管理面资产评估

- [ ] 固定 Bifrost 参考 commit SHA。
- [ ] 确认 Apache-2.0 许可证、NOTICE 和复制要求。
- [ ] 盘点与 Web UI、provider config、health/readiness、active model 相关的依赖闭包；route/fallback/quota/default model 只做后续预留评估。
- [ ] 输出“可直接复用 / 可重构 / 仅借鉴语义 / 不引入”清单。
- [ ] 按本计划 `5A` 模板补齐页面 IA/文件级上游映射表，并为每个对象冻结采用状态与不可复用原因。

### Slice 5.1：配置对象与权限矩阵冻结

- [ ] 冻结 provider、credential、active model、health 对象职责；route、alias、fallback、quota、default model 列为非首期暂缓。
- [ ] 冻结平台 admin、平台 operator、租户 admin、普通用户的 subject/domain/object/action 权限矩阵。
- [ ] 对齐 `DEV-PLAN-433` 的 provider capability / active model 配置命名。
- [ ] 明确哪些行为必须走二次验证或显式确认。

### Slice 5.2：模型供应商配置页

- [ ] 以 Bifrost 为主参考实现 provider 列表与详情页。
- [ ] 展示 provider 状态、能力、最近健康检查结果。
- [ ] 支持启用、停用和验证动作。
- [ ] 补路由、错误提示、i18n 和前端测试。

### Slice 5.3：active model 配置

- [ ] 以 Bifrost 为主参考实现 active model 配置面板。
- [ ] 以 One API 为补充参考优化表格信息架构和筛选方式。
- [ ] 支持在平台授权范围内选择当前 active model。
- [ ] route alias、fallback provider、timeout、quota、default model 不进入首期页面与 API。
- [ ] 补 Authz、错误映射和 E2E。

### Slice 5.4：密钥生命周期管理

- [ ] 实现新增、验证、启用、停用、轮换 API Key。
- [ ] 明文只用于输入和即时验证，之后仅存密文与掩码。
- [ ] 轮换要记录版本与审计事件。
- [ ] 补错误路径、权限路径和并发轮换测试。

### Slice 5.5：健康验证与故障信息展示

- [ ] 以 Bifrost 的 health/readiness 为主参考实现验证结果 UI。
- [ ] 展示最近验证时间、状态、错误摘要、延迟。
- [ ] 与 `433` 的 active model 与健康检查语义对齐。
- [ ] 补流式不可用、provider 不可达、配置错误等场景测试。

### Slice 5.6：430 回填与封板

- [ ] 更新 `DEV-PLAN-430` Slice 5 回链本计划。
- [ ] readiness 记录 Bifrost/One API/Codex 参考 commit、采纳矩阵、裁剪矩阵、IA snapshot、Authz/E2E 测试结果，以及 fallback/quota/route/default model 暂缓证据。
- [ ] 执行文档、前端、Go、routing、authz、preflight 和反回流门禁验证。

## 8. 验收标准

- [ ] 已固定 Bifrost 参考 commit 与许可证评估结果。
- [ ] `430` Slice 5 已明确由 Bifrost 主参考，而不是切换到另一套主参考。
- [ ] provider、credential、active model、health 对象语义已冻结。
- [ ] 支持新增、验证、启用、停用、轮换 API Key。
- [ ] 支持 active model 选择与基础 provider 配置展示。
- [ ] fallback/quota/route alias/default model 已明确列为非首期并暂缓。
- [ ] Authz、路由、错误提示、i18n 和 E2E 已纳入实施闭环。
- [ ] 平台 admin、平台 operator、租户 admin、普通用户权限矩阵已落为 subject/domain/object/action，并通过 Authz 测试覆盖。
- [ ] 密钥明文不回显，轮换可审计。
- [ ] PR 与 readiness 中都能把页面 IA、provider/config 对象与权限设计映射回 `5A` 的具体上游制品。
- [ ] `make check chat-surface-clean` 仍通过。

## 9. Stopline

- 不得为 Slice 5 再切换到第二套主参考。
- 不得直接复用外部项目的用户/角色/数据库模型。
- 不得以“本仓主导权限/审计”为理由跳过 Bifrost/One API 页面 IA、对象命名和验证交互复用评估。
- 不得在 `5A` 映射表缺失 `commit SHA`、页面 IA/文件级对象或采用状态时开始管理面页面、自定义对象命名或权限矩阵实现。
- 不得把“看起来像 Bifrost/One API”当作复用证明；若没有对象级映射与不可复用原因，即视为自研偏航。
- 不得把 fallback/quota/route alias/default model 加回首期 required gate。
- 不得在前端保存密钥明文。
- 不得绕过本仓 Authz、RLS、routing、错误码和 i18n 契约。
- 不得把配置管理页做成只读后端能力而无用户可见入口。
- 不得在未获用户手工确认前新增数据库表。

## 10. 本地必跑与门禁

- 文档变更：`make check doc && make markdownlint`
- 前端 UI：`pnpm --dir apps/web check`
- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- Routing/Authz/API 变更：`make check routing && make authz-pack && make authz-test && make authz-lint`
- DB/sqlc 变更：按模块执行 schema/migration/sqlc 闭环，新增表前必须获得用户手工确认
- 旧栈反回流：`make check chat-surface-clean`
- PR 前：`make preflight`

## 11. 参考链接

- Bifrost：`https://github.com/maximhq/bifrost`
- One API：`https://github.com/songquanpeng/one-api`
- OpenAI Codex：`https://github.com/openai/codex`
- DEV-PLAN-430：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- DEV-PLAN-433：`docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
