# DEV-PLAN-238 执行记录（LibreChat MongoDB 运行异常修复）

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

## 1. 记录信息
- 计划：`docs/archive/dev-plans/238-librechat-mongodb-runtime-failure-hardening-plan.md`
- 记录时间：2026-03-04 00:47 UTC
- 记录人：Codex

## 2. 根因与复现证据
- 根因：`LIBRECHAT_DATA_ROOT` 在 compose、清理脚本与文档之间存在口径漂移（相对路径解析不一致），导致 MongoDB 挂载源路径不可预测，出现 `mount ... no such file or directory`，进而触发 MongoDB `WT_PANIC`/不可用。
- 失败快照（容器停机场景）：
  - `make assistant-runtime-status` 返回 `status=unavailable`
  - `deploy/librechat/runtime-status.json` 出现 `services[].reason=container_not_running`
- 现场日志证据引用（计划输入）：`WT_PANIC`、`FileNotOpen`、`/data/db` 挂载源缺失（见 DEV-PLAN-238 背景条目）。

## 3. 实施变更
- 路径单主源收敛：
  - 新增 `scripts/librechat/common.sh`，统一 env 读取、数据目录归一、compose 命令构建。
  - `LIBRECHAT_DATA_ROOT` 统一解析为“仓库根目录下绝对路径”，并要求路径留在仓库内（fail-closed）。
- 启动前校验增强：
  - `scripts/librechat/up.sh` 新增数据目录可写检查与 `docker compose config --format json` 挂载源一致性断言。
- 清理脚本同源化：
  - `scripts/librechat/clean.sh` 改为按 `LIBRECHAT_DATA_ROOT` 清理 `api/mongodb/meilisearch/rag_api/vectordb` 子目录，不再硬编码路径。
  - 增加“权限兜底清理”：`rm -rf` 失败时回退到一次性 `docker run` 清理，避免 root-owned 文件导致清理中断。
- 运行状态可观测增强：
  - `deploy/librechat/healthcheck.sh` 区分 `mount_source_missing` / `container_not_running` / `upstream_unreachable`。
  - API 探针改为带超时重试（默认 60s），避免容器刚启动时的瞬时误判。
- 文档与模板收敛：
  - `deploy/librechat/.env.example` 明确 `LIBRECHAT_DATA_ROOT=.local/librechat`（相对仓库根目录）。
  - `deploy/librechat/docker-compose.overlay.yaml` 改为强制使用 `LIBRECHAT_DATA_ROOT`（缺失即报错）。
  - `deploy/librechat/README.md` 增补单一路径口径、故障处置与恢复步骤。

## 4. 回归验证与结果
- 目录与挂载一致性验证：
  - `make assistant-runtime-up` ✅（启动前通过 mount source 断言 + 启动后健康检查）
  - `make assistant-runtime-status` ✅（`status=healthy`，MongoDB `healthy`）
- 闭环验证（`down -> clean -> up -> status`）：
  - 连续执行 3 轮 ✅
- MongoDB 重启一致性验证：
  - `docker compose -p bugs-and-blossoms-librechat --env-file deploy/librechat/.env.example -f deploy/librechat/docker-compose.upstream.yaml -f deploy/librechat/docker-compose.overlay.yaml restart mongodb` ✅
  - 重启后 `make assistant-runtime-status` 仍为 `status=healthy` ✅
- 失败原因分层验证：
  - `down` 后 `status`：`reason=container_not_running` ✅
  - `down + clean` 后 `status`：`reason=mount_source_missing` ✅
  - 临时错误 upstream 后 `status`：`api.reason=upstream_unreachable` ✅
- 文档门禁：
  - `make check doc` ✅

## 5. 最小复现与最小恢复
- 最小复现（历史问题）：
  1. 数据目录口径漂移（运行/清理/文档不一致）；
  2. 启动时 MongoDB bind mount 指向缺失路径；
  3. 运行状态降级 `unavailable`。
- 最小恢复（现行标准流程）：
  1. `make assistant-runtime-down`
  2. `make assistant-runtime-clean`
  3. `make assistant-runtime-up`
  4. `make assistant-runtime-status`

## 6. 结论
- DEV-PLAN-238 目标完成：路径口径单一、启动前防漂移校验、状态诊断可区分根因、文档与证据已收口。
