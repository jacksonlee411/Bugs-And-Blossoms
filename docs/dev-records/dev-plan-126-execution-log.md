# DEV-PLAN-126 执行日志

## 2026-02-20（UTC）

- 2026-02-20 13:58 UTC：新增 `docs/dev-plans/126-go-1-26-upgrade-and-modernization-plan.md`（初稿，状态：规划中）。
- 2026-02-20 13:59 UTC：更新 `AGENTS.md` 文档地图，登记 DEV-PLAN-126 链接。
- 2026-02-20 14:00 UTC：执行 `make check doc`（OK）。
- 2026-02-20 14:01 UTC：根据评审意见修订 DEV-PLAN-126：冻结 `1.26.0`、执行记录路径、特性采用原则与性能阈值。
- 2026-02-20 14:14 UTC：按 `DEV-PLAN-003` 评审意见修订 DEV-PLAN-126：冻结工具链单一权威表达、补充 modernizer 失败/回退准则、补充性能采样与 benchstat 口径。
- 2026-02-20 14:20 UTC：升级版本入口：`go.mod` → `go 1.26.0`，`.tool-versions` → `golang 1.26.0`（`toolchain` 行经 `go mod tidy` 自动裁剪，不持久化）。
- 2026-02-20 14:24 UTC：执行 `go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 golang.org/x/tools/cmd/goimports@v0.38.0 github.com/pressly/goose/v3/cmd/goose@v3.26.0`（OK，写入 tool directives）。
- 2026-02-20 14:28 UTC：工具链收敛：删除 `scripts/sqlc/install_sqlc.sh`、`scripts/sqlc/install_goimports.sh`、`scripts/db/install_goose.sh`；新增 `scripts/go/verify-tools.sh`；`sqlc`/`goose` 执行切换到 `go tool`。
- 2026-02-20 14:30 UTC：执行 `./scripts/go/verify-tools.sh all`（OK）。
- 2026-02-20 14:31 UTC：执行 `make sqlc-generate`（OK）。
- 2026-02-20 14:32 UTC：执行 `./scripts/db/run_goose.sh -version`（OK，`v3.26.0`）。
- 2026-02-20 14:35 UTC：执行 `go fix ./...`（OK，完成 modernizers 批量改造）。
- 2026-02-20 14:40 UTC：落地 Go 1.26 新能力：`errors.AsType`（服务层错误映射）、`t.ArtifactDir()`（测试产物目录）、`B.Loop`（基准测试）。
- 2026-02-20 14:42 UTC：执行 `make check fmt`（OK）。
- 2026-02-20 14:44 UTC：执行 `go test ./...`（OK）。
- 2026-02-20 14:45 UTC：执行 `go test ./pkg/dict -run=^$ -bench . -benchmem -count=10`（OK，产出基准数据）。
- 2026-02-20 15:05 UTC：首次执行 `make preflight`（失败：e2e 阶段 `kratosstub`/`server` 端口占用导致启动冲突，非代码回归）。
- 2026-02-20 15:08 UTC：清理占用进程后执行 `make e2e`（OK，7/7 通过）。
- 2026-02-20 15:22 UTC：再次执行 `make preflight`（OK，全量门禁通过）。
- 2026-02-20 15:23 UTC：更新 DEV-PLAN-126 状态为“已完成”，并将全部实施项勾选为完成。
- 2026-02-20 15:29 UTC：复核时发现 Go 1.26 下 `go test -coverprofile` 对“无测试文件包”触发 `go tool covdata` 报错（`no such tool "covdata"`），导致 `make preflight` 失败。
- 2026-02-20 15:34 UTC：修复覆盖门禁脚本 `scripts/ci/coverage.sh`：`coverpkg` 口径保持不变，但测试执行目标收敛为“仅含 `*_test.go` 的包”，避免无测试包触发 covdata 路径。
- 2026-02-20 15:41 UTC：执行 `make test`（OK，覆盖门禁 100% 通过）。
- 2026-02-20 15:44 UTC：执行 `make preflight`（OK，全量门禁通过，含 e2e 7/7）。
- 2026-02-20 15:45 UTC：更新 DEV-PLAN-126 完成时间戳，记录上述收口修复。
