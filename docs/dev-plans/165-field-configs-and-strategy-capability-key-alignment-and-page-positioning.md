# DEV-PLAN-165：字段配置页与 Strategy capability_key 的对应关系调查与页面定位重评

**状态**: 规划中（2026-02-25 12:30 UTC）

## 1. 背景与目标
当前仓库同时存在两套“字段策略编辑入口”：
- 页面 A：`/app/org/units/field-configs`（字段配置页）
- 页面 B：`/app/org/setid` -> `Registry`（Strategy capability_key）

用户关心两套能力的关系是否一一对应，以及两个页面应该如何分工，避免“看起来可配、实际不生效”的认知偏差。

本计划聚焦：
1. 明确 `default_value / maintainable / allowed_value_codes / scope` 与字段配置项的映射关系；
2. 识别当前双入口的重叠与漂移点；
3. 给出页面定位重评与收敛路线。

## 2. 调查结论（按问题逐项）

### 2.1 `default_value` 与字段配置“默认值”的关系
**现状结论**：不是同一事实源，且当前运行时优先消费 Strategy Registry。  
- 字段配置页的“默认值”来自 `tenant_field_policies`（`default_mode + default_rule_expr`），不支持独立 `default_value` 字段。  
- Strategy Registry 使用 `default_rule_ref + default_value`。  
- `create_org` 写入链路在 store 支持时优先走 `ResolveSetIDStrategyFieldDecision(...)`，仅在不支持时才回退 `ResolveTenantFieldPolicy(...)`。

**影响**：在字段配置页改“默认值”，不等价于修改当前新建组织运行时默认值。

### 2.2 `maintainable` 与字段配置“可维护”的关系
**现状结论**：语义接近，但来源不同、约束不同。  
- 两边都有 `maintainable`，都表达“是否允许用户手填”。  
- 字段配置页来源：`tenant_field_policies`。  
- Strategy 页来源：`setid_strategy_registry`。  
- 约束差异：
  - 字段策略（field-policies）要求 `maintainable=false` 时 `default_mode` 必须为 `CEL`；
  - Strategy 要求 `maintainable=false` 时至少有 `default_rule_ref` 或 `default_value`。

**影响**：同名字段可能出现两套“可维护”配置结果不一致。

### 2.3 `allowed_value_codes (csv)` 与字段配置“数据源配置”的关系
**现状结论**：两者是“上游数据源”与“运行时白名单子集”的关系，不是一一映射。  
- 字段配置的数据源配置（`data_source_config`）定义候选来源（如 `dict_code`）。
- Strategy 的 `allowed_value_codes` 定义在该 capability/context 下可提交的值集合。  
- 前端创建弹窗对 DICT 字段执行“交集策略”：先拉字段配置来源选项，再按 `allowed_value_codes` 过滤；后端提交时再做一次白名单校验。

**影响**：字段配置决定“可从哪里取值”，Strategy 决定“在当前能力上下文允许哪些值”。

### 2.4 字段配置“策略作用域”与 Strategy capability_key 的对应关系
**现状结论**：是“语义近似映射”，不是结构化同字段映射。  
- 字段配置作用域：`scope_type + scope_key`（`GLOBAL/FORM` + 固定 form key）。
- Strategy 作用域：`capability_key + org_level + business_unit_id + effective_date`。  
- 当前仅 `orgunit.create_dialog` 有运行时 capability 对应（`org.orgunit_create.field_policy`）。
- `add_version / insert_version / correct` 等 form scope 尚未形成对应 capability 的统一运行时消费闭环。

**建议映射口径（冻结建议）**：
- `FORM + orgunit.create_dialog` -> `capability_key=org.orgunit_create.field_policy`
- `GLOBAL` -> 同 capability 下的 `org_level=tenant,business_unit_id=''` 兜底策略

### 2.5 其他对应关系（补充）
1. **共同键**：两侧都以 `field_key` + 生效日（day 粒度）为核心维度。  
2. **时间字段语义可对齐**：`enabled_on/disabled_on` 与 `effective_date/end_date` 均是有效期区间模型。  
3. **权限定位不同**：字段配置页入口是 `orgunit.admin`；SetID 页面可读入口更宽（`orgunit.read`），写操作再由 `setid.governance.manage` 限制。  
4. **路由 capability 映射存在语义漂移风险**：SetID Strategy Registry 路由当前绑定的 capability_key 与 Org 新建实际消费 key 不一致，易造成治理认知混淆。  
5. **运行时覆盖面差异**：Strategy 当前已接入 `create_org` 的关键字段决策；字段配置策略页仍呈现多 scope 编辑能力，但并非全部进入同一运行时链路。

