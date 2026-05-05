# DEV-PLAN-520：仓库定期清理与检索噪音治理方案

**状态**: 规划中（2026-05-05 15:17 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T1`
- **范围一句话**：建立仓库级定期清理机制，降低 AI/人工评审时被历史文档、旧设计、生成物、本地运行产物和大平面实现目录干扰的概率，同时不破坏契约可追溯性与 CI 门禁。
- **关联模块/目录**：`AGENTS.md`、`docs/dev-plans/`、`docs/archive/`、`docs/dev-records/`、`designs/`、`.local/`、`coverage/`、`e2e/_artifacts/`、`internal/server/`、`apps/web/`、`scripts/ci/*`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/013-docs-creation-and-governance-guide.md`、`docs/dev-plans/304-test-asset-tiering-and-remediation-plan.md`、`docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`、`docs/dev-plans/440-complete-setid-removal-plan.md`、`docs/dev-plans/521-dev-plan-purpose-classification-and-metadata-taxonomy.md`

### 0.1 Simple > Easy 三问

1. **边界**：本计划只治理“搜索面、文档面、设计面、本地产物面和大平面噪音”，不借机修改业务契约、数据库、授权、路由或 UI 行为。
2. **不变量**：当前有效契约必须可发现；历史来源必须可追溯但不得污染当前实现判断；本地运行产物不得进入默认评审面或入仓面。
3. **可解释**：任一文件应能被快速判断为 `active contract`、`current evidence`、`archive reference`、`generated artifact`、`local runtime artifact` 或 `implementation source`，否则需要补归类或清理。
4. **分类来源**：`docs/dev-plans/` 的 `doc_type`、`status` 与最小元数据口径统一由 `DEV-PLAN-521` 定义；本计划只负责清理节奏、索引收敛和噪音治理。

## 1. 背景与问题陈述

仓库已经具备较强的文档和门禁治理，但随着方案编号、历史归档、设计文件、E2E 产物和前端构建产物增加，AI 与人工评审会遇到两类噪音：

1. **入仓噪音**：历史 dev-plan、dev-record、旧 `.pen`、构建产物、生成代码、大平面实现目录导致搜索命中量高，容易把历史方案误读为当前契约。
2. **本地噪音**：`.local/`、`coverage/`、`e2e/node_modules/`、`e2e/playwright-report/`、`e2e/test-results/` 等目录不一定入仓，但会污染默认命令行搜索和上下文采集。

2026-05-05 本地抽样显示：

| 区域 | 当前观察 | 风险 |
| --- | --- | --- |
| `docs/dev-plans` | 约 150 个入仓文件 | active owner 不易快速定位 |
| `docs/archive/dev-plans` | 约 230 个入仓文件 | 旧方案关键词污染当前搜索 |
| `docs/dev-records` + `docs/archive/dev-records` | 约 215 个入仓文件 | 过程日志易干扰实现评审 |
| `designs` | 6 个入仓 `.pen` 文件 | 旧设计与当前设计源不易区分 |
| `internal/server` | 约 99 个入仓文件 | HTTP 组合层与模块实现职责混杂 |
| `.local` | 本地约 110M | 运行产物污染本地检索 |
| `coverage` / `e2e/node_modules` | 本地约 28M | 非源码内容污染本地检索 |

## 2. 目标与非目标

### 2.1 目标

1. [ ] 建立固定清理节奏：每周轻扫、每月归档、每季度结构复核。
2. [ ] 为活体方案、历史方案、证据记录、设计文件和本地产物建立一致归类规则。
3. [ ] 将默认检索面从“全仓文本”收敛为“当前契约 + 当前实现 + 必要证据”。
4. [ ] 降低历史关键词回流风险，尤其是 `SetID`、`scope_package`、`LibreChat`、`assistant`、`legacy`、`compat` 等已收敛或高风险词。
5. [ ] 让 AI/人工 reviewer 能用稳定入口快速定位 active owner、当前设计源和实现入口。
6. [ ] 所有清理动作可被 `make check doc`、`make check root-surface`、`make check no-legacy` 等既有门禁验证。

### 2.2 非目标

- 不在本计划中实际删除历史文档、设计文件、源码或测试；删除必须通过后续 PR 按本计划执行。
- 不降低覆盖率、文档、root surface、no-legacy、authz 或 DDD 分层门禁。
- 不把 archive 全部清空；仍有架构追溯价值的历史来源可以保留，但必须被明确标记为 archive reference。
- 不把本计划扩张成 `internal/server` DDD 重构实施方案；模块下沉应由具体业务计划承接。
- 不引入外部索引服务、专用文档数据库或复杂治理平台。

## 3. 清理对象与归类规则

### 3.1 文档

| 类别 | 位置 | 规则 |
| --- | --- | --- |
| 当前契约 | `docs/dev-plans/` | 必须有状态行、owner 编号、范围和验收；`doc_type` / `status` / 最小元数据口径见 `DEV-PLAN-521`；新增活体文档必须进入 `AGENTS.md` Doc Map |
| 当前证据 | `docs/dev-records/` | 只保留 readiness、验收记录和仍被当前计划引用的证据 |
| 历史来源 | `docs/archive/` | 仅保留仍有追溯价值的历史方案；不得作为当前实现前提 |
| 过程日志 | `docs/archive/dev-records/` | 可压缩为 summary；不逐条进入 Doc Map |
| 无参考价值历史 | 后续 PR 删除 | 删除前需说明被哪个活体 owner 替代，或说明无当前引用 |

### 3.2 设计文件

1. 当前设计源保留在 `designs/` 或业务子目录的当前路径。
2. 旧设计迁入 `designs/archive/` 或从仓库删除，具体动作由后续 PR 裁决。
3. 方案文档必须声明当前设计源，例如 `design_refs: designs/493.pen` 或正文“关联设计源”。
4. 禁止继续使用 `foo2.pen`、`new.pen`、`final.pen` 一类含糊版本命名作为当前设计源。

### 3.3 本地运行产物

默认不得入仓，且应被默认检索排除：

- `.local/`
- `coverage/`
- `e2e/_artifacts/`
- `e2e/node_modules/`
- `e2e/playwright-report/`
- `e2e/test-results/`
- `.playwright-mcp/`

### 3.4 生成物与构建产物

1. `*_templ.go`、`*/gen/*`、压缩构建 JS 等生成物不得作为人工评审的首选阅读入口。
2. 必须入仓的生成物需要在对应计划或 runbook 中说明原因与再生成命令。
3. 非必须入仓的构建产物应转为构建阶段生成，并由 root surface / generated artifact 门禁阻断回流。

### 3.5 大平面实现目录

`internal/server` 只应承载 HTTP 组合、middleware、route wiring、错误映射和跨模块编排。周期复核时应识别：

1. 可下沉到 `modules/orgunit` 的 OrgUnit 读写、字段配置、projection 逻辑。
2. 可下沉到 `modules/cubebox` 的 CubeBox query/tool/runtime 逻辑。
3. 可下沉到 `modules/iam` 的 IAM/authz/session 逻辑。
4. 仅为测试覆盖而存在的死分支、重复 helper 或同主题碎片化测试。

实际迁移不在本计划中直接执行，必须由对应领域 owner 另起或引用现有 dev-plan。

## 4. 定期执行节奏

### 4.1 每周轻扫

适用：任何开发周结束前或发 PR 前。

1. [ ] 执行 `git status --short`，确认没有意外运行产物、调试文件或未知根目录文件。
2. [ ] 执行 `make check root-surface`，确认根目录 surface 未漂移。
3. [ ] 检查 `.local/`、`coverage/`、`e2e/_artifacts/` 是否只包含本地运行产物，不整理入仓。
4. [ ] 对本周新增文档执行 `make check doc`。
5. [ ] 对本周新增设计文件确认是否有当前 owner 文档引用。

### 4.2 每月归档

适用：每月固定一次，或连续多个 dev-plan 完成后。

1. [ ] 列出本月已完成、已替代、已停止的 `docs/dev-plans/*.md`。
2. [ ] 将已作废且仍需追溯的计划移动到 `docs/archive/dev-plans/`。
3. [ ] 将过程性 dev-record 压缩为 summary，保留 readiness 与验收证据。
4. [ ] 更新 `AGENTS.md` Doc Map，只保留活体入口和必要 archive 入口。
5. [ ] 对旧 `.pen` 文件执行归属复核：当前源、archive、删除候选三选一。
6. [ ] 执行 `make check doc && make check root-surface`。

### 4.3 每季度结构复核

适用：季度末或大型系列计划收口后。

1. [ ] 统计 `docs/dev-plans`、`docs/archive/dev-plans`、`docs/dev-records`、`designs`、`internal/server`、`apps/web/src/pages/*` 文件数量与高频历史关键词命中。
2. [ ] 复核 `SetID`、`scope_package`、`LibreChat`、`assistant`、`legacy`、`compat` 等词在非 archive 路径中的合法性。
3. [ ] 识别 `internal/server` 中可下沉模块的候选文件，转成具体领域计划，不在复核 PR 中直接搬迁。
4. [ ] 复核测试资产命名，阻断 `*_coverage_test.go`、`*_gap_test.go`、`*_extra_test.go`、`*_more_test.go` 式补洞文件继续扩散。
5. [ ] 复核 `.gitignore`、`.rgignore`、root surface allowlist 是否覆盖新增本地产物目录。
6. [ ] 产出季度清理记录，放入 `docs/dev-records/` 或对应 owner 文档的执行记录章节。

## 5. 工具与门禁

### 5.1 推荐默认检索排除

后续实施 PR 应评估新增 `.rgignore` 或等价配置，至少默认排除：

```gitignore
.local/
coverage/
e2e/_artifacts/
e2e/node_modules/
e2e/playwright-report/
e2e/test-results/
.playwright-mcp/
third_party/
**/*_templ.go
**/gen/
internal/server/assets/web/assets/*.js
```

### 5.2 文档元数据收敛

后续实施 PR 应以 `DEV-PLAN-521` 为准，为活体 dev-plan 增加轻量机器可读头部或统一正文段落；本计划只定义落地节奏，不重复定义分类枚举：

```yaml
status: active
owner: DEV-PLAN-520
supersedes: []
implementation_entrypoints: []
design_refs: []
archive_refs: []
```

首期不强制全量回填历史文档，只对新增或被修改的活体计划执行。

### 5.3 必跑门禁

| 改动类型 | 必跑 |
| --- | --- |
| 新增/调整 dev-plan 或 Doc Map | `make check doc` |
| 根目录、本地产物、忽略规则、清理脚本 | `make check root-surface` |
| 删除/归档历史 legacy surface | `make check no-legacy`、按命中面补跑专题门禁 |
| 设计文件移动/删除 | `make check doc`，并确认引用方案同步更新 |
| `internal/server` 下沉或源码调整 | 按 `AGENTS.md` Go/前端/DDD 触发器执行 |

## 6. 删除与归档准入

文件进入删除候选必须满足至少一项：

1. 已被明确活体 owner 替代，且 Doc Map 或方案正文已有替代链接。
2. 属于过程性执行日志，关键信息已压缩到 readiness 或 summary。
3. 属于本地运行产物、调试快照、临时输出或构建缓存。
4. 属于旧设计草稿，当前设计源已明确且不再引用旧文件。
5. 属于生成物且可由既有命令稳定再生成，并且不需要入仓。

不得删除：

1. 仍被当前活体 dev-plan 引用的契约文档。
2. 仍承担 readiness、合规、迁移、授权或历史决策追溯的证据。
3. 当前实现、测试、CI 或构建所需文件。
4. 用户或 reviewer 明确要求保留的调查材料。

## 7. 实施切片

### 阶段 A：检索面止血

1. [ ] 新增 `.rgignore` 或等价配置，排除本地产物、生成物和第三方大目录。
2. [ ] 补充 `scripts/ci/check-root-surface.sh` 或相关文档，明确本地产物允许落点。
3. [ ] 在 `AGENTS.md` 中引用本计划，作为定期清理 owner。

### 阶段 B：活体索引收敛

1. [ ] 建立或强化 `docs/dev-plans/README.md`，作为 active/closed/superseded/archive owner 索引。
2. [ ] 将 `AGENTS.md` Doc Map 从长列表逐步收敛为“入口 + 当前关键 owner”，避免继续无限增长。
3. [ ] 新增/修改 dev-plan 时补最小元数据字段，首期不强制历史全量回填。

### 阶段 C：archive 与 dev-record 压缩

1. [ ] 对 `docs/archive/dev-plans` 按业务域和编号段分组，标记保留原因。
2. [ ] 删除无当前引用、无追溯价值的过程性历史文档。
3. [ ] 将 `docs/archive/dev-records` 中重复过程日志压缩为 summary。

### 阶段 D：设计文件收敛

1. [ ] 建立 `designs/README.md`，列出当前设计源与历史设计源。
2. [ ] 将旧 `.pen` 文件移动到 `designs/archive/` 或删除。
3. [ ] 禁止继续新增无 owner 的 `.pen` 文件。

### 阶段 E：大平面和测试资产复核

1. [ ] 输出 `internal/server` 文件职责归属清单。
2. [ ] 将明显属于模块内部职责的文件纳入对应领域计划。
3. [ ] 合并或重命名补洞式测试文件，遵循 `DEV-PLAN-304`。

## 8. 验收标准

1. [ ] 新增或修改文档后，`make check doc` 通过。
2. [ ] 根目录和本地产物 surface，`make check root-surface` 通过。
3. [ ] 默认 `rg` 不再扫描 `.local/`、`coverage/`、`e2e/node_modules/`、Playwright 报告和生成 JS。
4. [ ] 每个新增活体方案能在 1 分钟内定位到 owner、状态、实现入口和设计引用。
5. [ ] 历史方案不会被误认为当前 PoR；涉及 SetID、scope/package、LibreChat、legacy 的当前命中均有合法解释。
6. [ ] 旧 `.pen` 文件要么有当前 owner，要么进入 archive/delete 候选。
7. [ ] `internal/server` 新增文件能说明为何必须留在 server 层；否则进入模块下沉候选。

## 9. 风险与停止线

1. 若清理动作会删除仍被当前计划、测试、CI 或运行时引用的文件，立即停止并回到 owner 文档裁决。
2. 若 archive 删除会导致关键设计决策不可追溯，必须先压缩成 summary 再删除原过程文档。
3. 若 `.rgignore` 或忽略规则导致门禁、生成、测试遗漏真实源码，必须回滚该忽略项并补白名单。
4. 若清理 PR 开始混入业务行为修改、DB 迁移、授权调整或 UI 重构，必须拆分 PR。
5. 若为了减少噪音而降低门禁、扩大 coverage 排除项或删除真实分支，必须停止并按 `AGENTS.md` 死分支原则重新评估。

## 10. 当前基线与首轮建议

首轮实施建议控制在低风险范围：

1. [ ] 增加默认检索排除配置，先处理本地噪音。
2. [ ] 建立 `docs/dev-plans/README.md` 的 active owner 索引，减少依赖超长 Doc Map。
3. [ ] 建立 `designs/README.md`，标记 `493.pen` 等当前设计源。
4. [ ] 只对新增或本轮修改的 dev-plan 增加最小元数据，不全量回填历史。
5. [ ] 生成 archive 删除候选清单，但首轮不直接大规模删除。

## 11. 关联文档

1. 文档格式：`docs/dev-plans/000-docs-format.md`
2. Simple > Easy 评审：`docs/dev-plans/003-simple-not-easy-review-guide.md`
3. Docs 治理：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
4. 文档分类与元数据：`docs/dev-plans/521-dev-plan-purpose-classification-and-metadata-taxonomy.md`
5. 测试资产分级治理：`docs/dev-plans/304-test-asset-tiering-and-remediation-plan.md`
6. CubeBox 历史对话面硬删除：`docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`
7. SetID 根删除唯一 PoR：`docs/dev-plans/440-complete-setid-removal-plan.md`
