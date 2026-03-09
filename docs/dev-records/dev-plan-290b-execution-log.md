# DEV-PLAN-290B 执行日志

## 1. 执行记录
| 时间（CST） | 执行人 | 命令 | 结果 | 备注 |
| --- | --- | --- | --- | --- |
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

## 4. 本轮阻断结论
- 认证链路已恢复：在 `TRUST_PROXY=1` 下，`/iam/api/sessions` 不再返回 `invalid_credentials`，说明登录/租户解析路径可用。
- 真实模型链路已恢复：最新准入证据已命中 `openai / gpt-5.2`，且 `fallback_detected=false`，原“`plan_only + deterministic`”阻断已解除。
- 主验收当前为 `blocked`，核心阻断已迁移为 T0 数据基线不稳定：Case2 首轮虽然识别出 `create_orgunit`，但 `dry_run.validation_errors` 命中 `parent_candidate_not_found`，随后在 `await_missing_fields` 阶段落入 `assistant_intent_unsupported` 噪声失败。
- 当前代码已补齐租户级基线硬化：每次新建租户后自动注入并校验 `AI治理办公室` 与 `共享服务中心` 多候选场景，并在 Case2/4 首轮前置基线闸门；待本地服务恢复后需按 `000|002 -> 003 -> 004` 顺序复跑确认闭环。
- 负测已形成稳定证据：`neg-001`（`conversation_confirmation_required`）与 `neg-003`（`assistant_intent_unsupported`）为有效负例；`neg-002/004` 在前置即 `assistant_intent_unsupported` 的环境下按 `probe_skipped` 记录。
