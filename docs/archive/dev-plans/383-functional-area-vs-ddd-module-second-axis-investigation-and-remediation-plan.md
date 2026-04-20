# DEV-PLAN-383：Functional Area 与 DDD 模块并行第二维度风险专项调查与收敛建议

**状态**: 规划中（2026-04-17 08:31 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T1` 调查；若后续据此修改 capability contract / route-map / runtime / catalog / 前端治理页，则承接计划应按 `T2` 评审。
- **范围一句话**：专项调查 `DEV-PLAN-150/157` 引入的 `functional_area` 是否已经成为与 DDD bounded context / module 平行的第二套治理维度，并评估其带来的认知、契约与运行时混乱风险，给出收敛建议。
- **关联模块/目录**：`docs/dev-plans/015/016/150/157/381/382`、`AGENTS.md`、`config/capability`、`internal/server`
- **关联计划/标准**：
  - `AGENTS.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/015-ddd-layering-framework.md`
  - `docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
  - `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
  - `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
  - `docs/dev-plans/381-cubebox-capability-and-functional-area-lineage-investigation.md`
  - `docs/dev-plans/382-capability-functional-area-governance-impact-investigation.md`
- **用户入口/触点**：Capability Governance、Functional Area、Capability Catalog、route-capability-map、运行时 functional area gate、后续模块收口计划

### 0.1 Simple > Easy 三问

1. **边界**：DDD `module / bounded context`、`capability_key`、`functional_area_key` 必须是三层不同概念，不能互相偷换。
2. **不变量**：模块边界是仓库级 SSOT；任何新增治理维度都不能反客为主，变成新的事实边界。
3. **可解释**：reviewer 必须能在 5 分钟内说明“为什么一个能力属于某模块、为什么受某 functional area 开关控制、两者的关系到底是同构还是派生”。

### 0.2 现状研究摘要

- **现状实现**：
  - `AGENTS.md` 已冻结模块边界：业务域 4 模块 `orgunit/jobcatalog/staffing/person` + 平台模块 `iam`。
  - `DEV-PLAN-150` 冻结 `functional_area` 词汇表、唯一归属、生命周期、租户总开关与 fail-closed 语义。
  - `DEV-PLAN-157` 将 `functional_area` 落地成词汇层、归属层、执行层三层治理。
- **现状约束**：
  - `015/016` 的 DDD 口径要求以模块/bounded context 作为边界事实源，避免形成第二套权威表达。
  - `150/157` 的 `functional_area` 已不是只读标签，而是进入 contract、route-map、runtime 和 UI 的运行时治理字段。
- **最容易出错的位置**：
  - 把 `functional_area` 误读成 DDD 模块。
  - 把 `owner_module` 当成功能域归属的自动推导依据。
  - 在真实模块边界未变的情况下，用 `functional_area` 承担模块级启停、产品导航分组与 capability 聚合三种职责。
- **本次不沿用的“容易做法”**：
  - 不把 `functional_area` 轻描淡写为“只是个标签”。
  - 不把“运行时已经可用”误判为“概念层次就合理”。
  - 不把 `Workday functional area` 词汇直接等同于本仓 DDD 边界设计。

## 1. 背景与问题

- **需求来源**：在对 `DEV-PLAN-150/157` 的批判性评估中，用户明确要求从 DDD 角度审查其是否引入了与模块并行的第二套维度，并容易引发混乱。
- **当前痛点**：
  - `functional_area` 已经从治理辅助概念演进为运行时 gate、目录归属、错误语义和租户总开关。
  - 其首批词汇与仓库冻结模块边界并不完全同构，导致“模块归属”“功能域归属”“capability 归属”长期并存。
  - 后续如 `CubeBox`、Assistant、治理 UI、新 capability 迁移，都可能在三套语义之间来回借词，增加设计与评审成本。
- **业务价值**：
  - 澄清 `150/157` 的结构性风险，而不是继续把问题局限在单个 capability 命名争议。
  - 为后续是否保留、降级、重命名或重构 `functional_area` 提供调查依据与收敛方向。

## 2. 调查目标与非目标

### 2.1 核心目标

