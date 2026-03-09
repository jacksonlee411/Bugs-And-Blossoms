# DEV-PLAN-239 执行记录（LibreChat 聊天可写链路恢复与运行态稳定性收口）

## 1. 记录信息
- 计划：`docs/archive/dev-plans/239-librechat-chat-write-path-recovery-and-runtime-stability-plan.md`
- 记录时间：2026-03-04 03:43 UTC
- 记录人：Codex
- 关联 commit/PR：`N/A（当前工作区未提交）`

## 2. 实施摘要
- 完成 `assistant-ui` 代理最小可写放开（`GET/HEAD/POST/OPTIONS`），并补齐错误码、日志字段、请求头 allowlist 与敏感头剥离。
- 完成 runtime 脚本硬化：`OPENAI_API_KEY` 启动前与容器内双重非空校验；`clean` 权限冲突时给出可操作修复建议。
- 完成错误码目录、后端文案与前端映射收敛。
- 修复 e2e `tp220-e2e-007`：新增 assistant-ui 旁路路径阻断，防止通过 `/assistant-ui/org/**` 触达业务写接口。

## 3. 证据清单（UTC）
1. 2026-03-04 03:34 UTC  
   - 命令：`go fmt ./... && go vet ./... && make check lint && make test`  
   - 退出码：`0`  
   - 关键输出：`[coverage] OK: total 100.00% >= threshold 100.00%`
2. 2026-03-04 03:34 UTC  
   - 命令：`make check routing`  
   - 退出码：`0`  
   - 关键输出：`[routing] running routing gates`
3. 2026-03-04 03:35 UTC  
   - 命令：`make check capability-route-map`  
   - 退出码：`0`  
   - 关键输出：`[capability-route-map] OK`
4. 2026-03-04 03:35 UTC  
   - 命令：`make check error-message`  
   - 退出码：`0`  
   - 关键输出：`[error-message] OK`
5. 2026-03-04 03:37 UTC  
   - 命令：`make assistant-runtime-down && make assistant-runtime-clean && make assistant-runtime-up && make assistant-runtime-status`（连续 3 轮）  
   - 退出码：`0`  
   - 关键输出：`[round-1|2|3] healthy`；`[librechat-runtime] OK: status=healthy`
6. 2026-03-04 03:38 UTC  
   - 命令：`docker compose -p bugs-and-blossoms-librechat --env-file deploy/librechat/.env -f deploy/librechat/docker-compose.upstream.yaml -f deploy/librechat/docker-compose.overlay.yaml exec -T api sh -lc 'test -n "${OPENAI_API_KEY:-}"'`  
   - 退出码：`0`  
   - 关键输出：无输出（断言通过）
7. 2026-03-04 03:40 UTC（负向）  
   - 命令：`make assistant-runtime-down && make assistant-runtime-clean && make assistant-runtime-status`  
   - 退出码：`2`  
   - 关键输出：`[librechat-runtime] FAIL: status=unavailable`  
   - 运行态快照：`deploy/librechat/runtime-status.json` 中 `api/mongodb/meilisearch/rag_api/vectordb` 均为 `reason=mount_source_missing`
8. 2026-03-04 03:40 UTC  
   - 命令：`make assistant-runtime-up && make assistant-runtime-status`  
   - 退出码：`0`  
   - 关键输出：`[librechat-runtime] OK: status=healthy`
9. 2026-03-04 03:40 UTC  
   - 命令：`E2E_BASE_URL=http://localhost:18080 E2E_SUPERADMIN_BASE_URL=http://localhost:18081 KRATOS_PUBLIC_URL=http://127.0.0.1:14433 E2E_KRATOS_ADMIN_URL=http://127.0.0.1:14434 make e2e`  
   - 退出码：`0`  
   - 关键输出：`13 passed (14.7s)`（含 `tp220-e2e-007: librechat shell cannot bypass business write routes`）
10. 2026-03-04 03:44 UTC  
   - 命令：`make check doc`  
   - 退出码：`0`  
   - 关键输出：`[doc] OK`

## 4. 待人工验收项
- 真实模型闭环（登录 `/app/assistant` 后在 iframe 发送消息并收到真实外网模型回包，记录 provider/model 与响应片段）尚未在本次自动化执行中完成。
