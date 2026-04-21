# DEV-PLAN-015B：DDD 分层框架收口整改路线图（P0/P1/P2，承接 DEV-PLAN-015A）

**状态**: 草拟中（2026-04-09 08:08 CST）

## 背景

`DEV-PLAN-015A` 已确认：当前仓库在 DDD 分层框架上已完成“蓝图 + 骨架”，但仍存在以下主要缺口：

1. [ ] Composition Root 已有文件落点，但尚未真正承接模块装配职责。
2. [ ] `internal/server` 仍承担大量模块默认装配、PG store 实现与 Kernel 访问。
3. [ ] 存在 `infrastructure -> services` 的分层回流。
4. [ ] 当前 `CleanArchGuard` 仅覆盖基础层级映射，尚不足以验证更细颗粒度规则。

因此，需要一份专门承接 `015A` 的整改路线图，把“蓝图已建立、实现仍滞后”的状态拆成可执行的收口阶段。

## 目标与非目标

### 目标

1. [ ] 将 `015A` 中的缺口整理为分阶段、可实施的整改路径。
2. [ ] 明确哪些事项属于 `P0` 止血、哪些属于 `P1` 收口、哪些属于 `P2` 门禁增强。
3. [ ] 固定整改顺序，避免并发重构导致分层边界再次漂移。
4. [ ] 为后续代码 PR 提供统一承接计划，而不是继续零散修补。

### 非目标

1. [ ] 本文不直接修改生产代码。
2. [ ] 本文不在当前阶段要求“一次性把所有模块全部迁完”。
3. [ ] 本文不降低现有门禁阈值，不以放松 lint 替代架构收口。
4. [ ] 本文不改变 `015` / `015A` 的既有定性；本计划仅承接整改顺序。

## 事实源

- 主蓝图：`docs/dev-plans/015-ddd-layering-framework.md`
- 缺口评估：`docs/dev-plans/015a-ddd-layering-framework-implementation-gap-assessment.md`
- 仓库规则入口：`AGENTS.md`
- CI / 质量门禁：`docs/dev-plans/012-ci-quality-gates.md`

## 整改原则

1. [ ] 先收口“正在制造新漂移的入口”，再重构历史存量。
2. [ ] 先把职责边界讲清、挂好，再迁移代码；不做“边迁边猜”的大爆炸重构。
3. [ ] 先收敛装配入口，再收敛模块内部实现，最后再增强门禁。
4. [ ] 所有收口均按“实现向蓝图靠拢”推进，不把个别历史实现直接解释成新的标准。

## 分阶段路线图

### P0：止血阶段（阻断继续扩散）

目标：先阻断新的分层漂移继续进入代码树。

1. [ ] 冻结新增模式：
   - [ ] 禁止新增 `internal/server` 内部新的模块级 PG store 实现。
   - [ ] 禁止新增 `infrastructure -> services` 反向依赖。
   - [ ] 禁止新增把默认模块装配继续堆入 `internal/server/handler.go` 的实现。
2. [ ] 建立最小评审口径：
   - [ ] 所有命中 `modules/*` / `internal/server` 分层边界的 PR，必须引用 `015` 或 `015A/015B`。
   - [ ] 在 code review 中把 “是否继续扩大 `internal/server` 装配职责” 作为显式检查项。
3. [ ] 补最小结构约束文档：
   - [ ] 明确当前允许保留的历史例外点。
   - [ ] 明确哪些新实现必须直接走模块层，而不能再进 `internal/server`。

P0 完成标准：

1. [ ] 新增代码不再继续扩大 `internal/server` 的模块装配与 Kernel 访问职责。
2. [ ] 不再出现新的 `infrastructure -> services` 回流实例。
3. [ ] 评审口径已稳定，不再让 `015A` 的问题继续扩散。

### P1：结构收口阶段（把实现迁回蓝图）

目标：把主要的结构性偏差从 `internal/server` 与历史回流点中迁回模块分层。

1. [ ] Composition Root 收口：
   - [ ] 为当前活体模块 `orgunit`、`iam` 逐步让 `module.go` / `links.go` 真正承接默认装配。
   - [ ] `staffing`、`jobcatalog`、`person` 的相关收口项已随 `DEV-PLAN-450` 退出当前执行面，不再作为现行待办。
   - [ ] 让 `internal/server/handler.go` 从“自己组装模块实现”逐步收敛为“调用模块暴露的装配入口”。
