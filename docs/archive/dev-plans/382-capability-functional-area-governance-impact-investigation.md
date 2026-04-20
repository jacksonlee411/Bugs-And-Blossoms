# DEV-PLAN-382：Capability Functional Area 治理影响面专项调查

**状态**: 规划中（2026-04-17 07:26 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T1` 调查；若后续据此修改 capability / route-map / authz / 前端权限，则承接计划应按 `T2` 评审。
- **范围一句话**：调查 `DEV-PLAN-150/157` 引入 Functional Area 治理后，除 `CubeBox` 归属争议外，还对 contract、运行时、路由、前端、CI 与错误语义产生了哪些影响。
- **关联模块/目录**：`config/capability`、`internal/server`、`apps/web/src/api`、`apps/web/src/errors`、`scripts/ci`、`Makefile`
- **关联计划/标准**：
  - `AGENTS.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/archive/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
  - `docs/dev-plans/156-route-capability-map-convergence.md`
  - `docs/archive/dev-plans/157-capability-key-m7-functional-area-governance.md`
  - `docs/dev-plans/158-capability-m8-ui-delivery-plan.md`
  - `docs/dev-plans/381-cubebox-capability-and-functional-area-lineage-investigation.md`
- **用户入口/触点**：Capability Governance、Functional Area、Activation、Explain、`/internal/functional-areas/*`、`/internal/capabilities/catalog*`

### 0.1 Simple > Easy 三问

1. **边界**：`functional_area_key` 是 capability 上层治理域，不等于 DDD 模块、不等于路由 owner、不等于前端导航模块。
2. **不变量**：每个 `capability_key` 必须且仅能归属一个 `functional_area_key`；缺失、非 active 或租户关闭时必须 fail-closed。
3. **可解释**：任何后续新增能力都必须能回答“属于哪个业务能力、由哪个模块负责、受哪个 functional area 总开关控制、关闭时用户看到什么”。

### 0.2 现状研究摘要

- **设计来源**：
  - `DEV-PLAN-150` 在 `G9` 明确提出缺少 Functional Area 租户开关，目标是增加 `functional_area -> capability_key` 继承与 tenant opt-in/opt-out 总开关。
  - `DEV-PLAN-150` 在工作流 F 冻结首批 functional area，并要求每个 capability 必须且仅能归属一个 functional area。
  - `DEV-PLAN-157` 将该设计落地为词汇层、归属层、执行层，并选择“唯一归属”和“reserved 默认阻断”。
- **当前实现落点**：
  - `config/capability/contract-freeze.v1.json` 已把 functional area、reason code、explain 最小字段和门禁基线纳入 contract。
  - `config/capability/route-capability-map.v1.json` 要求 route 绑定的 capability 必须引用 active functional area。
  - `internal/server/functional_area_governance.go` 在运行时执行 lifecycle、capability status、租户开关三层 fail-closed。
  - `apps/web/src/api/setids.ts` 已暴露 Functional Area 与 Capability Catalog 前端 API DTO。
- **最容易出错的位置**：
  - 把 `org_foundation` 当作 org DDD 模块边界，而不是 capability governance 的 functional area。
  - 把 `owner_module`、`functional_area_key`、`capability_key` 三者混成一个字段。
  - 在新领域出现时复用旧 functional area，短期容易通过门禁，长期会固化错误归属。

## 1. 背景与问题

- **需求来源**：在 `DEV-PLAN-381` 明确 `CubeBox` 当前 capability / functional area 继承链路后，需要进一步调查 `150/157` 当初设计产生的其他影响。
- **当前痛点**：
  - `150/157` 的设计已经从文档进入运行时和 CI，不能只把它视为“命名问题”。
  - Functional Area 同时承担租户总开关、错误解释、catalog 分类和门禁校验，后续任何领域能力迁移都会受其约束。
  - 若不固化影响面，后续 `380*` 或新模块容易只改 route owner，却遗漏 catalog、前端 DTO、错误提示和门禁。
- **业务价值**：
  - 解释 `150/157` 为什么这样设计。
  - 明确它带来的正向治理收益和副作用。
  - 为后续是否新增 `cubebox` 独立 functional area / capability 提供输入。

## 2. 调查目标与非目标

### 2.1 核心目标

- [X] 梳理 `150/157` 对 contract 文件的影响。
- [X] 梳理 `150/157` 对运行时 fail-closed 的影响。
- [X] 梳理 `150/157` 对 route-capability-map / routing / authz 证据链的影响。
- [X] 梳理 `150/157` 对 capability catalog、前端 API、错误提示和 CI 门禁的影响。
- [X] 给出正向收益、副作用风险和后续承接建议。

