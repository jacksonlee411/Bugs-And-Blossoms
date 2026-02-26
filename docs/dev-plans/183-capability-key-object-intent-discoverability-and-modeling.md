# DEV-PLAN-183：Capability Key 配置可发现性与对象/意图显式建模方案

**状态**: 执行中（2026-02-26 14:06 UTC，后端目录化与门禁已验收，UI 验收待补）

## 1. 背景与问题
当前策略生效链路在运行时是可定位的，但在配置侧不可发现，导致用户“知道要配策略，却不知道该选哪个 capability_key”。

### 1.1 运行时如何定位（现状）
- 路由/动作通过注册表映射到 capability_key（代码侧）。
- 写入 intent（create/add/insert/correct）通过后端映射到 capability_key（代码侧）。
- 策略记录仅以 `capability_key` 作为业务语义主键，不显式存对象/表单/流程节点。

### 1.2 配置侧痛点（根因）
- [ ] capability_key 主要以字符串输入/选择，缺少“对象/意图目录”。
- [ ] 下拉候选来源偏“已有策略记录”，不是“全量可配置能力目录”。
- [ ] 缺少“对象/意图 -> capability_key -> 路由/动作”的可解释链路。
- [ ] 能力键承担了“业务语义 + 配置入口定位”双职责，学习成本高。

## 2. 目标
- [ ] 在配置侧显式呈现“对象/意图”，不再要求用户记忆 capability_key 字符串。
- [ ] 建立“对象/意图目录”作为可查询、可测试、可门禁的 SoT。
- [ ] 保留 capability_key 稳定键约束，不破坏现有运行时与审计链。
- [ ] 支持“基线策略 + 场景覆盖”模型下的可解释选择（承接 DEV-PLAN-182）。

## 3. 非目标
- 不改变 RLS/租户隔离/Authz 核心模型。
- 不重构策略决策语义（required/visible/maintainable/default/allowed）。
- 不在本计划直接推进表结构重构（优先目录化与可发现性）。

## 4. 目标态设计（对象/意图显式化）
### 4.1 新增“能力目录视图”（Catalog SoT）
定义统一目录项（建议）：
- `owner_module`：模块归属（如 `orgunit`，与 capability registry 主键口径一致）
- `target_object`：业务对象（如 `orgunit`）
- `surface`：交互面（如 `create_dialog` / `details_dialog` / `api_write`）
- `intent`：动作意图（如 `create_org` / `add_version` / `insert_version` / `correct`）
- `capability_key`：稳定能力键
- `route_class` / `actions`（可选扩展）

约束：
- [ ] 一个 `(target_object, surface, intent)` 必须唯一映射一个 capability_key。
- [ ] 一个 capability_key 可映射多个 route，但业务目录项必须可反查唯一对象/意图归属。
- [ ] `owner_module` 必须与 capability registry 注册值一致，禁止自由输入。

### 4.1.1 字段语义冻结（解决 `module` vs `owner_module` 重复）
- [ ] **主语义字段（SoT）**：`owner_module`、`target_object`、`surface`、`intent`、`capability_key`。
- [ ] **展示字段（派生）**：`module` 仅作为 UI 分组别名，始终由 `owner_module` 派生，不可独立存储/写入。
- [ ] 若 API 同时返回 `module` 与 `owner_module`，必须满足 `module == owner_module`，否则按契约错误处理并阻断发布。
- [ ] 目录与策略写入校验统一使用 `owner_module`，避免双字段导致语义漂移。

### 4.2 配置流程改为“先业务后键名”
UI 目标流程：
1) 先选 `module`
2) 再选 `target_object`
3) 再选 `surface + intent`
4) 自动带出 `capability_key`（默认只读）
5) 高级模式可切换为手工输入（但必须通过目录反查校验）

### 4.3 与 182 的协同
- “应用到全部写场景” -> 选择基线目录项（如 `orgunit_write`）
- “仅当前场景覆盖” -> 选择当前 intent 目录项
- 在策略列表中标记来源：`baseline` / `intent_override`

## 5. 数据与实现策略
### 5.1 实现优先级（Simple First）
第一阶段不引入新库表，采用“代码注册表导出目录”的方式：
- [ ] Go 内 capability 注册结构扩展对象/意图元数据字段（以 `owner_module` 为唯一模块字段）。
- [ ] 同步导出到 JSON 合约（与 route-capability-map 同级治理）。
- [ ] 目录 API 从该注册结构读取，避免双维护。

