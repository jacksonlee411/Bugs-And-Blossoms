# DEV-PLAN-250：Go 网关 + PostgreSQL RAG/鉴权接管实施方案（覆盖阶段一至阶段二）

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 规划中（2026-03-05 13:03 UTC）

## 1. 背景与目标

- 承接 `DEV-PLAN-230`~`DEV-PLAN-240` 的 LibreChat 集成与边界治理工作，新增一条“Go 核心驱动 + Node.js 轻代理”的渐进演进路径。
- 采用 Strangler Fig 增量替换策略，优先将 CPU 密集与安全敏感链路从 Node.js 下沉到 Go。
- 本计划仅覆盖：
  1. [ ] 阶段一：架构解耦与 RAG 下沉（速赢）。
  2. [ ] 阶段二：权限接管与数据穿透（Casbin + SetID）。
- 不覆盖：阶段三（全面替换 LibreChat 会话存储为 PostgreSQL）及其迁移脚本。

## 2. 范围与非目标

### 2.1 范围（In Scope）
1. [ ] 在前端/LibreChat 前新增 Go API Gateway（统一入口）。
2. [ ] 将文档上传、切片、向量化、检索链路迁移至 Go + PostgreSQL (`pgvector`)。
3. [ ] 建立 RAG 检索结果注入协议（Go 组装 context，Node.js 负责模型流式转发）。
4. [ ] 在 Go 网关启用 Casbin 与 SetID 校验，拦截所有 assistant 读/写/推理入口。
5. [ ] 建立跨租户负测与 fail-closed 验收基线。

### 2.2 非目标（Out of Scope）
1. [ ] 不在本计划内废弃 MongoDB 对 Conversations/Messages 的存储职责。
2. [ ] 不新增“第二写入口”绕过 One Door 业务写链。
3. [ ] 不引入 Redis/Ristretto/BigCache 等新缓存基础设施（除非后续审批）。
4. [ ] 不引入 legacy 双链路或回退分支。

## 3. 事实源（SSOT）

