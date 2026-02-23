# DEV-PLAN-161：Org 新建表单动态策略落地（org_code + d_org_type，承接 150）

**状态**: 规划中（2026-02-23 14:20 UTC，已根据评审意见修订）

## 1. 背景与上下文 (Context)
- `DEV-PLAN-150` 已完成 Capability Key 治理底座（Registry/Explain/Activation/Functional Area），但 Org 新建表单尚未完整接入该底座。
- 当前用户需求（`00000002` 与 `00000004` 差异化规则）无法同时落地，根因是“治理配置已存在，但写入与表单选项链路未统一消费”。
- 本计划以 `org_code`、`d_org_type` 为样板，把“平台治理能力”变成“用户可见、可操作、可校验”的业务行为。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 在 Org 新建链路实现按 `business_unit/as_of` 的动态默认规则：
  - `org_code`：`00000002 -> next_org_code("F", 8)`；`00000004 -> next_org_code("X", 8)`。
  - `d_org_type`：`00000002 -> default=11、required=true`；`00000004 -> default=10、required=false`。
- [ ] 在 Org 新建链路实现 `d_org_type` 的上下文化可选值约束：
  - `00000002` 仅可选 `11`
  - `00000004` 仅可选 `10`
- [ ] 前后端一致 fail-closed：UI 仅展示允许选项，后端提交再次校验，拒绝码稳定。
- [ ] 统一来源：动态行为由 150 策略底座（Registry + Activation + Explain）驱动，不再散落到多套配置。
- [ ] 决策与写入保持版本一致：创建决策响应返回 `policy_version`，写入时必须回传并做乐观并发校验（防 TOCTOU）。

### 2.2 非目标
- 不改动字典模块的基础 CRUD/发布机制（`/dicts` 现有流程保持）。
- 不引入新的业务模块或新的对外语义主键。
- 不增加 legacy 双链路（禁止“新链路失败回退旧链路”）。

### 2.3 工具链与门禁（SSOT 引用）
- 触发器预计命中：Go、Routing、Authz、DB Schema/迁移、sqlc、文档、E2E、capability-key、capability-route-map、request-code、error-message、no-legacy。
- 以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为 SSOT 执行验证，不在本文复制脚本细节。

## 3. Simple > Easy 约束（对齐 DEV-PLAN-003）
### 3.1 边界（Boundary）
- **字段静态定义**（字段类型/数据源/启停）继续由 `orgunit field configs` 管理。
- **字段动态行为**（required/visible/maintainable/default/allowed_options）统一由 SetID Strategy Registry 决策。
- **写入入口唯一**：Org 新建仍只走 OrgUnit One Door 写入口，不新增旁路。

### 3.2 不变量（Invariants）
- 同一时点同一上下文（tenant + capability_key + business_unit + as_of）决策结果确定且可重放。
- `capability_key` 仅用稳定键，禁止上下文编码与运行时拼接。
- UI 与后端必须共享同一决策结果；任一侧缺上下文均 fail-closed。
- 创建决策响应中的 `policy_version` 与创建写入请求中的 `policy_version` 必须一致；不一致即拒绝（不降级、不自动重试旧版本）。
- 字段默认值采用单一优先级链路（用户输入/规则默认/静态默认/空值），并在“默认值决议后”统一执行 `required` 与 `allowed_value_codes` 校验。

### 3.3 失败路径（Failure Paths）
- 上下文无法解析（如父组织找不到 BU，或 `parent_org_code` 为空且租户级策略缺失）：拒绝并返回稳定错误码（不默认放行）。
- 策略缺失/冲突：拒绝并可 Explain（含 `policy_version/reason_code`）。
- 提交值不在允许集合：`400` 明确错误码（不静默修正）。
- 前端使用过期 `policy_version` 提交：`409` 拒绝（提示刷新决策后重试）。

## 4. 方案总览（Architecture）
### 4.1 统一决策对象（新增）
- 在 Org 新建场景引入“创建表单字段决策”对象（只作为运行态 DTO，不引入第二事实源）：
  - `field_key`
  - `required/visible/maintainable`
  - `default_rule_ref/default_value`
  - `allowed_value_codes`（用于 DICT 选项白名单）
  - `reason_code/policy_version`

### 4.2 上下文解析
- 输入：`effective_date(as_of)` 必填；`parent_org_code` 可选。
- 服务端权威解析（双分支，均 fail-closed）：
  - 有 `parent_org_code`：`parent_org_code -> org_id -> business_unit_id -> setid`。
  - 无 `parent_org_code`（树初始化/根组织创建）：走租户级上下文（`org_level=tenant`，`business_unit_id=''`）解析策略。
