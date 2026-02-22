# DEV-PLAN-126：Go 1.26 升级与现代化改造计划（No-Compat）

**状态**: 已完成（2026-02-20 15:45 UTC）

## 1. 背景

当前仓库 Go 基线仍为 `1.24.10`（`go.mod`、`.tool-versions`），与“持续使用受支持版本 + 积极采用新能力”的目标不一致。  
Go 1.26 已于 **2026-02-10** 发布，且带来明确的语言、工具链、运行时与测试能力提升。

本计划定义一次 **不考虑向后兼容** 的升级：统一到 Go 1.26，并主动将代码与工具链迁移到 1.26 推荐范式。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. [X] 仓库运行时与 CI 工具链统一到 Go 1.26（单一版本口径）。
2. [X] 代码层面主动采用 Go 1.26 新范式（不仅“能编过”，而是“用起来”）。
3. [X] 开发工具链（sqlc/goimports/goose 等）与 Go 版本口径收敛，消除版本漂移。
4. [X] 文档 SSOT（`AGENTS.md`、`DEV-PLAN-011`、相关执行记录）完成同步。
5. [X] 全量门禁保持通过（`make preflight`）。
6. [X] 建立 Go 版本防回退门禁：阻断 `go.mod`/`.tool-versions` 从 `1.26.x` 漂移到更低版本。

### 2.2 非目标（Stopline）

1. 不保留 Go 1.24/1.25 的兼容分支、条件编译分叉或双链路脚本。
2. 不新增“临时回退开关”绕过 Go 1.26 新行为。
3. 不在本计划内处理与 Go 无关的功能需求扩展。

### 2.3 新特性采用原则（冻结）

1. [X] 新特性采用以“可读性/可维护性/可测试性收益”为先，不做机械替换。
2. [X] 允许非兼容重构，但每批改动必须可回归验证、可代码评审解释。

### 2.4 工具链权威表达（冻结）

1. [X] Go 工具（`sqlc`/`goimports`/`goose`）版本统一以 `go.mod`（tool directives）为唯一事实源。
2. [X] `Makefile` 仅保留命令入口，不再重复声明上述 Go 工具版本常量。
3. [X] 非 Go 工具（如 Atlas）版本仅保留一个权威入口（`Makefile`），脚本不得再声明第二套版本默认值。

### 2.5 `go mod init` 默认回退风险应对（补充，2026-02-22）

1. [X] 明确风险：在 Go 1.26 工具链下，新建模块执行 `go mod init` 可能默认写入更低 `go` 版本，导致版本口径漂移。
2. [X] 固化新模块初始化动作：`go mod init` 后立即执行 `go get go@1.26.0`（或 `go mod edit -go=1.26.0`）。
3. [X] 固化提交流程门禁：提交前必须通过 `make check go-version`（或直接 `make preflight`）。

## 3. 升级范围与影响面

### 3.1 影响范围

