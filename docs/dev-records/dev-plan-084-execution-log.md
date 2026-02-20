# DEV-PLAN-084 执行日志

## 2026-02-20（UTC）

- 2026-02-20 16:31 UTC：`make check doc`（OK，新增 `DEV-PLAN-084` 与 Doc Map 链接）
- 2026-02-20 16:47 UTC：`go fmt ./internal/server/... ./modules/... ./pkg/... ./cmd/... ./internal/...`（OK）
- 2026-02-20 16:53 UTC：`go test ./internal/server/...`（首次失败：`ListNodesCurrentWithVisibility` 覆盖测试未适配新增 `has_children` 扫描列）
- 2026-02-20 16:56 UTC：修复 `internal/server/orgunit_nodes_pgstore_read_paths_coverage_test.go` 记录列顺序后复测
- 2026-02-20 16:57 UTC：`go test ./internal/server/...`（OK）
- 2026-02-20 17:02 UTC：`pnpm --dir apps/web lint`（OK）
- 2026-02-20 17:03 UTC：`pnpm --dir apps/web typecheck`（OK）
- 2026-02-20 17:09 UTC：`pnpm --dir apps/web test`（OK，14 files / 48 tests）
- 2026-02-20 17:10 UTC：`go vet ./...`（OK）
- 2026-02-20 17:10 UTC：`make check lint`（OK）
- 2026-02-20 17:11 UTC：`make test`（OK，覆盖率门禁通过）
- 2026-02-20 17:12 UTC：`make generate && make css`（OK，前端构建产物已更新）
- 2026-02-20 17:15 UTC：`make preflight`（失败：`make e2e` 阶段 `kratosstub` 端口 `127.0.0.1:4433` 已被占用，`bind: address already in use`）
