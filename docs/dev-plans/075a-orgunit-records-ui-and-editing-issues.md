# DEV-PLAN-075A：OrgUnit 记录新增/插入 UI 与可编辑字段问题记录

**状态**: P0 已完成（2026-02-06 23:05 UTC）；P1（负责人变更/多字段同日更新）待内核能力

## 背景与问题
- 当前 OrgUnit 详情页的“新增记录”功能只允许修改名称，未覆盖所有应支持的可修改字段。
- “插入记录”和“修正记录”入口在同一页面区域混杂，造成操作语义不清晰。
- 根据当前页面截图与现有实现，用户容易把“新增版本（append/insert）”与“历史更正（correction）”混为一类操作，导致误操作风险。

## 目标
- 明确问题范围，形成可执行的修复方向与验收口径。
- 将“新增/插入/修正/删除”明确分层，降低误操作与认知负担。
- 对齐有效期（Valid Time，day 粒度）与事件语义（新增版本 vs 更正历史）。
- 分区模型的规则口径与 `DEV-PLAN-075` 完全一致，避免 UI 与后端校验漂移。

## 当前不合理点（基于现状）
1. **操作语义混杂**
   - 新增、插入、删除在同一层按钮并列，修正未提供独立入口。
   - 用户无法快速理解哪些操作会“新增版本”，哪些会“改写当前版本”。
2. **默认日期不稳定**
   - 新增记录默认日期可能与当前版本同日，容易触发冲突。
   - 行业惯例中“新增”应默认 `max_effective_date + 1 day`。
3. **字段覆盖不足**
   - 记录操作表单字段少于编辑区字段，导致“新增后再修正”的二次操作。
4. **约束信息后置**
   - 日期合法区间（min/max）与冲突提示未前置展示，用户在提交后才知道失败。
5. **时间语义不够显式**
   - 树日期（`tree_as_of`）与记录日期（`effective_date`）同时出现，但命名与说明不足，容易混淆。
6. **危险操作暴露过早**
   - 删除记录与常规操作并列，缺少分区与更强确认。

## 分区模型规则（对齐 DEV-PLAN-075）

### 0) 公共约束（全部操作共用）
- 日期格式必须为 `YYYY-MM-DD`。
- 不允许与已有记录同日冲突（`EVENT_DATE_CONFLICT`）。
- 不得跨越相邻记录（受控回溯，不允许越过上一条或下一条）。
- 上级组织在目标日期必须有效，且目标日期不得早于上级最早生效日。

### A) 新版本操作区（新增版本，不改写历史）

#### A1. 新增记录（add_record）
- 规则：仅允许在当前最晚记录之后新增。
- 合法条件：`effective_date > max_effective_date`。
- 默认值：`max_effective_date + 1 day`。
- UI 提示：`新增记录日期需晚于 {max}`。

#### A2. 插入记录（insert_record）
- 规则：默认继承“所选记录”字段，可在区间内向前或向后插入。
- 设所选记录日期为 `selected`，相邻记录为 `prev`（前一条）/`next`（后一条）：
  - 若存在 `next`：
    - 可选日期需落在“所选记录期间范围”内；
    - 若存在 `prev`，则 `effective_date` 必须满足 `prev < effective_date < next`；
    - 若不存在 `prev`（所选为最早），则 `effective_date < next`；
    - 且 `effective_date != selected`。
  - 若不存在 `next`（所选为最晚）：
    - 视同新增；必须 `effective_date > max_effective_date`。
- 默认值：`selected + 1 day`；若“最晚记录插入”，默认按新增规则（`max+1`）。
- UI 提示：
  - 区间插入：`插入日期需介于 {min} 与 {max} 之间（可早于所选记录）`。
  - 最晚插入：`当前为最晚记录，插入视同新增，日期需晚于 {max}`。