第二阶段（可选）再评估是否物化为数据库字典表（需单独评审）。

### 5.2 一致性约束
- [ ] 目录项必须在 capability registry 中存在。
- [ ] route-capability-map 的 capability_key 必须能在目录中反查到至少一个对象/意图。
- [ ] 策略写入时 capability_key 必须能反查目录项，否则拒绝。
- [ ] 目录导出中若出现 `module != owner_module` 视为契约漂移，CI 直接失败。

## 6. API 方案（建议）
1. [ ] `GET /internal/capabilities/catalog`
   - 返回全量目录项（含对象/意图维度）。
2. [ ] `GET /internal/capabilities/catalog:by-intent?...`
   - 支持按 module/object/surface/intent 过滤。
3. [ ] 策略写入接口增强校验
   - 若 `capability_key` 无目录映射：`invalid_request`（或新错误码）并拒绝。

## 7. UI 方案（SetID Governance）
1. [ ] 新增“对象/意图模式”配置面板（默认模式）。
2. [ ] capability_key 字段改为自动回填；高级模式可手填并实时校验。
3. [ ] 列表页新增列：`target_object` / `surface` / `intent` / `source_type`。
4. [ ] explain 面板支持按对象/意图反查 capability_key。

### 7.1 用户可见性与入口验收（对齐 AGENTS 3.8）
- [ ] **可发现入口固定**：对象/意图配置能力统一落在 `/org/setid` 治理页，不允许仅通过内部 API 提供。
- [ ] **默认可操作模式**：首次进入即展示“对象/意图模式”，用户无需手输 capability_key 即可完成配置。
- [ ] **高级模式受控**：手工 capability_key 仅作为高级入口，默认折叠；展开后仍必须通过 catalog 反查校验。
- [ ] **结果可见**：列表与 explain 面板都能看到 `target_object/surface/intent` 与实际 `capability_key` 的对应关系。
- [ ] **后端先行约束**：若目录 API 先发布，治理页需提供可见占位（入口已存在但功能标注“即将上线”），并附验收日期，禁止“无 UI 入口”的长期后端裸能力。
- [ ] **E2E 最低验收用例**（至少通过 1 条）：
  - `E2E-183-01`：用户仅按 `owner_module -> target_object -> surface+intent` 选择，自动带出 capability_key 并提交成功；
  - `E2E-183-02`：高级模式手填一个未注册 capability_key，页面实时报错并阻断提交。

## 8. 门禁与测试
### 8.1 新门禁（建议）
- [ ] `make check capability-catalog`  
  校验目录唯一性、反查完整性、与 route-capability-map 同步，以及 `module/owner_module` 一致性（若存在 `module` 字段）。

### 8.2 测试补齐
- [ ] 单测：目录生成、唯一性、反查逻辑。
- [ ] API 测试：目录查询与写入校验失败分支。
- [ ] UI 测试：对象/意图选择 -> capability 自动带出 -> 提交成功。
- [ ] E2E：用户不输入 capability_key 也能完成策略配置并生效。

## 9. 分阶段实施
1. [ ] M1（契约冻结）：定义目录模型、字段、唯一性规则。
2. [ ] M2（后端目录化）：注册结构扩展 + 目录 API + 写入校验。
3. [ ] M3（前端可发现性）：治理页改为对象/意图优先。
4. [ ] M4（门禁）：新增 capability-catalog 检查并接入 CI。
5. [ ] M5（证据收口）：补齐测试与操作手册，完成验收。

## 10. 验收标准
- [ ] 配置策略时不需要记忆 capability_key，可直接按对象/意图选择。
- [ ] 任一策略记录可反查其对象/意图归属。
- [ ] route、capability、catalog 三者一致性可被门禁自动验证。
- [ ] `owner_module` 成为目录唯一模块口径；`module`（若保留）仅为只读派生字段且始终一致。
- [ ] “选不到对象/意图”的问题在治理页消失（可用性验收通过）。

## 11. 风险与缓解
- **风险 1：目录字段设计过重**  
  缓解：M1 仅冻结最小字段（owner_module/target_object/surface/intent/capability_key）。

- **风险 2：与现有 capability 合约重复维护**  
  缓解：目录从同一注册源导出，禁止手工双写。

