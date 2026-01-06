# DEV-PLAN-009M1：Phase 4 下一大型里程碑执行计划（SetID + JobCatalog 首个可见样板闭环）

**状态**: 草拟中（2026-01-06 06:45 UTC）

> 本文是 `DEV-PLAN-009` 的 **执行计划补充（里程碑拆解）**：用于把 “`DEV-PLAN-028(SetID) + DEV-PLAN-029(JobCatalog)` 的首个可见样板闭环” 拆成可合并的 PR 序列与门禁对齐步骤。  
> **本文不约定具体功能细节**（例如具体实体/字段/页面形态/API 形状），也不替代 `DEV-PLAN-028/029` 的合同；任何功能契约变更必须先更新对应 dev-plan 再写代码。

## 1. 里程碑定义（M1）

- 目标（对齐 `DEV-PLAN-009` Phase 4 出口条件 #3）：在 JobCatalog 至少一个实体上形成“**解析 → 写入 → 列表读取**”的 **UI 可见且可操作** 闭环。
- 依赖（SSOT 引用）：
  - SetID：`docs/dev-plans/028-setid-management.md`
  - JobCatalog v4：`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
  - 门禁与触发器：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`
  - Tenancy/AuthN：`docs/dev-plans/019-tenant-and-authn.md`
  - RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
  - Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
  - Routing：`docs/dev-plans/017-routing-strategy.md`
  - Atlas+Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc：`docs/dev-plans/025-sqlc-guidelines.md`

## 2. 非目标（本执行计划不做）

- 不定义 JobCatalog 的具体业务数据模型/字段/页面交互细节（以 `DEV-PLAN-029` 为合同）。
- 不引入存量迁移/兼容/灰度策略。
- 不新增“第二写入口”：所有 v4 写入必须遵守 One Door（写入走 DB Kernel 的 `submit_*_event(...)`）。

## 3. Done 口径（验收/关闭条件）

### 3.1 端到端（用户可见）
- [ ] 在 tenant app 中，存在一个明确入口页面（导航可达），用户能完成至少一次“写入 → 列表读取”的闭环操作。
- [ ] SetID 解析在该闭环中被实际调用（而不是仅有工具函数或文档描述）。

### 3.2 安全与门禁（不可漂移）
- [ ] No Tx, No RLS：访问 v4 表的路径必须在事务内注入 `app.current_tenant`，缺失上下文 fail-closed（对齐 `DEV-PLAN-021`）。
- [ ] 授权可拒绝：至少对上述闭环路径接入统一 403 契约与策略 SSOT（对齐 `DEV-PLAN-022`）。
- [ ] 触发器矩阵命中项均通过本地门禁（按 `AGENTS.md`）：涉及 Go/路由/sqlc/迁移/authz 时，不允许“只跑一半”。

### 3.3 证据固化
- [ ] 将验证步骤与结果补到 `docs/dev-records/DEV-PLAN-010-READINESS.md`（作为本里程碑的可复现证据入口）。

## 4. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 GitHub Actions required checks 全绿且不 `skipped` 后合并；`main` 禁止直推与 force-push。

### PR 发起前的文档更新规则（强制）

- [ ] 每完成一个 PR 的开发任务，在 **发起 PR 之前**，必须先在“该 PR 对应的 dev-plan 合同文档”中登记本次已完成事项：将对应条目从 `[ ]` 更新为 `[X]`，并补充必要的证据（例如命令与时间戳、或 PR 链接）。
- [ ] 如果某个 dev-plan 文档的所有待办条目已全部完成，必须同步更新该文档头部 `**状态**` 为 `已完成（YYYY-MM-DD HH:MM UTC）`。
- [ ] 本规则适用于所有被本里程碑命中的合同文档（例如 `DEV-PLAN-028/029/022/021/010` 等）；本文只规定流程与记录方式，不替代各合同的内容。

### PR-1：前置收口（不引入新功能）
- [ ] 在 `DEV-PLAN-028/029` 中补齐本轮闭环所需的最小合同（仅描述边界/不变量/验收口径；不在本文定义具体功能）。
- [ ] 若本轮会新增路由/授权/生成物/迁移：先在对应 dev-plan 中记录触发器与门禁命中点（SSOT 引用）。

### PR-2：JobCatalog 模块 DB 闭环入口（024）
- [ ] 若 `jobcatalog` 模块尚未具备：补齐 `make jobcatalog plan/lint/migrate up` 的模块级闭环入口（对齐 `DEV-PLAN-024`）。
- [ ] **红线**：如需要新增表/新建迁移（`CREATE TABLE`），必须先获得用户手工确认再落盘。

### PR-3：SetID 最小可配置与解析闭环（028）
- [ ] 落地 SetID 的最小 SSOT（配置文件/枚举/解析入口），并提供可被业务模块复用的解析调用点（不得模块自造回退规则）。
- [ ] 在 tenant app 中提供一个可验证的调用入口（页面或表单流程的一部分），确保“解析”不是孤立实现。
- [ ] 门禁：命中项按 `AGENTS.md` 执行；必要时补齐 `make check routing` 与 `make authz-*` 的门禁证据。

### PR-4：JobCatalog v4 最小闭环（029）
- [ ] 依据 `DEV-PLAN-029` 合同实现 v4 Kernel 的最小闭环（事件 SoT + 同事务同步投射 + 读模型查询）。
- [ ] 读路径与写路径均在 tenant 事务内执行并注入租户上下文（对齐 `DEV-PLAN-021`）。
- [ ] 如命中 sqlc：执行 `make sqlc-generate`，并确保生成物提交且 `git status --short` 为空（对齐 `DEV-PLAN-025`）。

### PR-5：UI 可见样板闭环（028 + 029 + 018）
- [ ] 将 `/org/job-catalog` 从 placeholder 升级为一个“写入 → 列表读取”的最小交互闭环页面（具体交互细节以 `DEV-PLAN-029` 为准）。
- [ ] 通过 UI 实际调用 SetID 解析入口，并将解析结果用于写入路径（避免只在后端实现、UI 不可见）。

### PR-6：Authz 最小可拒绝（022）
- [ ] 将上述闭环路径接入统一 403 契约，并把策略落在 `config/access/policies/**`（由 `make authz-pack` 生成并提交 `config/access/policy.csv(.rev)`）。
- [ ] 门禁：`make authz-pack && make authz-test && make authz-lint`。

### PR-7：Readiness 证据补齐（010）
- [ ] 更新 `docs/dev-records/DEV-PLAN-010-READINESS.md`：记录从启动到完成闭环的浏览器验证脚本与结果（包含时间戳与链接）。

## 5. 本地验证（SSOT 引用）

- 一键启动：`make dev`（入口以 `Makefile` 为准；不要在本文复制细节脚本）。
- 质量门禁：按 `AGENTS.md` 的触发器矩阵执行（优先 `make preflight`）。
