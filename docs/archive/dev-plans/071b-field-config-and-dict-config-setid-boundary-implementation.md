# [Archived] DEV-PLAN-071B：字段配置/字典配置与 SetID 边界实施方案

**状态**: 规划中（2026-02-20 14:20 UTC；时间参数口径遵循 `STD-002`/`DEV-PLAN-102B`）

## 1. 背景与上下文 (Context)
- **需求来源**：基于 `DEV-PLAN-070` / `DEV-PLAN-071` 的现状评估，明确“字段配置模块、字典配置模块是否应引入集合 ID（SetID）管理”的落地路径。
- **当前事实**：
  - 070 已冻结“配置主数据显式 setid、业务数据按 org_unit 解析 setid”主流程，且强调全模块共享一个 SetID 集合（不按模块拆分）。
  - 071 已引入 `scope_code + package` 订阅机制，解决 SetID 膨胀与组合复用问题。
  - 字段配置（`orgunit.tenant_field_configs` / `tenant_field_policies`）当前是**租户级治理元数据**，未进入 SetID 维度。
  - 字典配置（`iam.dicts` + `iam.dict_value_segments/events`）当前是 **tenant/global 双层**，通过 `as_of` + tenant 覆盖 global 解析。
- **核心矛盾**：若“全部引入 SetID”，会把租户治理元数据过度复杂化；若“全部不引入 SetID”，将无法支撑未来“同租户不同组织差异化字典值方案”。
- **业务价值**：先冻结边界、再按需引入，避免系统复杂度和维护成本失控，同时保留 070/071 的可审计、可回放能力。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 冻结边界：字段配置模块不引入 SetID/Package 维度（保持 tenant 级）。
- [ ] 冻结边界：字典本体（dict registry）不引入 SetID/Package 维度（保持 tenant/global）。
- [ ] 定义“字典值是否纳入 Scope Package”的判定矩阵与触发条件（按 dict_code 分类）。
- [ ] 提供“字典值按需纳入 071 Scope Package”的实施路径（含数据模型、解析链路、权限与门禁）。
- [ ] 不破坏现有用户闭环：`/app/org/units/field-configs` 与 `/app/dicts` 行为保持可用且可解释。
- [ ] 对齐仓库级不变量：One Door / No Tx, No RLS / Valid Time(date) / No Legacy。

### 2.2 非目标
- 不把 `orgunit.tenant_field_configs`、`orgunit.tenant_field_policies` 改造成 SetID 维度配置表。
- 不把 `iam.dicts`（dict_code 本体）改造成 SetID 维度表。
- 不在本计划一次性改造全部 dict_code 为 Scope Package 模式（仅建立机制 + 样板）。
- 不新增“legacy 双链路”（禁止同时长期维护“旧解析+新解析”）。

## 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（勾选本计划命中的项）**：
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] 路由治理（`make check routing`）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [ ] DB 迁移 / Schema（按模块 `make <module> plan/lint/migrate up`）
  - [ ] sqlc（`make sqlc-generate`）
  - [x] 文档（`make check doc`）
- **SSOT 链接**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`、`docs/dev-plans/005-project-standards-and-spec-adoption.md`（`STD-002`）。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 架构图 (Mermaid)
```mermaid
graph TD
  UIField[Field Config UI] --> FieldAPI[/org/api/org-units/field-configs]
  FieldAPI --> FieldTables[tenant_field_configs / tenant_field_policies]

  UIDict[Dict UI] --> DictAPI[/iam/api/dicts*]
  DictAPI --> DictRegistry[iam.dicts]
  DictAPI --> DictValues[iam.dict_value_segments/events]

  BizWrite[业务写入口] --> DictResolver[pkg/dict resolver]
  DictResolver --> ModeCheck[dict_code 绑定模式判定]
  ModeCheck -->|tenant_global| DictValues
  ModeCheck -->|scope_package| ResolveSetID[ResolveSetID]
  ResolveSetID --> ResolveScopePkg[ResolveScopePackage]
  ResolveScopePkg --> DictValues
