# DEV-PLAN-300：全仓测试体系问题调查记录

**状态**: 已完成（2026-04-08 07:21 CST）

## 背景

本记录用于固化对 `Bugs-And-Blossoms` 全仓测试体系的调查结论，范围不限于 Assistant，覆盖 Go 单元/集成测试、E2E、覆盖率门禁与测试基础设施复用情况。

本次调查目标不是修改测试口径，也不是直接给出重构方案，而是先建立一份可引用的基线记录，明确当前测试体系的主要问题、证据与后续整治方向。

## 范围

1. [X] Go 测试：`internal/`、`modules/`、`pkg/`、`cmd/`
2. [X] E2E：`e2e/tests/`
3. [X] CI/覆盖率门禁：`scripts/ci/coverage.sh`、`scripts/ci/test.sh`、`config/coverage/policy.yaml`
4. [X] 文档事实源：`docs/dev-plans/012-ci-quality-gates.md`

## 调查方法

1. [X] 统计全仓测试文件数量、测试代码行数、主要目录分布。
2. [X] 抽样阅读大体量测试文件，识别重复样板、白盒耦合、全局状态覆盖、时间/环境依赖等模式。
3. [X] 抽样阅读 E2E 巨型 spec，识别共享 fixture 缺失与重复装配问题。
4. [X] 验证当前 Go 测试基本可运行：执行 `go test ./... -count=1`。

## 关键事实

### 1. 测试体量分布失衡

- 全仓 Go 测试文件：`184`
- 全仓 Go 测试代码行数：`74756`
- 全仓 E2E 文件：`16`
- 全仓 E2E 代码行数：`7774`
- `internal/server` 测试文件：`142`
- `internal/server` 测试代码行数：`60529`

结论：测试重心明显集中在 `internal/server`，约占全仓 Go 测试代码 `81%`。

### 2. 覆盖率门禁对测试结构的反向塑形明显

- 覆盖率阈值由 [config/coverage/policy.yaml](../../config/coverage/policy.yaml) 单主源定义，当前阈值为 `100%`。
- 执行入口由 [scripts/ci/coverage.sh](../../scripts/ci/coverage.sh) 与 [scripts/ci/test.sh](../../scripts/ci/test.sh) 固定。
- 与 `coverage/gap/more/extra/matrix/lifecycle/policy` 命名模式相关的 Go 测试文件：`61`
- 这类文件代码行数：`28383`
- 这类文件中的测试函数数量：`302`

结论：当前测试体系中有相当比例属于“补洞式/覆盖率导向”测试，而不是围绕稳定边界组织的测试。

### 3. 黑盒测试边界薄弱

- 未发现 `package xxx_test` 形式的黑盒 Go 测试文件。
- 多数业务测试与实现代码位于同包，默认可直接访问内部状态和包级变量。

结论：测试对实现细节天然可见，提升了编写便利，也提高了重构摩擦。

### 4. 并发友好性不足

- 全仓仅有少量包系统性使用 `t.Parallel()`，集中在 `internal/routing`。
- 复杂业务包如 `internal/server`、`modules/orgunit/services` 基本未采用并行测试模式。

结论：项目当前测试更偏顺序执行与共享可变状态，不利于进一步扩展和并发化。

### 5. 全局状态/环境变量覆盖较多

- 使用 `t.Setenv` / `os.Setenv` 的 Go 测试文件：`47`
- 覆盖包级函数、时间源、全局 map 等测试文件：`34`
- 使用 `time.Now()` 的测试文件：`23`

结论：测试可重复性当前尚可，但存在明显的隔离性与顺序依赖风险。

### 6. E2E 共享基础设施缺失

在 `e2e/tests/` 中，以下辅助逻辑存在显著重复：

- `ensureKratosIdentity`：出现 `12` 次
- `setupTenantAdminSession`：出现 `7` 次
- `parseJSONSafe`：出现 `3` 次
- `createIAMSessionWithRetry`：出现 `2` 次
- `waitForOrgUnitDetails`：出现 `2` 次

