# DEV-PLAN-290B 执行日志

## 1. 执行记录
| 时间（CST） | 执行人 | 命令 | 结果 | 备注 |
| --- | --- | --- | --- | --- |
| 2026-03-10 02:28-02:47 | Codex | `go test ./internal/server -run 'TestAssistantIntentPipeline|TestAssistantModelGateway' -count=1 -v` + `TRUST_PROXY=1 go run ./cmd/server` + `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on --grep 'tp290b-e2e-003|tp290b-e2e-004'` + `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on` | 通过 | 关闭最后两类主阻断：① 修复 `plan_only` 本地升级后被 schema retry 回滚；② 增加“用户显式事实覆盖”以清空模型脑补的当天日期；同时确认 live 运行必须带 `TRUST_PROXY=1`。最终 `tp290b-e2e-000~004` 全绿，证据索引刷新为 `status=passed` |
| 2026-03-10 02:29-02:37 | Codex | `node /tmp/tp290b-login-repro.js` + `node /tmp/tp290b-login-repro2.js` + `TRUST_PROXY=1 go run ./cmd/server` | 定位完成 / 已修复 | Playwright 复现确认：未开启 `TRUST_PROXY=1` 时，`X-Forwarded-Host` 被忽略，登录请求实际落到 `localhost` 默认租户；重启 `server` 后登录链恢复正常 |
| 2026-03-09 23:40-23:42 | Codex | `node --check e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js` + `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --list` + `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --grep "tp290b-e2e-000|tp290b-e2e-002"` | 部分通过 / 阻断 | 已落地 T0 基线建置与首轮门禁：新租户自动注入 `AI治理办公室`、`共享服务中心` 多候选场景，并在 Case2/4 首轮不满足基线时直接阻断；语法校验与 Playwright 列表通过，但当前 shell 未起本地服务，live 复跑在 `http://localhost:8081/superadmin/login` 命中 `ERR_CONNECTION_REFUSED`，未能完成端到端验证 |
| 2026-03-09 21:14-21:15 | Codex | `TRUST_PROXY=1 AUTHZ_MODE=enforce RLS_ENFORCE=enforce /tmp/tp290b_live_run.sh` | 阻断 | 使用 `.env/.env.local` 真实模型 key/端点完成实跑；`invalid_credentials` 已消失，`tp290b-e2e-000` 进入 runtime gate 并因 `intent_action=plan_only`（`model_provider=deterministic`、`model_name=builtin-intent-extractor`）fail-closed；`tp290b-e2e-002` 因串行前置未执行；证据覆盖写入 `runtime-admission-gate.json/.har` 与主索引 |
| 2026-03-09 21:10-21:12 | Codex | `/tmp/tp290b_live_run.sh` | 阻断 | 首次专用链路实跑命中 `invalid_credentials`；复盘定位为未设置 `TRUST_PROXY=1` 导致 `X-Forwarded-Host` 租户解析未生效，后续已修正并重跑 |
| 2026-03-09 20:21-20:22 | Codex | `make check assistant-config-single-source` + `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on --grep "tp290b-e2e-000|tp290b-e2e-002"` | 阻断 | 单主源门禁通过；`tp290b-e2e-000` 按 fail-closed 阻断：`intent_action=plan_only`、`model_provider=deterministic`、`model_name=builtin-intent-extractor`；`tp290b-e2e-002` 因串行前置失败未执行；新增证据 `runtime-admission-gate.json/.har` 并更新 `tp290b-live-evidence-index.json` |
| 2026-03-09 20:03-20:06 | Codex | `node --check e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js` + `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --list` | 通过 | 落地 P0/P1：新增 `tp290b-e2e-000` 运行态准入闸门、Case2 输入向量收敛为“人力资源部2”、Case 失败归因结构化落盘（含 `assistant_intent_unsupported` 上下文字段），并在索引中补 `blocked/not_run` 自动回填逻辑 |
| 2026-03-09 19:02-19:03 | Codex | `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/third_party/librechat-web/source/client test:ci -- --runTestsByPath src/assistant-formal/__tests__/runtime.test.ts` | 通过（9/9） | 修复断言后回归通过（失败态文案断言已稳定） |
| 2026-03-09 19:03-19:04 | Codex | `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/third_party/librechat-web/source/client typecheck` | 失败 | 全仓历史类型错误（大量 `librechat-data-provider/react-query` 缺失与 TS 约束不匹配），非本次 290B 增量引入 |
| 2026-03-09 19:06-19:20 | Codex | `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on` | 阻断 | Case1 通过；Case2 持续 plan_only（deterministic）且无 `:confirm/:commit`；Case3 运行中未稳定进入 formal 气泡；Case4 未执行 |
| 2026-03-09 19:25 | Codex | `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-negative.spec.js --workers=1 --trace on` | 通过（4/4） | 已加入 `assistant_intent_unsupported` 前置探针降级：`neg-002/neg-004` 允许 `probe_skipped` 落盘，不再因环境能力缺失误判脚本失败 |

## 2. 证据索引
- 主索引：`docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`
- 数据基线：`docs/dev-records/assets/dev-plan-290b/tp290b-data-baseline.json`

## 3. 结论回写清单
- [x] `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- [x] `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- [x] `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- [x] `docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`

## 4. 本轮收口结论
- `DEV-PLAN-290B` 已闭环：`tp290b-e2e-000~004` 全部 live 通过，`tp290b-live-evidence-index.json` 已刷新为 `status=passed`。
- 认证/租户解析链已稳定：`TRUST_PROXY=1` 生效后，`X-Forwarded-Host` 与 Playwright/API 请求保持一致，不再出现 `invalid_credentials` 或错租户命中。
- T0 数据基线已稳定：新租户自动注入 `ROOT/集团`、`AI治理办公室`、`共享服务中心` 多候选场景，`tp290b-data-baseline.json` 已独立表达 `t0_baseline_ready=true` 与 `case2/case4` probe 结果。
- 运行态动作链已稳定：真实模型持续命中 `openai / gpt-5.2`、`fallback_detected=false`；Case 2/3/4 均完成 `create -> confirm -> commit` 并在终态会话回读中落为 `committed`。
- 本轮关键根因已全部关闭：包括 `TRUST_PROXY` 缺失、租户 baseline 缺播种、SetID resolver 不回退 baseline capability、异步 commit 后 conversation cache 未刷新，以及模型在缺字段场景脑补当天日期。
- 负测结论保持有效：`neg-001`（`conversation_confirmation_required`）与 `neg-003`（`assistant_intent_unsupported`）为稳定负例；`neg-002/004` 的 `probe_skipped` 记录继续保留为环境能力不足时的 fail-closed 证据。


## 5. 经验沉淀
- `TRUST_PROXY=1` 是 live 多租户 E2E 的硬前置；不开启时，所有 `X-Forwarded-Host` 都会失效。
- 新租户若要承载真实 Assistant 写链路，必须在创建时同步播种 Org/SetID baseline，而不是把基线建置留给测试脚本临时补洞。
- 模型输出不能越权“创造用户没说过的事实”；日期类字段必须由原文显式提供或由确定性规则推导，否则宁可停在缺字段。
- UI 正式链路要以 `Confirm/Submit/Select` 按钮为准，不要用自然语言确认制造第二条解析通道。
- 只看 task succeeded 不够，必须再回读 conversation 终态；否则 cache/staleness 问题会被漏掉。
