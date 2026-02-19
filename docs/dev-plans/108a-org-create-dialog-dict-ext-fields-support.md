# DEV-PLAN-108A：Org 新建组织弹窗支持 DICT 扩展字段（下拉选择）

**状态**: 规划中（2026-02-19 04:54 UTC）

## 0. 与 DEV-PLAN-108 的关系（范围冻结）

1. 本计划是 `DEV-PLAN-108` 的**子里程碑**，只覆盖 `create_org`（新建组织）弹窗的 DICT 扩展字段可编辑性补齐。  
2. 本计划不声明 `add_version / insert_version / correct` 三个入口已完成 DICT UI 收敛；其余入口继续按 `DEV-PLAN-108` 主计划推进。  
3. 本计划完成后，`DEV-PLAN-108` 的“新建组织支持全字段（core + ext）”目标在 UI 层面补齐 DICT 缺口。

## 1. 背景

在 `http://localhost:8080/app/org/units?page=0&node=1` 的“新建组织”弹窗中，当前只展示 `PLAIN` 类型扩展字段（例如“描述”），未展示已启用的 `DICT` 类型扩展字段。  
这会导致“字段配置已启用，但新建入口不可填写”的交付缺口，违反“新增能力需可发现、可操作”的仓库约束。

## 2. 目标与非目标

### 2.1 目标

1. [ ] 新建组织弹窗展示并支持编辑 `DICT` 扩展字段（下拉选择）。
2. [ ] 选项加载复用现有字段选项 API（`/org/api/org-units/fields:options`），支持关键词检索。
3. [ ] 提交 `write(create_org)` 时将 DICT 值写入 `patch.ext`，并保持现有 `allowed_fields` fail-closed 约束。
4. [ ] 与现有 `PLAIN` 扩展字段并存，不回退当前行为。

### 2.2 非目标

1. 不改动字段配置管理页（`/org/units/field-configs`）的启停与策略流程。
2. 不改动后端 DICT 解析与 `ext_labels_snapshot` 内核语义。
3. 不引入新的写入 API 或 legacy 分支。
4. 不在本计划内实现 `add_version / insert_version / correct` 三个入口的 DICT 输入控件。

## 3. SSOT 与约束引用

- `AGENTS.md`（Contract First / No Legacy / 用户可见性原则）
- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- `docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md`
- `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md`

## 4. 现状与根因（实现口径）

1. 新建弹窗扩展字段列表当前仅筛选 `PLAIN`，`DICT` 被排除，导致不可见。  
2. 前端已有 DICT 选项能力（`getOrgUnitFieldOptions` + `Autocomplete`）可复用，但仅用于列表筛选，不在新建表单使用。  
3. 后端 `write(create_org)` 已支持 `ext` 与 DICT 标签快照解析，因此问题主要在前端渲染与提交拼装层。

## 5. 方案设计

### 5.1 字段分组与渲染

1. [ ] 在 `OrgUnitsPage` 将可编辑扩展字段拆分为 `PLAIN` / `DICT` 两组。
2. [ ] 继续沿用 `isCreateFieldEditable(fieldKey)` 与 `allowed_fields` 门禁，确保 fail-closed。

### 5.2 DICT 输入控件

1. [ ] 新增“新建弹窗专用 DICT 输入控件”（可复用当前 `ExtFilterValueInput` 的查询模式）。  
2. [ ] 控件形态：
   - 使用 `Autocomplete`（单选）；
   - 远程查询 `getOrgUnitFieldOptions({ fieldKey, asOf, keyword, limit })`；
   - 支持 loading、error、清空。

### 5.3 表单状态与提交

1. [ ] `DICT` 字段选中值写入 `createForm.extValues[fieldKey]`（字符串 code）。  
2. [ ] 组装 `extPatch` 时与 `PLAIN` 字段统一合并，仍以 `allowed_fields` 为唯一准入。  
3. [ ] 空值处理维持现状：未填写字段不进入 `patch.ext`。

### 5.4 可用性与提示

1. [ ] 当 DICT 选项加载失败时，控件展示错误文案并禁用提交该字段。
2. [ ] 保持现有“字段不可编辑”提示口径（`org_append_field_not_allowed_helper`）。

## 6. 实施步骤

1. [ ] 更新计划与文档地图（本计划 + `AGENTS.md` Doc Map）。
2. [ ] 前端实现 DICT 扩展字段渲染与选项加载：
   - `apps/web/src/pages/org/OrgUnitsPage.tsx`
3. [ ] 前端测试补齐（组件/函数级）：
   - `apps/web/src/pages/org/*.test.tsx`（按现有测试组织落位）
   - 覆盖 `allowed_fields` deny 场景（fail-closed：字段不可编辑/不入 patch）
4. [ ] 运行门禁并记录结果：
   - `pnpm -C apps/web exec tsc --noEmit`
   - `pnpm -C apps/web test -- org`（或同等前端测试入口，覆盖新增测试）
   - `make check doc`

## 7. 验收标准（DoD）

1. [ ] 字段配置页中“已启用且可写”的 DICT 扩展字段，在“新建组织”弹窗可见。
2. [ ] DICT 字段可搜索并选择选项，提交后创建成功。
3. [ ] 提交负载中 `patch.ext.<field_key>` 为字典 code（字符串），后端无 `PATCH_FIELD_NOT_ALLOWED` / `ORG_INVALID_ARGUMENT` 错误。
4. [ ] 不影响现有 PLAIN 扩展字段输入与校验。
5. [ ] 当字段不在 `allowed_fields`（或 capabilities disabled）时，DICT 字段 fail-closed：不可编辑且不会进入提交 payload。
6. [ ] 文档门禁通过，且 Doc Map 可发现本方案。

## 8. 风险与缓解

1. **风险：** DICT 选项 API 失败导致新建体验阻断。  
   **缓解：** 字段级错误显示 + 不影响其他字段编辑；仅该字段不可用。
2. **风险：** `allowed_fields` 与 UI 渲染不一致。  
   **缓解：** 统一通过 `isCreateFieldEditable` 判定并在提交前二次过滤。
3. **风险：** 大量 DICT 字段造成弹窗性能下降。  
   **缓解：** 懒加载选项 + 输入关键词后请求（防抖）。

## 9. 关联记录

- 执行记录（待实施后创建）：`docs/dev-records/dev-plan-108a-execution-log.md`