2. [ ] 模块内部实现回迁：
   - [ ] 将 `internal/server` 中属于模块内部的 PG store / Kernel access 逐步迁回模块的 `infrastructure`。
   - [ ] 将属于业务编排的逻辑收敛到模块 `services`，避免 `internal/server` 长期直接持有。
3. [ ] 分层回流修复：
   - [ ] 消除 `modules/staffing/infrastructure/persistence -> modules/staffing/services` 反向依赖。
   - [ ] 视职责拆分情况，把 Prepare/Normalize/Canonicalize 一类逻辑下沉到 `domain`、`pkg/**` 或更小的 seam。

P1 完成标准：

1. [ ] `internal/server` 不再作为主要模块装配入口。
2. [ ] `internal/server` 中历史遗留的模块级 PG store 实现显著减少，并形成明确退出清单。
3. [ ] 已知的 `infrastructure -> services` 回流实例被清理或进入带退出条件的显式例外名单。

### P2：门禁增强阶段（让 lint 开始兜底）

目标：让 `015` 中更细颗粒度的目标，逐步从“文档要求”变成“门禁可验证”。

1. [ ] 评估并补充分层门禁能力：
   - [ ] 识别 `CleanArchGuard` 当前能表达与不能表达的规则边界。
   - [ ] 对不能直接由 `.gocleanarch.yml` 表达的规则，评估是否需要补充仓库脚本门禁。
2. [ ] 优先增强的检查项：
   - [ ] 阻断新增 `internal/server` 直接 import 模块 `infrastructure/presentation` 的实现扩散。
   - [ ] 阻断新增 `modules/*/infrastructure -> modules/*/services` 反向依赖。
   - [ ] 阻断 `module.go` / `links.go` 长期空壳而继续把装配回堆到别处的模式扩散。
3. [ ] 文档与门禁统一：
   - [ ] 明确哪些规则由 `015` 定义、哪些由 `015B` 收口、哪些最终进入 CI gate。
   - [ ] 避免“文档说一套、lint 只查目录名”的长期分裂。

P2 完成标准：

1. [ ] 至少关键的新增漂移模式能够被脚本或 lint 阻断。
2. [ ] `015` / `015A` / `015B` 与实际门禁口径对齐。
3. [ ] 后续架构评审不再主要依赖人工记忆来识别这些已知偏差。

## 优先级判断

### 必须先做的事项

1. [ ] `P0` 必须先完成，否则新增代码会继续扩大历史偏差。
2. [ ] `P1` 中应优先处理 `internal/server` 的默认装配与最典型的回流点。

### 不应抢跑的事项

1. [ ] 在 `P0` 未冻结新增漂移前，不应直接启动大面积模块迁移。
2. [ ] 在 `P1` 尚未形成较稳定结构前，不应急于把所有规则硬塞进 lint。

## 建议拆分顺序

1. [ ] 第一批：`internal/server/handler.go` 默认装配收口。
2. [ ] 第二批：`staffing` 回流修复与模块内装配收口。
3. [ ] 第三批：`setid/jobcatalog` 从 `internal/server` 回迁模块内部实现。
4. [ ] 第四批：门禁增强与文档口径统一。

## 测试与覆盖率

覆盖率与门禁口径遵循仓库 SSOT：

- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`

本计划当前为路线图文档：

1. [ ] 不修改覆盖率阈值，不扩大排除项。
2. [ ] 后续进入代码实施时，仍按命中触发器执行 `go fmt ./...`、`go vet ./...`、`make check lint`、`make test`。
3. [ ] 若涉及分层调整导致测试迁移，须遵循 `300/301` 的分层整治口径，不新增补洞式测试文件。

## 验收标准

1. [ ] 已形成一份清晰的 `P0/P1/P2` 整改路线图。
2. [ ] 已把 `015A` 的缺口转成可执行的后续阶段。
3. [ ] 文档已加入 `AGENTS.md` 的 Doc Map，可被后续实施计划直接引用。