## 3. 两个页面定位重评（结论）

### 3.1 现状问题
- 两个页面都能改“默认/可维护”类信息，用户难以判断哪个会生效。
- 字段配置页承载了“静态元数据 + 动态策略”双角色，职责混叠。
- Strategy 页是 capability/context 驱动的运行时策略中心，但与字段配置页缺少明确主从关系。

### 3.2 重定位建议（冻结）
**页面 A（字段配置页）定位**：字段元数据与数据源治理页（Static Metadata）。  
- 负责：字段启停、类型、数据源配置、展示标签、排序筛选能力。  
- 不再作为运行时动态策略主写入口。

**页面 B（SetID Strategy Registry）定位**：能力上下文策略治理页（Dynamic Policy）。  
- 负责：`maintainable/default_rule_ref/default_value/allowed_value_codes/required/visible` 等运行时判定项。  
- 负责：按 `capability_key + org_level + business_unit + effective_date` 管理生效版本。

**页面联动原则**：
- 字段配置页展示“策略摘要”可保留，但应标注“来源：Strategy Registry（只读镜像）”。
- 从字段配置页跳转到 Strategy 页时，自动带 `capability_key + field_key` 过滤参数。

## 4. DEV-PLAN-165 实施方案

### 4.1 目标
- [ ] 冻结“字段元数据 SoT”与“动态策略 SoT”的双层边界。
- [ ] 消除“同一语义可在两页双写”的长期漂移风险。
- [ ] 建立 `scope_key -> capability_key` 明确映射表并纳入门禁。

### 4.2 非目标
- 不在本计划内重写 Org 全量写场景（聚焦页面职责与策略来源收敛）。
- 不引入 legacy 双链路保活。

### 4.3 分阶段里程碑
1. **M1（契约冻结）**
   - 冻结字段：
     - 静态层：`field_key/value_type/data_source_type/data_source_config/...`
     - 动态层：`maintainable/default_rule_ref/default_value/allowed_value_codes/...`
   - 冻结 `scope_key -> capability_key` 映射清单（至少覆盖 create/add/insert/correct）。

2. **M2（页面收敛）**
   - 字段配置页将动态策略改为只读镜像（来源标识清晰）。
   - 新增“一键跳转 Strategy”入口并自动带筛选条件。

3. **M3（API 收敛）**
   - 新增或改造“策略镜像查询 API”，统一从 Strategy 读取动态策略。
   - 对 `tenant_field_policies` 的写入口设定退役路径（先禁增量、后迁移）。

4. **M4（运行时一致性）**
   - 将非 create 的相关 scope（add_version/insert_version/correct）接入 capability 决策链路。
   - 保证同一字段在同一场景只命中一套策略事实源。

5. **M5（门禁与证据）**
   - 增加一致性检查：禁止同语义策略在两套 SoT 并行生效。
   - 补齐 E2E：字段配置改数据源 -> Strategy 改白名单 -> 业务表单可见/可选/提交一致。

## 5. 验收标准
- [ ] 用户在任一页面都能明确看到“该配置是否影响运行时、来源是哪套 SoT”。
- [ ] `default/maintainable/allowed_value_codes` 不再出现双入口双写。
- [ ] `scope_key` 与 `capability_key` 映射可查询、可测试、可门禁。
- [ ] create/add/insert/correct 的策略消费路径口径一致。

## 6. 风险与缓解
- **R1：短期认知切换成本**  
  缓解：字段配置页保留镜像展示与跳转，不做“硬删除入口”式突变。

- **R2：历史策略迁移不完整**  
  缓解：先做只读镜像 + 差异巡检，再执行逐字段迁移与门禁阻断。

- **R3：路由 capability 语义不一致**  
  缓解：把路由 capability 映射校正纳入 M1/M5 的强制检查项。

## 7. 关联文档
- `docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/161a-setid-capability-registry-editable-and-maintainable.md`
- `docs/dev-plans/164-org-type-policy-control-gap-analysis.md`