### 2.2 非目标

- 不在本文直接新增 `cubebox` functional area。
- 不在本文直接重命名 `org.assistant_conversation.manage`。
- 不在本文直接修改 route-map、Casbin policy、前端权限或运行时代码。
- 不替代 `380G` 的全量回归验收，也不替代后续 capability 重构计划。

### 2.3 工具链与门禁

- **命中触发器（勾选）**：
  - [ ] Go 代码
  - [ ] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [ ] DB Schema / Migration / Backfill / Correction
  - [ ] sqlc
  - [ ] Routing / allowlist / responder / capability-route-map
  - [ ] AuthN / Tenancy / RLS
  - [ ] Authz（Casbin）
  - [ ] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`make check doc`

## 3. 为什么 `150/157` 要这样设计

### 3.1 原始问题不是 CubeBox，而是 capability 粒度过细后的治理成本

`DEV-PLAN-150` 的差距基线中，`G9` 指出当时缺少 Functional Area 租户开关，导致只能管理细粒度 capability，成本过高。

这意味着 `150` 试图解决的是：

- capability_key 已经替代旧 scope / package 语义，但缺少上层分组。
- 租户启停、灰度、隔离和解释不能逐个 capability 手工管理。
- 需要一个比 capability 更粗、比 DDD module 更面向产品治理的层次。

因此 `150` 选择增加 `functional_area -> capability_key` 继承关系，并把 tenant opt-in/opt-out 做成总开关。

### 3.2 `157` 将设计落成三层治理模型

`DEV-PLAN-157` 把 Functional Area 分成三层：

- 词汇层：注册 `functional_area_key/display_name/owner_module/lifecycle_status`。
- 归属层：每个 capability 与一个 functional area 建立唯一归属。
- 执行层：运行时根据 capability 查 functional area，再校验 lifecycle 与租户开关。

这套设计的核心不是“把能力挂到 org”，而是用一个稳定上层域承载租户级启停和 fail-closed。

### 3.3 唯一归属与 reserved 阻断是为了降低不确定性

`157` 选择：

- capability 只允许唯一归属，避免一个 capability 被多个功能域开关同时影响。
- `reserved` 默认运行时阻断，避免未正式启用的功能域被路由或 API 提前接入。
- 缺失 functional area 直接 fail-closed，避免能力无主运行。

这些选择符合 Simple > Easy：实现者少一些自由度，但 reviewer 和门禁能得到一致答案。

### 3.4 Functional Area 应如何显式划分

Functional Area 不应自动划分，也不应从 route path、前端导航、`owner_module`、旧 capability 或文件目录隐式推导。它必须是一项显式设计决策。

最小判断句：

> 如果租户关闭这个 functional area，那么它下面所有 capability 一起不可用，这件事在业务上是否成立？

如果答案不是明确成立，则不能放在同一个 functional area。

划分时必须同时满足：

- **业务能力域一致**：同一 functional area 下的 capability 必须属于同一组业务能力域，例如组织基础主数据、任职岗位、职位目录、人员主档、IAM 平台治理。
- **租户开关爆炸半径一致**：关闭该 functional area 时，被关闭的 capability 应是业务上可解释的一组能力，不能顺带关闭不相干能力。
- **生命周期一致**：同一 functional area 下的 capability 应能接受同一套 `active/reserved/deprecated` 节奏。
- **责任 owner 清楚**：`owner_module` 只能辅助定位责任模块，不能自动决定 functional area。
- **用户可见能力同类**：治理页面中展示为同一组能力时，租户管理员应能理解为什么它们受同一个总开关控制。
- **失败语义一致**：当返回 `FUNCTIONAL_AREA_DISABLED` 时，用户应能理解“为什么这一组能力一起不可用”。

### 3.5 禁止“字段合法但语义不自然”的隐式归属

当前门禁主要能阻断“缺失 / 不存在 / 非 active”的 functional area，但不能自动识别“字段合法、语义不自然”的归属。例如 `org_foundation` 是合法 active area，因此历史 `org.assistant_conversation.manage` 继续挂在其下时，门禁不会自动判断 `CubeBox` 会话、文件、模型治理是否真的属于组织基础主数据能力域。

因此后续新增或迁移 capability 必须显式冻结：

- `capability_key`
- `functional_area_key`
- `owner_module`
- `capability_type`
- `target_object/surface/intent`
- route/action 映射
- lifecycle / activation / tenant switch 行为
- 关闭该 functional area 时的用户影响说明
- 用户可见错误码与 explain 字段

以下不一致不能静默通过，必须升级评审并提供计划编号、例外理由和收口时间：

