# DEV-PLAN-111：前端错误信息准确化与字段级提示收敛方案

**状态**: 规划中（2026-02-19 13:49 UTC）

## 1. 背景

当前 Org 模块已出现“错误码与前端文案不匹配”的用户可见问题，典型表现：

- `invalid_request` 被过度映射为“扩展筛选/排序请求不合法”，导致创建失败场景误报。
- 后端返回稳定业务码（如 `ORG_ROOT_ALREADY_EXISTS`）时，前端最终看到 `orgunit_write_failed`，缺少可操作信息。
- 表单错误多数只能显示全局 Alert，不能指出具体字段（如 `name`、`parent_org_code`、`effective_date`）。

该问题已经影响“新建组织”等主路径的可诊断性与可用性，需要以“单点错误翻译 + 字段级提示”收敛。

## 2. 目标与非目标

### 2.1 目标

- [ ] 前端优先展示准确、可操作的错误信息（避免泛化/误判文案）。
- [ ] 在可判定条件下，附带“具体错误字段”并在表单上定位（helper text / error state / focus）。
- [ ] 统一错误处理入口，避免各页面重复维护映射表导致漂移。
- [ ] 保持 fail-closed：未知错误不“猜测业务语义”，回退为后端原始 message。

### 2.2 非目标

- 不重做整套后端错误模型，不引入新的 legacy 错误通道。
- 不在本计划中覆盖所有模块页面，先完成高频写路径（Org 模块）并提供可复用能力。

## 3. 方案范围与SSOT

- 前端范围：`apps/web/src`（错误标准化、页面落地、i18n）。
- 后端配套范围（必要最小）：`internal/server`（补齐稳定错误码映射；可选补充字段元信息）。
- 契约与门禁引用：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`。

## 4. 设计方案

### 4.1 错误翻译单点化（Frontend SSOT）

新增前端错误翻译模块（建议：`apps/web/src/errors/presentError.ts`）：

- 输入：`ApiClientError` + 业务上下文（module/page/action）。
- 输出：`{ message: string; fieldKey?: string; code?: string }`。
- 规则优先级：
  1) 上下文专属稳定码映射；
  2) 通用稳定码映射；
  3) 若无法匹配，回退后端 `message`；
  4) 最后回退 `Error.message`。

禁止在页面内直接写大段 `switch(code)`，统一走单点函数。

### 4.2 字段级提示策略（Field Hint）

字段定位优先级：

1. **后端显式字段**（若返回 `meta.field` 或等价字段）；
2. **稳定码到字段映射**（如 `DEFAULT_RULE_REQUIRED -> org_code`，`ORG_PARENT_NOT_FOUND_AS_OF -> parent_org_code`）；
3. **安全正则兜底**（仅匹配白名单句式，如 `name is required`）。

页面落地要求：

- 全局 Alert 继续显示；
- 若识别到 `fieldKey`，对应输入框设置 `error + helperText`，并尝试 `focus`；
- 不可识别字段时，仅显示全局错误，不做猜测。

### 4.3 后端最小配套（提升准确率）

为减少 `orgunit_write_failed` 这类泛化提示，补齐后端稳定码映射：

- 在 `orgUnitAPIStatusForCode` 与 `orgNodeWriteErrorMessage` 补齐高频码（至少：`ORG_ROOT_ALREADY_EXISTS`、`ORG_TREE_NOT_INITIALIZED`、`ORG_NOT_FOUND_AS_OF`、`ORG_ALREADY_EXISTS`）。
- 可选：在 ErrorEnvelope `meta` 中增加可选 `field`（不破坏现有字段，向后兼容）。

承接 `DEV-PLAN-080B`，冻结以下错误提取口径：

1. API 错误码映射必须优先提取 PG stable message（如 `stablePgMessage(err)`），禁止直接使用 `err.Error()` 进行业务码判定。
2. 对外错误契约禁止透出 SQLSTATE/约束名原文；统一映射为稳定业务码后再由前端翻译。
3. 若无法识别 stable message，按 fail-closed 回退通用错误码，并保留后端原始 `message` 供排障。

### 4.4 i18n 收口

补充并冻结错误文案 key（`en/zh` 同步）：

- 高频稳定码 key（新增或复用）；
- 字段级提示 key（如“请检查字段：{fieldLabel}”）；
- 禁止复用语义不相干 key（例如将 `invalid_request` 固定翻译为某个特定子场景）。

## 5. 实施步骤

1. [ ] 盘点错误来源与映射现状（OrgUnits / OrgUnitDetails / OrgUnitFieldConfigs）。
2. [ ] 新增前端错误翻译单点模块，并完成单测（含优先级与回退链路）。
3. [ ] OrgUnits 页面接入单点模块，移除页面内分散映射逻辑。
4. [ ] 接入字段级提示：Create/Append 表单优先支持 `name`、`parent_org_code`、`effective_date`、`org_code`。
5. [ ] 后端补齐高频稳定码的状态与友好消息映射（最小必要集合）。
6. [ ] 增补 i18n 文案（`en/zh`），并通过 `make check tr`。
7. [ ] 补充测试：
   - 前端单测：错误翻译、字段定位、fallback；
   - Go 单测：稳定码映射；
   - E2E：创建组织失败场景文案与字段提示。
8. [ ] 运行门禁并记录证据（见第 7 节）。

## 6. 验收标准

- [ ] 不再出现“错误码 A 映射为无关文案 B”的误报（至少覆盖本次已发现场景）。
- [ ] 高优先级创建失败场景可显示准确原因；若可定位字段，字段控件显示错误态。
- [ ] 未识别错误码时显示后端 message，不显示误导性固定文案。
- [ ] 页面错误映射逻辑收敛到单点模块，避免重复实现。

## 7. 质量门禁与执行记录（待实施）

- [ ] `pnpm -C apps/web typecheck && pnpm -C apps/web test`
- [ ] `go test ./internal/server/...`
- [ ] `make check tr`
- [ ] `make preflight`
- [ ] 证据记录写入 `docs/dev-records/`（如需）。

## 8. 风险与回滚

- 风险：错误翻译收敛时遗漏旧分支导致回归。
  - 对策：保留 fallback 到后端 message；以用例驱动逐页迁移。
- 风险：字段定位误判造成错误高亮错位。
  - 对策：字段定位仅采用“显式字段/稳定码白名单/安全正则”，不做自由推断。
- 回滚：前端保留旧展示路径的最小开关（代码级回退到后端 message），不引入双链路长期并存。
