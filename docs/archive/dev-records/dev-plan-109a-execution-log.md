# DEV-PLAN-109A 执行日志

## 2026-02-22（UTC）

- 2026-02-22 08:12 UTC：完成业务幂等命名收敛（`request_code -> request_id`），覆盖后端 API/服务、前端 API、E2E、Schema 源与 SQL 相关资产。
- 2026-02-22 08:36 UTC：完成 Tracing 收敛：错误 envelope 字段改为 `trace_id`，前端错误解析改为 `trace_id`，HTTP 传播头改为 `traceparent`。
- 2026-02-22 08:48 UTC：完成 Gate-C 升级（`scripts/ci/check-request-code.sh`），阻断新增 `request_code` 与 tracing 场景 `request_id` / `X-Request-ID`。
- 2026-02-22 09:06 UTC：`make check request-code`（OK，full）。
- 2026-02-22 09:22 UTC：`go vet ./...`（OK）。
- 2026-02-22 09:24 UTC：`make check lint`（OK）。
- 2026-02-22 09:39 UTC：`make test`（OK，coverage 100%）。

## 例外与说明

- 未重写历史迁移文件（遵循 Stopline）。
- 为兼容旧测试数据库对象，在 `internal/server/orgunit_projection_integration_test.go` 增加了测试期列名自愈逻辑（仅测试装配路径）。
