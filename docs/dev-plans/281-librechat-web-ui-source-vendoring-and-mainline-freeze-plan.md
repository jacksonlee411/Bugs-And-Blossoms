# DEV-PLAN-281：LibreChat Web UI 源码纳管与新主链路冻结实施计划

**状态**: 规划中（2026-03-07 23:55 CST）

## 1. 背景
- 承接 `DEV-PLAN-280` 的 `280A`。
- 目标是在不继续投资旧桥接体系的前提下，完成 LibreChat Web UI 源码纳管、来源锁定、patch stack 骨架与本仓构建链冻结。
- 本计划是后续删除旧桥、切正式入口、源码级发送/渲染接管的前置。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 建立 vendored LibreChat Web UI 单一来源目录、来源元数据与 patch 清单骨架。
2. [ ] 建立本仓可重复构建的 Web UI 构建链与产物出口。
3. [ ] 冻结“新主链路”最小结构：源码目录、patch 入口、构建脚本、静态发布路径。
4. [ ] 自本计划完成后，停止向 `iframe`、`bridge.js`、HTML 注入、页面 helper 旧桥接体系继续投入新功能开发。

### 2.2 非目标
1. [ ] 不在本计划内删除旧桥接代码（由 `DEV-PLAN-282` 承接）。
2. [ ] 不在本计划内切换正式入口（由 `DEV-PLAN-283` 承接）。
3. [ ] 不在本计划内完成发送/渲染主链路源码级接管（由 `DEV-PLAN-284` 承接）。

## 3. 核心交付
1. [ ] `third_party/librechat-web/` 或等价目录落地。
2. [ ] `UPSTREAM.yaml`（repo/ref/imported_at/rollback_ref）落地。
3. [ ] `patches/` 清单目录落地。
4. [ ] `scripts/librechat-web/` 或等价脚本入口落地。
5. [ ] 本仓静态发布/打包路径冻结，并可在本地/CI 复现。

## 3.1 顺序与依赖
1. [ ] `281` 是 `282/283/284/285` 的起点子计划，必须先完成。
2. [ ] `281` 完成前，不应启动任何“正式入口切换”或“旧桥接彻底删除”的封板动作。
3. [ ] `281` 完成后，`282` 与 `235` 可并行推进。

## 3.2 禁止项与冻结点
1. [ ] 禁止在 `281` 期间继续向旧桥接链路增加新功能、测试或文档口径。
2. [ ] 禁止把 vendored UI 计划做成“先导入一份源码，正式入口仍长期停留在旧桥接结构”。
3. [ ] 禁止在未冻结来源元数据与 patch stack 前，直接散改 vendored 源码。

## 3.3 搜索型 stopline
1. [ ] 完成 `281` 时，新增主链路相关目录与脚本必须可被稳定搜索定位。
2. [ ] 完成 `281` 时，不应再出现把旧桥接链路表述为“继续新增功能承载面”的新文档或新代码说明。

## 4. 实施步骤
1. [ ] 选择 vendoring 边界：只纳管 Web UI 必需源码与构建资产，不纳管上游 Node backend runtime。
2. [ ] 导入来源元数据与 patch stack 目录骨架。
3. [ ] 建立本仓构建脚本、产物目录与最小验证命令。
4. [ ] 更新相关文档与文档地图，明确后续一律以 vendored UI 为新主链路基础。
5. [ ] 明确“停止继续投资旧桥”的 stopline，并在后续计划中引用。

## 5. 验收标准
1. [ ] vendored UI 来源、patch stack、构建路径三者均可审计。
2. [ ] 本地/CI 可重复构建 Web UI 产物。
3. [ ] 不存在“继续向旧桥接链路新增功能”的计划口径或实现口径。
4. [ ] `make check doc` 通过。

## 6. 关联文档
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `AGENTS.md`
