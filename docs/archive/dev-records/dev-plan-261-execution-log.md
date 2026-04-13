# DEV-PLAN-261 执行日志（LibreChat 助手对话失败排查与修复）

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

**状态**: 进行中（2026-03-06 07:32 CST）

## 1. 复现与证据采集（M1）
1. [X] 运行态基础检查：`make assistant-runtime-status` 返回 `status=healthy`。
2. [X] 真实接口复现“对话失败”：
   - `POST /internal/assistant/conversations/:id/turns`（提示词：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`）出现 `ai_model_provider_unavailable`（503）与 `ai_model_timeout`（504）。
3. [X] 模型探测：
   - `/v1/models` 可用；
   - 直接调用 `chat/completions` 时，`gpt-5-codex` 等模型频繁返回 `usage_limit_reached`（上游 500）；
   - `gpt-5.1`/`gpt-5.2` 存在可用响应。

## 2. 根因结论（M2）
1. [X] 主因确认：本地 `ASSISTANT_MODEL_CONFIG_JSON` 仅配置单模型（`gpt-5-codex`）时，受上游配额/可用性波动影响，导致 turn 创建失败（503/504）。
2. [X] 次因确认：运行态健康探针基于 `/models`，不能覆盖“具体模型 chat/completions 可用性”，容易出现“runtime healthy 但对话失败”的感知落差。

## 3. 修复动作（M3）
1. [X] 本机 `.env` 已调整为多模型优先级回退（同 provider）：
   - priority 10：`gpt-5.1`
   - priority 20：`gpt-5.2`
   - priority 30：`gpt-5-codex`
2. [X] 重启服务加载新配置：`DEV_SERVER_ENV_FILE=.env make dev-server`。
3. [X] 配置生效验证：`GET /internal/assistant/model-providers` 返回上述 3 个模型，`healthy=healthy`。

## 4. 修复后验证（M4）
1. [X] 真实接口正向验证：
   - 对同一提示词再次调用 `POST /internal/assistant/conversations/:id/turns`，返回 `200`，会话进入 `validated`，并产生 `plan.model_name=gpt-5.2`。
2. [ ] `/app/assistant/librechat` 页面级真实多轮 Case 2~4 回归（待补跑并固化截图/录像证据）。
3. [ ] 补充门禁回归（`make check doc` 已在计划创建阶段通过；其余按代码变更范围执行）。

## 5. 备注
1. 本次“解决”聚焦运行态配置收敛（本机 `.env`），未引入代码层回退通道。
2. `.env` 为本机忽略文件，不进入仓库提交；后续若需团队级长期修复，应在独立计划中补充“模型可用性探针口径”与“默认模型配置建议”契约。
