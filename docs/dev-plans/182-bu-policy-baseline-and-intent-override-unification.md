# DEV-PLAN-182：BU 策略“全 CRUD 默认生效”与场景覆盖收敛方案

**状态**: 执行中（2026-02-26 14:06 UTC，后端与门禁已验收，UI 验收待补）

## 1. 背景
当前 OrgUnit 策略按意图拆分为多条 capability（`create/add/insert/correct`）。  
这保证了隔离与审计，但也带来配置负担：当用户希望“某 BU 的主数据策略对全部 CRUD 默认生效”时，需要逐场景重复配置，和业务心智不一致。

用户期望是：
- 对某个 BU 配一次“主数据策略”，默认覆盖该对象全部写场景；
- 只有在个别场景存在差异时，才单独覆盖。

## 2. 目标
- [ ] 建立“**基线策略 + 场景覆盖**”模型：一次配置默认覆盖 `create/add/insert/correct`。
- [ ] 保持现有 capability 分层与审计能力，不引入 legacy 双链路。
- [ ] 明确策略解析优先级与冲突规则，保证后端单点裁决。
- [ ] 降低治理页配置成本：支持“一键应用到全部写场景”。
- [ ] 保持 `policy_version` 门禁有效，阻断旧页面提交。

## 3. 非目标
- 不改动租户隔离/RLS/Authz 基础模型。
- 不在本计划中重写 `tenant_field_policies` 历史窗口。
- 不新增“按页面前端自解释”的策略维度。

## 4. 目标模型（冻结建议）
### 4.1 新增基线 capability（L3）
- 建议新增：`org.orgunit_write.field_policy`（owner_module=`orgunit`）。
- 语义：OrgUnit 主数据写入基线策略，默认适用于四类写意图。

### 4.2 保留意图 capability（L3）
- `org.orgunit_create.field_policy`
- `org.orgunit_add_version.field_policy`
- `org.orgunit_insert_version.field_policy`
- `org.orgunit_correct.field_policy`

用途：仅在某意图需要差异化时作为覆盖层，不再要求每次都配置。

## 5. 解析优先级（后端唯一 SoT）
同一 `tenant + field_key + as_of + BU上下文` 下，按以下顺序命中第一条有效策略：
1. [ ] 意图 capability + `org_applicability=business_unit`
2. [ ] 基线 capability + `org_applicability=business_unit`
3. [ ] 意图 capability + `org_applicability=tenant`
4. [ ] 基线 capability + `org_applicability=tenant`
5. [ ] 仍未命中则返回 `FIELD_POLICY_MISSING`（fail-closed）

说明：
- 不改变现有字段决策语义（`required/visible/maintainable/default/allowed_value_codes`）。
- BU 策略优先于租户兜底；场景覆盖优先于基线默认。

### 5.1 与 `priority` 的关系（澄清）
- `priority` 仍保留在 `setid_strategy_registry` 中，用于**同一 capability 桶内**的冲突消解，不承担“意图 vs 基线”层级裁决。
- 本计划第 5 节定义的是**跨 capability 桶**优先级（intent 覆盖层优先于 baseline 默认层）。
- 实现时采用两层排序：
  1) 先按第 5 节链路确定命中桶（intent/baseline + business_unit/tenant）。
  2) 再在命中桶内按 `priority DESC, effective_date DESC`（再加稳定 tie-break）选最终记录。
- 禁止把四类桶混在一起只按 `priority` 直排，否则会出现 baseline 高优先级反压 intent 的语义倒挂。
- 因此二者不冲突：`capability_key` 作为能力语义主键，`priority` 仅是桶内决议参数；后端单点解析仍是唯一 SoT。

## 6. policy_version 一致性设计
为避免“只校验意图版本却漏掉基线变更”，新增组合版本口径：
- [ ] `effective_policy_version`（建议）：由“意图 capability active 版本 + 基线 capability active 版本”稳定拼接（或哈希）生成。
- [ ] `write-capabilities` 返回：
  - `capability_key`（当前意图 capability）
  - `baseline_capability_key`（基线 capability）
  - `policy_version`（`effective_policy_version`）
