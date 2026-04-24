# DEV-PLAN-465：CubeBox OrgUnit Executor 契约边界与字段归属收敛方案

**状态**: 已完成（2026-04-25 07:31 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结 `internal/server/cubebox_orgunit_executors.go` 与 `orgunit` 读契约之间的 owner 边界，明确字段展示归属、执行层职责与当前剩余风险，阻断 `CubeBox` 侧再次长出第二套 `orgunit` 响应契约。
- **关联模块/目录**：`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`docs/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`internal/server`、`modules/cubebox`、`modules/orgunit`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`、`DEV-PLAN-463`、`DEV-PLAN-464`
- **用户入口/触点**：Web Shell 右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、`internal/server/cubebox_orgunit_executors.go`、`internal/server/orgunit_api.go`

### 0.1 Simple > Easy 三问

1. **边界**：字段展示归属在 `orgunit` 读契约，`CubeBox` executor 只负责受控执行适配，不负责重新定义 `orgunit.details` 的字段标准。
2. **不变量**：不得新增 `CubeBox` 私有 `orgunit` 响应 DTO、第二读事实源、第二解释层或第二字段白名单 owner。
3. **可解释**：reviewer 必须能在 5 分钟内说明：当前实现为什么属于“复用既有契约但重复组装”，而不是“`CubeBox` 再发明一套字段契约”。

## 1. 背景与问题定义

在 `DEV-PLAN-463/464` 收口后，`internal/server/cubebox_orgunit_executors.go` 中仍存在手工把 `details` 组装为 `orgUnitDetailsAPIResponse` 的实现。围绕这段代码，产生了三个需要冻结的问题：

1. 该文件是否仍符合 `DEV-PLAN-460` 到 `DEV-PLAN-464` 的当前 owner 与 stopline；
2. 哪些字段能展示，是否应由 `orgunit` 模块决定，而不是由 `CubeBox` 再定一次；
3. 这里逐字段穷举，到底是必要的 API 白名单收口，还是 `CubeBox` 侧不必要的重复实现。

本计划用于冻结这三个问题的结论，并给出后续收敛建议，替代临时调查记录，避免双份 SSOT 并存。

## 2. 核心目标

1. [X] 冻结 `cubebox_orgunit_executors.go` 与 `DEV-PLAN-460` 到 `DEV-PLAN-464` 的契约关系。
2. [X] 冻结字段展示边界的 owner：应归 `orgunit` 读契约，不归 `CubeBox`。
3. [X] 冻结当前实现的准确性质：属于“复用同一契约但重复组装”，不是“新增第二套字段标准”。
4. [X] 记录当前实现仍存在的风险点，并给出后续收敛建议。

## 3. 非目标

- 不在本计划内新增查询能力、执行能力或新的运行时抽象。
- 不在本计划内把 `CubeBox` executor 改造成新的 DTO 平台、共享解释平台或字段投影平台。
- 不在本计划内直接实施代码整改；若后续命中实现修改，应以本计划冻结的 owner 边界作为契约前提。

## 4. 调查结论

### 4.1 关于 `460-464` 的符合性

结论：`internal/server/cubebox_orgunit_executors.go` 当前整体上基本符合 `DEV-PLAN-460` 到 `DEV-PLAN-464` 的收口方向，但仍保留一处值得收紧的参数 fail-closed 风险。

支持依据：

1. `CubeBox` 执行时仍沿用当前租户上下文，没有独立 `subject`、独立 service account 或第二授权面，符合 `460` 的权限继承边界。
2. 当前执行注册表仍以 `api_key -> executor` 白名单、参数白名单与顺序执行为主，没有在该文件继续长出 `SummaryRenderer`、模板摘要器或 `target_unique` 一类新的隐藏协议，符合 `461/464` 对执行层“变薄”的要求。
3. `orgunit.list.status` 已只接受 canonical 值 `active` / `disabled` / `all`，未继续保留 `inactive` 兼容语义，符合 `464` 的参数 owner 收敛要求。

### 4.2 关于“字段展示应该由谁决定”

结论：字段展示边界的 owner 应该是 `orgunit` 模块，不应该由 `CubeBox` 单独重新规定。

当前实现更准确的性质是：

- `CubeBox` 没有再发明一套新的 `orgunit.details` 字段标准；
- 但 `CubeBox` 在 executor 中重复组装了一次 `orgunit` 已有响应契约。

因此当前问题更接近“实现重复”，而不是“契约 owner 漂移”。

### 4.3 关于“为什么要穷举字段”

结论：这里穷举字段的直接目的，是把内部领域结构收口到对外 API 白名单，而不是单纯样板代码。

内部 `OrgUnitNodeDetails` 含有内部字段，例如：

- `OrgID`
- `OrgNodeKey`
- `ParentID`
- `ParentOrgNodeKey`
- `PathIDs`
- `PathOrgNodeKeys`

这些字段并不属于对外 `orgunit.details` API 应暴露的稳定契约。因此对外响应需要显式白名单映射，避免内部结构变更或内部标识泄漏直接污染对外接口。

## 5. 事实冻结

### 5.1 `CubeBox` 当前使用的是 `orgunit` 现有响应类型

`cubebox_orgunit_executors.go` 中 `orgunit.details` 当前组装的是：

- `orgUnitDetailsAPIResponse`
- `orgUnitDetailsAPIItem`

上述类型定义并不在 `CubeBox` 私有模块中，而定义在 `internal/server/orgunit_api.go`。因此当前字段集合的 owner 仍在 `orgunit` 读契约，而不是 `CubeBox` 新设。

### 5.2 `CubeBox` 当前确实又手写组装了一次响应

`internal/server/cubebox_orgunit_executors.go` 当前仍手工把 `details` 转成 `orgUnitDetailsAPIResponse`。这说明当前实现的核心问题是同一份契约被重复组装，未来存在漂移面。

### 5.3 内部领域对象与对外 API 契约并不等价

`OrgUnitNodeDetails` 同时持有内部主键、内部节点键、内部父节点键与内部路径标识。这说明：

1. 不能直接把内部领域对象原样作为对外 API 输出；
2. 必须存在从内部结构到对外稳定 DTO 的显式收口过程。

### 5.4 执行注册层仍是白名单执行，不是第二套运行时

`modules/cubebox/read_executor.go` 当前仍通过以下方式控制执行边界：

- `RegisteredExecutor` 持有 `APIKey`、`RequiredParams`、`OptionalParams`
- `ExecutionRegistry.ExecutePlan(...)` 先做 `ValidateReadPlan(plan)`
- 执行前通过 `validateRegisteredParams(...)` 拒绝未注册参数

因此当前执行层仍符合 `464` 的边界：它是白名单执行与护栏层，而不是第二套 `orgunit` 运行时。

## 6. Owner 边界冻结

### 6.1 应然边界

应然上：

1. 哪些字段能展示，由 `orgunit` 模块读契约决定。
2. `CubeBox` executor 只负责参数转换、调用读链路、返回原始业务 payload 或稳定领域结果。

这与 `DEV-PLAN-464` 第 8.2、8.4 节一致。

### 6.2 当前实现状态

当前状态不是“`CubeBox` 又规定一次字段”，而是：

1. 复用了 `orgunit` 已有响应类型；
2. 但在 `CubeBox` 侧重复写了一次相同的组装逻辑。

因此它更接近“实现重复”，而不是“契约 owner 漂移”。

### 6.3 风险

若后续 `orgunit.details` 契约新增、删除或重命名字段，而 `cubebox_orgunit_executors.go` 未同步更新，就会出现：

- 主 API 与 `CubeBox` payload 漂移；
- 评审时看起来仍然“类型相同”，但字段实际不一致；
- `CubeBox` 被动承担一份不该长期持有的重复维护责任。

## 7. 当前发现的问题

### 7.1 `include_disabled` 校验未完全 fail-closed

当前 `normalizeOptionalBool(...)` 在收到字符串时会调用 `parseIncludeDisabled(...)`，而后者只把：

- `1`
- `true`
- `yes`
- `on`

视为 `true`，其余任意字符串都会静默落为 `false`。

这意味着模型如果产出非法字符串，不会被拒绝，而会被当成合法的 `false` 继续执行。该行为弱化了 `461/464` 要求的 schema / fail-closed 校验边界。

## 8. 后续收敛建议

- 保持字段 owner 继续归属 `orgunit`，不要在 `CubeBox` 侧新增私有响应类型或私有字段白名单。
- 将 `orgunit.details`、`orgunit.audit` 等重复 DTO 组装逻辑进一步收口为共享 builder，或直接复用 `orgunit` 既有响应组装函数，减少双份维护。
- 将 `include_disabled` 的字符串校验收紧为严格白名单；非法字符串直接报错，不再静默落为 `false`。

## 9. 验收标准

1. [X] 本计划已明确冻结：字段展示边界 owner 属于 `orgunit`，不属于 `CubeBox`。
2. [X] 本计划已明确冻结：当前实现属于“复用同一契约但重复组装”，不是“新增第二套字段标准”。
3. [X] 本计划已明确冻结：`cubebox_orgunit_executors.go` 当前大体符合 `460-464`，但仍存在一处参数 fail-closed 风险。
4. [X] 本计划未新增第二套字段契约、第二读事实源或新的本地解释平台。

## 10. 门禁与执行记录

- 本轮交付只涉及文档：
  - `make check doc`

## 11. 交付物

- `docs/dev-plans/465-cubebox-orgunit-executor-contract-boundary-and-field-owner-convergence-plan.md`

## 12. 关联文档

- `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
- `docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`
- `docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`
- `docs/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md`
- `docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`