```

### 3.2 关键设计决策 (ADR 摘要)
- **决策 1：字段配置保持 tenant 级（选定）**
  - 选项 A：为 `tenant_field_configs/policies` 增加 setid/package 维度。缺点：槽位映射、表单策略、审计链全部复杂化。
  - 选项 B（选定）：保持 tenant 级，继续服务“租户内字段治理”，不承担组织差异化职责。
- **决策 2：dict registry 保持 tenant/global（选定）**
  - 选项 A：dict_code 本体随 SetID 变化。缺点：字典本体治理碎片化，跨模块复用能力下降。
  - 选项 B（选定）：dict_code 只表达“字典类型存在性与生命周期”，不承载 SetID 语义。
- **决策 3：仅字典值可按需纳入 Scope Package（选定）**
  - 选项 A：全部 dict 值都纳入 Scope Package。缺点：过度设计，迁移成本高。
  - 选项 B（选定）：按 dict_code 绑定模式分流：`tenant_global`（默认）/`scope_package`（按需）。
- **决策 4：scope_package 模式 fail-closed（选定）**
  - 一旦某 dict_code 声明为 `scope_package`，禁止 tenant/global fallback，缺失订阅直接失败（避免隐式回退）。

### 3.3 SetID 引入判定矩阵（冻结）
| 对象 | 是否引入 SetID 管理 | 结论 | 理由 |
| --- | --- | --- | --- |
| 字段配置（tenant_field_configs） | 否 | 保持 tenant 级 | 属于租户字段治理元数据，不是业务配置集选择 |
| 字段策略（tenant_field_policies） | 否 | 保持 tenant 级 | 作用域为 GLOBAL/FORM，不是 org/setid 作用域 |
| 字典本体（iam.dicts） | 否 | 保持 tenant/global | dict_code 是治理主键，不应与 SetID 耦合 |
| 字典值（iam.dict_value_segments/events） | 条件性 | 按 dict_code 分类 | 仅在确有“同租户跨组织差异化配置”时纳入 Scope Package |

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 边界冻结约束（必须）
- 冻结“不新增”：
  - `orgunit.tenant_field_configs`：禁止新增 `setid/package_id/scope_code` 字段。
  - `orgunit.tenant_field_policies`：禁止新增 `setid/package_id/scope_code` 字段。
  - `iam.dicts` / `iam.dict_events`：禁止新增 `setid/package_id/scope_code` 字段。
- 写入口继续 One Door：
  - 字段配置仅通过 `orgunit.enable_tenant_field_config/disable_tenant_field_config`。
  - 字典仅通过 `iam.submit_dict_event/submit_dict_value_event`。

### 4.2 字典值“按需纳入 Scope Package”的模型（阶段化）
> 本节为 071B 的落地目标模型；若涉及新增列/约束/函数，执行前需在对应子计划确认并按仓库红线审批。

1. **新增 dict_code 绑定模式注册表（SSOT）**
   - 建议函数：`iam.dict_binding_registry()`。
   - 返回：`dict_code`, `binding_mode`, `scope_code`。
   - `binding_mode` 枚举：
     - `tenant_global`：沿用现有 tenant 覆盖 global 规则。
     - `scope_package`：通过 070+071 解析包后再取值。
2. **scope_package 模式所需最小事实**
   - dict 值记录需能追溯到 `package_id`（及 owner tenant），用于审计和回放。
   - 解析链路必须固定 `as_of`，禁止隐式当前日推断。
3. **强约束**
   - `binding_mode=scope_package` 的 dict_code：读取/写入均要求 scope+package 上下文，缺失即失败。
   - `binding_mode=tenant_global` 的 dict_code：不得携带 package 上下文，避免语义混用。

## 5. 接口契约 (API Contracts)
### 5.1 保持不变（边界冻结）
- 字段配置：
  - `GET/POST /org/api/org-units/field-configs`
  - `GET /org/api/org-units/field-configs:enable-candidates`
  - 仍以 `as_of` / `enabled_on` / `field_key` 为主，不新增 setid/package 参数。
- 字典本体：
  - `GET /iam/api/dicts?as_of=...`
  - `POST /iam/api/dicts`
  - `POST /iam/api/dicts:disable`
  - 仍以 tenant/global + `as_of` 口径运行。

### 5.2 新增（仅针对 scope_package 字典值）
- 解析入口（建议）：
  - `iam.resolve_dict_value_source(tenant_uuid, dict_code, as_of_date, org_unit_id)`  
    - 输出 `source_mode`, `package_id`, `package_owner_tenant_uuid`（tenant_global 模式下 package 为空）。
- 业务读取/写入调用约定：
  - 若 dict_code 为 `tenant_global`：沿用现有 `pkg/dict` 行为。
  - 若 dict_code 为 `scope_package`：必须提供 org_unit 上下文并走 `ResolveSetID + ResolveScopePackage`。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 字典值来源解析（统一算法）
1. 输入：`tenant_id`, `dict_code`, `as_of_date`, `org_unit_id(optional)`。
2. 从 `dict_binding_registry()` 获取绑定模式；未知 dict_code fail-closed。
3. `tenant_global`：
   - 忽略/拒绝 package 上下文。
   - 按既有 tenant 优先、global fallback 规则取值。
4. `scope_package`：
   - `org_unit_id` 必填。
   - 调用 `ResolveSetID(tenant, org_unit, as_of)`。
   - 调用 `ResolveScopePackage(tenant, setid, scope_code, as_of)`。
   - 在命中 package 的值集查询；未命中即 `DICT_SCOPE_SUBSCRIPTION_MISSING`。
5. 返回值与来源（用于审计日志）。

### 6.2 字段配置候选加载（保持现状）
1. `enabled_on` 必填。
2. 调用 `GET /iam/api/dicts?as_of=enabled_on` 获取 dict_code 候选。
3. 生成 `d_<dict_code>` 候选，继续写入 `tenant_field_configs`（不引入 SetID）。

## 7. 安全与鉴权 (Security & Authz)
- 字段配置权限保持：
  - `orgunit.admin`（页面与写操作）。
- 字典模块权限保持：
  - `dict.read` / `dict.admin`（不复用 `orgunit.admin`）。
- scope_package 字典值（当启用）：
  - 读取遵循 071 的 scope/package 订阅与 shared-only 读开关约束。
  - 写入遵循 071A 的 owner_setid 可编辑规则（订阅者只读）。
- 继续 No Tx, No RLS：所有读写必须显式事务 + tenant 注入，缺失即拒绝。

## 8. 依赖与里程碑 (Dependencies & Milestones)
### 8.1 依赖
- `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/archive/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/archive/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- `docs/dev-plans/105-dict-config-platform-module.md`
- `docs/dev-plans/105b-dict-code-management-and-governance.md`
- `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md`

