# DEV-PLAN-140 执行日志

## 2026-02-23（UTC）

- 2026-02-23 00:48 UTC：新增错误提示门禁脚本 `scripts/ci/check-error-message.sh`，接入 `make check error-message`。
- 2026-02-23 00:49 UTC：`Makefile` 接入 `check error-message` 与 `preflight` 聚合入口；`Quality Gates` 接入 required check。
- 2026-02-23 00:57 UTC：完成后端统一错误文案归一（`internal/routing/responder.go`）与配套测试，修复覆盖率缺口至 100%。
- 2026-02-23 01:00 UTC：前端新增统一错误展示入口 `apps/web/src/errors/presentApiError.ts`，并接入 `apps/web/src/api/errors.ts` 与 SetID 相关页面。
- 2026-02-23 01:02 UTC：`make check error-message`、`go vet ./...`、`make check lint`、`make test`、`make check doc`、`make e2e`、`make preflight` 全部通过。
- 2026-02-23 09:34 UTC：新增错误目录 `config/errors/catalog.yaml`，并将 `check-error-message` 升级为 catalog 一致性校验（catalog ↔ backend known map ↔ frontend map 全量比对）。
- 2026-02-23 09:35 UTC：前端补齐 `tenant_resolve_error` 显式提示映射与单测断言。
- 2026-02-23 09:36 UTC：再次执行 `make check error-message`，门禁通过（Go + Web + catalog consistency）。
- 2026-02-23 10:08 UTC：将 `error catalog` 扩展至全仓用户可见错误码全集（88 个 code），并补齐 `presentApiError` 单点映射（en/zh）。
- 2026-02-23 10:08 UTC：新增 `internal/routing/error_catalog_coverage_test.go`，基于 Go AST 扫描 `WriteError/writeError` 产出的 code，缺失 catalog 即 fail。
- 2026-02-23 10:09 UTC：`scripts/ci/check-error-message.sh` 接入 `TestErrorCatalog_CoversWriteErrorCodes`，实现“稳定错误码漏登记”门禁阻断。
- 2026-02-23 10:09 UTC：执行 `make check error-message` 通过，验证 catalog ↔ backend known map ↔ frontend map 全量一致。
- 2026-02-23 10:45 UTC：E2E 补齐跨模块失败提示断言（Org/Person/Staffing/Dict/SetID/JobCatalog/IAM），新增 `e2e/tests/helpers/error-message-assert.js` 并接入 `tp060-01/tp060-02/tp060-03/tp070b`。
- 2026-02-23 10:46 UTC：修复 Staffing 控制器错误文案退化：`modules/staffing/presentation/controllers/assignments_api.go` 的 `writeError` 接入 message 规范化（禁止泛化 `*_failed` 直出）。
- 2026-02-23 10:48 UTC：执行 `make e2e` 全量回归通过（8/8），验证失败提示断言矩阵生效。
- 2026-02-23 10:49 UTC：执行 No-Legacy 故障处置演练：注入 `forbidden_failed` 回归后 `make check error-message` 失败；修复映射后再次通过，形成“停写保护→修复→重试→恢复”证据链。
- 2026-02-23 10:49 UTC：补跑门禁 `make check error-message` / `make check doc` / `make check no-legacy` / `make check tr` / `go vet ./...`，全部通过。

## 验收抽样（运行态）

- 2026-02-23 06:51 UTC：本地重启服务并验证登录链路（`/iam/api/sessions` 返回 204 且下发 `sid` cookie）。
- 2026-02-23 06:52 UTC：调用 `POST /org/api/org-units` 触发根组织已存在场景，返回：
  - `code=ORG_ROOT_ALREADY_EXISTS`
  - `message=根组织已存在，请改为选择上级组织后新建。`
  - 未出现 `orgunit_write_failed` / `*_failed` 泛化文案。

## 剩余风险与待办

- 新增稳定错误码时仍需同步维护 `config/errors/catalog.yaml` 与 `apps/web/src/errors/presentApiError.ts`，否则会被 `check error-message` 阻断（预期行为）。
