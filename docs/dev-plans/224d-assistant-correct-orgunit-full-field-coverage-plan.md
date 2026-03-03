# DEV-PLAN-224D：Assistant `correct_orgunit` 意图全字段覆盖实施计划

**状态**: 草拟中（2026-03-03 10:08 UTC）

## 1. 背景与问题
- 关联计划：
  - `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
  - `docs/dev-plans/224c-assistant-intent-registry-and-multi-scenario-expansion-plan.md`
  - `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
  - `docs/dev-plans/106b-orgunit-corrections-effective-date-sticky-semantics.md`
- 当前差距：
  1. [ ] Assistant 提交链路仍以 `create_orgunit` 为主，缺少 `correct_orgunit` 可提交能力。
  2. [ ] 用户在 AI 对话里无法完成“更正组织记录”的完整闭环（生成计划 → 确认 → 提交）。
  3. [ ] “更正”需要覆盖的字段范围较大（core + status + effective_date + ext），当前未在 Assistant 侧形成明确契约。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 新增 Assistant 意图：`correct_orgunit`，支持端到端受控提交。
2. [ ] 覆盖“可更正字段全集”（见 §3），不做“只支持部分字段”的临时口径。
3. [ ] 保持 One Door / strict decode / boundary lint / confirm gate / drift guard 全部不回退。
4. [ ] 错误码、权限、审计链与现有 Org 写入契约保持一致。

### 2.2 非目标
1. [ ] 不新增业务写入旁路，不直接写 DB。
2. [ ] 不改变 Org Kernel 的更正语义（沿用 108/106B 既有口径）。
3. [ ] 不在本计划引入多模块跨域更正（仅 orgunit）。

## 3. `correct_orgunit` 可更正字段全集（冻结）
> 说明：本节为 224D 的强制验收范围；实现不得只交付子集。

### 3.1 业务定位字段
1. [ ] `org_code`（被更正组织编码，必填）
2. [ ] `target_effective_date`（被更正目标记录日期，必填）
3. [ ] `effective_date`（更正后生效日；允许与其他字段同次提交）

### 3.2 Core patch 字段（全部支持）
1. [ ] `name`
2. [ ] `parent_org_code`
3. [ ] `status`
4. [ ] `is_business_unit`
5. [ ] `manager_pernr`（支持显式清空）

### 3.3 扩展字段（全部支持）
1. [ ] 内置 ext 字段：`short_name`、`description`、`org_type`、`location_code`、`cost_center`
2. [ ] 租户 DICT 扩展字段：`d_<dict_code>`（符合 `^d_[a-z][a-z0-9_]{0,60}$`）
3. [ ] 租户 PLAIN 扩展字段：`x_<custom_key>`（符合 `^x_[a-z0-9_]{1,60}$`）
4. [ ] 运行时仅允许“该日 enabled”的字段进入 patch（fail-closed）

### 3.4 明确禁止字段
1. [ ] `ext_labels_snapshot`（只允许服务端生成）
2. [ ] 任意未注册/未启用字段

## 4. 方案设计
### 4.1 意图契约（Assistant）
1. [ ] 新增 `action=correct_orgunit`，payload 至少包含：
   - `org_code`
   - `target_effective_date`
   - `patch`（至少 1 个字段）
2. [ ] 支持“字段同次更正”组合（例如：`effective_date + status + parent_org_code + ext.*`）。
3. [ ] strict decode 维持 `additionalProperties=false`，非法结构直接拒绝。

### 4.2 规划编译（Intent -> Plan）
1. [ ] `correct_orgunit` 映射 capability：`org.orgunit_correct.field_policy`。
2. [ ] `SkillExecutionPlan` 新增 `org.orgunit_correct`（并保留 `strict_decode/boundary_lint/dry_run` 必检项）。
3. [ ] `ConfigDeltaPlan` 与 `DryRunResult` 必须逐字段展示（含 ext 字段差异）。
4. [ ] 提交前校验 patch 非空；空 patch 返回稳定错误码。