### 8.2 里程碑
1. [ ] **M1 边界冻结**：补齐文档与代码注释/契约，明确“字段配置与 dict registry 不引入 SetID”。
2. [ ] **M2 防漂移门禁**：新增检查（或测试）阻断对上述三类表/API 引入 setid/package 参数漂移。
3. [ ] **M3 绑定模式注册**：落地 `dict_binding_registry` 与解析分流（默认 `tenant_global`）。
4. [ ] **M4 scope_package 样板**：选 1 个 dict_code 作为 `scope_package` 试点，跑通 070+071 链路。
5. [ ] **M5 证据闭环**：在 `docs/dev-records/` 落档命令与结果，更新相关 dev-plan 状态。

### 8.3 推荐首批模式划分（初版）
- `tenant_global`（默认）：`org_type`（保持现状，不引入 SetID）。
- `scope_package`（按模块推进）：与 071 `scope_code_registry` 中已定义 scope 对齐（如 `person_education_type`、`person_credential_type` 等），由对应模块子计划逐个落地。

### 8.4 实施步骤（按 PR 拆分）
> 说明：以下为建议拆分；可按依赖合并/细拆，但必须保证“单 PR 单主轴、可验证、可回滚”。

#### PR-071B-1：边界冻结与契约收口（Docs/Contract）
- [ ] 更新并冻结 071B 的“边界不变量、判定矩阵、验收口径”。
- [ ] 在相关计划文档中增加交叉引用（070/071/071A/101/105/105B/106A）。
- [ ] 明确“字段配置与 dict registry 不引入 SetID”作为评审必检项。
- [ ] 门禁：`make check doc`。

#### PR-071B-2：防漂移门禁（Schema/API/测试）
- [ ] 新增防漂移检查（测试或 lint 规则）：
  - 阻断对 `tenant_field_configs` / `tenant_field_policies` / `iam.dicts` 引入 `setid/package/scope_code` 字段漂移。
  - 阻断字段配置 API 合约引入 `setid/package` 入参漂移。