- `OwnerModule=cubebox`，但 `capability_key=org.*`
- `capability_key=org.*`，但 route 是 `/internal/cubebox/*`
- `functional_area_key=org_foundation`，但用户能力是 CubeBox 会话、文件、模型治理
- catalog 的 `target_object/surface/intent` 与 functional area 的业务能力域不一致
- 新能力找不到合适 functional area，却临时挂到一个过大的 active area

**调查结论**：
Functional Area 的正确用法是显式业务治理分组，不是默认桶。`org_foundation` 不能成为“暂时不知道放哪就先挂这里”的兜底；找不到合适 area 时应触发 stopline：新增 functional area、调整 capability 设计，或明确写成迁移期例外并给出收口计划。

## 4. 影响面总览

| 影响层 | 当前落点 | 来自 `150/157` 的设计 | 当前效果 | 主要风险 |
| --- | --- | --- | --- | --- |
| Contract | `config/capability/contract-freeze.v1.json` | 冻结 functional area 词汇、reason code、explain 字段、gate baseline | functional area 成为 capability contract 的一等字段 | 后续新增领域若不更新 contract，会被门禁或运行时阻断 |
| Route Map | `config/capability/route-capability-map.v1.json` | route capability 必须引用 active functional area | internal route 与 capability 归属形成证据链 | 迁移期容易复用旧 active area 来绕开新增功能域流程 |
| Runtime Gate | `internal/server/functional_area_governance.go` | lifecycle + capability status + tenant switch fail-closed | 功能域关闭会影响其下全部 capability | 粒度过粗时会误伤同域下无关能力 |
| Capability Catalog | `internal/server/capability_catalog_api.go` | capability 需要可检索、可解释、按 owner/intent 分类 | UI 和 API 可以展示 capability 目录 | metadata 会固化早期 target_object / surface / intent 命名 |
| Frontend API | `apps/web/src/api/setids.ts` | Functional Area / Activation / Catalog 可操作 | 前端已出现功能域状态与开关 DTO | 前端模块命名仍在 SetID API 下，容易混淆治理域和业务域 |
| Error / Explain | `presentApiError.ts`、contract reason codes | functional area 缺失/禁用/未激活有稳定错误码 | 用户可见错误不再泛化 | 错误码覆盖不完整会导致前后端语义不一致 |
| CI Gates | `Makefile`、`scripts/ci/check-capability-*.sh` | contract / route-map / catalog 必须可校验 | 漂移会在 CI 阶段暴露 | 改一处 capability 可能触发多处门禁联动 |
| CubeBox / Assistant | `DEV-PLAN-381` 已调查 | 旧 Assistant capability 归属 `org_foundation` | CubeBox successor 路由继承旧 capability | 容易被误读为 CubeBox DDD 归属 org |

## 5. 详细影响

### 5.1 Contract 层：Functional Area 成为 capability 主契约的一部分

`config/capability/contract-freeze.v1.json` 当前已经包含：

- `functional_areas`：`org_foundation/staffing/jobcatalog/person/iam_platform/compensation/benefits`
- `lifecycle_statuses`：`active/reserved/deprecated`
- `reason_codes`：`FUNCTIONAL_AREA_MISSING/FUNCTIONAL_AREA_DISABLED/FUNCTIONAL_AREA_NOT_ACTIVE`
- `explain_min_fields`：包含 `functional_area_key`
- `gate_baseline`：包含 `check capability-contract`、`check capability-route-map`、`check error-message`、`check routing`

**影响结论**：
Functional Area 不是文档备注，而是 capability contract 的一等结构。新增、迁移或拆分能力时，必须同步回答 functional area 归属和生命周期。

### 5.2 Route Map 层：路由能力映射必须穿透到 active functional area

`route-capability-map.v1.json` 将 capability 定义和 route 绑定放在同一份配置中。`check-capability-route-map.sh` 会校验：

- 每个 capability 必须有合法 `capability_key`。
- 每个 capability 的 `functional_area_key` 必须存在于 contract。
- 被 route 使用的 functional area 必须是 `active`。
- route 必须绑定已注册 capability。

**影响结论**：
路由不再只是 `method + path + action` 的授权表，还继承了 functional area lifecycle。一个新 route 想接入 runtime，必须先完成 capability 与 active functional area 的归属。

### 5.3 Runtime 层：Functional Area 是运行时 fail-closed gate

`internal/server/functional_area_governance.go` 当前实现：

- capability 不存在或 functional area 缺失：`FUNCTIONAL_AREA_MISSING`
- functional area lifecycle 非 active：`FUNCTIONAL_AREA_NOT_ACTIVE`
- capability status 非 active 或租户关闭 functional area：`FUNCTIONAL_AREA_DISABLED`
- 通过后才允许继续执行

