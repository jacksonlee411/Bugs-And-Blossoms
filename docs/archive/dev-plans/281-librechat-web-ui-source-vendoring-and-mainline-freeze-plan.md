# DEV-PLAN-281：LibreChat Web UI 源码纳管与新主链路冻结实施计划

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-07 CST，已完成来源冻结、patch stack、构建链与可重复构建验证）

## 0. 当前实施进度（2026-03-07）
- [x] `third_party/librechat-web/` 骨架已落地。
- [x] `UPSTREAM.yaml` 已冻结到 `refs/tags/v0.8.0`（commit `b7d13cec6f3a63c7b81f5781f6b5cab289e33d70`）。
- [x] 上游 LibreChat Web UI 前端源码快照已导入 `third_party/librechat-web/source/`。
- [x] `patches/series` 与 `scripts/librechat-web/{verify,build}.sh` 已落地。
- [x] patch stack 首个补丁 `0001-use-node-fs-in-post-build.patch` 已落地，用于消除未声明的 `fs-extra` 依赖。
- [x] 静态产物出口已冻结并完成首次成功构建：`internal/server/assets/librechat-web/`。
- [x] `make librechat-web-build` 已在本地通过；构建链采用“临时目录构建 + patch 回放 + 产物复制”，不直接污染 `source/`。

## 1. 背景
- 承接 `DEV-PLAN-280` 的 `280A`。
- 目标是在不继续投资旧桥接体系的前提下，完成 LibreChat Web UI 源码纳管、来源锁定、patch stack 骨架与本仓构建链冻结。
- 本计划是后续删除旧桥、切正式入口、源码级发送/渲染接管的前置。

## 2. 目标与非目标
### 2.1 目标
1. [X] 建立 vendored LibreChat Web UI 单一来源目录、来源元数据与 patch 清单骨架。
2. [X] 建立本仓可重复构建的 Web UI 构建链与产物出口。
3. [X] 冻结“新主链路”最小结构：源码目录、patch 入口、构建脚本、静态发布路径。
4. [X] 自本计划完成后，停止向 `iframe`、`bridge.js`、HTML 注入、页面 helper 旧桥接体系继续投入新功能开发。

### 2.2 非目标
1. [ ] 不在本计划内删除旧桥接代码（由 `DEV-PLAN-282` 承接）。
2. [ ] 不在本计划内切换正式入口（由 `DEV-PLAN-283` 承接）。
3. [ ] 不在本计划内完成发送/渲染主链路源码级接管（由 `DEV-PLAN-284` 承接）。

## 3. 核心交付
1. [X] `third_party/librechat-web/` 或等价目录落地。
2. [X] `UPSTREAM.yaml`（repo/ref/imported_at/rollback_ref）落地。
3. [X] `patches/` 清单目录落地。
4. [X] `scripts/librechat-web/` 或等价脚本入口落地。
5. [X] 本仓静态发布/打包路径冻结，并可在本地/CI 复现。

## 3.1 顺序与依赖
1. [X] `281` 是 `282/283/284/285` 的起点子计划，必须先完成。
2. [X] `281` 完成前，不应启动任何“正式入口切换”或“旧桥接彻底删除”的封板动作。
3. [X] `281` 完成后，`282` 与 `235` 可并行推进。

## 3.2 禁止项与冻结点
1. [X] 禁止在 `281` 期间继续向旧桥接链路增加新功能、测试或文档口径。
2. [X] 禁止把 vendored UI 计划做成“先导入一份源码，正式入口仍长期停留在旧桥接结构”。
3. [X] 禁止在未冻结来源元数据与 patch stack 前，直接散改 vendored 源码。

## 3.3 搜索型 stopline
1. [X] 完成 `281` 时，新增主链路相关目录与脚本必须可被稳定搜索定位。
2. [X] 完成 `281` 时，不应再出现把旧桥接链路表述为“继续新增功能承载面”的新文档或新代码说明。

## 4. 实施步骤
1. [X] 选择 vendoring 边界：只纳管 Web UI 必需源码与构建资产，不纳管上游 Node backend runtime。
2. [X] 导入来源元数据与 patch stack 目录骨架。
3. [X] 建立本仓构建脚本、产物目录与最小验证命令。
4. [X] 更新相关文档与文档地图，明确后续一律以 vendored UI 为新主链路基础。
5. [X] 明确“停止继续投资旧桥”的 stopline，并在后续计划中引用。

## 5. 验收标准
1. [X] vendored UI 来源、patch stack、构建路径三者均可审计。
2. [X] 本地/CI 可重复构建 Web UI 产物（两轮构建产物 `sha256` 对比一致）。
3. [X] 不存在“继续向旧桥接链路新增功能”的计划口径或实现口径。
4. [X] `make check doc` 通过。

## 6. 关联文档
- `docs/archive/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/archive/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/archive/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/archive/dev-records/dev-plan-281-execution-log.md`
- `AGENTS.md`
