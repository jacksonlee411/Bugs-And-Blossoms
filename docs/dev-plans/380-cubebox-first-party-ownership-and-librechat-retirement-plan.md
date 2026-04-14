# DEV-PLAN-380：CubeBox 一方资产化与 LibreChat 完整退役重构方案（v1 去 Prompt 版）

**状态**: 进行中（2026-04-15；Phase 0/1/2/3 的正式入口、最小路由接线/代理接线、文件最小闭环与旧 LibreChat UI 退役已落地；`380A` 数据面 contract 已批准可依赖，但这不等于 `380B` 已完成；`380B` 后端正式切面、`iam.cubebox_*` PostgreSQL 数据面主链接入、`/internal/assistant/*` 正式退役与全量门禁回归仍未完成）

## 1. 背景

1. [X] 已确认仓库启动时同时存在 `LibreChat` 命名、vendored Web UI、upstream runtime 与 `/internal/assistant/*` API 语义，和本仓 Go + PostgreSQL + `apps/web` 的正式技术栈不一致。
2. [X] 已确认 `DEV-PLAN-370` 继续独占知识 runtime 的 Markdown 单主源与 direct runtime 边界；`380` 只接管产品实现面、品牌、路由、API、数据面与旧资产退役。
3. [X] 已冻结 `CubeBox v1` 只承接会话、消息、流式回复、历史列表、任务状态、模型只读展示、文件上传与展示；`Prompt` 不进入本期范围，避免与 `370` 的知识 Prompt 形成双语义。

## 2. 目标与非目标

### 2.1 目标

1. [X] 将正式用户可见入口与导航主文案切换到 `CubeBox`。已落地 `/app/cubebox` 正式入口、导航标签与 `apps/web` 新页面。
2. [X] 建立 `/app/cubebox` 与 `/internal/cubebox/*` 主链入口。
3. [X] 建立 `modules/cubebox` 一方模块骨架，并承接文件存储等正式能力的最小闭环。
4. [X] 将旧 `/app/assistant/librechat`、`/assistant-ui/*`、`/assets/librechat-web/**` 退役为 `410 Gone`。
5. [X] 让 `apps/web` 成为 `CubeBox` 的正式前端承载面，不再把 vendored LibreChat Web UI 作为正式入口。

### 2.2 非目标

1. [X] 不在本期实现产品级 Prompt/Template 系统。
2. [X] 不在本期保留 upstream Node backend 为正式运行面。
3. [X] 不在本期继续扩张 `LibreChat` static assets / bridge / compat 页面。

## 3. 实施批次

### 3.1 Phase 0：契约冻结

1. [X] 新建 `DEV-PLAN-380`。
2. [X] 已回写 `AGENTS.md` 文档地图；本次实施状态与完成事项继续回写到本文档。

### 3.2 Phase 1：模块与 API

1. [X] 已建立 `modules/cubebox` 模块骨架。
2. [X] 已建立 `/internal/cubebox/*` API 命名空间。
3. [X] 已建立 `/internal/cubebox/*` 的最小 successor 路由与代理接线；正式后端实现、删除语义、`task poll_uri` 正式生成、`runtime-status`/`files` 模块化仍待 `380B/380D` 完成。

### 3.3 Phase 2：前端入口

1. [X] 已在 `apps/web` 增加 `CubeBox` 原生页面。
2. [X] 已将导航主入口切到 `/app/cubebox`。
3. [X] 已增加文件页与模型页。

### 3.4 Phase 3：退役旧入口

1. [X] `/app/assistant/librechat` 已返回 `410 Gone`。
2. [X] `/assistant-ui/*` 继续返回 `410 Gone`。
3. [X] `/assets/librechat-web/**` 已返回 `410 Gone`。

## 4. 当前实施范围

1. [X] `CubeBox` 页面与 `/internal/cubebox/*` 主链接线。
2. [X] `modules/cubebox` 一方文件能力最小闭环。
3. [X] 旧 `LibreChat` 正式入口退役。
4. [X] 本轮未引入新的产品级 Prompt DTO。
5. [ ] 当前后端仍以 `assistant` 复用链路为主；`380B` stopline 尚未清零，`/internal/cubebox/*` 现阶段只能视为 successor 路由入口 + 临时代理接线，而不是正式后端主链。

