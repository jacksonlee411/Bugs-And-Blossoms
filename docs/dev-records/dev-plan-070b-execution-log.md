# DEV-PLAN-070B 执行日志

> 目的：记录 DEV-PLAN-070B（取消 `global_tenant` 运行时读取、收敛为租户本地发布）的实施证据与门禁结果。

## 2026-02-22：PR-070B-1（契约冻结与前置同步）

### 已完成

- 完成 P0 文档口径同步（历史口径降级为勘误，现行口径统一指向 070B tenant-only）：
  - `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
  - `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
  - `docs/dev-plans/071a-package-selection-ownership-and-subscription.md`
  - `docs/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- 明确 071B 默认绑定模式收敛为 `tenant_only`，并将 `tenant_global/global fallback` 标注为历史阶段口径。
- 完成 070B 启动前置核验：`DEV-PLAN-102B` 的 M2/M3/M5 已完成，并在 `docs/dev-records/dev-plan-102b-execution-log.md` 留证。

### 待完成（承接后续 PR）

- 防漂移门禁：阻断字典链路新增 `global_tenant` 读取分支。
- 字典读链路 tenant-only 改造、`dict_baseline_not_ready` fail-closed、发布链路与迁移工具。

## 2026-02-22：PR-070B-2 / PR-070B-3（发布基座 + 控制面 API）

### 已完成

- 新增发布基座与控制面 API（不新增表）：
  - `internal/server/dicts_release.go`
  - `internal/server/dicts_release_api.go`
  - `internal/server/handler.go`（新增 `/iam/api/dicts:release` 与 `/iam/api/dicts:release:preview`）
- 发布写入全部走 One Door：
  - `iam.submit_dict_event(...)`
  - `iam.submit_dict_value_event(...)`
- 发布幂等与审计：
  - 事件 `request_id` 统一衍生为 `request_id#dict#<source_event_id>` / `request_id#value#<source_event_id>`
  - 审计元数据写入事件 payload：`release_id`、`operator`、`source_tenant_id`、`source_event_id`、`source_request_id`、`as_of`
- 控制面权限与路由门禁：
  - `pkg/authz/registry.go` 新增 `iam.dict_release`
  - `internal/server/authz_middleware.go` 接入路由授权
  - `config/access/policy.csv` 与 `config/access/policies/00-bootstrap.csv` 增加授权
  - `config/routing/allowlist.yaml` 增加发布/预检路由

## 2026-02-22：PR-070B-5 / PR-070B-7（回填对账工具 + 收口）

### 已完成

- 新增回填与对账脚本（Migration Tooling）：
  - `scripts/db/dict-baseline-release.sh`
  - `scripts/db/dict-baseline-reconcile.sh`
- 运行态 tenant-only 收口清理：
  - `internal/server/dicts_store.go`（memory store 不再运行时回退 global）
- 新增/修复覆盖：
  - `internal/server/dicts_release_test.go`
  - `internal/server/dicts_release_api_test.go`
  - `internal/server/dicts_store_test.go`
  - `internal/server/dicts_api_test.go`
  - `internal/server/authz_middleware_test.go`
  - `internal/server/handler_dicts_routes_coverage_test.go`

### 门禁与命令结果

- `go fmt ./... && go vet ./... && make check lint`：通过
- `make test`：通过（coverage 100%）
- `make check routing`：通过
- `make authz-pack && make authz-test && make authz-lint`：通过
- `make check dict-tenant-only`：通过
- `make check doc`：通过
- `make iam plan && make iam lint`：通过（no drift）
- `make sqlc-generate`：通过

## 2026-02-22：PR-070B-6（切流 Runbook）

### 已完成

- 交付预演脚本与执行模板：
  - 回填脚本：`scripts/db/dict-baseline-release.sh`
  - 对账脚本：`scripts/db/dict-baseline-reconcile.sh`
- 生产窗口执行项（停写 -> 最终增量发布 -> 开写）保留为运维变更动作，不在开发仓库内直接执行。

## 2026-02-22：PR-070B1 / DEV-PLAN-070B-T（UI 发布闭环 + 测试收敛）

### 已完成

- 文档收敛：
  - `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`：发布幂等命名统一为 `request_id`。
  - `docs/dev-plans/070b-t-dict-tenant-release-test-plan.md`：`STD-001` 测试口径统一为“不得新增/外泄 `request_code`”。
- 070B1 UI 闭环实现：
  - `/dicts` 页面新增“字典基线发布”操作区（预检 -> 冲突/就绪 -> 发布 -> 结果）。
  - 发布状态机（`idle/previewing/conflict/ready/releasing/success/fail`）与参数校验落地。
  - 发布权限细粒度收敛：页面访问继续使用 `dict.admin`，发布动作使用 `dict.release.admin`。
  - 关键文件：
    - `apps/web/src/pages/dicts/DictConfigsPage.tsx`
    - `apps/web/src/pages/dicts/dictReleaseFlow.ts`
    - `apps/web/src/api/dicts.ts`
    - `apps/web/src/i18n/messages.ts`
    - `apps/web/README.md`
- 测试补齐：
  - 前端 API/状态机单测：
    - `apps/web/src/api/dicts.test.ts`
    - `apps/web/src/pages/dicts/dictReleaseFlow.test.ts`
  - E2E 专项：`e2e/tests/tp070b-dict-release-ui.spec.js`（覆盖 UI 完整链路 + 无发布权限 403）。

### 门禁与命令结果（2026-02-22）

- `pnpm --dir apps/web lint && pnpm --dir apps/web typecheck && pnpm --dir apps/web test`：通过。
- `make generate && make css`：通过（产物已同步到 `internal/server/assets/web/`）。
- `make e2e`：通过（8/8，含 `tp070b-dict-release-ui.spec.js`）。
- `make check doc`：通过。
- `make preflight`：通过（含 `no-legacy/request-code/as-of-explicit/dict-tenant-only/go-version/doc/go test/routing/e2e`）。

### 关键通过项（输入 / 期望 / 实际 / 证据）

- 输入：`/dicts` 页面填写 `source_tenant_id/as_of/release_id/request_id`，先执行 preview。  
  期望：冲突为 0 时进入可发布态。  
  实际：进入 `ready` 态并显示“预检通过，可执行发布”。  
  证据：`e2e/tests/tp070b-dict-release-ui.spec.js`。
- 输入：点击“执行发布”。  
  期望：返回发布结果并展示 `release_id/request_id/status/计数/started_at/finished_at`。  
  实际：发布成功结果区完整展示并支持复制 ID。  
  证据：`apps/web/src/pages/dicts/DictConfigsPage.tsx` + `e2e/tests/tp070b-dict-release-ui.spec.js`。
- 输入：`tenant-viewer` 调用 preview/release API。  
  期望：后端 403 fail-closed。  
  实际：preview 与 release 均返回 403。  
  证据：`e2e/tests/tp070b-dict-release-ui.spec.js`。

### 缺陷分类与处置

- 阻塞缺陷：无。
- 分类汇总：`BUG=0`、`CONTRACT_DRIFT=0`、`CONTRACT_MISSING=0`、`ENV_DRIFT=0`。