1. **版本入口**：`go.mod`、`.tool-versions`。
2. **CI**：`.github/workflows/quality-gates.yml`（当前由 `go-version-file: go.mod` 驱动，主要是验证而非结构改造）。
3. **工具链脚本**：`scripts/sqlc/*.sh`、`scripts/db/run_goose.sh`、`scripts/go/verify-tools.sh`、`scripts/ci/check-go-version.sh`、`Makefile`。
4. **代码现代化**：`internal/**`、`modules/**`、`pkg/**`（尤其错误处理、测试与基准）。
5. **文档**：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`、本计划执行记录（`docs/dev-records/`）。

### 3.2 影响评估（初判）

1. 构建/CI：低风险（当前 CI 已跟随 `go.mod`）。
2. 代码编译兼容：低风险（已用 Go 1.26 本地跑通 `go test ./...`、`go vet ./...`）。
3. 代码改造工作量：中等（主动拥抱新特性会引入批量重构）。
4. 文档与流程收敛：中等（需同步多个 SSOT 文档与计划状态）。

## 4. Go 1.26 特性采纳清单（必须落地）

> 原则：优先落地“可稳定提升可读性/可维护性/可测试性”的能力；实验特性只在受控范围试点。

### 4.1 语言与类型系统

1. [X] 在合适位置采用 `new(expr)`（替换“临时变量 + 取地址”的旧写法）。
2. [X] 在需要递归约束的泛型场景，采用 Go 1.26 放宽后的自引用泛型约束，减少样板约束代码。

### 4.2 标准库与测试能力

1. [X] 将 `errors.As(err, &target)` 的样板模式逐步替换为 `errors.AsType[T](err)`。
2. [X] 需要产物留存的测试统一使用 `t.ArtifactDir()`/`b.ArtifactDir()`/`f.ArtifactDir()`。
3. [X] 新增或改造基准测试统一采用 `B.Loop()` 风格（不再新增 `B.N` 风格基准）。

### 4.3 工具链与现代化自动改造

1. [X] 执行 `go fix` 现代化改造（基于 Go 1.26 新版 modernizers），并按模块审阅落地。
2. [X] 将“人工散落的风格升级”改为“可重复执行的 modernizer + review 清单”流程。

### 4.4 运行时与性能（积极拥抱）

1. [X] 以 Go 1.26 默认 GC（Green Tea）作为新基线，不回退旧 GC。
2. [X] 新增一次基准对比（至少核心请求链路），记录升级后 CPU/内存/GC 开销变化。
3. [X] 固定性能口径与阈值：`ns/op`、`allocs/op`、`B/op`、关键接口 `P95`，无业务理由不接受 >5% 回退。
4. [X] 评估并接入新增运行时指标（如 `/sched/*`）到现有诊断脚本（以轻量方式，不做过度运维）。

## 5. 实施步骤（分阶段）

### Phase 0：契约冻结与准备

1. [X] 冻结“Go 1.26 单版本”原则：不接受向后兼容需求。
2. [X] 在 `docs/dev-records/dev-plan-126-execution-log.md` 建立并维护执行记录（命令/结果/时间戳）。
3. [X] 明确升级前基线：记录 `go test ./...`、`go vet ./...`、`make check lint`、`make test` 结果。

### Phase 1：版本升级与入口统一

1. [X] `go.mod` 升级到 `go 1.26.0`（`toolchain` directive 由 `go mod tidy` 自动裁剪，仓库当前不持久化该行）。
2. [X] `.tool-versions` 升级到 `golang 1.26.0`。
3. [X] 建立 patch 升级节奏：每月跟进 Go 1.26.x 最新 patch（同计划内无兼容保留）。
4. [X] 若存在 Go 构建镜像入口（Dockerfile/CI image），统一到 1.26 对应镜像版本。
5. [X] 运行 `go mod tidy`，确认 `go.sum` 与依赖图一致。

### Phase 2：工具链收敛（重点）

1. [X] 在 `go.mod` 增加并冻结 `sqlc`/`goimports`/`goose` 的 tool directives，统一使用 `go tool <name>` 执行。
2. [X] 脚本改造为“显式版本校验 + 不匹配即失败”（禁止仅凭二进制存在即通过）。
3. [X] 删除 `Makefile` 中 Go 工具版本常量，消除 `go.mod`、脚本、文档三套版本并存。
4. [X] 更新 `DEV-PLAN-011` 对应版本表与 drift 清单状态。

### Phase 3：代码现代化改造（主动拥抱）

1. [X] 执行 `go fix ./...` 并按目录分批提交（避免超大 PR）。
2. [X] 错误处理改造：优先替换 `internal/server/**`、`modules/**` 中高频 `errors.As` 样板。
3. [X] 测试改造：为需要落盘证据的测试接入 `ArtifactDir`，并规范产物路径。
4. [X] 基准建设：为关键路径新增 `B.Loop` 基准（至少覆盖 API 解析/核心服务逻辑 2 个热点）。
5. [X] 为每批 modernizer 改造定义失败判定与回退准则：行为变更/可读性下降/性能回退超阈值即停止并回滚该批。

### Phase 4：验证与门禁

1. [X] `go fmt ./...`
2. [X] `go vet ./...`
3. [X] `make check lint`
4. [X] `make check go-version`
5. [X] `make test`
6. [X] `make preflight`
7. [X] 覆盖门禁在 Go 1.26 下保持稳定（`scripts/ci/coverage.sh` 收敛为“仅执行有测试文件的包”，继续使用全量 `coverpkg` 口径）。
8. [X] 生成并归档升级前后对比数据（测试耗时、关键基准、二进制体积可选）。
9. [X] 性能证据口径固定：同机型/同负载下执行 `go test -run=^$ -bench . -benchmem -count=10`，并用 `benchstat` 对比基线与升级后结果。

### Phase 5：文档收口

1. [X] 更新 `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`（Go 基线与相关说明）。
2. [X] 必要时更新 `docs/dev-plans/012-ci-quality-gates.md` 的工具链口径引用。
3. [X] 在 `AGENTS.md` 文档地图中登记执行记录链接（若新增）。
4. [X] 将本计划状态推进为“进行中/已完成”并同步时间戳。

## 6. 验收标准（DoD）

1. [X] 仓库内不存在 `1.24.x` Go 基线残留（代码/脚本/SSOT 文档）。
2. [X] CI 与本地命令在 Go 1.26 下全部通过（以 `make preflight` 为准）。
3. [X] 至少完成 3 项“Go 1.26 新范式”落地（`new(expr)` / `errors.AsType` / `ArtifactDir` / `B.Loop` / `go fix modernizers`）。
4. [X] 工具链安装与执行口径单一、可复现，不再依赖“本机残留二进制”。
5. [X] `DEV-PLAN-126` 执行记录完整可追溯（命令、时间、结果、结论）。
6. [X] 版本门禁可阻断 `go.mod`/`.tool-versions` 回退，并将“新建模块后固定 Go 1.26”固化为团队默认流程。

## 7. 风险与应对

1. **modernizer 批量改动面大**：按目录分批提交，保持每批可回归验证。
2. **工具链版本漂移回潮**：将版本声明收敛到少数 SSOT（`go.mod` + `Makefile` + `DEV-PLAN-011`）。
3. **团队习惯阻力（旧写法惯性）**：通过 lint/review 清单把新范式变成默认约束。
4. **性能波动**：升级前后保留基准证据，以数据驱动修正，不引入兼容分支。

## 8. 关联 SSOT

- `AGENTS.md`
- `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`

## 9. 参考资料（Go 官方）

- Go 1.26 Release Notes: https://go.dev/doc/go1.26
- Go 1.26 发布博客（2026-02-10）: https://go.dev/blog/go1.26
- Go Release History / Release Policy: https://go.dev/doc/devel/release