## 5. 已完成事项

1. [X] 已新增 `modules/cubebox` 骨架：
   `modules/cubebox/module.go`、`modules/cubebox/links.go`、`modules/cubebox/services/files.go`、`modules/cubebox/infrastructure/local_file_store.go`，并以仓库内本地目录文件存储实现 `CubeBox` 文件最小闭环。
2. [X] 已新增 `/internal/cubebox/*` successor 路由与兼容接线：
   `conversations`、`turns`、`tasks`、`models`、`runtime-status`、`files` 已具备入口，但当前后端仍大量代理 `assistant` 逻辑；task 回执仍通过响应改写承接 `/internal/cubebox/tasks/*` 语义，这些都不构成 `380B` 已完成证据。
3. [X] 已新增 `CubeBox` 前端正式页面：
   `apps/web/src/pages/cubebox/CubeBoxPage.tsx`、`CubeBoxFilesPage.tsx`、`CubeBoxModelsPage.tsx` 与 `apps/web/src/api/cubebox.ts` 已落地；路由与导航已切到 `/app/cubebox`。
4. [X] 已退役旧 `LibreChat` UI 正式入口：
   `/app/assistant/librechat`、`/assistant-ui/*`、`/assets/librechat-web/**` 现在由退役 handler 承接，不再进入旧 vendored Web UI。
5. [X] 已同步路由与能力治理配套：
   `config/routing/allowlist.yaml`、`config/capability/route-capability-map.v1.json`、`internal/server/capability_route_registry.go` 与相关测试已补齐 `CubeBox` 新路径。
6. [X] 已完成一次实施提交：
   提交 `47233768` `feat: establish cubebox formal entry and retire librechat ui` 已将本轮主链改动入库。

## 6. 已执行验证

1. [X] 2026-04-14 20:42 CST：`go test ./internal/server -run 'TestLibreChatLegacyUIRetired|TestCapabilityRouteBindingForRoute|TestNewHandler_|TestWithTenantAndSession_|TestAssistantFormalSuccessorRoutesAreWired'`
2. [X] 2026-04-14 20:42 CST：`go test ./internal/server -run 'TestCapabilityRouteRegistryContract|TestCapabilityRouteBindingForRoute'`
3. [X] 2026-04-14 20:42 CST：`pnpm --dir apps/web typecheck`
4. [X] 2026-04-14 20:42 CST：`pnpm --dir apps/web test src/pages/cubebox/CubeBoxPage.test.tsx src/pages/cubebox/CubeBoxFilesPage.test.tsx`
5. [X] 2026-04-14 20:42 CST：`pnpm --dir apps/web build`
6. [X] 2026-04-14 20:42 CST：`pnpm --dir apps/web lint` 已执行，结果为 `0 error / 2 warning`；warning 位于既有 [FreeSoloDropdownField.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/components/FreeSoloDropdownField.tsx)，非本计划新增文件。

## 7. 剩余事项

1. [ ] 以 PostgreSQL 建立 `iam.cubebox_*` 正式数据面，并完成必要的前向迁移。
2. [ ] 去掉 `/internal/cubebox/*` 对 `handleAssistant*API` 的代理依赖，完成 `380B` 的正式后端切面切换。
3. [ ] 实现 `DELETE /internal/cubebox/conversations/{conversation_id}` 的正式删除语义；当前仍为 `501/未实现`。
4. [ ] 将 `poll_uri` 收口为正式 service/DTO 生成，移除响应后改写桥接。
5. [ ] 将 `runtime-status` 从直接读取 `assistantSvc` 内部字段收敛为 `cubebox` facade 聚合。
6. [ ] 将 files 正式接入 `modules/cubebox`，不再由 `internal/server` 直接决定文件根目录并构造本地文件服务。
7. [ ] 补齐 `modules/cubebox` 的 conversations/turns/tasks/models/runtime-status/files facade、domain、tests 与 readiness 证据。
8. [ ] 完成 `/internal/assistant/*` 正式 API 的退役策略与主链收口，不再长期保留双正式命名空间。
9. [ ] 跑完 `380` 终态所需的全量仓库门禁与构建回归。

