# LibreChat Runtime Baseline

该目录承载 `DEV-PLAN-232` 的官方运行基线资产，目标是让 LibreChat 在仓库内可复现启动。

## 文件说明

- `docker-compose.upstream.yaml`：官方基线（最小改动）。
- `docker-compose.overlay.yaml`：本仓覆盖层（卷路径与本地运行参数）。
- `.env.example`：环境变量模板。
- `.env.mongo-debug.example`：external Mongo 调试模板（仅在你确实需要启动 upstream backend 时使用）。
- `versions.lock.yaml`：版本元数据（tag/digest/imported_at/rollback_ref）。
- `healthcheck.sh`：依赖健康检查并产出 `runtime-status.json`。

## 快速开始

1. 复制环境变量模板：
   - `cp deploy/librechat/.env.example deploy/librechat/.env`
   - 然后至少补齐：
     - `OPENAI_API_KEY`
     - `MONGO_URI`：必须指向一个外部可达的 Mongo；当前 upstream LibreChat backend 仍把它当启动硬依赖
2. 启动：
   - `make assistant-runtime-up`
3. 查看状态：
   - `make assistant-runtime-status`
4. 停止：
   - `make assistant-runtime-down`

## external Mongo 调试模式

当你需要真正启动 upstream LibreChat backend（例如排查 vendored upstream 自身行为），请不要改默认 `.env`，而是单独使用 external Mongo 调试模板：

1. `cp deploy/librechat/.env.mongo-debug.example deploy/librechat/.env.mongo-debug`
2. 填写：
   - `OPENAI_API_KEY`
   - `MONGO_URI`（外部可达 Mongo）
3. 启动：
   - `make assistant-runtime-up-mongo-debug`
4. 查看状态：
   - `make assistant-runtime-status-mongo-debug`
5. 停止：
   - `make assistant-runtime-down-mongo-debug`

说明：
- 调试模式默认使用独立 project：`bugs-and-blossoms-librechat-mongo-debug`
- 默认端口为 `3081`
- 默认数据根为 `.local/librechat-mongo-debug`
- 该模式只用于 upstream 调试，不代表默认开发基线

## 真实模型能力前置条件（DEV-PLAN-239）

- `OPENAI_API_KEY` 必须在 **容器内** 可见且非空（以容器内检查为准，不以页面展示为准）。
- 推荐在启动后执行：
  - `docker compose -p ${LIBRECHAT_COMPOSE_PROJECT:-bugs-and-blossoms-librechat} --env-file deploy/librechat/.env -f deploy/librechat/docker-compose.upstream.yaml -f deploy/librechat/docker-compose.overlay.yaml exec -T api sh -lc 'test -n "${OPENAI_API_KEY:-}"'`
- 模型治理页面 `/app/assistant/models` 仅提供“只读展示 + Validate”，不是模型/密钥写入口；运行时最终以环境变量注入结果为准。
- 密钥仅允许放在本机私有环境文件（如 `.env.local` / `deploy/librechat/.env`），禁止提交到仓库。

## 数据目录单一口径（DEV-PLAN-238）

- `LIBRECHAT_DATA_ROOT` 是数据目录唯一入口，默认值：`.local/librechat`（相对仓库根目录解析）。
- 运行脚本会将 `LIBRECHAT_DATA_ROOT` 归一为绝对路径，并在启动前校验：
  1. 必需子目录可创建且可写；
  2. compose 解析出的 bind mount source 与预期路径完全一致；
  3. 缺失或漂移时 fail-fast（阻断启动）。

## 依赖退役口径（DEV-PLAN-360A Phase 3）

- 默认部署仅保留 `api` 服务；`mongodb`、`meilisearch`、`rag_api`、`vectordb` 已从 compose 主链移除。
- 上述四项仍保留在 `versions.lock.yaml` 中，仅用于 `runtime-status` 暴露 `retired_by_design` 语义，不再作为默认运行前置。
- 若后续调试仍需单独拉起历史依赖，必须通过临时 patch / 私有调试脚本完成，不得回写默认主干 compose。
- 额外注意：当前 upstream `api` 进程仍在源码层硬依赖 `MONGO_URI`，见容器内 `/app/api/db/connect.js`。因此“默认 compose 只保留 api”并不等于“api 可以在无 Mongo 的情况下健康启动”；现阶段若要让 `3080` 健康，仍需提供一个外部 Mongo 端点。
- 因此，仓库现提供单独的 external Mongo 调试入口，而不是把 Mongo 重新混回默认 baseline。

## 清理边界

`make assistant-runtime-clean` 仅清理 `${LIBRECHAT_DATA_ROOT}` 下列目录（与默认运行时同源）：

- `${LIBRECHAT_DATA_ROOT}/api`

禁止清理上述目录之外的路径（脚本对仓库外路径 fail-closed）。

## 故障处置（API 上游异常）

最小恢复流程（建议按顺序）：

1. `make assistant-runtime-down`
2. `make assistant-runtime-clean`
3. `make assistant-runtime-up`
4. `make assistant-runtime-status`

当 `status` 为 `unavailable` 时，`services[].reason` 重点关注：

- `upstream_unreachable`：API 容器已运行但上游探针不可达（查看 api 服务日志与端口绑定）。
- `retired_by_design`：依赖已按设计退役，不参与默认部署故障判定。
- 若 `make assistant-runtime-up` 在启动前直接失败：
  - `retired dependency env drift detected`：说明 `.env` 里仍保留 `MEILI/RAG/QDRANT` 这类已退役依赖变量；
  - `MONGO_URI still points at removed compose service 'mongodb'`：说明当前 `.env` 仍在引用已从默认 compose 移除的 `mongodb` 服务名，需要改成外部 Mongo 地址。