结论：E2E 的租户初始化、登录装配、数据基线准备尚未沉淀为共享 fixture/helper。

## 主要问题

### 1. `server` 层过重，模块层测试边界不足

当前项目的主要测试压力集中在 `internal/server`，而不是在模块服务层、领域层与端口边界上平均分布。结果是：

- 许多业务规则最终在 `server` 层被验证；
- 模块级边界测试不足，导致问题更晚暴露；
- 一旦 `server` 层重构，回归影响面很大。

典型对照：

- `internal/server`：`142` 个测试文件，`60529` 行
- `modules/orgunit/services`：`9` 个测试文件，`6641` 行
- `internal/routing`：`8` 个测试文件，`1118` 行

### 2. 覆盖率导向测试过多，测试命名本身已暴露结构问题

大量测试以 `coverage`、`gap`、`more`、`extra` 等命名，说明测试常常是在“发现覆盖缺口后补一个文件”，而不是在设计之初围绕稳定职责划分。

这会带来两个后果：

- 测试文件越来越大，阅读门槛越来越高；
- 一处实现改动可能需要同时修改多个“补洞文件”。

该模式不仅存在于 Assistant，也存在于 OrgUnit 等模块。

### 3. 测试对白盒实现细节耦合过深

本次调查中，多处测试直接依赖：

- 包级函数覆盖
- 包级 map 覆盖
- 时间源替换
- 内部 helper 或内部状态断言
- SQL 字符串分支匹配

这种写法适合快速补齐门禁，但不利于后续：

- 重构函数拆分
- 替换底层实现
- 调整 SQL 或状态推进内部步骤

即使外部行为不变，也可能需要同步重写测试。

### 4. 重复 stub / fake / recorder 较多，测试代码本身负担偏重

本次调查中统计到 `55` 处 `errors.New("not implemented")` 式测试占位实现，说明测试内部维护了大量“一次性 stub”。

这类重复主要出现在：

- `internal/server/*_test.go`
- `modules/orgunit/services/*_test.go`

直接后果是：

- 测试代码本身难维护；
- 同一接口在不同文件中被以不同方式 stub；
- 测试基础设施没有沉淀为统一 helper。

### 5. E2E 巨型脚本化，装配复杂度高

当前 E2E 中已经出现多个大文件：

- `tp290b-librechat-live-intent-action-chain.spec.js`：`1964` 行
- `tp288b-librechat-live-task-receipt-contract.spec.js`：`846` 行
- `tp288-librechat-real-entry-evidence.spec.js`：`819` 行
- `tp290-librechat-real-case-matrix.spec.js`：`767` 行

这些文件覆盖了真实流程，但同时承载了大量：

- 租户创建
- 超管登录
- Kratos identity 准备
- 基线数据检查
- 轮询与证据落盘

说明 E2E 目前不只是“验收层”，也在承担较重的测试基础设施职责。

### 6. 并发/模糊/性能测试能力不足

- 仅发现 `pkg/dict` 下的 `2` 个 benchmark。
- 未发现 fuzz 测试。
- `t.Parallel()` 主要集中于 `internal/routing` 这种纯函数/轻状态包。

结论：项目当前对输入鲁棒性、边界随机性和性能回归的专项测试能力偏弱。

### 7. 存在“有代码但无直接测试”的包

通过 `go list -json ./...` 抽样统计，发现存在多个“有生产代码但无直接测试”的包，例如：

- `modules/staffing/services`
- `modules/jobcatalog`
- `modules/person`
- `modules/orgunit/domain/types`

其中部分包代码量很小或偏装配层，但这一事实仍说明当前测试覆盖更偏“上层集成兜底”，而不是“每层都有直接边界测试”。

## Assistant 在全仓中的位置

Assistant 不是孤立问题，而是全仓测试风格的放大镜。