- [ ] `org-units/write` 使用同口径校验 `policy_version`，保持 `FIELD_POLICY_VERSION_REQUIRED/STALE` 错误码契约。

### 6.1 组合版本编码规范（冻结建议）
- [ ] 算法标识固定：`epv1`（后续若升级算法，必须新增 `epv2`，禁止覆盖解释）。
- [ ] 版本材料（最小集，键名固定）：
  - `intent_capability_key`
  - `intent_policy_version`
  - `baseline_capability_key`
  - `baseline_policy_version`
- [ ] 规范化规则：
  1. 字段按固定键序列化为 canonical JSON（无多余空白、键序稳定）；
  2. 对 `null/空` 统一写为 `""`（避免语言实现差异）；
  3. `effective_policy_version = "epv1:" + sha256hex(canonical_json)`。
- [ ] API 可选返回调试字段（只读）：`intent_policy_version`、`baseline_policy_version`、`policy_version_alg=epv1`，用于定位 stale 根因。

### 6.2 向后兼容窗口（冻结建议）
- [ ] **双口径兼容窗口**：`2026-03-01` 至 `2026-04-30`（UTC）。
- [ ] 兼容规则：
  1. 请求携带 `epv1:*` 且匹配 active 组合版本：通过；
  2. 请求携带旧“意图单版本”仅在 `baseline_policy_version=""` 时通过；
  3. 一旦存在有效 baseline 版本，旧“意图单版本”一律按 stale 拒绝。
- [ ] **硬切换日期**：`2026-05-01`（UTC）起，写接口仅接受 `epv1:*` 组合版本。

## 7. API / UI 改造范围
### 7.1 后端
- [ ] capability 注册表补入 `org.orgunit_write.field_policy`（Go + JSON 合约）。
- [ ] 策略解析器支持“基线+覆盖”优先级链（服务层/registry store 单点实现）。
- [ ] `write-capabilities` 响应扩展基线信息与组合版本。
- [ ] `org-units/write` 统一按组合版本校验。
- [ ] 策略写入接口对“无差异意图覆盖”返回 `FIELD_POLICY_REDUNDANT_OVERRIDE`（避免重复配置回流）。

### 7.2 前端
- [ ] OrgUnit Details/OrgUnits 创建页：提交使用新的 `policy_version`（组合版，来自 write-capabilities）。
- [ ] SetID Governance：
  - 提供“应用到全部写场景”快捷动作（写入基线 capability）；
  - 提供“仅当前场景覆盖”选项（写入意图 capability）；
  - 在列表中标记来源：`baseline` / `intent_override`。

### 7.3 用户可见性与入口验收（对齐 AGENTS 3.8）
- [ ] **可发现入口固定**：从主导航 `org-setid` 进入 SetID 治理页，路由固定为 `/org/setid`（禁止隐藏为仅直链页面）。
- [ ] **可操作入口固定**：在治理页策略表单中必须同时提供：
  - “应用到全部写场景”（baseline）
  - “仅当前场景覆盖”（intent override）
- [ ] **结果可见**：策略列表必须展示 `source_type`（`baseline` / `intent_override`），用户可直接确认命中来源。
- [ ] **后端先行约束**：若 M2 先于 M3 上线，治理页必须给出“基线已生效、UI 快捷入口待发布”的占位提示与预计版本，不得无入口沉默发布。
- [ ] **E2E 最低验收用例**（至少通过 1 条）：
  - `E2E-182-01`：在 `/org/setid` 配置 baseline 后，不做 intent 配置，`create/add/insert/correct` 至少一条写链路可成功并命中 baseline；
  - `E2E-182-02`：仅对 `correct` 配置 intent override，验证 `correct` 命中 override，其余 intent 继续命中 baseline。

