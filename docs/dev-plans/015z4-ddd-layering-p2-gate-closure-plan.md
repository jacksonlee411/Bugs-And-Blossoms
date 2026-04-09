# DEV-PLAN-015Z4：DDD 分层 P2 组合根门禁封板（承接 DEV-PLAN-015Z）

**状态**: 已完成（2026-04-09 14:08 CST）

## 背景

`DEV-PLAN-015Z` 已将 `015` 剩余尾巴收敛为两类：

1. [ ] 少量高耦合的 `setid/dict` server store 存量。
2. [ ] `P2` 层面的更细颗粒度门禁尚未封板。

其中，`DEV-PLAN-015B` 已明确把以下事项列为 `P2` 目标：

1. [ ] 让更细颗粒度的分层目标从“文档要求”进入“门禁可验证”。
2. [ ] 阻断 `module.go` / `links.go` 长期空壳而继续把装配回堆到别处的模式扩散。

当前仓库的实际状态是：

1. [X] `module.go` 已在多个模块开始承接默认装配。
2. [ ] 各模块 `links.go` 仍基本是 package-only 空壳。
3. [ ] 若后续直接继续扩张 `modules/*/presentation/**` 或其他分层实现，而组合根文件仍完全不动，会继续形成“有分层目录、无组合根承接”的名实偏差。

因此，本计划将 `015B/P2` 中最小、最稳、最可执行的一条规则先落为增量门禁。

## 目标与非目标

### 目标

1. [X] 新增一个 P2 组合根增量门禁，不追杀历史空壳存量。
2. [X] 当模块 `domain/services/infrastructure/presentation` 发生新增改动时，要求相应 `module.go` / `links.go` 已不是 package-only 空壳。
3. [X] 将该门禁接入 `make check lint` 与 `make preflight`。
4. [X] 在 `AGENTS.md` 中补齐入口、触发器与 Doc Map。

### 非目标

1. [ ] 本计划不直接清空所有既有 `links.go` 空壳。
2. [ ] 本计划不把 `setid/dict` 的剩余 server store 一并迁走。
3. [ ] 本计划不要求 `links.go` 立即承接全部路由挂载历史存量。
4. [ ] 本计划不修改 `.gocleanarch.yml`。

## 规则口径

本次新增门禁名称：

- [X] `make check ddd-layering-p2`

本次仅检查变更集中的 Go 生产代码，且只阻断以下新增扩张模式：

1. [X] 当 `modules/<m>/domain/**`、`services/**`、`infrastructure/**` 出现改动时：
   - [X] `modules/<m>/module.go` 不得缺失。
   - [X] `modules/<m>/module.go` 不得仍是 package-only 空壳。
2. [X] 当 `modules/<m>/presentation/**` 出现改动时：
   - [X] `modules/<m>/links.go` 不得缺失。
   - [X] `modules/<m>/links.go` 不得仍是 package-only 空壳。

### 增量策略

1. [X] 门禁仅看当前变更集，不因仓库里已有空壳文件直接全红。
2. [X] 只有在模块分层继续扩张时，才要求组合根同步承接。
3. [X] 这样既符合 `015B/P2` 的“阻断扩散”，也避免把历史补账和新改动绑成一刀。

## 实施步骤

1. [X] 新增 `scripts/ci/check-ddd-layering-p2.sh`。
2. [X] 在 `Makefile` 中新增 `ddd-layering-p2` 目标，并接入 `preflight` 与 `lint`。
3. [X] 在 `AGENTS.md` 中补充 TL;DR、触发器矩阵与 Doc Map。
4. [X] 完成最小自检：
   - [X] `make check ddd-layering-p2`
   - [X] `make check doc`
   - [X] `make check lint`（2026-04-09 14:08 CST，本地通过）

## 测试与覆盖率

覆盖率与门禁口径遵循仓库 SSOT：

- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`

本次变更属于门禁/文档收口：

1. [X] 不降低覆盖率阈值。
2. [X] 不扩大排除项。
3. [X] 以脚本自检与 `lint/preflight` 接线验证为主。

## 验收标准

1. [X] `make check ddd-layering-p2` 可执行并给出稳定结果。
2. [X] `make check lint` 与 `make preflight` 已接入该门禁。
3. [X] `AGENTS.md` 已能指向该门禁与 `015Z4` 文档。
4. [X] 本门禁只阻断“模块继续扩张但组合根仍空壳”的新增漂移。
