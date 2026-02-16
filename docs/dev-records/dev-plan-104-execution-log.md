# DEV-PLAN-104 / 104A 执行日志

> 目的：记录 DEV-PLAN-104（Job Catalog UI 信息架构收敛）与 DEV-PLAN-104A（对齐 DEV-PLAN-002）的实施与验证证据。

## 变更摘要

- 文档与契约：
  - 新增 `docs/dev-plans/104-jobcatalog-ui-optimization.md`
  - 新增 `docs/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md`
  - `AGENTS.md` 文档地图新增 104 / 104A 入口
- JobCatalog 页面重构（单页 + Tabs，不新增二级菜单）：
  - `apps/web/src/pages/jobcatalog/JobCatalogPage.tsx`
  - 上下文工具条（sticky）收敛 `as_of + package_code + owner_setid + read_only`
  - `DatePicker`（day 粒度）统一 `as_of/effective_date`
  - 主工作区改为 `Tabs + DataGridPage`，移除裸 `<table>`
  - Families/Profiles 落地两栏（左 Group 面板，右列表）
  - 写操作统一迁入 `Dialog`，并在提交前展示“将对 YYYY-MM-DD 生效”
  - 行级操作统一 `More(⋯)` 菜单（含 aria/tooltip）
- 主题与文案：
  - `apps/web/src/theme/theme.ts` 增补 `primary.contrastText = '#ffffff'`
  - `apps/web/src/i18n/messages.ts` 补齐 JobCatalog 相关 en/zh 文案
- 设计稿（Pencil MCP）：
  - `designs/orgunit/orgunit-details-ui2.pen`

## 提交与 PR

- 主要提交：
  - `baad100` feat(web): implement DEV-PLAN-104 jobcatalog ui revamp
  - `f1b5e06` chore(web): regenerate bundled web assets
  - `410de84` fix(web): stabilize jobcatalog heading text for e2e
- PR：#362 `feat(web): DEV-PLAN-104/104A JobCatalog UI revamp`
  - 链接：<https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/362>
  - 合并时间：2026-02-16 03:34 UTC
  - merge commit：`ee0112a4a34b7fb48dc6cc8484f86e3d9d8ab992`

## 验证记录

- 本地验证（2026-02-16）：
  - `pnpm --dir apps/web typecheck`
  - `pnpm --dir apps/web exec eslint src/pages/jobcatalog/JobCatalogPage.tsx src/i18n/messages.ts src/theme/theme.ts`
  - `make css`
- CI 验证（PR #362）：
  - Code Quality & Formatting：PASS
  - Routing Gates：PASS
  - Unit & Integration Tests：PASS
  - E2E Tests：PASS

## 约束符合性

- 未引入 legacy/双链路回退，保持单链路原则。
- 未变更后端 API/业务契约，仅重构 UI 信息架构与交互承载。
- 日期语义维持 day 粒度（`as_of/effective_date` 统一 `YYYY-MM-DD`）。