- [ ] 在 CI 入口落地（命中 `make check lint` / `make test` 即可触发）。
- [ ] 门禁：`go fmt ./... && go vet ./... && make check lint && make test`。

#### PR-071B-3：dict 绑定模式注册基座（DB + Store）
- [ ] 新增 `iam.dict_binding_registry()`（默认全量 `tenant_global`；不改变现网行为）。
- [ ] 新增读取封装（store/pkg 层）以获取 dict_code 绑定模式与 scope_code。
- [ ] 补齐单测：未知 dict_code / 非法模式 fail-closed。
- [ ] 门禁：Go + DB（`make iam plan && make iam lint && make iam migrate up`）。

#### PR-071B-4：解析分流引擎（tenant_global no-op + scope_package fail-closed）
- [ ] 在 `pkg/dict` 与服务层加入统一分流：先判绑定模式，再决定解析路径。
- [ ] `tenant_global` 路径保持现状（tenant 优先 + global fallback，不回归）。
- [ ] `scope_package` 路径先实现 fail-closed 框架（缺 org_unit/scope 直接拒绝）。
- [ ] 门禁：Go + 相关集成测试。

#### PR-071B-5：scope_package 数据模型扩展（字典值层）
- [ ] 为 dict 值层增加 package 关联所需字段/约束（仅扩展，不改 dict registry 边界）。
- [ ] 写入口（One Door）补齐 package 上下文校验与审计字段落库。
- [ ] 读取入口支持按 `package_id + as_of` 命中值集。
- [ ] 门禁：DB + Go + 回放相关测试。

#### PR-071B-6：scope_package 样板落地（单 dict_code 试点）
- [ ] 选择 1 个 dict_code（建议 `person_education_type`）切换到 `scope_package`。
- [ ] 串通 `ResolveSetID -> ResolveScopePackage -> DictValues` 全链路。
- [ ] 验证“缺订阅 fail-closed、命中订阅可读写、历史 as_of 可回放”。
- [ ] 门禁：Go / 路由 / Authz / E2E（按触发器命中执行）。

#### PR-071B-7：收口与证据归档
- [ ] 更新 `docs/dev-records/` 执行日志（命令、结果、时间戳、风险与回滚说明）。
- [ ] 回填 071B 勾选项与状态（完成后改为“已完成”）。
- [ ] 文档地图与关联文档二次校验，防止遗漏。
- [ ] 门禁：`make check doc` + 本计划命中的最终 preflight。

## 9. 测试与验收标准 (Acceptance Criteria)
- **单元测试**：
  - [ ] dict 绑定模式分流（tenant_global/scope_package）覆盖完整。
  - [ ] scope_package 缺订阅/包失效/上下文缺失均 fail-closed。
  - [ ] tenant_global 路径保持 tenant 优先 + global fallback，不发生语义回归。
- **集成测试**：
  - [ ] 字段配置页面与 API 在无 SetID 参数下行为不变。
  - [ ] 字典本体 API 行为不变（`as_of` 必填、写入口幂等）。
  - [ ] scope_package 样板在 `as_of` 回放下结果可重现。
- **验收标准**：
  - [ ] 字段配置模块未引入 setid/package 维度漂移。
  - [ ] dict registry 未引入 setid/package 维度漂移。
  - [ ] 至少 1 个 dict_code 完成 `scope_package` 样板并记录证据。
  - [ ] 无 legacy 双链路。

## 10. 运维与监控 (Ops & Monitoring)
- 不引入 Feature Flag（对齐仓库“早期阶段不过度运维”原则）。
- 关键日志补齐：`tenant_id`, `dict_code`, `binding_mode`, `as_of`, `org_unit_id`, `setid`, `scope_code`, `package_id`, `request_code`。
- 回滚策略：环境级停写 + 修复后重试；禁止“旧逻辑回退通道”。

## 11. 关联文档
- `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/archive/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/archive/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- `docs/dev-plans/105-dict-config-platform-module.md`
- `docs/dev-plans/105b-dict-code-management-and-governance.md`
- `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md`
- `AGENTS.md`