## 8. 迁移策略
1. [ ] M1 契约冻结：新增基线 capability 与解析优先级，不改历史数据。
2. [ ] M2 运行时支持：先上解析链与组合版本（兼容旧数据）。
3. [ ] M3 UI 收敛：治理页默认写基线，差异才写覆盖。
4. [ ] M4 数据收敛：提供巡检脚本，识别“可下沉到基线”的重复策略。
5. [ ] M5 门禁补强：阻断新增“全量重复配置但无差异”的反模式。

### 8.1 “无差异覆盖”门禁规格（冻结建议）
- [ ] 新增检查：`make check policy-baseline-dup`（接入 `make preflight`）。
- [ ] 输入：当前策略快照（按 tenant/org_applicability/business_unit/field/effective window 展开）+ 解析器同口径结果。
- [ ] 判定规则（机检）：
  1. 在同一上下文窗口中，若 intent 桶命中结果与 baseline 桶命中结果的**最终字段决策**完全一致（`required/visible/maintainable/default_rule_ref/default_value/allowed_value_codes`），则判定为冗余覆盖；
  2. 冗余覆盖在 CI 与服务端写入均阻断（统一错误语义：`FIELD_POLICY_REDUNDANT_OVERRIDE`）。
- [ ] 说明：`priority` 仅用于桶内决议，不可作为“intent 与 baseline 语义不同”的豁免条件。

## 9. 验收标准
- [ ] 业务上“某 BU 一次配置”即可在四类写场景默认生效。
- [ ] 某一场景配置覆盖后，仅该场景生效，其余场景继续走基线。
- [ ] `write-capabilities` 可解释命中来源（基线或覆盖）与组合版本。
- [ ] `org-units/write` 能正确拦截陈旧 `policy_version`。
- [ ] 解析器验证“先分桶后比 `priority`”语义：intent 记录存在时不得被 baseline 仅凭更高 `priority` 覆盖。
- [ ] 兼容窗口内旧“意图单版本”仅在无 baseline 生效时可通过；`2026-05-01`（UTC）后仅接受 `epv1:*` 组合版本。
- [ ] `make check policy-baseline-dup` 能阻断“intent 与 baseline 无差异”的重复配置。
- [ ] `make check capability-route-map`、相关 server tests、关键 e2e 通过。

## 10. 风险与缓解
- **风险 1：组合版本口径引入理解成本**  
  缓解：在 API 返回中显式给出意图版本与基线版本（可选扩展字段），并在 UI 提示来源。

- **风险 2：历史重复策略导致行为不易预测**  
  缓解：上线前执行“命中优先级巡检报告”，先可视化再收敛。

- **风险 3：前后端混部窗口导致版本误判**  
  缓解：保持错误码不变，前端先兼容新字段，后端再切换严格校验。

## 10A. 2026-02-26 验收记录（后端 + 门禁）
- [x] `make check policy-baseline-dup` 通过；`make check capability-route-map` 通过。
- [x] `write-capabilities` 返回 `baseline_capability_key=org.orgunit_write.field_policy`，`policy_version_alg=epv1`，`policy_version` 为 `epv1:*`。
- [x] `setid-strategy-registry` 冗余 override 写入被 `422 FIELD_POLICY_REDUNDANT_OVERRIDE` 阻断；非冗余 override 可成功写入。
- [x] E2E 回归通过：`make e2e`（8/8 passed，执行时间约 13.5s）。
- [ ] `/org/setid` 页面“应用到全部写场景 / 仅当前场景覆盖 / source_type 可视化”仍需 UI 专项验收。
- [ ] `E2E-182-01`、`E2E-182-02` 用例尚未在报告中以该编号显式沉淀（需补齐命名与证据链接）。

## 11. 关联文档
- `docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md`
- `docs/dev-plans/181-orgunit-details-form-capability-mapping-implementation.md`
- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`

## 12. 设计反思拆分说明
“为什么配置 capability_key 时选不到对象/意图”已拆分为独立详细方案：
- `docs/dev-plans/183-capability-key-object-intent-discoverability-and-modeling.md`

本计划（182）继续聚焦“基线 + 场景覆盖”的策略生效语义，不展开配置可发现性的细化设计。
