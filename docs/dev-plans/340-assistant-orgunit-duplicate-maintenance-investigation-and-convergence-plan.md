# DEV-PLAN-340：Assistant 组织架构重复维护调查与收敛方案

**状态**: 规划中（2026-04-10 18:20 CST）

## 1. 背景

近期对 `assistant` 模块进行代码级调查时，出现一个明确疑问：Assistant 是否又在 `orgunit` 正式维护链路之外，维护了一套组织架构变更实现。

该问题之所以需要单独立计划文档固化，原因有三：

1. Assistant 已从早期“单一 create 场景”扩展到多组织动作纳管，代码体量已不再是临时壳层。
2. 仓库规则要求遵守 `One Door`、`No Legacy`、`Contract First`，任何“第二写入口”或“领域规则重复实现”都属于高风险漂移。
3. 当前仅口头结论不足以支撑后续整改，需要把“哪些属于可接受的事务编排层、哪些已经重复进入 orgunit 领域规则”收敛成正式事实源。

## 2. 调查范围

本次调查覆盖以下对象：

1. `internal/server/assistant_*` 中与组织架构动作相关的注册、语义、dry-run、confirm/commit 代码。
2. `internal/server/orgunit_*` 中正式的 OrgUnit API / capability / field policy / store 读写链路。
3. `modules/orgunit/services` 中正式的 OrgUnit 写服务与字段策略决策实现。
4. `docs/dev-plans/240*`、`272*` 中对 Assistant 组织动作编排的既有设计承诺。

本次调查不覆盖：

1. `staffing/person/jobcatalog` 等非 OrgUnit 领域动作。
2. 前端页面视觉交互细节。
3. 新的数据库 schema / migration 设计；若后续整改需要新增表结构，必须另行获批。

## 3. 调查问题

本计划聚焦回答以下问题：

1. [ ] Assistant 是否自行维护了第二套 OrgUnit 数据写入口或数据库写内核。
2. [ ] Assistant 是否维护了“一套独立的组织架构动作编排层”。
3. [ ] Assistant 当前哪些逻辑只是事务编排，哪些逻辑已经与 `modules/orgunit/services` 发生领域规则重复。
4. [ ] 若存在重复维护，应如何收敛为“Assistant 只编排、OrgUnit 只裁决”的边界。

## 4. 已确认结论

### 4.1 未发现第二套 OrgUnit 写内核或第二写入口

1. [X] Assistant 服务构造时复用同一个 `orgStore` 与 `orgUnitWriteService`，未见独立 OrgUnit 写存储或第二条 DB 写链。
2. [X] Assistant 各类 commit adapter 最终统一调用 `orgunitservices.OrgUnitWriteService` 的 `Write/Correct/Disable/Enable/Move/Rename`。
3. [X] 未发现 Assistant 通过旁路 `/org/api/*` 或自写 SQL/Store 直接维护 OrgUnit 表。
4. [X] 结论：**Assistant 不是第二套 OrgUnit 写模型，也不是第二写入口。**

### 4.2 已形成一套 Assistant 专属的 OrgUnit 动作编排层

1. [X] Assistant 在服务端维护了 `assistantActionRegistry`，注册 `create/add_version/insert_version/correct/disable/enable/move/rename` 等 OrgUnit 动作。
2. [X] Assistant 在语义层维护了组织动作对应的 required slots、route kind、intent normalize、skill/config delta plan 编译逻辑。
3. [X] Assistant 在运行时维护了候选组织搜索、候选确认、dry-run 结果组装、confirm gate、commit gate、version tuple 校验。
4. [X] 结论：**Assistant 当前并非“薄 UI 壳 + 统一动作总线”，而是维护了一套明显偏 OrgUnit 专用的事务编排层。**

### 4.3 已出现与 OrgUnit 领域规则重叠的前置信息裁决

1. [X] Assistant 在 `create_orgunit` dry-run 阶段，会自行解析父组织、查询 `org_code` / `d_org_type` 的 field decision，并根据租户字段配置回填 `FIELD_REQUIRED_VALUE_MISSING` / `PATCH_FIELD_NOT_ALLOWED`。
2. [X] `modules/orgunit/services/orgunit_write_service.go` 已在正式写链路内实现 `applyCreatePolicyDefaults(...)`，负责同一组字段策略默认值、必填、allowed values 与自动编码决策。
3. [X] 结论：**Assistant 虽未重写最终写库内核，但已经重复承担了部分 create 场景的领域前置裁决逻辑。**

### 4.4 当前重复维护的严重程度判断

1. [X] 若以“是否存在第二套 DB Kernel / 第二写入口”为标准，当前风险为“未命中”。
2. [X] 若以“是否存在 OrgUnit 领域专属逻辑在 Assistant 侧再次实现”为标准，当前风险为“已命中”。
3. [X] 当前最明显的重复点集中在：`create_orgunit` 的字段策略预检，而不是最终写入。
4. [X] 当前次级风险在于：Assistant 对 OrgUnit 八动作的注册、plan 编译、候选解析均采用显式动作枚举与 if/switch 扩展，长期会提高维护成本与漂移概率。

## 5. 重复维护点清单

### 5.1 可接受的 Assistant 编排职责

以下职责当前可视为“Assistant 应保留的编排层”：

1. [X] 语义模型输入输出适配：自然语言 -> `assistantIntentSpec`。
2. [X] 会话状态机：`draft/proposed/validated/confirmed/committed`。
3. [X] 候选组织确认与人工 disambiguation。
4. [X] 风险分级、确认窗口、任务编排、receipt/poll/refresh。
5. [X] commit adapter 作为“把 Assistant turn 映射为正式领域请求”的薄适配层。

