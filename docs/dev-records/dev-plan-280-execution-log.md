# DEV-PLAN-280 执行日志：LibreChat Web UI 源码纳管与 Runtime 分层复用主计划收口

> 对应计划：`docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`

## 1. 执行时间

- 主计划建立与分解：2026-03-07 至 2026-03-08（CST）
- 子计划实施窗口：2026-03-07 至 2026-03-10（CST）
- 主计划收口回写：2026-03-16（CST）

## 2. 主计划完成态汇总

1. `280A / DEV-PLAN-281` 已完成来源冻结与构建基线。
- `third_party/librechat-web/`、`UPSTREAM.yaml`、`patches/series`、`scripts/librechat-web/` 与静态产物出口已落地。
- vendored Web UI 的来源、patch stack 与可重复构建路径已冻结，不再继续投资旧桥接主链。

2. `280B / DEV-PLAN-282` 已完成旧桥接职责删除。
- `iframe` 正式承载、`bridge.js` 注入、HTML rewrite、外挂消息流与页面级业务编排职责已退役。
- 旧桥专用测试、文案与入口语义已随主链切换一并收口。

3. `280C / DEV-PLAN-283` 已完成正式入口唯一化。
- `/app/assistant/librechat` 已冻结为唯一正式聊天入口。
- `/assets/librechat-web/**` 已冻结为唯一正式静态资源前缀。
- 历史 `/assistant-ui/*` 与 `/app/assistant` 不再承担正式聊天职责。

4. `280D / DEV-PLAN-284` 已完成 send/store/render 源码级接管。
- 正式入口下的发送、消息渲染、候选/确认/提交回执已切入本仓 Assistant 主链。
- 官方消息树成为唯一用户可见渲染面，前端仅消费后端 DTO。

5. `280E / DEV-PLAN-285` 已完成总封板回归与归档一致性收口。
- `240F/288/288B/290/290B/291/235` 的封板输入已统一汇总。
- 已形成 `285-readiness-checklist.md`、`285-cutover-closure-matrix.md`、`285-stopline-search-report.md`、`285-execution-report.md`、`285-evidence-index.json` 与 `285-final-closure-report.md`。

## 3. 关键证据索引

- 主计划：`docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `280A`：`docs/dev-plans/281-librechat-web-ui-source-vendoring-and-mainline-freeze-plan.md`
- `280A` 执行记录：`docs/archive/dev-records/dev-plan-281-execution-log.md`
- `280B`：`docs/dev-plans/282-librechat-old-bridge-deletion-plan.md`
- `280C`：`docs/dev-plans/283-librechat-formal-entry-cutover-plan.md`
- `280D`：`docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `280E`：`docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `280E` 封板资产：
  - `docs/dev-records/assets/dev-plan-285/285-readiness-checklist.md`
  - `docs/dev-records/assets/dev-plan-285/285-cutover-closure-matrix.md`
  - `docs/dev-records/assets/dev-plan-285/285-stopline-search-report.md`
  - `docs/dev-records/assets/dev-plan-285/285-execution-report.md`
  - `docs/dev-records/assets/dev-plan-285/285-final-closure-report.md`
- 跨计划总路线：`docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`

## 4. 完成态判定

1. `DEV-PLAN-280` 的主计划目标是“把 LibreChat 从旧桥接黑盒接入，收口到 vendored Web UI + runtime 分层复用 + 正式入口唯一化 + 源码级消息主链”。
2. 上述目标已经由 `281/282/283/284/285` 全部落地：
- 来源与构建链已冻结。
- 旧桥正式职责已删除。
- 正式入口已唯一化。
- send/store/render 已完成源码级接管。
- 切换封板、stopline 搜索与归档一致性复核已完成。
3. `271` 也已将 `240F + 285` 视为封板主路径完成，因此 `280` 不应继续保留“进行中，仅剩 285”的旧状态。
4. 因此，`280` 主计划应在 2026-03-16 回写为“已完成”。

## 5. 验证与门禁

1. 本次主计划收口以子计划既有验证结果为准：
- `281` 已记录来源冻结、构建链与可重复构建验证通过。
- `283` 已记录正式入口 smoke 与路径边界验证通过。
- `284` 已记录源码级测试与 `make librechat-web-build` 通过。
- `285` 已记录封板矩阵、stopline 搜索与 `make check doc` 通过。

2. 2026-03-16（CST）主计划文档收口复核：
- `make check doc`：通过。

## 6. 结论

- `DEV-PLAN-280` 主计划已完成，不再存在“只剩 `285` 未收口”的现实缺口。
- 后续若发生升级兼容或运行时影响性合入，应沿 `237/291` 与 `271-S5` 的新鲜度规则刷新对应证据，而不是回退 `280` 主计划状态。
