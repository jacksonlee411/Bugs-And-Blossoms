# DEV-PLAN-380：CubeBox 一方资产化与 LibreChat 完整退役重构方案（v1 去 Prompt 版）

**状态**: 进行中（2026-04-13 21:10 CST）

## 1. 背景

1. [ ] 当前仓库仍保留 `LibreChat` 命名、vendored Web UI、upstream runtime 与 `/internal/assistant/*` API 语义，和本仓 Go + PostgreSQL + `apps/web` 的正式技术栈不一致。
2. [ ] `DEV-PLAN-370` 已冻结知识 runtime 的 Markdown 单主源与 direct runtime 边界；`380` 不接管知识 runtime，只接管产品实现面、品牌、路由、API、数据面与旧资产退役。
3. [ ] `CubeBox v1` 只承接会话、消息、流式回复、历史列表、任务状态、模型只读展示、文件上传与展示；`Prompt` 不进入本期范围，避免与 `370` 的知识 Prompt 形成双语义。

## 2. 目标与非目标

### 2.1 目标

1. [ ] 将用户可见正式命名从 `LibreChat` 统一切换为 `CubeBox`。
2. [ ] 建立 `/app/cubebox` 与 `/internal/cubebox/*` 主链入口。
3. [ ] 建立 `modules/cubebox` 一方模块骨架，并承接文件存储等正式能力。
4. [ ] 将旧 `/app/assistant/librechat`、`/assistant-ui/*`、`/assets/librechat-web/**` 退役为 `410 Gone`。
5. [ ] 让 `apps/web` 成为 `CubeBox` 的正式前端承载面，不再把 vendored LibreChat Web UI 作为正式入口。

### 2.2 非目标

1. [ ] 不在本期实现产品级 Prompt/Template 系统。
2. [ ] 不在本期保留 upstream Node backend 为正式运行面。
3. [ ] 不在本期继续扩张 `LibreChat` static assets / bridge / compat 页面。

## 3. 实施批次

### 3.1 Phase 0：契约冻结

1. [X] 新建 `DEV-PLAN-380`。
2. [ ] 回写 `AGENTS.md` 文档地图与相关实施记录。

### 3.2 Phase 1：模块与 API

1. [ ] 建立 `modules/cubebox` 模块骨架。
2. [ ] 建立 `/internal/cubebox/*` API 命名空间。
3. [ ] 接通会话、任务、模型状态与文件上传/列出/删除的最小闭环。

### 3.3 Phase 2：前端入口

1. [ ] 在 `apps/web` 增加 `CubeBox` 原生页面。
2. [ ] 将导航主入口切到 `/app/cubebox`。
3. [ ] 增加文件页与模型页。

### 3.4 Phase 3：退役旧入口

1. [ ] `/app/assistant/librechat` 返回 `410 Gone`。
2. [ ] `/assistant-ui/*` 继续返回 `410 Gone`。
3. [ ] `/assets/librechat-web/**` 返回 `410 Gone`。

## 4. 当前实施范围

1. [ ] `CubeBox` 页面与 `/internal/cubebox/*` 主链接线。
2. [ ] `modules/cubebox` 一方文件能力最小闭环。
3. [ ] 旧 `LibreChat` 正式入口退役。
4. [ ] 不在本轮引入新的产品级 Prompt DTO。

## 5. 验收

1. [ ] `go fmt ./...`
2. [ ] `go vet ./...`
3. [ ] `make check lint`
4. [ ] `make test`
5. [ ] `pnpm --dir apps/web check`
6. [ ] `make css`
7. [ ] `/app/cubebox` 可访问，旧 `LibreChat` 正式入口退役。