### 5.2 已进入重复维护风险区的职责

以下职责已超出“纯编排”边界，属于应重点收敛的重复维护点：

1. [X] `create_orgunit` 的字段策略预检在 Assistant 侧再次解析 `org_code` / `d_org_type` 决议。
2. [X] Assistant 直接感知租户字段配置是否启用，并据此推导 `PATCH_FIELD_NOT_ALLOWED`。
3. [X] Assistant 自行理解 `SetID Strategy Registry` 对 OrgUnit 创建字段的影响，而不是通过统一的 OrgUnit 只读能力/预检入口获取结果。
4. [X] Assistant 维护按动作展开的 orgunit plan/delta 编译逻辑，导致“新增/调整 OrgUnit 动作”时仍需同时修改 Assistant 主链。

### 5.3 目前尚未定性为问题、但需要持续关注的点

1. [X] Assistant 当前使用候选搜索与详情读取补足 `path`、`org_node_key`、`version_tuple`，这属于 confirm/commit 前 OCC 与用户确认所需事实，短期仍有保留合理性。
2. [X] 但若 OrgUnit 未来提供正式的“候选确认上下文快照/提交前校验”服务接口，Assistant 应优先收口到正式只读接口，而不是继续扩展本地拼装逻辑。

## 6. 根因归纳

1. [X] `DEV-PLAN-240/272` 为快速打通 OrgUnit 多动作闭环，在 Assistant 层逐步长出了动作注册、编排与预检逻辑。
2. [X] 当前缺少一个“OrgUnit 正式预检/只读裁决接口”，导致 Assistant 为了在 createTurn/dry-run 阶段给出可解释错误，不得不直接读取策略决议与字段配置。
3. [X] Assistant 与 OrgUnit 的边界目前更接近“共享同一个写服务”，但尚未达到“共享同一个正式预检/解释入口”。

## 7. 收敛原则（冻结）

1. [ ] Assistant 可以维护会话编排，但不得成为第二个 OrgUnit 领域裁决器。
2. [ ] OrgUnit 写前字段策略、默认值、allowed values、maintainable/required 判定，应以 `modules/orgunit/services` 或其正式只读能力 API 为唯一事实源。
3. [ ] Assistant 不应直接依赖 `SetID Strategy Registry` 的低层决策查询来解释 OrgUnit 创建规则；若必须读取，也应通过 OrgUnit 提供的稳定能力边界读取。
4. [ ] `One Door` 不只约束“最终写入”，也应约束“关键领域裁决”的唯一主源。

## 8. 收敛方案

### 8.1 目标边界

将边界收敛为：

`Assistant Intent/State Machine -> OrgUnit Capability/Precheck API or Service -> OrgUnit WriteService -> DB Kernel`

其中：

1. Assistant 负责语义、会话、确认、任务编排。
2. OrgUnit 负责字段策略解释、字段可维护性、默认值、候选提交前裁决。
3. 最终写入仍必须走 `OrgUnitWriteService -> SubmitEvent/SubmitCorrection -> DB Kernel`。

### 8.2 Phase 1：先消除最明显的重复裁决

1. [ ] 为 `create_orgunit` 提供正式的 OrgUnit 预检/能力读取入口，返回：
   - `effective_policy_version`
   - create required/maintainable/default/allowed values 结果
   - 对 `org_code` / `d_org_type` 的正式判定
   - 稳定错误码
2. [ ] Assistant 删除直接读取 `SetID Strategy Registry` 与租户字段配置的 create 预检逻辑。
3. [ ] `assistant_create_policy_precheck.go` 退化为“调用正式预检入口并回填 dry-run”，不再本地理解字段决议。

### 8.3 Phase 2：压缩 OrgUnit 动作专用编译分支

1. [ ] 逐步把按动作枚举的 `plan/skill/config_delta` 编译逻辑从 Assistant 主链中抽离。
2. [ ] Assistant 保留最小动作注册表，但动作 required slots / summary / precheck contract 尽量转向知识包或声明式注册。
3. [ ] 对 OrgUnit 八动作建立“声明式动作元数据 + 统一 adapter 输入模型”，减少 if/switch 扩散。

### 8.4 Phase 3：统一候选与 OCC 只读边界

1. [ ] 若后续继续保留 `version_tuple` / `candidate confirmation`，应优先提供 OrgUnit 正式只读快照接口。
2. [ ] Assistant 不再自行拼装过多 OrgUnit 详情衍生事实，避免 confirm/commit 前的只读口径继续漂移。

## 9. 验收标准

1. [ ] Assistant 不再直接查询 OrgUnit 创建字段策略底层决议与租户字段配置。
2. [ ] `create_orgunit` dry-run 的 `FIELD_REQUIRED_VALUE_MISSING` / `PATCH_FIELD_NOT_ALLOWED` 等错误，来源于 OrgUnit 正式预检边界，而不是 Assistant 本地重复推导。
3. [ ] Assistant 最终仍只通过 `orgunitservices.OrgUnitWriteService` 提交，不新增第二写入口。
4. [ ] 新增或调整 OrgUnit 动作时，Assistant 不需要同步修改多处 org 专属 if/switch 才能保持行为一致。

## 10. 关联事实源

1. `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
2. `docs/dev-plans/272-assistant-orgunit-seven-actions-expansion-plan.md`
3. `modules/orgunit/services/orgunit_write_service.go`
4. `internal/server/assistant_action_registry.go`
5. `internal/server/assistant_create_policy_precheck.go`
6. `internal/server/assistant_intent_pipeline.go`
