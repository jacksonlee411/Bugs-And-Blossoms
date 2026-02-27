# DEV-PLAN-185：字段配置页字典值列表 SetID 列展示与主数据取数控制策略收敛

**状态**: 规划中（2026-02-27 08:51 UTC）

## 1. 背景

承接 `DEV-PLAN-165/184` 的“字段配置（静态）与策略（动态）”边界收敛，以及 `DEV-PLAN-070B` 的“运行时 tenant-only、共享改发布”口径，当前仍有三个明显缺口：

1. 字段配置相关字典候选/字典值列表缺少 SetID 可见性（用户无法直观看到“当前命中的集合”）。
2. 主数据取数链路对 SetID 的控制在不同入口口径不一：有的入口先 `ResolveSetID` 再取数，有的入口仅按租户取数。
3. `DEFLT / SHARE / 自定义 SetID` 的优先级与覆盖语义分散在历史计划与实现细节中，尚无单页冻结口径。

## 2. 目标与非目标

### 2.1 目标（本计划冻结）

1. [ ] 在字段配置页的字典值列表（含启用候选与值候选展示）增加 `setid` 列/等效可视字段。  
2. [ ] 收敛“主数据取数必须受 SetID 上下文控制”的统一策略（含参数、解析、拒绝语义）。  
3. [ ] 明确 `DEFLT`、`SHARE` 与自定义 SetID 的优先级、覆盖范围与冲突处理。  
4. [ ] 完成对既有 dev-plan 中 SetID 规定的系统化梳理，并沉淀成可执行约束。  

### 2.2 非目标

1. 不在本计划内直接重做全部字典/主数据 Schema。  
2. 不引入 legacy 双链路（包括“旧取数兜底”或“静默 fallback”）。  
3. 不改变 `DEV-PLAN-070B` 已冻结的 tenant-only 运行时原则。  

## 3. 前序计划研究结论（SetID 规定汇总）

| 来源 | 已冻结口径（提炼） | 对 185 的约束 |
| --- | --- | --- |
| `DEV-PLAN-070`（归档） | `DEFLT` 为租户默认、根组织强制绑定；`SHARE` 保留且不可绑定业务组织；`resolve_setid` 按组织祖先就近解析 | 185 不得引入“业务读路径读取 SHARE” |
| `DEV-PLAN-070B` | 运行时 tenant-only；共享通过发布落地租户，不走 global fallback | 185 的主数据取数默认只能走租户数据 |
| `DEV-PLAN-102B` | `as_of/effective_date` 必填，禁止 default today | 185 新增/改造接口必须显式时间参数 |
| `DEV-PLAN-105/105B` | DICT 运行时 tenant-only，缺基线 fail-closed | 185 不能把字典读链路改回跨租户兜底 |
| `DEV-PLAN-106/106A` | DICT 字段启用来自 dict registry，字段键与 dict_code 收敛 | 185 的 SetID 展示需与 `d_<dict_code>` 口径一致 |
| `DEV-PLAN-161/165/184` | 字段配置页偏静态治理，策略页偏动态决策；避免双事实源双写 | 185 要把 SetID 可见性做成“展示+决策上下文”，不制造第二 SoT |
| `DEV-PLAN-182` | 先分桶再比优先级（基线/覆盖不可倒挂） | 185 的 SetID 优先级同样采用“先层级后优先级” |
| `DEV-PLAN-191` | `/app/org/setid` 导航/IA 收敛，治理入口统一 | 185 的交互变更要保持单入口，不新增并行页面 |

## 4. 现状差距（As-Is）

1. `GET /org/api/org-units/field-configs:enable-candidates` 的 `dict_fields` 仅返回 `field_key/dict_code/name`，无 `setid` 维度。  
2. `GET /org/api/org-units/fields:options` 返回 `options[value,label]`，无“命中 SetID”回显，排障与审计成本高。  
3. 主数据取数“按 SetID 控制”的实现样板已存在（如 `positions:options` 先 `ResolveSetID`），但未沉淀为统一契约。  

## 5. 方案设计（To-Be）

### 5.1 字段配置页字典值列表增加 SetID 列

1. [ ] 扩展字段候选/值候选响应，增加：
   - `setid`：当前候选对应的生效 SetID
   - `setid_source`：`custom` / `deflt` / `share_preview`
2. [ ] 前端在字段配置页相关字典列表中展示 `setid`（DataGrid 列或等效明确列位，不做隐式拼接）。
3. [ ] 默认展示口径：
   - 有组织上下文：按 `ResolveSetID(tenant, org_unit_id, as_of)` 回显
   - 无组织上下文：回显 `DEFLT`（治理基线视角）

### 5.2 主数据取数 SetID 控制策略（统一策略标识：`setid_fetch_v1`）

1. [ ] 统一输入上下文：`tenant + as_of + (org_unit_id | business_unit_id) + capability_key(optional)`。  
2. [ ] 统一解析入口：先解析 `resolved_setid`，再执行主数据查询；禁止“查完再猜 setid”。  
3. [ ] 统一拒绝语义：
   - 解析失败：`SETID_BINDING_MISSING` / `SETID_NOT_FOUND` / `SETID_DISABLED`
   - 上下文冲突：`CAPABILITY_CONTEXT_MISMATCH`
