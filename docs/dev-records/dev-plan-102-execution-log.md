# DEV-PLAN-102 执行日志（Execution Log）

日期：2026-02-14

## 1. 目标回顾
- 收敛全项目时间上下文：冻结 A/B/C 路由分类与参数职责（`as_of/tree_as_of/effective_date`）。
- 去除壳层 Topbar 的全局 `as_of` 选择器与“强灌/透传”机制（过渡期实现，承接 `DEV-PLAN-103`）。
- MUI Org（OrgUnits）页面整改：移除未交付的批量启用/停用入口；修正把 `as_of` 误标为 `effective_date` 的列表列。

## 2. 实施内容（按里程碑）

### M0：契约冻结与矩阵
- 已在 `docs/dev-plans/102-as-of-time-context-convergence-and-critique.md` 补齐：
  - 路由-时间参数矩阵（A/B/C、缺省/错误码/重定向）
  - 跨层语义映射（UI ↔ 服务 ↔ SQL）
  - Stopline（禁止新增/回漂点）

### M1：去壳层全局日期（过渡策略）
- Shell 模板不再使用 `__BB_AS_OF__` 进行全局注入；`/ui/nav`、`/ui/topbar` 不再要求 `as_of`。
- Topbar 移除全局 `as_of` 日期选择器（不再提交/改写页面时间上下文）。
- 导航（Nav）不再拼接时间参数；目标页面自行负责缺省/校验（必要时 302 补齐）。

### M2：MUI X Org 模块整改
- OrgUnits 列表页移除“批量启用/停用”相关 UI 与逻辑（checkbox selection、bulk buttons、前端循环调用）。
- 移除 OrgUnits 列表中误导性的 `effectiveDate` 列（此前展示的是 `as_of` 视图日期，而非记录版本 `effective_date`）。

### M4：门禁与测试
- Go tests：覆盖 shell/topbar/nav 行为变更（去 token、去全局 as_of 依赖）。
- MUI tests：保持 `effective_date` 默认选中算法的单测覆盖（承接 `DEV-PLAN-076` 的“禁止伪版本/条目数稳定”口径）。

## 3. 变更清单（关键文件）
- `apps/web/src/pages/index.astro`：Shell hx-get 改为 `/ui/nav`、`/ui/topbar`（不再拼接 `as_of`）。
- `internal/server/handler.go`：`/ui/nav`、`/ui/topbar` 不再 require `as_of`；Topbar 去日期选择器；Nav 不拼接时间参数；Astro shell token 注入改为可选（兼容历史模板）。
- `apps/web-mui/src/pages/org/OrgUnitsPage.tsx`：去 bulk actions；移除误导列。
- `docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`：补齐矩阵与 stopline，并标记里程碑完成。
- `docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`：同步壳层不再全局注入 `as_of` 的最新口径（引用 `DEV-PLAN-102`）。
- `docs/dev-records/DEV-PLAN-010-READINESS.md`：更新 Shell 描述，避免引用已移除的 token 注入机制。

## 4. 本地验证记录
- `pnpm -C apps/web-mui test`：通过（13 tests）
- `make css`：通过（重建并同步 `internal/server/assets/astro`、`internal/server/assets/web-mui`）
- `go fmt ./... && go vet ./... && make check lint && make test`：通过（含 `go-cleanarch` 与覆盖率门禁）
- `make check doc && make check routing`：通过

如需追溯输出细节，请以本地终端日志/CI 记录为准。
