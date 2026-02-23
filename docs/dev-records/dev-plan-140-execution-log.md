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

## 验收抽样（运行态）

- 2026-02-23 06:51 UTC：本地重启服务并验证登录链路（`/iam/api/sessions` 返回 204 且下发 `sid` cookie）。
- 2026-02-23 06:52 UTC：调用 `POST /org/api/org-units` 触发根组织已存在场景，返回：
  - `code=ORG_ROOT_ALREADY_EXISTS`
  - `message=根组织已存在，请改为选择上级组织后新建。`
  - 未出现 `orgunit_write_failed` / `*_failed` 泛化文案。

## 剩余风险与待办

- 全仓“用户可见稳定错误码全集”仍需持续补齐（当前 catalog 以已识别稳定码为主）。
- 需补一轮跨模块 E2E 失败提示断言矩阵（Org/Person/Staffing/Dict/SetID/JobCatalog/IAM）以完成 DoD 全量闭环。
- No-Legacy 故障处置演练记录（只读/停写→修复→重试/重放→恢复）尚未单独固化证据。
