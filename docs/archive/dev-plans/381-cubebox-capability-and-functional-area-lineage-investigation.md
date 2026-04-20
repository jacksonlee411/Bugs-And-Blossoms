# DEV-PLAN-381：CubeBox capability 与 functional area 历史来源专项调查

**状态**: 规划中（2026-04-16 14:35 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T1`
- **范围一句话**：调查 `CubeBox` 当前为何仍绑定 `org.assistant_conversation.manage` 与 `org_foundation`，并冻结“历史来源是谁、继承链路是什么、哪些文档负责收敛”的结论。
- **关联模块/目录**：`internal/server`、`modules/cubebox`、`docs/dev-plans/150/157/380*`、`docs/archive/dev-plans/220*`
- **关联计划/标准**：
  - `AGENTS.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/015-ddd-layering-framework.md`
  - `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
  - `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
  - `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
  - `docs/archive/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- **用户入口/触点**：`/app/cubebox*`、`/internal/cubebox/*`、capability registry / route-capability-map / functional area 治理

### 0.1 Simple > Easy 三问

1. **边界**：本调查区分三条边界：`owner_module`、`capability_key`、`functional_area_key`。三者不是同一概念，不能互相替代。
2. **不变量**：同一 capability 只能归属一个 functional area；CubeBox successor 路由的 `OwnerModule` 已是 `cubebox`，但 capability / functional area 仍可继承旧链路。
3. **可解释**：必须能在 5 分钟内讲清“为什么现在看起来像挂在 org 下”，并指出是哪个历史批次先冻结、哪个批次只是继承。

### 0.2 现状研究摘要

- **现状实现**：
  - capability 定义中，`org.assistant_conversation.manage` 归属 `FunctionalAreaKey=org_foundation`、`OwnerModule=orgunit`，见 [capability_route_registry.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/capability_route_registry.go#L95)。
  - `/internal/cubebox/*` successor 路由的 `OwnerModule` 已切为 `cubebox`，但 capability 仍继续使用 `org.assistant_conversation.manage`，见 [capability_route_registry.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/capability_route_registry.go#L397)。
- **现状约束**：
  - `functional_area_key` 是 capability catalog 体系的稳定归属字段，不是 CubeBox 计划阶段单独发明。
  - `org.assistant_conversation.manage` 是早期 Assistant/LibreChat 集成批次遗留 capability，不是 `380E` 或 `380C` 首次命名。
- **最容易出错的位置**：
  - 把 `OwnerModule=cubebox` 误读为 capability / functional area 也已经切到 `cubebox`
  - 把 `380*` 的 successor 切路由动作误读为 capability 命名冻结来源
  - 把 DDD 模块边界与 capability catalog 归属混成同一层语义
- **本次不沿用的“容易做法”**：
  - 不用“因为 CubeBox 属于 org 模块”这种简化说法
  - 不把 `380E` 评审意见口头外推为 capability 历史来源

## 1. 背景与上下文

- **需求来源**：对 `380E` 评审与后续追问中，用户明确质疑：
  - 为什么 `CubeBox` 看起来还挂在 `org` 模块
  - `org.assistant_conversation.manage` / `org_foundation` 是哪份计划文档导致的
- **当前痛点**：
  - 代码里同时存在 `OwnerModule=cubebox` 与 `CapabilityKey=org.assistant_conversation.manage`
  - 若不追溯历史来源，后续很容易把“当前过渡态”误写成“正式领域归属”
- **业务价值**：
  - 给 `380B/380C/380E/380G` 后续收口提供可引用的调查事实源
  - 避免 capability / functional area / DDD module 三个层次继续漂移
- **仓库级约束**：
  - 计划文档是 contract-first 事实源
  - 同一概念只能有一种权威表达

## 2. 调查目标与非目标

### 2.1 核心目标

- [X] 明确 `org_foundation` 作为 functional area 的冻结来源
- [X] 明确 `org.assistant_conversation.manage` 作为 capability 的首次引入批次
- [X] 明确 `/internal/cubebox/*` 为什么继续继承这条 capability
- [X] 给出“哪个计划负责来源、哪个计划只是继承”的结论，供 `380*` 系列引用

### 2.2 非目标

- 不在本文直接改 capability registry 命名
- 不在本文直接决定是否新增 `cubebox.read` / `cubebox.admin`
- 不替代 `380B/380C` 的正式收口计划，只提供历史来源调查结论

### 2.3 用户可见性交付

- **用户可见入口**：本调查文档本身，以及后续可被 `380*` 计划直接引用的结论段
- **最小可操作闭环**：reviewer 能据此回答：
  - `org_foundation` 从哪来
  - `org.assistant_conversation.manage` 从哪来
  - `380` 是否只是继承旧链路

## 2.4 工具链与门禁

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

## 3. 调查范围与方法

### 3.1 调查对象

- capability registry：`internal/server/capability_route_registry.go`
- capability / functional area 计划：
  - `DEV-PLAN-150`
  - `DEV-PLAN-157`
- 早期 Assistant 总纲与实现批次：
  - `docs/archive/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - 提交 `dde191e8`
- CubeBox successor 批次：
  - `DEV-PLAN-380`
  - 提交 `47233768`

### 3.2 调查方法

1. 检索仓库文档中 `org_foundation` 与 `org.assistant_conversation.manage` 的显式/隐式来源。
2. 用 `git log -S` 与 `git blame` 追溯 capability registry 中对应字段的首次引入提交。
3. 对比计划文档责任边界，区分“概念冻结来源”和“代码继承落地批次”。

## 4. 调查发现

### 4.1 `org_foundation` 的来源不是 CubeBox，而是 capability functional area 治理体系

最早的计划级冻结来源是 [150-capability-key-workday-alignment-gap-closure-plan.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md#L104)：

- `5.6.1 Functional Area 词汇表（M1 冻结）`
- 首批冻结清单中明确包含 `org_foundation`
- 并要求“每个 capability_key 必须且仅能归属 1 个 functional_area_key” [150-capability-key-workday-alignment-gap-closure-plan.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md#L123)

随后 [157-capability-key-m7-functional-area-governance.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/157-capability-key-m7-functional-area-governance.md#L12) 将这套功能域治理正式落地，并冻结：

- `functional_area_key/display_name/owner_module/lifecycle_status`
- capability 与功能域建立唯一归属关系

**调查结论 1**：
`org_foundation` 的来源是 `DEV-PLAN-150/157` 代表的 capability functional area 治理体系，而不是 `380*` 或 `CubeBox` 专项计划。

### 4.2 `org.assistant_conversation.manage` 的来源是早期 Assistant 集成批次

`git blame` 显示 capability 定义中的这条记录由提交 `dde191e8` 首次写入 [capability_route_registry.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/capability_route_registry.go#L95)：

- `CapabilityKey: "org.assistant_conversation.manage"`
- `FunctionalAreaKey: "org_foundation"`
- `OwnerModule: "orgunit"`

该提交同时新增的计划文档是归档后的 [220-chat-assistant-upgrade-implementation-plan.md](/home/lee/Projects/Bugs-And-Blossoms/docs/archive/dev-plans/220-chat-assistant-upgrade-implementation-plan.md#L1)，提交说明为：

- `feat(assistant): add librechat-integrated assistant workspace and apis`

虽然 `DEV-PLAN-220` 文本本身未直接写出 `org.assistant_conversation.manage` 字面量，但从提交绑定关系可以确认：

- 早期 Assistant/LibreChat 集成批次把这项会话能力接入了 capability registry
- 其归属沿用了当时已有的 `org_foundation` 功能域口径

**调查结论 2**：
`org.assistant_conversation.manage` 的直接来源是早期 Assistant/LibreChat 集成实现批次；对应历史方案背景文档是 `DEV-PLAN-220`，对应代码落地提交是 `dde191e8`。

### 4.3 CubeBox 没有重新设计这条 capability，而是继承了旧链路

`git blame` 显示 `/internal/cubebox/*` successor 路由是在提交 `47233768` 中加入 capability-route registry 的 [capability_route_registry.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/capability_route_registry.go#L397)。

这一批次的特征是：

- `OwnerModule` 已改为 `cubebox`
- `CapabilityKey` 仍继续使用 `org.assistant_conversation.manage`

与 [380-cubebox-first-party-ownership-and-librechat-retirement-plan.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md#L64) 的表述一致：`/internal/cubebox/*` 在早期只是 successor 路由与兼容接线，后端仍大量复用 `assistant` 逻辑。

**调查结论 3**：
`380` 系列不是这条 capability 命名的起源。`380` 第一轮做的是 successor 路由/正式入口切换，但在 capability 层继续继承了已有 `assistant` 口径。

## 5. 归因结论（冻结版）

### 5.1 结论表

| 调查对象 | 真正来源 | 责任文档/批次 | 说明 |
| --- | --- | --- | --- |
| `org_foundation` | capability functional area 词汇表冻结 | `DEV-PLAN-150` / `DEV-PLAN-157` | 这是功能域治理层概念，不是 CubeBox 专项命名 |
| `org.assistant_conversation.manage` | 早期 Assistant/LibreChat 集成 capability 引入 | `DEV-PLAN-220` 对应实现批次（提交 `dde191e8`） | capability registry 中首次登记为 assistant 会话能力 |
| `/internal/cubebox/* -> org.assistant_conversation.manage` | successor 路由继承旧 capability | `DEV-PLAN-380` 第一轮 formal entry / successor 接线（提交 `47233768`） | `380` 继承旧口径，不是 capability 起源 |

### 5.2 最终结论

本仓当前出现“`CubeBox` 的 owner module 已是 `cubebox`，但 capability 仍是 `org.assistant_conversation.manage`，functional area 仍是 `org_foundation`”的直接原因，不是某一份 `380*` 文档单独决定的，而是：

1. `DEV-PLAN-150/157` 先冻结了 `org_foundation` 功能域治理体系。
2. 早期 Assistant 集成批次再把会话能力登记成 `org.assistant_conversation.manage`，并归属到 `org_foundation`。
3. `380` 第一轮 successor 切路由时，为了保持主链迁移连续性，继续沿用了这条旧 capability，而没有在同批次重新设计独立 `cubebox.*` 能力。

## 6. 对 `380*` 系列的影响评估

### 6.1 对 `380B/380C/380E` 的输入

- `380B/380C/380E` 不应把当前 capability 口径表述成“CubeBox 正式领域归属已经是 org”
- 若后续要收口为 `cubebox.read` / `cubebox.admin` / `cubebox.conversation.manage` 等独立能力，必须明确这是**新一轮 capability 重构**，而不是“修正文档措辞”
- 在此之前，`380*` 计划应把现状表述为：
  - 模块归属：`cubebox`
  - capability / functional area：仍继承 assistant 历史口径
  - 性质：迁移期遗留，而非最终 DDD 完成态

### 6.2 stopline 建议

- [ ] 未明确区分 `OwnerModule`、`CapabilityKey`、`FunctionalAreaKey` 三层语义
- [ ] 将现状 capability 继承误写为正式领域归属
- [ ] 在未新开 capability 重构计划前，暗中把 `cubebox` 权限口径写成“已独立完成”

## 7. 建议的后续承接

1. 若仅做文档纠偏：
   - 在 `380B/380C/380E` 引用本调查文档，明确“当前 capability 仍属历史继承”
2. 若做正式收口：
   - 另立 capability 重构计划
   - 同步改：
     - capability registry
     - route-capability-map
     - authz / role policy
     - `apps/web` `permissionKey`
     - 测试与 readiness 证据

## 8. 验收标准

- [X] 能明确指出 `org_foundation` 的计划来源是 `DEV-PLAN-150/157`
- [X] 能明确指出 `org.assistant_conversation.manage` 的首次引入批次是早期 Assistant 集成提交 `dde191e8`
- [X] 能明确指出 `380` 只是继承这条 capability 到 `/internal/cubebox/*`
- [X] 本文可被后续 `380*` 计划直接引用，而不再依赖口头说明

## 9. 关联文档

- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
- `docs/archive/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
- `docs/dev-plans/380b-cubebox-backend-formal-implementation-cutover-plan.md`
- `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
- `docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
