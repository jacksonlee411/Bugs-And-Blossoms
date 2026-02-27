# DEV-PLAN-184：字段配置与策略规则双层 SoT 收敛方案（Static Metadata vs Dynamic Policy）

**状态**: 规划中（2026-02-27 12:20 UTC，已按 DEV-PLAN-185 双枚举口径与 DEV-PLAN-003“有限枚举”原则完成契约对齐，待实现）

## 1. 背景

承接 `DEV-PLAN-165` 的定位重评与 `DEV-PLAN-161` 的运行时消费改造，当前存在以下高风险点：

1. 字段配置页与 Strategy Registry 均可编辑“默认值/可维护/可选值”等语义，形成双写与认知冲突。
2. 同一字段在同一业务场景可能命中两套事实源，无法保证确定性与可重放。
3. 页面职责边界不清，导致“页面可改但运行时不生效”或“生效但不可解释”。

本计划给出专门收敛方案：以“分层事实源 + 单一运行时决策入口 + 页面主从关系”完成字段治理的一致性落地。

## 2. 目标与非目标

### 2.1 目标

1. [ ] 冻结双层事实源边界：
   - 静态层（Static Metadata SoT）
   - 动态层（Dynamic Policy SoT）
2. [ ] 冻结字段级职责矩阵，消除 `data_source/maintainable/default/allowed` 双入口双写。
3. [ ] 建立统一策略决策流水线（含 `policy_version` 防 TOCTOU）。
4. [ ] 字段配置页改为“静态治理 + 动态镜像”，Strategy 页改为“动态策略唯一写入口”。
5. [ ] 增加门禁与巡检，阻断回流到双事实源。

### 2.2 非目标

1. 不改 One Door 写入模型（仍通过现有事件写入口）。
2. 不新增 legacy 回退分支或灰度双链路。
3. 不在本计划内扩展到 Org 之外模块的全量迁移（先完成 Org 场景收敛）。

## 3. 架构原则（对齐行业最佳实践）

1. **Separation of Concerns**：字段结构治理与上下文策略治理分层，禁止同一语义跨层重复主写。
2. **Policy as Data（可解释）**：运行时策略来自可版本化注册表，决策可 explain。
3. **Deterministic Resolution**：同一 `(tenant, capability_key, context, as_of, field_key)` 结果确定可重放。
4. **Fail-Closed**：缺上下文/缺策略/版本冲突时拒绝，不隐式放行。
5. **Single PDP, Multiple PEPs**：后端策略解析器是唯一 PDP；前端仅做体验增强，不作为信任边界。
6. **Simple > Easy（对齐 DEV-PLAN-003）**：策略模式采用有限枚举，不引入任意表达式与动态脚本。

## 4. 双层 SoT 设计

### 4.1 静态层（Static Metadata SoT）

事实源：Field Config（字段配置）。

负责属性（唯一主写）：
- `field_key`
- `value_type`
- `data_source_type`
- `data_source_config`
- `enabled_on/disabled_on`
- 展示元数据（label、排序、列表可筛选/可排序能力等）

语义：定义“这个字段是什么、从哪里取候选值、能否被渲染”。

### 4.2 动态层（Dynamic Policy SoT）

事实源：Strategy Registry（按 capability/context 生效）。

负责属性（唯一主写）：
- `required`
- `visible`
- `maintainable`
- `default_rule_ref`
- `default_value`
- `allowed_value_codes`
- `priority_mode`（有限枚举：`blend_custom_first` / `blend_deflt_first` / `deflt_unsubscribed`）
- `local_override_mode`（有限枚举：`allow` / `no_override` / `no_local`）
- `effective_date/end_date` + `policy_version`

语义：定义“在当前能力上下文下，这个字段如何判定与约束”。

## 5. 字段职责矩阵（冻结）

| 语义项 | 主事实源 | 字段配置页 | Strategy 页 | 运行时是否消费 |
| --- | --- | --- | --- | --- |
| `data_source_type/config` | Static Metadata | 可写 | 只读 | 是（候选来源） |
| `required/visible` | Dynamic Policy | 只读镜像 | 可写 | 是 |
| `maintainable` | Dynamic Policy | 只读镜像 | 可写 | 是 |
| `default_rule_ref/default_value` | Dynamic Policy | 只读镜像 | 可写 | 是 |
| `allowed_value_codes` | Dynamic Policy | 只读镜像 | 可写 | 是 |
| `priority_mode/local_override_mode` | Dynamic Policy | 只读镜像 | 可写 | 是（取值排序与覆盖治理） |
| label/排序/展示 | Static Metadata | 可写 | 只读 | 是（UI 呈现） |

冻结约束：
1. `allowed_value_codes` 必须是静态数据源候选集的子集。
2. `maintainable=false` 时必须存在 `default_rule_ref` 或 `default_value`，否则拒绝发布。
3. 任何页面都不得出现同语义双写保存按钮。
4. `priority_mode/local_override_mode` 仅允许有限枚举；非法值必须 fail-closed。
5. 组合合法性固定并可回归：
   - `blend_custom_first` + (`allow`/`no_override`/`no_local`)
   - `blend_deflt_first` + (`allow`/`no_override`/`no_local`)
   - `deflt_unsubscribed` + (`allow`/`no_override`)（`no_local` 视为非法组合）