- [X] 对照 `015/016` 与 `150/157`，判断 `functional_area` 是否已形成与 DDD 模块并行的第二套维度。
- [X] 识别这套第二维度在命名、责任归属、运行时 gate、UI 治理和评审语义上的主要混乱风险。
- [X] 区分 `functional_area` 当前承担的不同职责，判断哪些职责合理，哪些职责越界。
- [X] 给出面向后续计划的收敛建议：保留条件、降级条件、替代方案与 stopline。

### 2.2 非目标

- 不在本文直接修改 `150/157` 原文状态或结论。
- 不在本文直接改动 capability contract、route-map、runtime 或前端实现。
- 不在本文直接决定 `functional_area` 必须立即删除；本文先冻结调查结论与修订方向。

### 2.3 用户可见性交付

- **用户可见入口**：本调查文档本身，供后续 capability / governance / CubeBox / 模块边界计划引用。
- **最小可操作闭环**：reviewer 能据本文回答：
  - `functional_area` 当前是不是第二套维度
  - 风险主要来自哪里
  - 后续应该怎么收敛，而不是继续在术语上打补丁

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

## 3. 调查方法

### 3.1 对照对象

- DDD / 模块边界 SSOT：
  - `AGENTS.md`
  - `DEV-PLAN-015`
  - `DEV-PLAN-016`
- capability / functional area 治理设计：
  - `DEV-PLAN-150`
  - `DEV-PLAN-157`
- 已有调查结论：
  - `DEV-PLAN-381`
  - `DEV-PLAN-382`

### 3.2 判定问题

1. `functional_area` 是否只是展示/运营标签，还是已经拥有独立的运行时语义与归属权力？
2. 它与 `module / bounded context` 是同构、派生，还是并行且不完全重合？
3. 若不完全重合，这种不重合是必要抽象，还是引入了长期混乱成本？

### 3.3 判定标准

- 若一个字段同时具备“稳定词汇表 + 唯一归属 + 运行时 fail-closed + UI 入口 + CI 门禁”，则它已经不是弱标签，而是强治理维度。
- 若该维度与模块边界不完全同构，却又被赋予 owner、生命周期和租户开关，则它已具备与模块并行竞争解释权的风险。
- 若 reviewer 无法仅凭一个事实源判断“边界归属是谁”，则说明发生了语义分叉。

## 4. 调查发现

### 4.1 仓库级 SSOT 已经冻结“模块边界是第一边界”

`AGENTS.md` 已明确仓库模块边界为 `orgunit/jobcatalog/staffing/person + iam`，并要求跨模块通过 `pkg/**` 与 HTTP/JSON API 组合，而不是引入新的平行模块体系。

`DEV-PLAN-015` 进一步要求：

- 模块四层目录是默认承载形态。
- 任何新抽象都不能形成第二套权威表达。
- 评审 stopline 明确阻断“为了更容易实现而引入两套权威表达”。

`DEV-PLAN-016` 也把 bounded context / module 冻结为 4 个业务模块 + 平台模块，并明确这些模块负责数据所有权与写入口。

**调查结论 1**：
DDD module / bounded context 在本仓不是参考意见，而是已经冻结的一级事实边界。

### 4.2 `150/157` 中的 `functional_area` 已经是强治理维度，不是弱标签

`DEV-PLAN-150` 为 `functional_area` 冻结了：

- 稳定词汇表
- `owner_module`
- 生命周期 `active/reserved/deprecated`
- 每个 capability 的唯一归属
- 租户级总开关
- 关闭后全链路 fail-closed

`DEV-PLAN-157` 进一步将它落地为：

- 词汇层：注册与生命周期
- 归属层：capability 唯一绑定
- 执行层：运行时根据 capability 回查 functional area 并阻断

这意味着 `functional_area` 已拥有：

- 命名权
- 归属权
- 生命周期权
- 租户治理权
- 运行时阻断权

**调查结论 2**：
`functional_area` 不是辅助标签，而是已成体系的强治理维度。

### 4.3 这套维度与模块并不完全同构，而是“相似但不相同”

`150` 冻结的 functional area 首批词汇包括：

- `org_foundation`
- `staffing`
- `jobcatalog`
- `person`
- `iam_platform`
- `compensation`
- `benefits`

其中至少存在三类不对齐：

1. **命名不对齐**：
   - `org_foundation` 对应的不是模块名 `orgunit`
   - `iam_platform` 对应的不是模块名 `iam`