- 决策维度：`tenant + capability_key + field_key + business_unit_id + as_of`。

### 4.3 策略存储扩展（在现有 Registry 上增量扩展）
- 复用 `orgunit.setid_strategy_registry`，新增可选字段（不新建表）：
  - `maintainable boolean`（默认 `true`）
  - `allowed_value_codes jsonb`（字符串数组；仅 DICT 字段使用）
- 约束：
  - `allowed_value_codes` 必须是字符串数组且去重。
  - 当字段为 DICT 且配置了 `allowed_value_codes` 时，若 `default_value` 非空则必须命中该集合。

### 4.4 运行链路改造
1. 前端打开“新建组织”弹窗时，调用“创建字段决策 API”。
2. 后端基于 Registry 返回 `org_code` 与 `d_org_type` 决策，并返回本次 `policy_version`。
3. UI 按决策初始化：
   - `org_code` 自动带出规则结果（F/X）。
   - `d_org_type` 自动带出默认值并限制下拉仅显示允许值。
4. UI 提交 `create_org` 时必须携带 `policy_version`。
5. 提交时后端二次校验同一决策 + 校验 `policy_version` 一致性（防绕过/防 TOCTOU）。
6. Explain 可追踪该次决策（含 `policy_version`）。

### 4.5 字段默认值分层策略（评审补充）
#### 4.5.1 默认值优先级决策表（服务端权威）

| 优先级 | 条件 | 结果值 | 说明 |
| --- | --- | --- | --- |
| P0 | `maintainable=false` 且存在 `default_rule_ref` | `evaluate(default_rule_ref)` | 忽略客户端同名字段输入。 |
| P1 | `maintainable=false` 且无规则、`default_value` 非空 | `default_value` | 作为不可维护字段的兜底默认。 |
| P2 | `maintainable=false` 且规则/默认都为空 | 空值 | 后续由 `required` 规则决定是否拒绝。 |
| P3 | `maintainable=true` 且客户端提供非空值 | 客户端值 | 用户显式输入优先。 |
| P4 | `maintainable=true` 且客户端空值、存在 `default_rule_ref` | `evaluate(default_rule_ref)` | 仅在用户未提供值时回填动态默认。 |
| P5 | `maintainable=true` 且客户端空值、无规则、`default_value` 非空 | `default_value` | 仅在用户未提供值时回填静态默认。 |
| P6 | `maintainable=true` 且客户端空值、规则/默认都为空 | 空值 | 保持空值，不做隐式修正。 |

> 统一校验顺序：先决议最终值（P0-P6）→ 再做 `required` 校验 → 再做 `allowed_value_codes` 校验。

#### 4.5.2 空值语义决策表（`d_org_type`）

| 场景 | 空值判定 | 处理规则 |
| --- | --- | --- |
| 请求缺字段 / `null` / `""` / 全空格 | 统一视为“空值” | 进入默认值优先级决议，不做特殊分支。 |
| `required=false` 且最终值为空 | 允许 | 保持为空，不强制回填默认值。 |
| `required=true` 且最终值为空 | 拒绝 | 返回稳定错误码 `FIELD_REQUIRED_VALUE_MISSING`。 |
| 最终值非空且配置了 `allowed_value_codes` | 非空值校验 | 不在集合内返回 `FIELD_OPTION_NOT_ALLOWED`。 |

## 5. 接口与契约（API Contracts）
### 5.1 新增：创建字段决策 API
- `GET /org/api/org-units/create-field-decisions?parent_org_code={code}&effective_date={yyyy-mm-dd}`
- `parent_org_code` 允许为空；为空时按租户级上下文解析。
- 响应字段（最小集）：
  - `capability_key`
  - `business_unit_id`
  - `as_of`
  - `policy_version`
  - `field_decisions[]`（含 `field_key/required/visible/maintainable/default_rule_ref/default_value/allowed_value_codes/reason_code`）

### 5.2 写入校验补强
- Org 新建（`POST /org/api/org-units/write` + `intent=create_org`）写入前，强制验证：
  - 请求必须携带 `policy_version`（由创建字段决策 API 返回）。
  - `policy_version` 必须与服务端当前可用版本一致（不一致返回冲突）。
  - 默认值处理必须遵循 **4.5 默认值分层策略**（后端权威）。
  - `org_code` 是否符合当前上下文默认规则约束（若字段不可维护则禁止手输）。
  - `d_org_type` 约束：
    - `required=true`：默认值决议后必须非空；并在配置 `allowed_value_codes` 时命中集合。
    - `required=false`：允许空值；非空时必须在 `allowed_value_codes` 内，且空值不自动回填默认。