## 6. 统一决策流水线（运行时）

1. 解析上下文：`tenant + capability_key + business_unit + as_of + field_key`。
2. 读取静态层：字段定义与数据源候选集。
3. 读取动态层：命中策略（含 `policy_version`）。
4. 应用治理枚举：
   - 先按 `priority_mode` 形成层顺序
   - 再按 `local_override_mode` 决定 local 是否可补充/覆盖/参与
5. 合并决策：
   - 值来源：用户输入 / 动态默认 / 空值（遵循既有优先级契约）
   - 校验顺序：`required` -> `allowed_value_codes` -> maintainable 约束
6. 写入前复核：`policy_version` 一致性校验；不一致返回冲突。
7. Explain 输出：返回命中规则、拒绝原因、最终决策快照（含 `priority_mode/local_override_mode`）。

## 7. 页面信息架构与交互规范

### 7.1 字段配置页（重定位）

1. [ ] 保留静态治理编辑能力。
2. [ ] 动态项仅展示“只读镜像（来源：Strategy Registry）”。
3. [ ] 增加“跳转 Strategy”动作，自动携带 `capability_key + field_key + as_of` 过滤条件。
4. [ ] 明确角标：
   - `Static`（本页可维护）
   - `Dynamic`（策略页维护）

### 7.2 Strategy 页（重定位）

1. [ ] 作为动态项唯一写入口。
2. [ ] 支持按 `capability_key / org_applicability / business_unit_id / effective_date` 管理版本。
3. [ ] 支持“命中解释预览”（写前可见最终决策）。

## 8. API 与数据契约收敛

1. [ ] 新增/改造“动态镜像查询接口”：字段配置页动态信息统一从 Strategy 读取。
2. [ ] 停止对旧动态策略写口的增量写入（先软封禁，再硬门禁）。
3. [ ] 为写入链路统一要求 `policy_version`（缺失或过期拒绝）。
4. [ ] 错误码统一：
   - `FIELD_POLICY_MISSING`
   - `FIELD_POLICY_CONFLICT`
   - `FIELD_POLICY_VERSION_REQUIRED`
   - `FIELD_POLICY_VERSION_STALE`
   - `FIELD_OPTION_NOT_ALLOWED`
   - `FIELD_POLICY_PRIORITY_MODE_INVALID`
   - `FIELD_POLICY_LOCAL_OVERRIDE_MODE_INVALID`
   - `FIELD_POLICY_MODE_COMBINATION_INVALID`

## 9. 迁移与收口里程碑

1. [ ] **M1 契约冻结**：完成职责矩阵、接口契约、错误码矩阵冻结。
2. [ ] **M2 页面收敛**：字段页动态项只读化 + 跳转联动。
3. [ ] **M3 API 收敛**：镜像读取统一改为 Strategy；旧动态写口禁增量。
4. [ ] **M4 数据迁移**：把历史动态策略从旧入口迁移到 Strategy，生成差异报告。
5. [ ] **M5 运行时一致性**：create/add/insert/correct 四类场景接入同一决策器。
6. [ ] **M6 门禁固化**：CI 阻断双写回流并沉淀 `docs/dev-records/` 证据。

## 10. 门禁与验证（入口引用）

按 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 执行，不在本文复制脚本细节。预计触发：

- Go/API 变更：`go fmt ./... && go vet ./... && make check lint && make test`
- Routing/Capability：`make check routing && make check capability-route-map && make check capability-key`
- Legacy 防回流：`make check no-legacy`
- i18n：`make check tr`
- 文档：`make check doc`

## 11. 验收标准

1. [ ] `data_source`、`maintainable`、`default`、`allowed` 不再出现双入口双写。
2. [ ] 字段配置页可清晰区分静态项与动态项，动态项仅镜像展示。
3. [ ] Strategy 页改动可在目标业务场景稳定生效，且 explain 可追踪。
4. [ ] create/add/insert/correct 同字段同上下文命中同一事实源。
5. [ ] `policy_version` 冲突可稳定复现并返回明确错误码。
6. [ ] `priority_mode/local_override_mode` 非法值与非法组合均 fail-closed。
7. [ ] `priority_mode/local_override_mode` 组合矩阵具备回归测试与 explain 证据。
8. [ ] 质量门禁全绿，且无 legacy 回退路径。

## 12. 风险与缓解

1. **历史配置迁移不全**：先做只读镜像与差异巡检，再执行分批迁移。
2. **用户认知成本上升**：页面增加来源标签与跳转联动，不做硬切断。
3. **路由 capability 语义漂移**：把 `scope_key -> capability_key` 映射校验纳入强制门禁。

## 13. 关联文档

- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md`
- `docs/dev-plans/185-field-config-dict-values-setid-column-and-master-data-fetch-control.md`