**影响结论**：
Functional Area 已经成为 API/internal 运行时开关。关闭一个 functional area 会阻断其下全部 capability，而不仅是隐藏导航或 UI 按钮。

### 5.4 Capability Catalog 层：对象、界面和意图被二次固化

`internal/server/capability_catalog_api.go` 为 capability 增加了 `target_object/surface/intent` 元数据，并输出：

- `module`
- `owner_module`
- `target_object`
- `surface`
- `intent`
- `capability_key`
- `route_class`
- `actions`
- `status`

同时它会拒绝 `module` 与 `owner_module` 查询参数不一致。

**影响结论**：
`150/157` 后续治理不只看 capability_key，还把“这个能力面向哪个对象、在哪个界面、用于什么意图”暴露给前端与 reviewer。早期命名若不收口，会在 catalog 中继续放大。

### 5.5 前端 API 层：SetID Governance 承载了 Functional Area UI

`apps/web/src/api/setids.ts` 当前包含：

- policy activation API：`/internal/policies/state|draft|activate|rollback`
- functional area API：`/internal/functional-areas/state|switch`
- capability catalog API：`/internal/capabilities/catalog`

**影响结论**：
Functional Area 治理已经用户可见、可操作，但前端 API 仍挂在 `setids.ts` 这个历史文件中。这不会改变运行时语义，但会增加读者把 capability governance、SetID 与 org 混读的概率。

### 5.6 错误与 Explain 层：功能域失败有专用语义

Contract 已冻结 `FUNCTIONAL_AREA_MISSING/FUNCTIONAL_AREA_DISABLED/FUNCTIONAL_AREA_NOT_ACTIVE`。前端错误提示中已存在：

- `FUNCTIONAL_AREA_MISSING`
- `FUNCTIONAL_AREA_NOT_ACTIVE`

但当前调查只确认到前端有上述两项显式映射，未确认 `FUNCTIONAL_AREA_DISABLED` 是否已完成同等用户可见文案收口。

**影响结论**：
Functional Area 的失败语义已经进入用户提示与 explain 证据链。若错误码覆盖不完整，会出现后端给出稳定 reason code、前端却退回泛化错误的漂移。

### 5.7 CI / 门禁层：Functional Area 变更会触发多门禁联动

`Makefile` 已提供：

- `capability-key`
- `capability-contract`
- `capability-route-map`
- `capability-catalog`

`check-capability-contract.sh` 校验 functional area 词汇、生命周期、reason code、explain 字段与 gate baseline。

`check-capability-route-map.sh` 校验 route-map 中 capability 的 functional area 必须存在且 active。

**影响结论**：
`150/157` 的影响已经进入 CI，后续不能只改代码或只改文档。任何 capability 迁移都至少要同步 contract、route-map、runtime/catalog 测试和错误提示证据。

### 5.8 Assistant / CubeBox 层：历史继承被 functional area 放大

`DEV-PLAN-381` 已确认：

- `org_foundation` 来源是 `150/157` 的 functional area 治理体系。
- `org.assistant_conversation.manage` 来源是早期 Assistant/LibreChat 集成。
- `380` 只是把 `/internal/cubebox/*` successor 路由继承到这条旧 capability。

**影响结论**：
Functional Area 的唯一归属与 route-map active 校验，让历史 capability 复用更容易“看起来合理”。这不是 `150/157` 的设计错误，但它放大了早期命名没有及时重构的后果。

## 6. 正向收益

- **单主源更强**：Functional Area 词汇、生命周期、reason code 与门禁基线有 contract 文件承载。
- **租户治理更简单**：租户可以按功能域启停一组 capability，而不是逐项配置细粒度 key。
- **失败路径更明确**：缺归属、未激活、被关闭分别有稳定 reason code。
- **门禁防漂移**：route-map 不能绑定不存在或非 active 的 functional area。
- **审计与 Explain 更完整**：`functional_area_key` 进入 explain 最小字段，方便定位为何被拒绝。
- **用户入口可操作**：Functional Area / Activation / Catalog 已经进入前端 API 与治理页面设计。

## 7. 副作用与风险