2. **粒度不对齐**：
   - `org_foundation` 更像产品/主数据能力域，不是严格 bounded context 名称
3. **版图不对齐**：
   - `compensation/benefits` 在当前模块骨架中仍是预留，而不是现有模块

**调查结论 3**：
`functional_area` 与 `module` 不是一套词汇的别名，而是另一套相似但不相同的分类体系。

### 4.4 `owner_module` 被降级成 `functional_area` 的属性，权威关系发生反转

在 `150/157` 里，`owner_module` 被定义为 functional area 词汇表中的一个字段，而不是整个系统的一级归属事实。

这会造成表达上的反转：

- 按 DDD 语义，模块应是边界主语，其他分类应附着其上。
- 按 `150/157` 的表达，functional area 成了主表，module 变成其中一列元数据。

一旦 reviewer 先看到的是 capability catalog / functional area 治理页，就容易把“属于哪个 functional area”误读成“属于哪个模块/业务边界”。

**调查结论 4**：
`150/157` 在表达结构上弱化了 DDD module 的主语地位，提升了第二维度的解释权。

### 4.5 `functional_area` 同时承担了过多职责，导致语义复用过载

当前 `functional_area` 至少同时承担：

1. capability 的产品治理分组
2. 租户级总开关
3. 生命周期冻结
4. 运行时 fail-closed gate
5. UI 治理页面展示分组
6. explain / error reason 的语义来源
7. reviewer 对“这组能力是否同属一域”的解释入口

这些职责中，只有第 1、5 类相对接近“运营/展示分组”；第 2、3、4、6 类已经是强运行时语义。

**调查结论 5**：
问题不只是“多了一层分类”，而是这层分类同时承担了展示、治理、授权前置与生命周期冻结，导致它自然会与模块边界竞争解释权。

### 4.6 `domain_capability` 命名进一步放大了与 DDD 的撞词风险

`150` 同时引入 `domain_capability/process_capability` 与 `StaticContext/ProcessContext`。这里的 `domain` 明显不是 DDD 中的 domain/bounded context，而是在表达 capability 的静态/流程类型。

但对仓库 reviewer 来说，当前系统已同时存在：

- DDD domain / bounded context
- module
- functional area
- domain_capability

这会形成一组高度相近却层次不同的术语。

**调查结论 6**：
`domain_capability` 的命名会把“第二维度问题”从结构层进一步扩散到词汇层。

## 5. 风险判断

### 5.1 结论：`150/157` 确实引入了与 DDD 模块并行的第二套维度

综合第 4 章结论，本调查给出冻结判断：

- `functional_area` 已具备强治理维度的全部特征。
- 它与 DDD module 并不同构。
- 它已进入 contract、route-map、runtime、UI 和错误语义。
- 因此它不是“无害的补充标签”，而是与 DDD module 并行、且会竞争解释权的第二套维度。

### 5.2 混乱风险等级

| 风险 | 等级 | 说明 |
| --- | --- | --- |
| 概念混淆 | 高 | reviewer 容易混淆 `module / functional_area / capability` 三层归属 |
| 命名漂移 | 高 | `org_foundation`、`iam_platform` 这类词汇与模块名不一，长期需要映射记忆 |
| 责任错置 | 高 | `owner_module` 成为 functional area 附属字段，弱化模块主边界 |
| 迁移误判 | 高 | 新能力容易先挂到“最近的 active area”，形成临时归属固化 |
| UI 误导 | 中 | 治理页若直接按 functional area 呈现，用户可能把它理解为正式业务域 |
| 运行时爆炸半径 | 中 | 一个 functional area 关闭会影响整组 capability，若分组不自然会误伤 |
| 词汇学习成本 | 中 | `domain_capability` 等命名继续增加近义术语数量 |

## 6. 建议的收敛方向

### 6.1 原则：模块边界必须重新回到第一主语

后续任何 capability / governance / catalog / UI 计划，都应明确：

- `module / bounded context` 是边界事实源
- `functional_area` 只能是派生治理视角，不能替代模块事实
- 文档、API、UI 若需要同时显示两者，必须显式标注“module 是 owner，functional area 是治理分组”

### 6.2 三种可选收敛路径