### B) 历史修正区（correction，历史纠错）
- 规则：对“目标记录（`target_effective_date`）”做受控回溯/前移，允许区间内向前或向后调整。
- 设目标记录相邻记录为 `prev`/`next`：
  - 若 `prev` 存在：新日期必须 `> prev`；否则下界单侧无界。
  - 若 `next` 存在：新日期必须 `< next`；否则上界单侧无界。
  - 同时仍需满足公共约束（同日冲突、上级有效性）。
- 默认值：当前选中版本日期（即 `target_effective_date`）。
- UI 提示：`更正仅允许在 {min} ~ {max} 之间`（单侧无界时展示为“≤ {max}”或“≥ {min}”）。

### C) 危险操作区（删除/停用）
- 删除/停用独立于常规按钮区，采用折叠区 + 二次确认。
- 不允许删除最后一条记录（沿用现有后端约束）。
- 提示文案：`删除后不可恢复，请确认本次停用符合业务要求。`

## 分步流程（分区内统一）
- **步骤 1：选择操作意图**（新增 / 插入 / 修正 / 删除）
- **步骤 2：选择生效日期**（实时展示允许区间与冲突校验）
- **步骤 3：编辑字段**（字段集与后端允许 patch 对齐）
- **步骤 4：确认提交**（展示“将新增版本”或“将更正历史”的结果摘要）

## 字段覆盖建议（UI 与后端对齐）
| 操作 | 日期字段 | 主要可编辑字段（建议） | 结果语义 |
| --- | --- | --- | --- |
| 新增记录 | `effective_date`（> max） | 变更类型三选一（rename/move/set_business_unit），对应字段单选 | 新增一条最新版本 |
| 插入记录 | `effective_date`（区间内） | 变更类型三选一（rename/move/set_business_unit），对应字段单选 | 在区间内新增版本 |
| 修正记录 | `target_effective_date` + `effective_date`（受控回溯） | name、parent_org_code、manager_pernr、is_business_unit | 更正指定历史版本 |
| 删除记录 | `effective_date` | 无（仅确认） | 停用/删除指定版本（受规则约束） |

## 解决方案（分阶段）

### P0（本轮就做）：新增/插入字段补齐 + 单事件提交
- **表单字段补齐**：新增/插入支持“变更类型”选择（`rename` / `move` / `set_business_unit`），只显示对应字段并默认继承“所选记录”字段。
- **提交策略**：每次仅提交一个事件（`RENAME` / `MOVE` / `SET_BUSINESS_UNIT`），保证与“同日单事件”约束一致。
- **默认值与校验**：沿用 `DEV-PLAN-075` 的日期规则（add: `> max`，insert: 区间内；最晚插入视同新增），前端即时提示，后端强校验。
- **避免隐式更正**：插入最早记录时不再隐式走 correction；修正必须通过独立“历史修正区”入口。

### P1（需新增内核能力）：负责人变更 + 多字段同日更新
- **原因**：当前负责人变更仅允许在 `CREATE` 事件更正中使用，新增/插入记录无法安全表达。
- **方案**：新增 `SET_MANAGER` 事件类型（或等价语义），并评估 `UPDATE` 类多字段事件以支持“同日多字段”一次提交。
- **过渡策略**：P0 阶段对新增/插入隐藏或只读 `manager_pernr`，多字段同日更新暂不支持。

## 文案建议（首批可落地）
- 新增记录：`在最新记录之后创建新版本，不修改历史记录。`
- 插入记录：`在相邻记录之间创建新版本，不覆盖现有记录。`
- 修正记录：`更正当前选中版本（历史纠错），请谨慎操作。`
- 删除记录：`删除后不可恢复，请确认本次停用符合业务要求。`
- 日期提示：`允许日期范围：{min} ~ {max}`（单侧无界时使用 `≤ {max}` 或 `≥ {min}`）。