## 8. 验收

1. [ ] `go fmt ./...`
2. [ ] `go vet ./...`
3. [ ] `make check lint`
4. [ ] `make test`
5. [ ] `pnpm --dir apps/web check`
6. [ ] `make css`
7. [X] `/app/cubebox` 已成为正式入口，旧 `LibreChat` 正式入口已退役为 `410 Gone`；本轮通过服务端 handler 测试、路由/能力映射测试与 `apps/web` 页面/构建验证完成最小 successor 验收。
8. [ ] 现有验证只证明入口、路由、UI successor 与基础脚手架有效，不构成 `380B` 已完成证据；`380B` 仍需以后端去代理化、正式删除语义、`poll_uri` 收口、`runtime-status/files` 模块化与 readiness 补齐为完成条件。

## 9. 子计划分解与依赖

1. [X] 已登记并创建骨架文档：`380A` PostgreSQL 数据面与迁移契约  
   文档：`docs/dev-plans/380a-cubebox-postgresql-data-plane-and-migration-contract.md`  
   负责 `iam.cubebox_*` 表、索引、约束、sqlc 与前向迁移策略；当前 contract 已批准，可作为 `380B/380D` 依赖，但不等于 `380B` 已完成，也不构成 `380C` 启动条件。
2. [X] 已登记并创建骨架文档：`380B` 后端正式实现面切换  
   文档：`docs/dev-plans/380b-cubebox-backend-formal-implementation-cutover-plan.md`  
   负责 `modules/cubebox` 的 `domain / services / infrastructure / presentation` 正式实现与从 `assistant` 复用链路迁出；当前仅完成 groundwork，尚未达到 `380B` stopline 清零条件。
3. [X] 已登记并创建骨架文档：`380C` API/DTO 收口与 `/internal/assistant/*` 退役  
   文档：`docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`  
   负责 `/internal/cubebox/*` 成为唯一正式 API、DTO 收口、错误码/状态契约与旧命名空间退役策略。
4. [X] 已登记并创建骨架文档：`380D` 文件面正式化  
   文档：`docs/dev-plans/380d-cubebox-file-plane-formalization-plan.md`  
   负责 `cubebox_files / cubebox_file_links`、文件元数据 SoT、存储适配器与删除/引用一致性。
5. [X] 已登记并创建骨架文档：`380E` `apps/web` 正式前端收口  
   文档：`docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`  
   负责会话页、文件页、模型页、导航、i18n、页面级测试与残留前端命名清理，并承接 `DEV-PLAN-360A` 已冻结的“保留聊天 UI 壳、消息树、输入框、基础展示组件” successor UX 边界；若 `CubeBox` 正式入口不再尽量接近 LibreChat 式聊天骨架，必须由 `380E` 显式改写契约与验收。
6. [X] 已登记并创建骨架文档：`380F` LibreChat 资产退役与部署链收口  
   文档：`docs/dev-plans/380f-librechat-vendored-runtime-and-deploy-retirement-plan.md`  
   负责 `third_party/librechat-web`、`deploy/librechat/*`、`scripts/librechat/*`、旧构建链与相关文档收口。
7. [X] 已登记并创建骨架文档：`380G` 全量回归、门禁与封板验收  
   文档：`docs/dev-plans/380g-cubebox-regression-gates-and-final-closure-plan.md`  
   负责最终门禁、E2E、退役断言、dev-record 与封板条件。
8. [X] 依赖顺序已冻结：
   `380A -> 380B`
   `380A -> 380D`
   `380B + 380D -> 380C`
   `380C -> 380E`
   `380C + 380E -> 380F`
   `380F -> 380G`
9. [ ] `380C` 的启动前提冻结为：`380B` stopline 1/2/3/5/6 清零，而不是只要 `/internal/cubebox/*` 路由存在即可推进旧 API 正式退役。