#### 方案 A：直接退回模块级开关

- 将当前 `functional_area` 治理能力收敛为 `module_key` 治理。
- capability 直接归属模块，租户总开关也以模块为单位。

**优点**：
- 语义最简单，与 DDD 完全同构。
- 几乎消除第二维度。

**缺点**：
- 如果确实需要比模块更粗或更产品化的治理分组，会损失灵活性。

#### 方案 B：保留 `functional_area`，但降级为只读派生视图

- `functional_area` 只用于 catalog / UI / 运营分组展示。
- 不再持有运行时 fail-closed、唯一归属、租户总开关和生命周期阻断权。
- 真正的运行时 owner 仍是 module 与 capability policy。

**优点**：
- 保留产品视角分组价值。
- 最大幅度降低与 DDD 竞争解释权的风险。

**缺点**：
- 需要从 runtime / contract / route-map 中回收现有功能域权力。

#### 方案 C：保留 `functional_area` 为强治理维度，但强制与模块形成显式从属关系

- 明确 `functional_area` 只能从属于一个 module family，不能跨越多个无关模块。
- 词汇必须尽量与模块同构或可无歧义映射。
- `owner_module` 不能再是 metadata，而应由 module 反向定义其可用的 functional areas。

**优点**：
- 保留现有治理能力与 runtime gate。
- 比彻底回滚改动小。

**缺点**：
- 仍然保留第二维度，只是试图把它驯化。
- 文档与评审复杂度仍高于 A/B。

### 6.3 当前推荐

本调查的推荐顺序是：

1. **优先 B**：若产品上仍需要“能力治理分组”，则把 `functional_area` 降级为派生视图最稳妥。
2. **其次 A**：若治理诉求本质就是模块启停，则直接删掉 `functional_area` 这一层。
3. **最后才考虑 C**：只有在确认运行时必须保留“非模块级治理分组”时，才接受继续保留第二维度，但必须补严从属约束。

原因是：A/B 都在减少第二维度权力，而 C 只是约束它；从 Simple > Easy 角度，减权优于继续补规则。

## 7. 建议的 stopline

后续若继续保留 `functional_area`，必须新增以下 stopline：

- [ ] 任何文档把 `functional_area` 直接表述为 DDD 模块或 bounded context。
- [ ] 新增 `functional_area_key` 与现有模块边界无清晰映射关系，却直接接入 runtime gate。
- [ ] `owner_module` 继续作为 `functional_area` 的附属 metadata，而非由模块事实源反向约束。
- [ ] 新 capability 因“暂时没有合适 area”而挂入某个大而泛的 active area。
- [ ] 在 UI / API / catalog 中只暴露 `functional_area`，却不同时暴露真实 `owner module`。
- [ ] 继续使用 `domain_capability` 这类会与 DDD `domain` 撞词的命名而不解释。

## 8. 建议的后续承接计划

1. 若选择 **A 或 B**：
   - 新开 `T2` 收口计划，覆盖：
     - `contract-freeze.v1.json`
     - `route-capability-map.v1.json`
     - runtime functional area gate
     - capability catalog DTO
     - front-end governance 页面与文案
     - 错误码 / explain 语义
2. 若选择 **C**：
   - 新开 `T2` 约束计划，至少补：
     - module -> functional_area 显式从属契约
     - 词汇表与模块同构规则
     - stopline 与 CI 门禁
     - owner module / functional area 双字段展示规范
3. 无论选哪条路径：
   - 后续涉及 `CubeBox`、Assistant 或新 capability 迁移时，都应先引用本文，再决定是否需要新 functional area 或 capability 重构。

## 9. 验收标准

- [X] 明确回答 `functional_area` 是否已形成与 DDD 模块并行的第二套维度。
- [X] 明确说明这种第二维度为什么会带来混乱，而不是只指出单个命名问题。
- [X] 给出不少于两条可执行的收敛方向，并说明取舍。
- [X] 给出后续计划可以直接复用的 stopline 与承接建议。

## 10. 关联文档

- `docs/dev-plans/015-ddd-layering-framework.md`
- `docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
- `docs/dev-plans/381-cubebox-capability-and-functional-area-lineage-investigation.md`
- `docs/dev-plans/382-capability-functional-area-governance-impact-investigation.md`
