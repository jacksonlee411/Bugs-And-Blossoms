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

## 清理边界

`make assistant-runtime-clean` 仅清理以下本地目录：

- `.local/librechat/api`
- `.local/librechat/mongodb`
- `.local/librechat/meilisearch`
- `.local/librechat/rag_api`
- `.local/librechat/vectordb`

禁止在脚本中清理上述目录之外的路径。