### 5.3 错误码（新增/复用）
- 复用：`FIELD_POLICY_MISSING`、`FIELD_POLICY_CONFLICT`、`capability_context_mismatch`。
- 新增：`FIELD_OPTION_NOT_ALLOWED`（提交值不在允许集合）。
- 新增：`FIELD_REQUIRED_VALUE_MISSING`（默认值决议后仍为空且字段为必填）。
- 新增：`FIELD_POLICY_VERSION_REQUIRED`（缺少策略版本）。
- 新增：`FIELD_POLICY_VERSION_STALE`（提交策略版本过期/冲突，建议刷新决策后重试）。

## 6. 配置样板（本计划样板数据）
> capability_key 固定为稳定键：`org.orgunit_create.field_policy`

| org_level | business_unit_id | field_key | required | maintainable | default_rule_ref | default_value | allowed_value_codes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `business_unit` | `10000001`（org_code=00000002） | `org_code` | true | false | `next_org_code("F", 8)` | - | - |
| `business_unit` | `10000003`（org_code=00000004） | `org_code` | true | false | `next_org_code("X", 8)` | - | - |
| `business_unit` | `10000001`（org_code=00000002） | `d_org_type` | true | true | - | `11` | `["11"]` |
| `business_unit` | `10000003`（org_code=00000004） | `d_org_type` | false | true | - | `10` | `["10"]` |

> 说明：为避免树初始化回归，必须同时提供租户级（`org_level=tenant`）基线策略用于 `parent_org_code` 为空场景；缺失则按 fail-closed 返回 `FIELD_POLICY_MISSING`。

## 7. 分阶段实施（可回归）
1. [ ] **M1 契约冻结**：冻结 capability_key、字段决策 DTO、错误码、迁移约束。
2. [ ] **M2 后端实现**：Registry 扩展 + 决策 API + 写入二次校验 + `policy_version` 乐观并发校验。
3. [ ] **M3 前端接入**：新建弹窗接入决策 API，按决策渲染默认值与可选项。
4. [ ] **M4 解释与审计**：Explain/日志补齐 `policy_version/reason_code`。
5. [ ] **M5 门禁与证据**：按触发器跑绿并沉淀 `docs/dev-records/`。

## 8. 测试与验收标准 (Acceptance Criteria)
- [ ] 在 `00000002` 下新建时：`org_code` 自动按 `F` 前缀生成，`d_org_type` 仅可选且默认 `11`。
- [ ] 在 `00000004` 下新建时：`org_code` 自动按 `X` 前缀生成，`d_org_type` 仅可选 `10` 且默认 `10`。
- [ ] 人工构造非法提交（如 `00000004` 提交 `11`）被后端拒绝并返回 `FIELD_OPTION_NOT_ALLOWED`。
- [ ] `d_org_type required=false` 场景下允许空值提交；若提交非空且不在允许集合，返回 `FIELD_OPTION_NOT_ALLOWED`。
- [ ] `d_org_type required=false` 场景下，用户主动清空后保持空值写入，不被默认值二次回填。
- [ ] `d_org_type required=true` 且默认值决议后为空时，后端返回 `FIELD_REQUIRED_VALUE_MISSING`。
- [ ] `parent_org_code` 为空（树初始化/根组织创建）时，系统按租户级策略决策；租户级策略缺失时 fail-closed。
- [ ] 同一上下文重复请求结果一致，`policy_version` 可追踪。
- [ ] 使用旧 `policy_version` 提交时返回 `FIELD_POLICY_VERSION_STALE`（HTTP 409）。
- [ ] 无 legacy 回退通道；`make check no-legacy`、`make check capability-key`、`make check error-message` 通过。

## 9. 风险与缓解
- **R1：策略扩展后出现双来源冲突（Field Policy vs Registry）**  
  缓解：新建链路单读 Registry；旧 Field Policy 不再参与新建默认注入。
- **R2：上下文解析失败导致误放行**  
  缓解：缺上下文一律 fail-closed，并输出明确 reason_code。
- **R3：前后端决策不一致**  
  缓解：后端写入时重复验证，前端仅做用户体验优化不作为可信边界。
- **R4：决策获取与提交存在版本竞争（TOCTOU）**  
  缓解：写入请求强制携带 `policy_version`，后端做版本一致性校验，不一致返回冲突并要求刷新决策。

## 10. 依赖与关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/151-capability-key-m1-contract-freeze-and-gates-baseline.md`
- `docs/dev-plans/154-capability-key-m5-explain-and-audit-convergence.md`
- `docs/dev-plans/158-capability-key-m6-policy-activation-and-version-consistency.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
