# DEV-PLAN-440 Readiness

## 状态

- 日期：2026-04-20
- owner：`DEV-PLAN-440`
- 当前结论：`Phase 0` 已进入落地，`Phase 1` 已切断 `/app/org/setid/**` 用户入口，并完成 `jobcatalog` 外部协议从 `setid` 到 `package_code` 的收口；`staffing` 已完成外层 API / 前端类型的用户可见 SetID 字段收口，但 `Phase 2/3` 仍被 `jobcatalog` / `staffing` 的 runtime/schema 依赖阻塞，禁止以 compat 方式硬推进。

## 当前态命中面

### 1. 已确认可直接切断的用户入口

- `apps/web/src/router/index.tsx`
  - `/app/org/setid`
  - `/app/org/setid/base`
  - `/app/org/setid/registry`
  - `/app/org/setid/explain`
  - `/app/org/setid/ops`
- `apps/web/src/navigation/config.tsx`
  - SetID Governance 一级/二级导航
- `apps/web/src/pages/org/SetIDGovernancePage.tsx`
  - SetID 页面壳、registry/explain/ops 入口
- `config/routing/allowlist.yaml`
  - `/app/org/setid/**` UI allowlist 条目

### 2. 现阶段不可硬删的运行时依赖

- `modules/jobcatalog/services/view_rules.go`
  - `SetIDStore`
  - `ResolveJobCatalogPackageBySetID(...)`
  - `OwnerSetID*`
- `internal/server/jobcatalog_api.go`
  - `GET/POST /org/api/jobcatalog` 仍以 `setid` 作为显式输入
- `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`
  - `staffing.replay_position_versions(...)` 仍依赖 `v_jobcatalog_setid`
- `modules/staffing/infrastructure/persistence/position_pg_store.go`
  - Position 写入/读取仍保留 `jobcatalog_setid` / `jobcatalog_setid_as_of`
- `modules/staffing/domain/types/position.go`
  - Position 运行时结构仍保留 `JobCatalogSetID` / `JobCatalogSetIDAsOf`

### 3. 当前不能删的 API / Authz / Capability 映射

- `/org/api/setids`
- `/org/api/setid-bindings`
- `/org/api/global-setids`
- `/org/api/setid-strategy-registry`
- `/org/api/setid-strategy-registry:disable`
- `/org/api/setid-explain`
- `config/access/policy.csv`
  - `orgunit.setid`
  - `org.setid_capability_config`
  - `setid-strategy-registry`
  - `setid-explain`

这些对象当前仍被现行页面或主流程真实消费，删除前必须先完成无 SetID 正式契约切换。

## 已完成的 Phase 0 收口

- `AGENTS.md`
  - 保持 `440` 为 SetID 根删除唯一 PoR
  - 将仍保留的 SetID 计划条目标注为历史来源/历史合同/待归档材料，而非现行主线
- `docs/dev-plans/060-business-e2e-test-suite.md`
  - SetID 主链已降级为历史合同样本
- `docs/archive/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`
  - SetID 段已降级为历史测试样本

## 已完成的 Phase 1 收口

- 删除 `/app/org/setid/**` 用户入口
- 删除 SetID Governance 导航
- 删除 `SetIDGovernancePage` 页面壳与对应前端测试
- 删除 `config/routing/allowlist.yaml` 中 `/app/org/setid/**` 条目
- 删除 `OrgUnitFieldConfigsPage` 中跳转已退役页面的入口
- 删除 `AssignmentsPage` 中用户可见的 `SetID Explain` 面板
- 删除 `PositionsPage` 中用户可见的 `SetID` 上下文/列表列展示
- `jobcatalog` 对外协议改为 `package_code`
  - `apps/web/src/api/jobCatalog.ts`
  - `apps/web/src/pages/jobcatalog/JobCatalogPage.tsx`
  - `internal/server/jobcatalog_api.go`
- `jobcatalog` 页面不再向用户显示 `owner_setid` / `SetID Explain`
- `jobcatalog` 定向单测已随 `package_code` 契约同步
  - `apps/web/src/pages/jobcatalog/JobCatalogPage.test.tsx`
  - `internal/server/jobcatalog_api_test.go`
