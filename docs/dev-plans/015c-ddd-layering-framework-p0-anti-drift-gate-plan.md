# DEV-PLAN-015C：DDD 分层框架 P0 反漂移门禁实施计划（承接 DEV-PLAN-015B）

**状态**: 已完成（2026-04-09 08:36 CST）

## 背景

`DEV-PLAN-015B` 已将 DDD 分层框架收口拆为 `P0/P1/P2` 三阶段，其中 `P0` 的目标是先止血，阻断新的分层漂移继续进入代码树。

当前最需要先阻断的新增模式有三类：

1. [ ] 在 `internal/server` 新增模块级 `infrastructure` / `presentation` 依赖，继续扩大“服务器层直接吃模块实现”的范围。
2. [ ] 在 `internal/server` 新增模块级 PG store / Kernel 调用实现，继续扩大“模块内部实现堆在 server”的范围。
3. [ ] 在 `modules/*/infrastructure/**` 新增对同模块 `services` 的反向依赖，继续扩大 `infrastructure -> services` 回流。

本计划用于将上述 `P0` 目标落实为最小、增量、可执行的反漂移门禁。

## 目标与非目标

### 目标

1. [ ] 新增一个仅阻断“新增漂移”的 P0 门禁，不一次性追杀历史存量。
2. [ ] 将该门禁接入 `make check lint` 与 `make preflight`，使其进入日常开发与 CI 主链。
3. [ ] 在 `AGENTS.md` 中补齐该门禁的入口与触发器说明，确保可发现。

### 非目标

1. [ ] 本计划不直接迁移 `internal/server` 既有模块实现。
2. [ ] 本计划不直接修复现存 `infrastructure -> services` 历史回流。
3. [ ] 本计划不扩写 `.gocleanarch.yml`；本次以独立脚本门禁先完成 P0 止血。
4. [ ] 本计划不把“有历史例外”当成放弃门禁的理由。

## 方案

### 门禁名称

- [ ] `make check ddd-layering-p0`

### 阻断口径

本次仅阻断新增行中的以下模式：

1. [ ] `internal/server/*.go` 新增 import `modules/*/infrastructure/**`
2. [ ] `internal/server/*.go` 新增 import `modules/*/presentation/**`
3. [ ] `internal/server/*.go` 新增模块级 PG store 结构或构造器（例如 `type *PGStore struct`、`func new*PGStore`）
4. [ ] `internal/server/*.go` 新增直接 Kernel 写入口标记（`submit_*_event`、`apply_*_logic`）
5. [ ] `modules/<m>/infrastructure/**/*.go` 新增 import `modules/<m>/services`

### 增量策略

1. [ ] 门禁仅检查新增内容，不因历史存量直接让仓库全红。
2. [ ] 对已有历史点，后续由 `015B P1` 承接迁移。
3. [ ] 对新增偏差，P0 阶段直接阻断，不再允许“先加进去以后再收”。

## 实施步骤

1. [X] 新建 `015C` 文档，冻结 P0 门禁范围与非目标。
2. [X] 新增 `scripts/ci/check-ddd-layering-p0.sh`，实现增量反漂移扫描。
3. [X] 在 `Makefile` 中新增 `ddd-layering-p0` 目标，并接入 `preflight` 与 `lint`。
4. [X] 在 `AGENTS.md` 中补充 TL;DR、触发器矩阵与 Doc Map 入口。
5. [X] 执行最小自检：`make check ddd-layering-p0`、`make check doc`、`make check lint`（2026-04-09 08:36 CST，本地通过）

## 测试与覆盖率

覆盖率与门禁口径遵循仓库 SSOT：

- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`

本计划为门禁/文档变更：

1. [ ] 不调整覆盖率阈值。
2. [ ] 不扩大排除项。
3. [ ] 以脚本自检与 `make check lint` 接线验证为主。

## 验收标准

1. [ ] `make check ddd-layering-p0` 可执行并给出稳定结果。
2. [ ] `make check lint` 与 `make preflight` 已接入该门禁。
3. [ ] `AGENTS.md` 已能指向该门禁与 `015C` 文档。
4. [ ] 门禁只阻断新增漂移，不因历史存量直接失控。
