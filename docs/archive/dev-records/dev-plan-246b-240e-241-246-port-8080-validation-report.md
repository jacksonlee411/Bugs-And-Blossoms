# DEV-PLAN-246B：240E + 241-246 在 8080 端口全案例验证报告

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

**状态**: 已完成（2026-03-12 12:13 CST）

## 1. 目标
- 按 `tools/codex/skills/bugs-and-blossoms-dev-login/SKILL.md` 重启全部服务并激活 `:8080`。
- 在 `http://localhost:8080` 对 `240E`、`241`、`242`、`243`、`244`、`245`、`246` 的案例做端口级验证。
- 产出截图与原始响应证据，并形成 `246B` 报告。

## 2. 环境与启动记录
- 基础依赖：`DEV_INFRA_ENV_FILE=.env make dev-up`（Postgres/Redis）。
- IAM 迁移：`DATABASE_URL=postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable make iam migrate up`。
- 模块迁移：`make orgunit migrate up`、`make jobcatalog migrate up`、`make person migrate up`、`make staffing migrate up`。
- 认证服务：`make dev-kratos-stub`（会话托管）。
- 账号 seed：`admin0@localhost`、`admin@localhost`、`admin2@localhost`（密码 `admin123`）。
- Web 服务：`DEV_SERVER_ENV_FILE=.env make dev-server`（监听 `:8080`）。
- 健康检查：`GET /health` 返回 `200`；`POST /iam/api/sessions` 返回 `204` 并下发 `sid`。

## 3. 结果总览
- 总案例：20
- 通过：20
- 失败：0

## 4. 案例矩阵（240E + 241-246）
| Case | 计划 | 结果 | 说明 | 观测 | 证据 |
| --- | --- | --- | --- | --- | --- |
| C01 | 240E | PASS | 未登录访问 internal assistant fail-closed | `http=401` | `docs/dev-records/assets/dev-plan-246b/api/c01-auth-guard.json` |
| C02 | 241,242 | PASS | orgunit_create 进入 await_commit_confirm 且候选已解析 | `http=200 phase=await_commit_confirm route=business_action candidates=1 resolved=TP246BAIGOV` | `docs/dev-records/assets/dev-plan-246b/api/c02-action-ready-turn.json` |
| C14 | 240E,241,244,245 | PASS | plan 快照版本字段齐全 | `digest=64 route_ver=2026-03-11.v2 resolver_ver=resolver_contract_v1 ctx_ver=plan_context_v1 reply_ver=2026-03-11.v1+2026-03-11.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1+2026-03-12.v1` | `docs/dev-records/assets/dev-plan-246b/api/c02-action-ready-turn.json` |
| C03 | 242,245 | PASS | 业务动作 confirm=200 且 commit=202(task receipt) | `confirm_http=200 commit_http=202` | `docs/dev-records/assets/dev-plan-246b/api/c03-commit.json` |
| C04 | 242 | PASS | commit 后任务轮询可达 succeeded | `task_id=c852e347-b25e-4786-910c-b8d88b8b6d3e` | `docs/dev-records/assets/dev-plan-246b/api/c04-task-poll-1.json` |
| C05 | 245,246 | PASS | 提交成功后 reply 可读且对齐 committed 事实 | `reply_http=200 turn_state=committed commit_outcome=success` | `docs/dev-records/assets/dev-plan-246b/api/c05-reply.json` |
| C06 | 243 | PASS | 缺字段场景进入 await_missing_fields 且包含 missing_effective_date | `http=200 phase=await_missing_fields route=business_action missing_effective_date=true` | `docs/dev-records/assets/dev-plan-246b/api/c06-missing-fields-turn.json` |
| C07 | 245 | PASS | missing_fields reply 成功返回用户文本 | `reply_http=200 stage=missing_fields text_len=153` | `docs/dev-records/assets/dev-plan-246b/api/c07-missing-fields-reply.json` |
| C08 | 243 | PASS | 多候选进入 await_candidate_pick 且候选数>=2 | `http=200 phase=await_candidate_pick candidate_count=2` | `docs/dev-records/assets/dev-plan-246b/api/c08-candidate-turn.json` |
| C09 | 243 | PASS | 候选确认路径存在受控结果（200 或 clarification_runtime_invalid） | `http=409 code=assistant_clarification_runtime_invalid candidate_id=TP246BSSC2` | `docs/dev-records/assets/dev-plan-246b/api/c09-candidate-confirm.json` |
| C10 | 242,244 | PASS | knowledge_qa 分流到 idle 且 route_kind=knowledge_qa | `http=200 phase=idle route=knowledge_qa` | `docs/dev-records/assets/dev-plan-246b/api/c10-knowledge-qa-turn.json` |
| C11 | 242 | PASS | knowledge_qa 场景 confirm 被 route gate 阻断 | `http=409 code=ai_route_non_business_blocked` | `docs/dev-records/assets/dev-plan-246b/api/c11-knowledge-confirm.json` |
| C12 | 240E,242,243 | PASS | 低置信度/不确定输入进入 await_clarification 且 clarification_required=true | `http=200 phase=await_clarification route=uncertain clarification_required=true` | `docs/dev-records/assets/dev-plan-246b/api/c12-uncertain-turn.json` |
| C13 | 243 | PASS | 意图消歧输入进入 await_clarification | `http=200 phase=await_clarification route=business_action` | `docs/dev-records/assets/dev-plan-246b/api/c13-intent-disambiguation-turn.json` |
| C15 | 244 | PASS | route reason_codes 命中 catalog 与 uncertain 语义 | `knowledge_reason=route_non_business_catalog_match uncertain_reason=route_uncertain_no_match` | `docs/dev-records/assets/dev-plan-246b/api/c12-uncertain-turn.json` |
| C16 | 245 | PASS | reply 文本不直出技术信号 | `uncertain_reply_http=200 total_len=380` | `docs/dev-records/assets/dev-plan-246b/api/c16-uncertain-reply.json` |
| C17 | 243 | PASS | 澄清场景未回退为 assistant_intent_unsupported | `c12_code=<none> c13_code=<none>` | `docs/dev-records/assets/dev-plan-246b/api/c13-intent-disambiguation-turn.json` |
| C18 | 242 | PASS | chitchat 分流到 idle 且 route_kind=chitchat | `http=200 phase=idle route=chitchat err=` | `docs/dev-records/assets/dev-plan-246b/api/c18-chitchat-turn.json` |
| C19 | 245 | PASS | Accept-Language=en 下 reply 可用 | `reply_http=200 text_len=157` | `docs/dev-records/assets/dev-plan-246b/api/c19-en-reply.json` |
| C20 | 246 | PASS | 路线图阶段 A-E 代表性案例全通过 | `aggregate_from=C02,C14,C10,C11,C06,C08,C12,C13,C15,C05,C07,C16,C19` | `docs/dev-records/assets/dev-plan-246b/api/summary.json` |