- `staffing` 外层响应已不再暴露用户可见 SetID 字段
  - `apps/web/src/api/positions.ts`
  - `internal/server/staffing_handlers.go`
  - `apps/web/src/pages/staffing/StaffingViewAsOfPages.test.tsx`
  - `internal/server/staffing_positions_options_api_test.go`

## 停止线

### 停止线 A：`jobcatalog` 仍以 `setid` 为业务主键

证据：

- `modules/jobcatalog/services/view_rules.go`
- `internal/server/jobcatalog_api.go`
- `apps/web/src/pages/jobcatalog/JobCatalogPage.tsx`

结论：

- 已完成“外部协议/用户可见语义”切换，但内部仍通过 `ResolveJobCatalogPackageByCode(...) -> owner_setid -> 现有 store` 复用现有存储层。
- 未冻结无 SetID runtime/schema 契约前，不得删除 `SetIDStore`、`ResolveJobCatalogPackageBySetID(...)`、底层 `owner_setid` 相关持久化逻辑。

### 停止线 B：`staffing` 仍依赖 `jobcatalog_setid`

证据：

- `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`
- `modules/staffing/infrastructure/persistence/position_pg_store.go`
- `modules/staffing/domain/types/position.go`

结论：

- 已完成 `staffing` 页面与外层 API 的用户可见 SetID 输出收口，但未完成 Position/Assignment 无 SetID 归属语义改造前，不得删除内部 resolver/store 适配与相关 schema。

### 停止线 C：Schema 删除闭环尚不成立

证据：

- `internal/sqlc/schema.sql`
- `migrations/orgunit/**`
- `migrations/jobcatalog/**`
- `migrations/staffing/**`

结论：

- 当前不能进入 `Phase 3`；否则会造成 runtime 与 schema 不一致。

## 分阶段 owner 与建议拆分

### PR-A：Phase 0 + 可见入口下线

- 文档收口
- readiness 建立
- `/app/org/setid/**` 路由与导航删除

### PR-B：JobCatalog 无 SetID 正式契约冻结

- 明确新的业务归属键
- 替换 `ResolveJobCatalogPackageBySetID(...)`
- 删除页面显式 `setid` 输入

### PR-C：Staffing 无 SetID 正式契约冻结

- 删除 `jobcatalog_setid`
- 删除 `SetIDExplainPanel` 对 staffing 主流程的依赖
- 调整 positions options / assignments precheck 输出

### PR-D：Schema / sqlc / migration 根删除

- 仅在 PR-B / PR-C 完成后进入

## 验证矩阵

- 文档：
  - `make check doc`
- 路由：
  - `make check routing`
- Authz：
  - `make authz-pack && make authz-test && make authz-lint`
- 前端定向：
  - `pnpm -C apps/web test -- src/router/index.test.tsx src/pages/org/OrgUnitFieldConfigsPage.test.tsx`
  - `pnpm -C apps/web exec vitest run src/pages/jobcatalog/JobCatalogPage.test.tsx`
  - `pnpm -C apps/web exec vitest run src/pages/staffing/StaffingViewAsOfPages.test.tsx`
- 后端定向：
  - `go test ./internal/server ./modules/jobcatalog/services`
  - `go test ./internal/server`

## 剩余阻塞

1. `jobcatalog` 仍无无-SetID 正式契约。
2. `staffing` 虽已去掉外层用户可见 SetID 字段，但 runtime/store/schema 仍把 `jobcatalog_setid` 作为内部上下文。
3. `pkg/fieldpolicy` / `setid_strategy_registry` 仍是活体运行时，而非单纯历史命名残余。
4. `config/access` / `pkg/authz` / `config/capability` 中的 SetID API 权限不能先删，否则会打断仍在使用的主流程。

## 禁止事项

- 不得把 `/org/api/setid-*` 返回空值后声称“已删除 SetID”。
- 不得保留隐藏的 SetID 页面跳转。
- 不得只删 UI 不记录 runtime/schema 阻塞。
- 不得为了通过 grep 门禁新增 compat alias 或空壳 DTO。