- `AGENTS.md`（触发器矩阵、单链路、门禁红线）
- `docs/dev-plans/012-ci-quality-gates.md`（CI 门禁口径）
- `docs/dev-plans/017-routing-strategy.md`（路由治理）
- `docs/dev-plans/019-tenant-and-authn.md`（会话与租户注入）
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`（RLS 强隔离）
- `docs/dev-plans/022-authz-casbin-toolchain.md`（Casbin）
- `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`（迁移闭环）
- `docs/dev-plans/025-sqlc-guidelines.md`（sqlc）
- `docs/archive/dev-plans/230-librechat-project-level-integration-plan.md`（LibreChat 项目级边界）
- `docs/archive/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`（assistant-ui 身份边界）

## 4. 架构与边界冻结（阶段一~二）

1. [ ] **入口冻结**：前端到 AI 的请求统一先到 Go Gateway，再转发 LibreChat（Node.js）。
2. [ ] **职责冻结**：
   - Go：鉴权、租户隔离、RAG 预处理、上下文组装、审计。
   - Node.js：LLM Provider 适配、SSE/WebSocket 流式回传。
3. [ ] **数据冻结**：
   - 结构化业务数据与 RAG 向量数据进入 PostgreSQL。
   - Conversations/Messages 暂维持 LibreChat 现状（阶段三再迁移）。
4. [ ] **安全冻结**：任何 assistant 调用必须经过 Casbin + SetID 校验；命中异常即 fail-closed。

## 5. 分阶段实施步骤

### 5.1 阶段一：架构解耦与 RAG 下沉（速赢）

#### PR-250-01：Go RAG 基础设施
1. [ ] 固化 RAG DDL 契约（文档、切片、embedding、检索日志）；落地前先完成人工评审确认。
2. [ ] 按 `Atlas + Goose` 建立迁移脚本，按 `sqlc` 生成查询与类型。
3. [ ] 启用 `pgvector` 并冻结向量维度、索引策略、距离函数（cosine/l2 选型结论需留档）。

#### PR-250-02：Go 文档处理管道
1. [ ] 建立上传任务流：提取文本 -> 切片 -> 向量化 -> 入库（幂等 + 可重试）。
2. [ ] 以 goroutine worker pool 控制并发与背压，避免瞬时大文件压垮运行时。
3. [ ] 输出任务审计字段：`trace_id`、租户、文档版本、失败原因。

#### PR-250-03：检索与上下文注入
1. [ ] 提供检索接口（TopK + 相似度阈值 + SetID 过滤）。
2. [ ] Go 在转发前完成 context 拼装，并附带 explain 元数据（命中条数/片段来源）。
3. [ ] Node.js 保持无状态代理，仅负责对接模型与流式回传。

### 5.2 阶段二：权限接管与数据穿透

#### PR-250-04：统一鉴权与租户隔离接管
1. [ ] Gateway 全面启用 Kratos 会话校验、SetID 注入、Casbin 判定。
2. [ ] assistant 相关历史查询、检索、推理入口统一收口到 Gateway。
3. [ ] 未登录、越权、跨租户、SetID 不匹配统一返回明确错误码，不直出泛化失败文案。

#### PR-250-05：提示词与数据访问安全加固
1. [ ] 建立“先鉴权后组装”顺序：授权未通过时禁止触发任何业务数据查询与向量检索。
2. [ ] 对 RAG 检索增加租户 + SetID 双重过滤与审计记录。
3. [ ] 对 prompt 注入场景补充负测：确保无法跨租户读到 B 租户内容。

#### PR-250-06：阶段二封板验收
1. [ ] 完成跨租户隔离 E2E（含正测/负测）与回归清单。
2. [ ] 补齐网关链路观测（请求量、检索时延、拒绝率、错误码分布）。
3. [ ] 更新 `docs/dev-records/` 证据日志，形成阶段一~二的可审计闭环。

## 6. 验收标准（截至阶段二）

1. [ ] 大文件上传与向量化不再阻塞 Node.js 事件循环（由 Go 管道承接 CPU 负载）。
2. [ ] RAG 检索结果可稳定注入模型请求，并具备可追溯的命中解释信息。
3. [ ] 所有 assistant 调用均经过 Gateway 的会话、Casbin、SetID 校验。
4. [ ] 跨租户提示词注入与越权访问负测均失败（系统返回受控错误码）。
5. [ ] 阶段一与阶段二不引入 legacy 双链路或第二写入口。

## 7. 门禁与验证（与 CI 对齐）

1. [ ] `go fmt ./...`
2. [ ] `go vet ./...`
3. [ ] `make check lint && make test`
4. [ ] `make check routing`
5. [ ] `make authz-pack && make authz-test && make authz-lint`
6. [ ] `make check capability-route-map`
7. [ ] `make check no-legacy`
8. [ ] `make check error-message`
9. [ ] `make check doc`

## 8. 风险与缓解

1. [ ] **风险：RAG 向量检索性能不达标。** 缓解：先做基线压测，保留索引与参数调优窗口；不提前引入外部缓存。
2. [ ] **风险：Gateway 与 LibreChat 接口漂移。** 缓解：冻结转发协议与回归用例，升级前先跑契约测试。
3. [ ] **风险：权限规则复杂导致误拒绝。** 缓解：Casbin 策略灰度验证 + explain 日志，先观测再收紧。
4. [ ] **风险：阶段边界失控。** 缓解：明确阶段三出界项（消息存储迁移）在本计划内禁止启动。

## 9. 交付物

1. [ ] 本计划文档（DEV-PLAN-250）。
2. [ ] 阶段一~二设计说明与 API/DDL 契约附件。
3. [ ] 独立执行记录未单列沉淀（未形成 `dev-plan-250-execution-log.md`）。
4. [ ] 阶段二封板验收报告（性能、隔离、门禁结果）。

## 10. 后续衔接（非本计划实施）

1. [ ] 阶段三（MongoDB -> PostgreSQL JSONB）单列新计划（建议 `DEV-PLAN-250A`）。
2. [ ] 阶段三开始前需再次评审会话存储兼容与迁移回滚策略。
