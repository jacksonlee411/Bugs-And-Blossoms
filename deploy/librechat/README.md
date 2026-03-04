# LibreChat Runtime Baseline

该目录承载 `DEV-PLAN-232` 的官方运行基线资产，目标是让 LibreChat 在仓库内可复现启动。

## 文件说明

- `docker-compose.upstream.yaml`：官方基线（最小改动）。
- `docker-compose.overlay.yaml`：本仓覆盖层（卷路径与本地运行参数）。
- `.env.example`：环境变量模板。
- `versions.lock.yaml`：版本元数据（tag/digest/imported_at/rollback_ref）。
- `healthcheck.sh`：依赖健康检查并产出 `runtime-status.json`。

## 快速开始

1. 复制环境变量模板：
   - `cp deploy/librechat/.env.example deploy/librechat/.env`
2. 启动：
   - `make assistant-runtime-up`
3. 查看状态：
   - `make assistant-runtime-status`
4. 停止：
   - `make assistant-runtime-down`

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

## rag_api 运行口径（DEV-PLAN-239）

- `rag_api` 镜像固定为 `ghcr.io/danny-avila/librechat-rag-api-dev-lite@sha256:201958505e21...`。
- `rag_api` 统一使用 `atlas-mongo` 模式，默认连接 `mongodb://mongodb:27017/LibreChat`，避免隐式回退到 `db:5432`（pgvector）导致重启风暴。
- `rag_api` 数据卷挂载目标为 `/app/uploads`（宿主机目录：`${LIBRECHAT_DATA_ROOT}/rag_api`）。

## 清理边界

`make assistant-runtime-clean` 仅清理 `${LIBRECHAT_DATA_ROOT}` 下列目录（与运行时同源）：

- `${LIBRECHAT_DATA_ROOT}/api`
- `${LIBRECHAT_DATA_ROOT}/mongodb`
- `${LIBRECHAT_DATA_ROOT}/meilisearch`
- `${LIBRECHAT_DATA_ROOT}/rag_api`
- `${LIBRECHAT_DATA_ROOT}/vectordb`

禁止清理上述目录之外的路径（脚本对仓库外路径 fail-closed）。

## 故障处置（MongoDB 挂载异常）

最小恢复流程（建议按顺序）：

1. `make assistant-runtime-down`
2. `make assistant-runtime-clean`
3. `make assistant-runtime-up`
4. `make assistant-runtime-status`

当 `status` 为 `unavailable` 时，`services[].reason` 重点关注：

- `mount_source_missing`：宿主机挂载目录缺失（先执行 `clean -> up`，或检查 `LIBRECHAT_DATA_ROOT`）。
- `container_not_running`：容器未运行（查看 `docker compose ps`/容器日志）。
- `upstream_unreachable`：API 容器已运行但上游探针不可达（查看 api 服务日志与端口绑定）。