### 4.3 提交流程（Commit）
1. [ ] Assistant commit 分发到统一写入口：
   - `intent=correct`
   - `org_code`
   - `target_effective_date`
   - `effective_date`
   - `patch(core + ext)`
2. [ ] 不新增 AI 专用写链路；继续走现有 Org 写服务与审计机制。
3. [ ] 版本漂移、契约漂移、角色漂移、候选确认等守卫维持既有策略。

### 4.4 可更正字段能力对齐
1. [ ] 生成计划前读取并使用“写能力口径”（`write-capabilities(intent=correct)`）作为字段白名单来源。
2. [ ] 当用户请求字段不在 allowed_fields：直接 fail-closed，返回明确错误码。
3. [ ] DICT/PLAIN 扩展字段标签快照由服务端生成，前端/模型不得注入。

## 5. 接口与错误码契约
1. [ ] 新增/扩展 Assistant 错误映射：
   - `assistant_intent_unsupported`（未注册意图）
   - `ai_plan_schema_constrained_decode_failed`（结构失败）
   - `ai_plan_boundary_violation`（越界字段/越界能力）
   - Org 写入稳定错误码透传（如 `PATCH_FIELD_NOT_ALLOWED`、`ORG_EVENT_NOT_FOUND`、`ORG_EVENT_RESCINDED`）
2. [ ] 前端提示禁止泛化“提交失败”，必须显示字段级或语义级明确提示。

## 6. 实施切片（PR 建议）
### PR-224D-01：契约与后端骨架
1. [ ] 新增 `correct_orgunit` intent schema 与 action 归一化。
2. [ ] 接入 plan compiler（capability/skill/dry-run）。
3. [ ] commit 分发新增 `correct_orgunit` 分支（调用 unified write `intent=correct`）。

### PR-224D-02：全字段覆盖与能力校验
1. [ ] 接入 `write-capabilities(intent=correct)` 白名单校验。
2. [ ] 实现 core + ext 全字段 patch 组装（含 `effective_date` 同次提交）。
3. [ ] 对齐错误码与审计字段。

### PR-224D-03：前端与 E2E
1. [ ] `/app/assistant` 增加“组织更正”示例与差异可视化。
2. [ ] 增加字段全集场景测试（core + status + effective_date + ext）。
3. [ ] 收口错误提示映射与文档证据。

## 7. 测试与验收
1. [ ] 单元测试（后端）：
   - `correct_orgunit` strict decode/boundary/compile/commit 全链路；
   - 全字段组合用例（core + ext + status + effective_date）；
   - 未启用字段、非法字段、空 patch、事件不存在/已撤销路径。
2. [ ] 前端测试：
   - 计划差异渲染覆盖全字段；
   - 错误码映射准确。
3. [ ] E2E：
   - 组织更正闭环（生成→确认→提交→详情回显）；
   - 同次提交 `effective_date + status + ext`；
   - 字段越界阻断。
4. [ ] 门禁：
   - `make check routing`
   - `make check capability-route-map`
   - `make authz-pack && make authz-test && make authz-lint`
   - `make check error-message`
   - `make test`
   - `make e2e`

## 8. 完成定义（DoD）
1. [ ] Assistant 支持 `correct_orgunit` 提交闭环，且不依赖固定提示词模板。
2. [ ] §3 字段全集全部可用（在权限与字段启用前提下）。
3. [ ] 越界字段、未启用字段、非法结构全部 fail-closed。
4. [ ] One Door、RLS、Authz、审计与版本漂移保护无回退。
5. [ ] 产出执行证据到 `docs/dev-records/` 并通过 CI 门禁。

## 9. 风险与回滚
1. [ ] 风险：字段全集接入后，测试矩阵显著扩大。
2. [ ] 缓解：先骨架，再全字段，再前端/E2E，按 PR 切片渐进交付。
3. [ ] 回滚：仅允许回滚到上一稳定 Assistant 版本；禁止引入临时 legacy 双链路。

## 10. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/106b-orgunit-corrections-effective-date-sticky-semantics.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/224c-assistant-intent-registry-and-multi-scenario-expansion-plan.md`