它之所以更突出，是因为它同时叠加了以下因素：

1. 状态机与持久化路径更复杂；
2. 治理与 fail-closed 要求更高；
3. 覆盖率门禁下产生了更多补洞型测试；
4. 同时连接 API、持久化、模型网关、任务执行与 E2E。

因此，Assistant 暴露的问题在全仓都存在，只是在 Assistant 上最集中、最明显。

## 后续建议（调查结论，不代表已批准实施）

1. [ ] 建立全仓测试分层整治计划，目标是把验证责任从 `internal/server` 向模块服务层与端口边界下沉。
2. [ ] 为 E2E 抽取共享 fixture：租户初始化、登录、Kratos identity、基线数据、轮询与证据写入。
3. [ ] 识别高价值补洞型测试文件，优先做“按职责拆分”而不是继续累加 `*_coverage_test.go`。
4. [ ] 为高频全局覆盖点引入可注入依赖，减少测试直接改包级变量。
5. [ ] 为关键纯函数和解析链路补充 fuzz / benchmark / parallel-friendly 测试模式。
6. [ ] 明确哪些“有代码但无直接测试”的包需要补最小边界测试，哪些仅由上层集成覆盖即可接受。

## Go 官方最佳实践对照（2026-04-08 补充）

以下结论基于 Go 官方文档与官方博客，不引入第三方风格指南：

1. **黑盒测试应成为导出边界的默认形态之一**
   - `pkg.go.dev/testing` 明确支持将测试文件置于 `package xxx_test`，并说明这类测试会像外部客户端一样导入被测包，只暴露导出标识符。
   - 对照本仓：`DEV-PLAN-300` 调查时未发现任何 `package xxx_test`，说明导出 API 的稳定边界测试不足。

2. **子测试（subtests）应替代“继续追加 coverage 文件”**
   - Go 官方博客《Using Subtests and Sub-benchmarks》建议使用 `t.Run` / `b.Run` 组织表驱动场景，便于筛选、复用 setup、按场景并行。
   - 对照本仓：大量 `*_coverage_test.go` / `*_gap_test.go` 更像“文件级补洞”，而不是围绕职责/场景组织的子测试。

3. **`t.Parallel()` 适用于隔离良好的纯函数与只读场景**
   - `pkg.go.dev/testing` 说明 `Parallel` 用于声明当前测试可与其他 parallel test 并发执行。
   - 对照本仓：`internal/routing` 这类纯函数包已经能较好并行；`internal/server` 等大包因共享状态较多，不宜直接机械推广。

4. **全局环境修改必须更谨慎，且不能与并行误混**
   - `pkg.go.dev/testing` 对 `Setenv` 的说明明确指出：由于它影响整个进程，不能在 parallel test 或其祖先为 parallel 的测试中使用。
   - 对照本仓：当前 `t.Setenv` / `os.Setenv` 使用较多，说明并行化之前必须先做隔离治理。

5. **fuzz 是官方支持的一等测试能力，适合输入归一化/解析链路**
   - Go 官方 fuzz 教程说明 fuzzing 可以发现开发者未预料的输入边界，并把失败样本沉淀为回归样例。
   - 对照本仓：当前未发现 fuzz 测试，导致输入鲁棒性主要依赖人工枚举。

6. **benchmark 也应回到标准工具链，而不是临时脚本**
   - `pkg.go.dev/testing` 已把 benchmark 作为标准形态，Go 1.26 还提供 `b.Loop()` 让基准测试更直接。
   - 对照本仓：目前 benchmark 极少，尚不足以支撑对热点归一化/解析链路的回归观察。

7. **覆盖率不应只靠“单测补洞”，官方同样支持 integration coverage**
   - Go 官方博客《Code coverage for Go integration tests》说明覆盖率也可以来自集成路径，而不只是把所有压力压回白盒单测。
   - 对照本仓：当前 `100%` 覆盖率阈值已经明显反向塑形测试结构，后续整治应优先调整“测试分层与责任归属”，而不是继续堆同类补洞文件。

