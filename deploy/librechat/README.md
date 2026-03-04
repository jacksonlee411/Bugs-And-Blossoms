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

## 数据目录单一口径（DEV-PLAN-238）

- `LIBRECHAT_DATA_ROOT` 是数据目录唯一入口，默认值：`.local/librechat`（相对仓库根目录解析）。
- 运行脚本会将 `LIBRECHAT_DATA_ROOT` 归一为绝对路径，并在启动前校验：
  1. 必需子目录可创建且可写；
  2. compose 解析出的 bind mount source 与预期路径完全一致；
  3. 缺失或漂移时 fail-fast（阻断启动）。

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