## 与当前实现的映射（落地切入点）
- 记录操作按钮区：`internal/server/orgunit_nodes.go`（`renderOrgNodeDetails` 内 `.org-node-records-actions`）。
- 记录操作配置：`internal/server/orgunit_nodes.go`（`recordActionConfig`）。
- 记录动作处理：`internal/server/orgunit_nodes.go`（`action == add_record / insert_record / delete_record` 分支）。
- 日期区间校验（含最早记录回溯插入）：`internal/server/orgunit_nodes.go`（`insert_record` 校验逻辑）。

## 实施步骤（更新）
1. [x] 盘点后端允许字段（新增/插入/修正）与当前 UI 字段差异，形成字段矩阵。
2. [x] 在详情区落地“三分区模型”（新版本操作 / 历史修正 / 危险操作），并按本文件规则计算日期范围。
3. [x] 将记录操作改为“意图 -> 日期 -> 字段 -> 确认”的分步流程（轻量单表单向导）。
4. [x] 新增 `修正记录` 的独立前端入口与后端动作对接，移除“隐式 correction”认知负担。
5. [ ] 统一日期组件展示与提示文案（`tree_as_of` 与 `effective_date` 语义分离）。
6. [x] 补齐单测/E2E（日期边界、冲突、误操作防护）。
7. [x] 记录验收与执行日志（`docs/dev-records/dev-plan-075a-execution-log.md`）。

## 本轮实施结果（2026-02-06）
- 详情页记录操作区改为三分区：新版本操作（新增/插入）、历史修正（入口直达编辑区）、危险操作（删除+二次确认）。
- 新增/插入记录支持 `record_change_type`（`rename` / `move` / `set_business_unit`）并按类型显示对应字段，默认继承当前选中版本字段。
- 后端记录动作改为“单事件提交”：按 `record_change_type` 分别落到 `RenameNodeCurrent` / `MoveNodeCurrent` / `SetBusinessUnitCurrent`。
- 删除“最早记录插入时隐式 correction”分支；最早记录插入遵循区间规则，直接走新增版本语义。
- 日期默认值与提示文案按分区场景更新：新增默认 `max+1`，最晚插入视同新增，区间插入展示上下界提示。
- 已补充并更新 `internal/server/orgunit_nodes_test.go` 覆盖（移除隐式 correction 预期，新增 move/set_business_unit/invalid change type 分支）。
- 记录动作弹层已改为轻量四步向导（意图/日期/字段/确认）：不引入后端草稿、仍单次提交，保持实现简单。
- 已新增 E2E 用例：`e2e/tests/tp060-02-orgunit-record-wizard.spec.js`（覆盖四步向导 add_record + 危险操作区 delete_record 可见性）；本地 `make e2e`（2026-02-06）已通过。

## 验收标准（新增）
- [ ] 用户可在 10 秒内区分“新增/插入/修正”语义（通过页面文案与分区可见性验证）。
- [ ] 分区模型中各操作日期规则与 `DEV-PLAN-075` 一致（抽样覆盖：最早/中间/最晚记录三类场景）。
- [ ] 新增/插入/修正均支持一致的可编辑字段集合，且后端校验通过。
- [ ] 默认日期命中合法区间，不再出现“默认即冲突”的首击失败。
- [ ] 删除动作不与常规动作并列，且具备二次确认。
- [ ] 失败提示指向明确（范围冲突/日期冲突/格式错误/上级无效），不出现“笼统失败”。

## 交付物
- 字段映射与差异清单。
- UI 交互调整方案与最终实现记录。
- 文案与错误提示对照表（中英文可扩展，但当前仅 en/zh）。

## 关联文档
- `AGENTS.md`
- `docs/dev-plans/073-orgunit-crud-implementation-status.md`
- `docs/dev-plans/074-orgunit-details-update-ui-optimization.md`
- `docs/dev-plans/075-orgunit-effective-date-backdating-assessment.md`
- `docs/dev-plans/076-orgunit-version-switch-selection-retention.md`
