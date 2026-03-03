# DEV-PLAN-230：LibreChat 项目级集成实施方案（复用优先 + 边界自建）

**状态**: 草拟中（2026-03-03 10:30 UTC）

## 1. 背景与问题
- 关联计划：
  - `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
  - `docs/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
  - `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
  - `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- 当前实现以“UI 壳层接入”为主：
  1. [ ] `/assistant-ui/*` 反向代理 + iframe 已存在，但 LibreChat 仍是外部上游依赖。
  2. [ ] 仓库未形成官方版本/镜像/配置的纳管闭环，环境漂移导致“页面可见但不可用”。
  3. [ ] 模型与 Provider 治理主要在本仓自建，未充分复用 LibreChat 原生 AI 配置能力。
- 批判结论（本计划修订输入）：
  1. [ ] 若继续在模型配置、会话壳层、运行基线重复自建，属于“重复造轮子”。
  2. [ ] 项目级集成应优先复用成熟开源能力，仅在业务边界与合规约束处自建。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] **复用优先（Upstream-first）**：默认采用 LibreChat 官方成熟能力；任何自建必须给出“不可复用”证据（合规/边界/性能）。
2. [ ] 将 LibreChat 以项目级依赖纳入仓库治理：版本冻结、来源可追溯、可复现运行。
3. [ ] 提供仓库内可启动的 LibreChat 运行基线（本地/CI 一致），不再依赖手工外置上游。
4. [ ] 将“模型/Provider 配置”收敛为 LibreChat 原生配置为主源；本仓仅保留适配层与边界校验。
5. [ ] 保持 One Door 与授权边界不变：LibreChat 不获得业务写入口。
6. [ ] 固化升级路径：版本升级、回归检查、回滚策略可执行且有门禁证据。
7. [ ] 在 `/app/assistant` 形成壳可用性可观测闭环（健康、错误、来源版本可见）。

### 2.2 非目标（Out of Scope）
1. [ ] 不将 `confirm/commit/re-auth/One Door` 下放给 LibreChat。
2. [ ] 不替换现有 `internal/assistant/*` 业务契约与状态机。
3. [ ] 不在本计划扩展多业务域 Agent 能力（仍聚焦 assistant 集成边界）。
4. [ ] 不引入 legacy 双链路（禁止“旧壳/新壳并行长期运行”）。

## 3. Build vs Buy 评审门槛（新增）
### 3.1 反造轮子准则
1. [ ] 先评估“官方能力直用”→“薄适配”→“自建替代”，按顺序决策。
2. [ ] 触发自建前必须满足以下任一条件并留档：
   - [ ] 官方能力无法满足租户隔离/授权边界（RLS + Casbin + One Door）。
   - [ ] 官方能力无法满足仓库级契约（幂等、审计快照、路由门禁）。
   - [ ] 官方能力在关键指标上不达标且无法通过配置/插件解决。
3. [ ] 自建项必须定义“退出策略”：若上游后续提供等价能力，优先回归上游实现。

### 3.2 能力分界矩阵（Must Reuse / Must Build）
- **应优先复用（Must Reuse）**
  1. [ ] LibreChat 聊天 UI 壳层（会话、消息渲染、输入交互、流式显示）。
  2. [ ] LibreChat 官方模型/Provider 配置机制（作为主配置源）。
  3. [ ] LibreChat 官方运行与发布基线（镜像、环境变量约定、健康检查范式）。
  4. [ ] LibreChat 上游升级节奏与版本元数据（tag/commit/digest）。
- **必须自建（Must Build）**
  1. [ ] One Door 业务提交裁决链（confirm/commit/re-auth）。
  2. [ ] 租户隔离与授权边界（RLS/Casbin/route-capability-map）。
  3. [ ] `internal/assistant/*` 业务契约（幂等、审计快照、任务编排与补偿）。
  4. [ ] 项目特有的合规审计与错误码契约。
- **可薄适配（Adapter-first）**
  1. [ ] 将现有 `assistantModelGateway` 收敛为适配层，避免继续扩张为“第二套配置中心”。
  2. [ ] 将现有模型配置 UI 收敛为“查看/校验/迁移”入口，不再演进为独立主配置面。

## 4. 架构与关键决策
### 4.1 决策 1：集成形态（修订）
- 方案 A：继续“外部上游 URL + 本仓自建配置中心”模式。  
  缺点：不可复现，且继续重复造轮子。
- 方案 B（选定）：**官方能力优先 + 仓库级纳管**（官方镜像/配置契约 + 本仓边界适配）。  
  优点：版本可控、环境一致、减少重复实现、边界可验证。

