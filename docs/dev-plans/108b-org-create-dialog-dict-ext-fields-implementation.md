# DEV-PLAN-108B：Org 新建组织弹窗 DICT 扩展字段实现（承接 108A）

**状态**: 已完成（2026-02-19 08:04 UTC）

## 1. 背景

`DEV-PLAN-108A` 冻结了“新建组织弹窗支持 DICT 扩展字段（下拉选择）”的范围与验收口径。  
本计划用于落地实现与收口验证，确保 `create_org` 入口与 `DEV-PLAN-108` 的“core + ext 全字段编辑”目标对齐。

## 2. 实施范围（冻结）

1. 仅覆盖 `apps/web/src/pages/org/OrgUnitsPage.tsx` 的“新建组织”弹窗。  
2. 不扩展 `add_version / insert_version / correct`（仍按 `DEV-PLAN-108` 主计划推进）。  
3. 不改后端写接口契约，仅复用现有 `write(create_org)` 与字段选项 API。

## 3. 实施步骤

1. [X] 将新建弹窗扩展字段来源从“字段定义”收敛到“enabled field configs + allowed_fields”。  
2. [X] 在新建弹窗新增 DICT 字段输入（Autocomplete 下拉 + 远程 options 查询）。  
3. [X] 提交时 DICT 字段按 `patch.ext.<field_key>=code` 写入，并保持 fail-closed（不可编辑字段不允许进入可交互状态）。  
4. [X] 为未知 `data_source_type` 增加显式 warning，避免静默丢失字段可见性。  
5. [X] 运行并记录前端测试门禁（`pnpm -C apps/web test -- org` 或同等入口）。  
6. [X] 补充执行记录到 `docs/dev-records/dev-plan-108b-execution-log.md`。

## 4. 当前实现结论

1. 新建弹窗现已同时渲染 `PLAIN` 与 `DICT` 扩展字段。  
2. DICT 字段使用下拉搜索输入，支持按 as-of 拉取候选值。  
3. 当字段不在 `allowed_fields` 时，控件被禁用并显示“不可编辑”提示。  
4. 当存在未知扩展字段类型时，UI 显示 warning，避免误判为“无字段”。

## 5. 验收口径（对应 108A）

- 可见性：已启用且允许编辑的 DICT 字段在新建弹窗可见。  
- 可操作性：可检索、可选择、可提交。  
- 一致性：提交 payload 维持 `patch.ext` 结构，不引入新接口。  
- 安全性：继续遵循 `allowed_fields` fail-closed。

## 6. 关联文档

- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/108a-org-create-dialog-dict-ext-fields-support.md`