4. [ ] 统一回显：所有主数据候选接口返回 `resolved_setid`（或列表项 `setid`）。

### 5.3 `DEFLT/SHARE/自定义 SetID` 优先级与覆盖关系（冻结）

#### 5.3.1 字段级优先级枚举（`priority_mode`，有限枚举）
1. [ ] `blend_custom_first`（默认）：`custom > DEFLT > SHARE`。  
2. [ ] `blend_deflt_first`（集团优先）：`DEFLT > custom > SHARE`。  
3. [ ] `deflt_unsubscribed`（取消订阅 DEFLT）：`custom > SHARE`。  

#### 5.3.2 字段级本地覆盖治理枚举（`local_override_mode`，有限枚举）
1. [ ] `allow`：允许本地（custom）补充与覆盖。  
2. [ ] `no_override`：允许补充，不允许覆盖（同 `code` 时不采纳 local 覆盖）。  
3. [ ] `no_local`：禁止 local 参与运行时求值（等价强管控 deflt-only 语义）。  

#### 5.3.3 `SHARE` 发布与读写约束（冻结）
1. [ ] 发布链路只更新目标租户内 `setid=SHARE` 层字典值，不直接改写 `DEFLT` 或自定义 SetID 层。  
2. [ ] 租户侧 `setid=SHARE` 视图为只读：业务写接口不得新增/修改/停用 `setid=SHARE` 值。  
3. [ ] 租户可在同字段下新增或维护 `DEFLT` / 自定义 SetID 的字典值，用于补充或覆盖 `SHARE` 基线。  

#### 5.3.4 同字段求值规则（`priority_mode + local_override_mode` 组合）
1. [ ] 先按 `priority_mode` 形成层顺序，再按 `local_override_mode` 决定 local 是否可补充/覆盖。  
2. [ ] **补充**：高优先级层新增了低优先级层不存在的 `code`，该值可进入可选集（`no_local` 例外）。  
3. [ ] **覆盖**：高优先级层与低优先级层同 `code` 时，是否允许 local 覆盖由 `local_override_mode` 决定。  
4. [ ] **强管控**：`blend_deflt_first + no_local` 下，运行时等价 `DEFLT > SHARE`。  
5. [ ] 全程保持 `tenant-only`：不引入跨租户/global fallback。  

#### 5.3.5 完备性评估结论（冻结）
1. [ ] 仅“优先级顺序”三枚举在排序维度完备，但不足以覆盖“是否允许 local 覆盖/补充”的治理诉求。  
2. [ ] 采用“双枚举（`priority_mode` + `local_override_mode`）”后，可覆盖组合、集团优先、取消订阅、强管控等场景。  
3. [ ] 仍坚持有限枚举，不引入任意表达式，保证可校验、可测试、可 Explain。  

### 5.4 实施分期

1. [ ] **M1 契约冻结**：冻结字段、错误码、优先级表与 API 口径。  
2. [ ] **M2 接口扩展**：候选接口补 `setid/setid_source` 回显。  
3. [ ] **M3 前端展示**：字段配置页字典列表补 `setid` 列。  
4. [ ] **M4 取数策略统一**：把已存在样板抽象为统一策略函数，覆盖相关入口。  
5. [ ] **M5 验证与证据**：补齐测试与 `docs/dev-records/` 证据。  

## 6. 门禁与验证（SSOT 引用）

按 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 执行；本计划预计命中：

- Go/API：`go fmt ./... && go vet ./... && make check lint && make test`
- Routing/Capability：`make check routing && make check capability-route-map && make check capability-key`
- Legacy 防回流：`make check no-legacy`
- 文档：`make check doc`

## 7. 验收标准

1. [ ] 字段配置页字典值列表可直接看到 `setid` 列，且与后端回显一致。  
2. [ ] 主数据取数入口均遵循“先解析 setid，再取数”单链路。  
3. [ ] `priority_mode` 与 `local_override_mode` 均为有限枚举，非法值 fail-closed。  
4. [ ] `SHARE` 发布仅影响 `setid=SHARE` 层，且租户侧 `setid=SHARE` 为只读。  
5. [ ] 同字段下 `自定义/DEFLT/SHARE` 的补充、覆盖、屏蔽行为有回归用例。  
6. [ ] 不出现 global fallback、silent fallback、legacy 双链路。  

## 8. 风险与缓解

1. **风险：tenant-only 与 setid 细分口径冲突**  
   缓解：保持 070B tenant-only 不变；SetID 先作为“上下文控制与可见性”收敛，不反向引入跨租户读取。
2. **风险：接口扩展导致前后端不兼容**  
   缓解：先补响应可选字段，再升级前端列渲染，分阶段发布。
3. **风险：优先级实现倒挂**  
   缓解：采用“先 `priority_mode` 后 `local_override_mode`”的固定求值流程，并加回归测试。

## 9. 关联文档

- `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/105-dict-config-platform-module.md`
- `docs/dev-plans/105b-dict-code-management-and-governance.md`
- `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md`
- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- `docs/dev-plans/182-bu-policy-baseline-and-intent-override-unification.md`
- `docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md`
- `docs/dev-plans/191-setid-governance-navigation-and-layout-optimization.md`
