# DEV-PLAN-102A 执行日志：Org Code 默认规则生效日可见性修复

> 对应计划：`docs/archive/dev-plans/102a-org-code-default-policy-effective-date-visibility-fix.md`

## 1. 记录范围

- 记录时间：2026-02-20（UTC）
- 目标：完成“保存后无变化”的可见性修复，并收敛 `next_org_code` 为双引号唯一口径。

## 2. 变更摘要

- 前端字段策略保存后，若 `enabled_on > as_of`，自动推进 URL `as_of` 到 `enabled_on`，并给出成功提示。
- 策略弹窗新增“生效日晚于当前视图日”的显式提示。
- 默认规则输入文案统一为 `next_org_code("PREFIX", WIDTH)`（双引号唯一写法）。
- 后端 `parseNextOrgCodeRule` 与保存校验均收敛为双引号写法；单引号在保存阶段直接拒绝。
- 新增前端/后端测试覆盖上述逻辑。

## 3. 代码落点

- `apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx`
- `apps/web/src/pages/org/orgUnitFieldPolicyAsOf.ts`
- `apps/web/src/pages/org/orgUnitFieldPolicyAsOf.test.ts`
- `apps/web/src/i18n/messages.ts`
- `modules/orgunit/services/orgunit_write_service.go`
- `modules/orgunit/services/orgunit_write_service_policy_defaults_test.go`
- `internal/server/orgunit_field_policy_api_test.go`

## 4. 验证记录

- Go（后端）：
  - `go test ./modules/orgunit/services ./internal/server -run "TestParseCompileAndMapCreateAutoCodeHelpers|TestWrite_CreateOrg_AutoCodeBranches|TestHandleOrgUnitFieldPoliciesAPI_CoverageAndHelpers" -count=1`：通过。
  - `go test ./internal/server -run TestHandleOrgUnitFieldConfigsAPI_WithPolicyStoreCoverage -count=1`：通过。
- Web（前端）：
  - `pnpm --dir apps/web test src/pages/org/orgUnitFieldPolicyAsOf.test.ts`：通过（4 tests）。
  - `pnpm --dir apps/web typecheck`：通过。
  - `pnpm --dir apps/web lint`：通过。
- 文档门禁：
  - `make check tr`：通过（当前为 placeholder no-op）。
  - `make check doc`：通过。