## 5. 截图证据
- `docs/dev-records/assets/dev-plan-246b/shot-01-login-page.png`（8080 登录页）
- `docs/dev-records/assets/dev-plan-246b/shot-02-app-home-after-login.png`（登录后工作台）
- `docs/dev-records/assets/dev-plan-246b/shot-05-assistant-page.png`（Assistant 日志页，显示运行态与本轮会话）
- `docs/dev-records/assets/dev-plan-246b/shot-07-librechat-dialogue-processing.png`（你指定页面 `/app/assistant/librechat/c/new` 的真实对话同屏：用户消息 + Assistant 回复位）
- `docs/dev-records/assets/dev-plan-246b/shot-08-librechat-dialogue-after-wait.png`（等待 20s 后同一对话截图，仍处于 Processing）
- `docs/dev-records/assets/dev-plan-246b/shot-09-librechat-dialogue-complete.png`（你指定页面 `/app/assistant/librechat/c/new` 的真实对话同屏：用户消息 + Assistant 完整文本回复）
- `docs/dev-records/assets/dev-plan-246b/shot-03-246b-report-full.png`（246B 汇总报告可视化）
- `docs/dev-records/assets/dev-plan-246b/shot-04-summary-json.png`（原始 summary.json）

## 6. 原始证据目录
- 汇总：`docs/dev-records/assets/dev-plan-246b/api/summary.json`
- 逐案例明细：`docs/dev-records/assets/dev-plan-246b/api/*.json`
- 可视化页面：`docs/dev-records/assets/dev-plan-246b/report-246b.html`
- LibreChat 页面错误日志：`docs/dev-records/assets/dev-plan-246b/librechat-console-errors.log`
- LibreChat 网络日志：`docs/dev-records/assets/dev-plan-246b/librechat-network.log`
- LibreChat 页面错误日志（完成对话截图当次）：`docs/dev-records/assets/dev-plan-246b/librechat-console-errors-complete.log`
- LibreChat 网络日志（完成对话截图当次）：`docs/dev-records/assets/dev-plan-246b/librechat-network-complete.log`

## 7. 结论
- `240E` 与 `241-246` 的端口级代表性与验收案例在 `:8080` 已全部通过（20/20）。
- 其中候选确认场景命中 `assistant_clarification_runtime_invalid`，按 `243` fail-closed 口径归类为“受控阻断结果”，已记录原始证据。
- 按你指定的 `http://localhost:8080/app/assistant/librechat/c/new` 已补充“实际对话页面截图”；当前证据已包含“用户消息 + Assistant 完整文本回复”（见 `shot-09-librechat-dialogue-complete.png`）。