## 本仓最小实践（2026-04-08）

为验证上述结论，本次补充了一个低风险试点，不修改任何生产逻辑：

- 新增黑盒测试文件：`pkg/orgunit/normalize_blackbox_test.go`
- 试点对象：`pkg/orgunit.NormalizeOrgCode`
- 采用模式：
  - `package orgunit_test`：只通过导出 API 断言行为，不读取包内私有状态；
  - 表驱动 + `t.Run`：把合法/非法输入归为稳定场景；
  - `t.Parallel()`：仅在纯函数、无共享可变状态的场景开启并行；
  - `FuzzNormalizeOrgCode_BlackBox`：验证归一化函数在随机输入下的错误稳定性与幂等性；
  - `BenchmarkNormalizeOrgCode_BlackBox`：补一个标准 benchmark 样板。

本次实践的意图不是“多写几个测试”，而是验证：对这类纯函数边界，Go 官方推荐形态完全可以在本仓低成本落地。

## 建议落地顺序（最佳实践，按收益/风险排序）

1. **先从 `pkg/**` 与纯函数边界补黑盒测试**
   - 优先对象：归一化、解析、鉴权 mode 解析、路由分类、错误码映射。
   - 原因：最容易应用 `package xxx_test`、`t.Parallel()`、fuzz，且不会牵动 DB/全局状态。

2. **再把模块服务层的稳定端口测试从 `internal/server` 下沉出来**
   - 优先对象：`modules/orgunit/services`、`modules/staffing/services` 中可通过 interface/port 断言的规则。
   - 原因：这是降低 `internal/server` 过重的主路径。

3. **最后处理 `internal/server` 的大体量补洞测试**
   - 做法：不是继续拆更多 `*_coverage_test.go`，而是先识别共享 fixture、可复用 fake、可抽出的纯函数，再重组。
   - 原因：这里耦合最深，直接大改风险最高。

4. **E2E 单独按“fixture 化”治理，不与 Go 单测问题混做**
   - 重点先抽 `ensureKratosIdentity`、`setupTenantAdminSession`、基线数据准备与轮询 helpers。
   - 原因：E2E 的主要问题是装配重复，而不是是否采用黑盒测试包。

## 官方参考

- `testing` 包文档：<https://pkg.go.dev/testing>
- Go 官方博客《Using Subtests and Sub-benchmarks》：<https://go.dev/blog/subtests>
- Go 官方教程《Fuzzing》：<https://go.dev/doc/tutorial/fuzz>
- Go 官方博客《Code coverage for Go integration tests》：<https://go.dev/blog/integration-test-coverage>

## 命令记录

以下命令用于本次调查取证：

1. [X] `go test ./... -count=1`
   - 时间：`2026-04-08 07:21 CST`
   - 结果：通过
2. [X] 测试体量统计
   - 命令：`python3` 遍历 `*_test.go` 与 `e2e/tests/*.js`
   - 结果：完成全仓测试文件/行数/目录分布统计
3. [X] E2E helper 重复统计
   - 命令：`python3` 搜索 `ensureKratosIdentity`、`setupTenantAdminSession` 等 helper
   - 结果：确认存在明显重复
4. [X] 黑盒测试与并发测试模式扫描
   - 命令：`rg -n '^package .*_test$' --glob '*_test.go'`、`rg -n 't\\.Parallel\\(' --glob '*_test.go'`
   - 结果：未发现黑盒测试文件；`t.Parallel()` 主要集中在 `internal/routing`

## 交付物

1. [X] 本调查记录：[docs/dev-plans/300-test-system-investigation-report.md](300-test-system-investigation-report.md)
2. [X] 文档入口更新：`docs/dev-records/README.md`
3. [X] 文档地图更新：`AGENTS.md`