- **粒度过粗风险**：一个 functional area 关闭会阻断其下所有 capability，若早期把多个领域能力放进同一 area，会产生误伤。
- **归属错位风险**：为了通过 active area 门禁，新能力可能临时挂到 `org_foundation`，长期固化错误归属。
- **概念混淆风险**：`owner_module`、`functional_area_key`、DDD module、前端导航模块都像“模块”，但语义不同。
- **迁移窗口变复杂**：新增独立 functional area 需要 contract、runtime lifecycle、route-map、catalog、前端、错误提示和 CI 同步准备。
- **历史命名放大风险**：catalog 会把旧 capability 的 target_object / surface / intent 暴露给 UI 和 reviewer，导致早期 assistant 命名继续影响 CubeBox。
- **错误覆盖缺口风险**：若 `FUNCTIONAL_AREA_DISABLED` 等 reason code 未在前端错误提示完整收口，用户可见语义会弱于 contract。

## 8. 对后续计划的建议

### 8.1 新能力准入清单

后续新增或迁移 capability 时，必须在同一计划中冻结：

- `capability_key`
- `functional_area_key`
- `owner_module`
- `capability_type`
- `target_object/surface/intent`
- route/action 映射
- lifecycle / activation / tenant switch 行为
- 关闭该 functional area 时的业务合理性说明
- 用户可见错误码与 explain 字段

同时必须回答一条爆炸半径问题：

- 关闭该 functional area 时，这个 capability 被关闭是否业务合理？若不合理，必须另选或新增 functional area。

### 8.2 CubeBox 后续收口建议

若 `CubeBox` 要从历史 Assistant 能力独立出来，不应只把 `OwnerModule` 改成 `cubebox`。应另立或承接一个 capability 重构计划，同步处理：

- 是否新增 `cubebox` 或类似独立 functional area。
- 是否新增 `cubebox.*` capability key。
- `/internal/cubebox/*` route-map 的 capability 绑定。
- Capability catalog 的 `target_object/surface/intent`。
- 前端导航权限、路由权限、错误提示和 E2E。
- 旧 `org.assistant_conversation.manage` 的退役或保留边界。
- 若短期继续继承 `org_foundation`，必须标记为迁移期例外，并冻结退出条件。

### 8.3 文档表述建议

后续 `380*` 文档应避免写成“CubeBox 挂在 org 模块”。更准确的表述是：

- DDD / route owner：部分 successor 路由已经是 `cubebox`。
- capability：仍继承早期 `org.assistant_conversation.manage`。
- functional area：因 capability 唯一归属，仍落在 `org_foundation`。
- 性质：迁移期历史继承，不是最终领域归属结论。

## 9. Stopline

- [ ] 新 capability 没有明确 `functional_area_key`。
- [ ] route-map 使用不存在或非 active functional area。
- [ ] 只改 route owner，却不改 capability catalog / route-map / 前端权限 / 错误提示。
- [ ] 把 `owner_module` 当成 DDD 边界完成态证明。
- [ ] 把历史 capability 继承写成正式业务域归属。
- [ ] 新增 functional area 但未补 contract、runtime lifecycle、CI 门禁和用户可见入口。
- [ ] 从 route path、前端导航、`owner_module`、旧 capability 或文件目录隐式推导 `functional_area_key`。
- [ ] 关闭某个 functional area 会误伤不相干 capability，但计划没有爆炸半径说明。
- [ ] capability prefix、route owner、functional area、catalog intent 明显不一致，却没有计划编号、例外理由和收口时间。
- [ ] 使用 `org_foundation` 或其他 active area 作为新能力“临时兜底桶”。

## 10. 验收标准

- [X] 能解释 `150/157` 为什么引入 Functional Area：降低细粒度 capability 管理成本，并提供租户级总开关。
- [X] 能列出 `150/157` 对 contract、route-map、runtime、catalog、frontend、error、CI 的影响。
- [X] 能区分正向收益与副作用风险。
- [X] 能给 `380*` 后续 capability 收口提供明确输入。
- [X] 能说明 Functional Area 的显式划分规则，并阻断类似 CubeBox 的隐式过大归属继续静默发生。
- [ ] 若进入实施：补齐对应 code / route-map / frontend / tests / readiness 证据。

## 11. 关联文档

- `docs/archive/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/156-route-capability-map-convergence.md`
- `docs/archive/dev-plans/157-capability-key-m7-functional-area-governance.md`
- `docs/dev-plans/158-capability-m8-ui-delivery-plan.md`
- `docs/dev-plans/381-cubebox-capability-and-functional-area-lineage-investigation.md`
- `config/capability/contract-freeze.v1.json`
- `config/capability/route-capability-map.v1.json`
- `internal/server/functional_area_governance.go`
- `internal/server/capability_catalog_api.go`
- `apps/web/src/api/setids.ts`
- `apps/web/src/errors/presentApiError.ts`
- `scripts/ci/check-capability-contract.sh`
- `scripts/ci/check-capability-route-map.sh`