### 4.2 目录与归属（草案）
1. [ ] `deploy/librechat/`：官方镜像编排、环境模板、健康检查脚本。
2. [ ] `scripts/librechat/`：版本同步、差异审计、升级检查脚本。
3. [ ] `docs/dev-records/`：升级回归证据、Build-vs-Buy 决策记录。
4. [ ] 可选 `third_party/librechat/`：仅在“必须源码纳管”时启用，默认优先官方镜像而非 fork 改造。

### 4.3 运行拓扑（不变）
```mermaid
graph TD
    A[/app/assistant/] --> B[iframe /assistant-ui/*]
    B --> C[server reverse proxy]
    C --> D[LibreChat service (repo managed)]
    A --> E[/internal/assistant/*]
    E --> F[Assistant service / One Door chain]
```

### 4.4 配置与启动基线（修订）
1. [ ] 增加 LibreChat 官方配置模板（开发/CI），补齐必填变量说明。
2. [ ] `LIBRECHAT_UPSTREAM` 默认指向仓库内 LibreChat 服务，非默认外部地址。
3. [ ] 启动前执行配置校验（缺变量/非法 URL/危险配置直接 fail-fast）。
4. [ ] `/assistant-ui/*` 不可用时返回稳定错误码与可观测日志，不允许静默失败。
5. [ ] 模型/Provider 配置读取路径改为“LibreChat 主源 + 本仓边界校验”，禁止双主源漂移。

### 4.5 安全边界与路由治理（不变）
1. [ ] 保持“LibreChat 仅可通过 `/internal/assistant/*` 编排接口交互”不变。
2. [ ] 阻断从 `/assistant-ui/*` 触达业务写路由（持续保留并强化 E2E 断言）。
3. [ ] 固化 postMessage 三重校验（origin/schema/nonce-channel）并确保升级后不退化。
4. [ ] 路由 allowlist、capability-route-map、authz requirement 同步校验，防漂移。

### 4.6 升级与回滚策略（补充）
1. [ ] 版本冻结：记录上游版本标识（tag + commit/digest）与引入时间。
2. [ ] 升级流程：`升级候选评估（复用优先） -> 集成验证 -> 自动化回归 -> 证据归档 -> 发布`。
3. [ ] 回滚原则：仅允许“版本回滚”，不允许引入临时 legacy 分支规避边界约束。
4. [ ] 每次升级输出“复用率变化”记录（新增自建项必须给出豁免理由）。

## 5. 实施切片（修订顺序）
### PR-230-01：最小复用落地（官方能力先跑通）
1. [ ] 引入官方镜像 + 官方环境模板 + 本地/CI 编排，形成可启动基线。
2. [ ] 服务器默认上游切换为仓库内 LibreChat 服务。
3. [ ] 在 `/app/assistant` 明确壳层不可用提示与诊断信息。

### PR-230-02：配置主源收敛（停止重复造轮子）
1. [ ] 将模型/Provider 配置主源收敛到 LibreChat 原生配置。
2. [ ] 将现有 `assistantModelGateway` 改为适配与边界校验层（不再扩张配置能力）。
3. [ ] 补齐配置迁移与一致性校验脚本，避免双主源冲突。

### PR-230-03：边界硬化与门禁
1. [ ] 强化 `/assistant-ui/*` 代理边界（路径、方法、头透传最小化）。
2. [ ] 增补边界测试：`assistant-ui` 不得旁路业务写路由。
3. [ ] 路由与授权门禁对齐（`routing/capability/authz`）。

### PR-230-04：可观测与升级回归
1. [ ] 展示 LibreChat 版本标识、健康态、错误码映射。
2. [ ] 固化升级回归脚本并纳入 `make e2e` 或等效门禁。
3. [ ] 输出 `docs/dev-records/` 证据模板并完成首轮留档。

## 6. 测试与验收标准（修订）
1. [ ] 可用性：未配置外部上游时，仓库内编排可直接启动并访问 LibreChat 壳层。
2. [ ] 复用性：模型/Provider 配置以 LibreChat 原生主源生效，本仓无第二主配置源。
3. [ ] 边界：`/assistant-ui/*` 无法触发业务写旁路（保留并升级 `tp220-e2e-007`）。
4. [ ] 兼容：postMessage 三重校验用例全部通过（222 基线不退化）。
5. [ ] 稳定：`make check routing`、`make check capability-route-map`、`make check error-message` 通过。
6. [ ] 文档：`make check doc` 通过，且 AGENTS Doc Map 可发现本计划。

## 7. 风险与缓解
1. [ ] 风险：对官方配置机制理解不足，迁移期出现错配。  
   缓解：迁移脚本 + 双向一致性校验（短期只读比对，长期单主源）。
2. [ ] 风险：上游版本变化导致消息桥协议漂移。  
   缓解：协议契约测试前置，失败即阻断升级。
3. [ ] 风险：为了赶进度继续扩张自建配置功能。  
   缓解：Build-vs-Buy 门禁 + 自建项豁免审批 + 退出策略留档。

## 8. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
