# DEV-PLAN-380F Readiness

## 1. 本轮时间与范围

1. 时间：2026-04-17 CST
2. 计划：`docs/dev-plans/380f-librechat-vendored-runtime-and-deploy-retirement-plan.md`
3. 本轮命中切片：
   - [x] Contract Slice
   - [x] Delivery Slice
   - [x] Test & Gates Slice
   - [x] Readiness Slice

## 2. 本轮实施结论

1. [x] `/internal/assistant/runtime-status` 完成态冻结为“保留旧路径，但只返回退役语义”：
   - 服务端入口仍存在，返回 `410 Gone + assistant_api_gone`
   - 响应 message 明确指向 `/internal/cubebox/runtime-status`
   - 不再读取 `deploy/librechat/**` 作为默认正式诊断输入
2. [x] `/internal/assistant/runtime-status` 已从正式能力面摘除：
   - `config/capability/route-capability-map.v1.json`
   - `internal/server/capability_route_registry.go`
   - `config/routing/allowlist.yaml`
   - `internal/server/authz_middleware_test.go`
   - `internal/server/capability_route_registry_test.go`
3. [x] vendored compat API canonical retired code 已收敛为 `librechat_retired`：
   - `/app/assistant/librechat/api/*`
   - `/assets/librechat-web/api/*`
   - 不再保留 `assistant_vendored_api_retired` 作为活体 contract
4. [x] `assistant_vendored_api_retired` 已从活体错误码解释层移除：
   - `config/errors/catalog.yaml`
   - `apps/web/src/errors/presentApiError.ts`
   - `internal/routing/responder.go`
   - 对应单测已同步收口
5. [x] 本地联调 skill 已改为只承认 CubeBox 正式链路：
   - `tools/codex/skills/bugs-and-blossoms-dev-login/SKILL.md`
   - 不再引导 `make assistant-runtime-*`
   - 不再把 `/app/assistant/librechat` 描述为正式入口
6. [x] 旧 LibreChat 入口相关 E2E 已完成分类收口：
   - `tp220`、`tp283` 保留为退役负向断言
   - `tp288`、`tp290` 已保持历史 skip，不纳入活体封板主链
   - `tp284`、`tp288b` 已补充“历史命名残留”说明，避免误判为活体 LibreChat coverage

## 3. 用户可见结果

1. [x] `/app/cubebox` 是唯一正式聊天入口。
2. [x] `/app/assistant/librechat*` 稳定返回 `410 Gone + librechat_retired`。
3. [x] `/assistant-ui*` 稳定返回 `410 Gone + assistant_ui_retired`。
4. [x] `/assets/librechat-web/**` 稳定返回 `410 Gone + librechat_retired`，不再回落到 vendored 静态文件内容。
5. [x] `/internal/assistant/runtime-status` 不再被视为正式运行态接口；唯一正式运行态接口为 `/internal/cubebox/runtime-status`。

## 4. 资产状态裁决

1. [x] 已完成“零活体推荐入口”收口：
   - `Makefile` 已删除 `assistant-runtime-*` 与 `librechat-web-*` 目标及 help 暴露
   - skill 已停止推荐旧 runtime / build 流程
2. [x] 已完成“零活体正式能力映射”收口：
   - old runtime-status 已从 capability/authz/allowlist 主面摘除
3. [ ] 暂未做大体量物理删除：
   - `third_party/librechat-web/**`
   - `internal/server/assets/librechat-web/**`
   - `deploy/librechat/**`
   - `scripts/librechat/**`
   - `scripts/librechat-web/**`
4. [x] 本轮裁决：以上目录在 `380F` 中不再是正式构建/运行主链依赖，但物理删除留待后续专门批次，不阻塞封板前的“零活体引用”目标。

## 5. 验证记录

1. [x] `go test ./internal/server/...`
   - 结果：通过
2. [x] `go test ./internal/routing/...`
   - 结果：通过
3. [x] `make check routing`
   - 结果：通过
4. [x] `make check capability-route-map`
   - 结果：通过
5. [x] `make check error-message`
   - 结果：通过
6. [x] `pnpm --dir apps/web check`
   - 结果：通过
   - 备注：仍有既有 `FreeSoloDropdownField.tsx` fast-refresh warning，非本轮新增
7. [x] `pnpm --dir apps/web build`
   - 结果：通过
   - 备注：仍有既有 Vite chunk size warning，非本轮新增
8. [x] `make css`
   - 结果：通过
   - 备注：已同步内嵌静态资产到 `internal/server/assets/web/assets/index-P-Mhe3ti.js`
9. [ ] `git status --short` 为空
   - 当前不为空，原因是本轮代码、文档与前端生成物尚未提交

## 6. 本轮关键证据

1. [x] 旧 runtime-status handler 已改为 retired API：
   - `internal/server/handler.go`
   - `internal/server/assistant_runtime_status_test.go`
   - `internal/server/handler_test.go`
2. [x] 旧 runtime-status 不再 active capability：
   - `internal/server/capability_route_registry.go`
   - `config/capability/route-capability-map.v1.json`
3. [x] 旧 runtime-status 不再位于 allowlist 正式入口：
   - `config/routing/allowlist.yaml`
4. [x] 退役 E2E 已校验 retired code：
   - `e2e/tests/tp220-assistant.spec.js`
   - `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`

## 7. 残留与后续

1. [x] 历史文档与历史 evidence 中仍会出现 `assistant_vendored_api_retired`、`/app/assistant/librechat` 等旧口径。
   - 这些记录属于历史证据，不代表当前活体 contract
2. [x] `third_party/librechat-web/**` 与 `internal/server/assets/librechat-web/**` 仍在仓库中。
   - 本轮已确保它们不再通过正式路径被推荐或依赖
   - 若后续物理删除，需单独评估仓库体积、生成物策略与 `go:embed` 影响
3. [x] `380G` 封板时应继续沿用以下最小断言集：
   - `/app/cubebox` 正向
   - `/app/assistant/librechat*` 负向 `410 + librechat_retired`
   - `/assistant-ui*` 负向 `410 + assistant_ui_retired`
   - `/internal/assistant/runtime-status` 负向 `410 + assistant_api_gone`