- **风险 3：高级模式绕过目录**  
  缓解：手工 capability_key 仍强制反查校验，失败即阻断写入。

## 11A. 2026-02-26 验收记录（后端 + 门禁）
- [x] `make check capability-catalog` 通过（目录唯一性/注册与路由契约一致性检查通过）。
- [x] `GET /internal/capabilities/catalog`、`GET /internal/capabilities/catalog:by-intent` 运行态验证通过（`owner_module/target_object/surface/intent` 可反查 `capability_key`）。
- [x] 目录包含 `org.orgunit_write.field_policy` 与四类 intent key，满足 182/183 协同输入要求。
- [x] 策略写入校验已接 catalog：未注册或无法反查的 `capability_key` 会被拒绝（server test 通过）。
- [ ] `/org/setid` “对象/意图模式默认可操作”与“高级模式受控”仍需 UI 专项验收。
- [ ] `E2E-183-01`、`E2E-183-02` 尚未在 E2E 报告中以该编号显式沉淀（需补齐命名与证据链接）。

## 12. 与其他计划关系
- DEV-PLAN-182 负责“基线 + 覆盖”的策略生效语义；
- DEV-PLAN-183 负责“对象/意图可发现性与配置体验”；
- 两者并行推进时，以 183 的目录模型作为 182 UI 的输入来源。

## 13. 关联文档
- `docs/dev-plans/182-bu-policy-baseline-and-intent-override-unification.md`
- `docs/dev-plans/181-orgunit-details-form-capability-mapping-implementation.md`
- `docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`

## 14. 结论补充：从“对象/意图开发实现”到“可配置”的标准环节
### 14.1 标准接入链路（必须经过）
1. [ ] **能力注册**：为新对象/意图定义并注册 `capability_key`（未注册不得写策略）。
2. [ ] **对象/意图映射**：在后端单点建立 `object/surface/intent -> capability_key` 映射，禁止运行时拼接。
3. [ ] **写能力出口**：`write-capabilities` 返回 `capability_key + policy_version`，前端只消费服务端返回值。
4. [ ] **写入门禁**：`org-units/write`（及同类写接口）统一校验 `policy_version`，拒绝 stale。
5. [ ] **策略写入口校验**：策略注册接口必须校验 `capability_key` 合法、已注册、`owner_module` 匹配、禁上下文编码。
6. [ ] **可发现性入口**：治理页应从“能力目录（catalog）”读取候选，而不是仅依赖“已有策略记录”。

### 14.2 “是否每次都要开发”判定
- [ ] **仅新增 BU/租户策略配置**（对象/意图已接入、字段已接入）：不需要开发，只需配置。
- [ ] **新增对象或新增意图**（如新页面/新流程动作）：需要一次性开发接入上述 14.1 全链路后，才能纳入配置。
- [ ] **新增受控字段**（默认值/必填选填/可编辑）但尚未纳入能力返回字段集合：需要开发接入字段映射后，才可配置。

### 14.3 新功能开发强制要求（Capability 同步）
- [ ] 涉及策略治理的新功能，必须同步落地 capability 设计（命名、注册、意图映射、接口输出、写入校验）。
- [ ] 涉及内部 API 路由时，必须同步 route-capability 映射与合约文件。
- [ ] 必须补齐单测/API 测试（至少覆盖：映射正确、未注册拒绝、版本校验、路由合约同步）。

### 14.4 门禁要求（现行 + 本计划新增）
现行门禁（已在 CI）：
- [ ] `make check capability-key`：阻断 capability_key 上下文编码与动态拼接。
- [ ] `make check capability-contract`：阻断 capability 契约漂移。
- [ ] `make check capability-route-map`：阻断路由映射缺失/重复/未注册。
- [ ] `make check granularity`：阻断 `scope_type/scope_key/org_level` 回流。

本计划新增门禁（承接第 8 节）：
- [ ] `make check capability-catalog`：校验“对象/意图目录”唯一性、反查完整性、与 route/capability 一致性。

### 14.5 与 DEV-PLAN-182 的分工澄清
- 182 解决“为什么要一个个表单配”的语义问题：通过“基线 capability + 意图覆盖 capability”实现“默认全 CRUD 生效、差异场景单独覆盖”。
- 183 解决“为什么选不到对象/意图”的可发现性问题：通过目录建模让用户按对象/意图选，而不是记忆 capability_key 字符串。
