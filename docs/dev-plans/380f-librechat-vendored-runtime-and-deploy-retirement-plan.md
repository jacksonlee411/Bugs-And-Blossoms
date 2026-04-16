# DEV-PLAN-380F：LibreChat vendored/runtime/deploy 资产退役与收口

**状态**: 待启动（2026-04-17 CST；基于 `380A/380B/380C/380E` 已完成、`380D` 主链已切但仍有少量 file-plane 收尾的最新状态重评后更新。本文现作为“旧资产退役批次”的正式 SSOT，不再沿用 2026-04-14 的骨架草案口径）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `LibreChat` vendored Web UI、runtime、部署链、Makefile/debug 入口与相关历史资产退役的实施 SSOT。  
> `380A` 持有 PostgreSQL 数据面 contract，`380B` 持有后端正式实现面切换，`380C` 持有 API/DTO 收口与旧 `/internal/assistant/*` 退役，`380D` 持有文件面正式化，`380E` 持有 `apps/web` 正式前端收口，`380G` 持有最终回归与封板。  
> 本文只裁决“哪些 LibreChat 历史资产仍应保留为退役解释层/历史证据，哪些必须从仓库正式运行基线、脚本入口、构建链、文档说明与调试技能中删除或改口”。

## 0. 2026-04-17 重评结论

- `380C` 已完成：`/internal/cubebox/*` 已成为唯一正式 API 命名空间；旧 `/internal/assistant/*` formal entry、`model-providers*` 与 direct gone 路由已冻结退役语义；仓内正式 consumer 已切离旧 namespace。
- `380E` 已完成：`apps/web` 已收口到 `/app/cubebox`、`/app/cubebox/files`、`/app/cubebox/models`；`/app/assistant/librechat` 继续保持 `410 Gone` 退役态，而不是 redirect 或第二前端入口。
- `380B` 已完成：后端正式主链已切到 `modules/cubebox`，仅剩 bounded legacy executor/adapter bridge，不再构成 `380F` 的主阻塞。
- `380D` 已进入“主链已切、少量收尾待补”的状态：文件元数据/links/object store/cleanup contract 已能支撑 `380F` 判定“LibreChat 资产不再承担正式文件面职责”；但 `380D` 自己的 error-message 与 cleanup worker 收尾仍需继续关闭。
- 因此，`380F` 不再需要等待“是否已有 successor UI/API”这种前提问题；它的任务已经从“先盘点再说”升级为“明确哪些是待删除正式资产，哪些是临时保留的退役解释层，哪些只能归档不能继续挂在主干运行入口上”。

## 1. 背景与上下文（Context）

- **关联文档（SSOT）**
  - `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
  - `docs/dev-plans/380a-cubebox-postgresql-data-plane-and-migration-contract.md`
  - `docs/dev-plans/380b-cubebox-backend-formal-implementation-cutover-plan.md`
  - `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/dev-plans/380d-cubebox-file-plane-formalization-plan.md`
  - `docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
  - `docs/dev-plans/380g-cubebox-regression-gates-and-final-closure-plan.md`
  - `docs/dev-plans/360-librechat-depower-and-langgraph-langchain-layered-takeover-plan.md`
  - `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`

- **当前仓内真实残留资产**
  - vendored Web UI 来源与 patch stack：`third_party/librechat-web/**`
  - vendored Web UI 构建脚本：`scripts/librechat-web/**`
  - vendored 静态产物：`internal/server/assets/librechat-web/**`
  - 上游 runtime baseline / compose / healthcheck：`deploy/librechat/**`、`scripts/librechat/**`
  - Makefile 正式入口：`assistant-runtime-*`、`librechat-web-*`
  - 退役解释层/错误码/allowlist：`config/routing/allowlist.yaml`、`config/errors/catalog.yaml`、`internal/server/cubebox_retired.go`
  - 历史技能与开发说明：`tools/codex/skills/bugs-and-blossoms-dev-login/SKILL.md` 等仍把 LibreChat runtime 作为可选联调基线

- **当前实现的关键事实**
  - `/app/assistant/librechat`、`/assets/librechat-web/**`、`/assistant-ui/*` 当前已不是正式入口，而是由退役 handler 统一返回 `410 Gone`。
  - `apps/web` 已不存在正式 `assistant/librechat` 页面入口；E2E 也已经把它作为负向断言，而不是主链用例入口。
  - 但仓库仍保留一整套 vendored source/build/runtime/deploy/debug 资产，并在 Makefile/README/技能文档中以“可启动 runtime”“可构建 vendored UI”的方式暴露。
  - 这意味着“对外入口已退役”与“仓库内部仍把 LibreChat 当作可操作运行基线”之间仍存在明显漂移，`380F` 需要把这部分责任面正式切掉。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 核心目标

1. [ ] 把 `LibreChat` 从仓库的**正式运行基线 / 正式构建链 / 正式调试入口**中移除，不再允许通过 `Makefile`、README、skill、runbook 暗示“这是可选正式链路”。
2. [ ] 明确资产分类：
   - 必须删除的正式残留资产；
   - 可以临时保留、但只能作为 `410 Gone` 退役解释层支撑的极小服务端代码；
   - 允许归档保留、但不得继续作为主干运行入口的历史证据与上游快照。
3. [ ] 收敛仓库文档、脚本和说明，统一改口为：
   - 正式产品入口与联调链路是 `CubeBox + apps/web + /internal/cubebox/*`
   - `LibreChat` 仅剩历史退役语义与归档证据，不再是默认调试对象。
4. [ ] 为 `380G` 提供可验收输入：证明仓库已不再把 LibreChat vendored/runtime/deploy 资产当作正式基线使用。

### 2.2 非目标

1. [ ] 不在本文重新设计 `CubeBox` 数据面、后端、API、文件面或前端页面。
2. [ ] 不在本文改写 `370`/`375` Assistant knowledge/runtime contract。
3. [ ] 不要求删除所有带有 `librechat` 字样的历史证据、归档文档或 dev-record。
4. [ ] 不要求在本文立刻删除一切服务端退役解释层；若某段代码仍承担 `410 Gone`、错误码或负向断言职责，可暂留到 `380G` 封板前的零行为差异清理。

## 3. 当前资产分类与边界

### 3.1 必须删除或退出正式入口的资产

以下资产在 `380F` 完成态中，不得再作为主干的正式运行/构建/调试入口暴露：

1. `Makefile` 中的 `assistant-runtime-up/down/status/clean` 与 `librechat-web-verify/build` 正式帮助入口。
2. `deploy/librechat/**` 与 `scripts/librechat/**` 作为默认运行基线的说明。
3. `tools/codex/skills/bugs-and-blossoms-dev-login/SKILL.md` 中把 LibreChat runtime、`/app/assistant/librechat`、`/assistant-ui/*` 当作现行联调目标的内容。
4. `third_party/librechat-web/README.md`、`deploy/librechat/README.md` 等仍以“可构建/可运行 baseline”口吻描述的活体文档。

### 3.2 允许临时保留的退役解释层

以下内容在 `380F` 不要求立即删除，但必须在文档中明确标注为“退役解释层”，而不是“正式运行层”：

1. `config/routing/allowlist.yaml` 中为 `/app/assistant/librechat`、`/assets/librechat-web/**`、`/assistant-ui/*` 保留的路由 allowlist。
2. `config/errors/catalog.yaml` 中的 `librechat_retired`、`assistant_vendored_api_retired` 等错误码。
3. `internal/server/cubebox_retired.go` 及相关测试中用于返回 `410 Gone` 的 handler。
4. 如仍有必要的零行为差异退役测试，可暂时保留对应断言。

说明：

- 这些内容当前属于“旧入口退役 contract 的承载面”，不是“LibreChat 仍在运行”的证据。
- 是否在 `380G` 前进一步物理删路由/删错误码，由 `380G` 结合最终负向断言与零行为差异评估统一裁决。

### 3.3 允许归档但不得继续主干暴露的历史资产

以下内容允许保留，但必须转入“归档/历史证据”语义，不能继续被主干文档、技能或 Makefile 当作现行入口推荐：

1. `docs/archive/dev-plans/**` 与 `docs/archive/dev-records/**` 中的 LibreChat 历史计划与执行记录。
2. 仅为审计上游来源保留的 `UPSTREAM.yaml`、历史 patch stack、历史 build 产物证据。
3. 仅为历史 E2E 证据索引而保留的截图、HAR、DOM、JSON 资产。

## 4. 与 380A-380E 的重新对齐

### 4.1 为什么 380F 现在必须调整

2026-04-14 的骨架版 `380F` 存在三个过时前提：

1. 它假定 `CubeBox` 前端/API 仍未稳定，因此只写了粗粒度“盘点旧资产”。
2. 它把 vendored UI、runtime baseline、退役解释层、历史证据混成一类，没有区分“必须删除”和“允许保留的 410 contract”。
3. 它没有反映 `380C/380E` 已经完成这一事实，导致 `380F` 的验收口径仍停留在“不要再宣称 LibreChat 是正式基线”，而没有升级到“具体删哪些入口、改哪些说明、留下哪些最小 contract”。

### 4.2 对 380F 的新前置条件判定

1. [X] `380B` 已完成到足以支撑 `380F` 的程度：后端正式主链已切到 `modules/cubebox`。
2. [X] `380C` 已完成：正式 API/DTO 已收口，旧 assistant formal API 已进入退役 contract。
3. [X] `380E` 已完成：正式前端入口已迁到 `apps/web` 的 CubeBox 页面。
4. [~] `380D` 主链已切：足以证明文件能力不再依赖 LibreChat；但 file-plane 自身少量收尾仍继续由 `380D` 持有，不转嫁给 `380F`。

结论：

- `380F` 无需再等待 `380C/380E`；
- `380D` 的少量收尾不阻塞 `380F` 重写和启动，但会影响 `380G` 的最终封板。

## 5. 实施切片（Implementation Slices）

### 5.1 Contract Slice：冻结删除面与保留面

1. [ ] 逐项登记以下资产的去向：删除 / 改口保留 / 归档保留。
2. [ ] 在本文与关联 readiness 中写清每一类资产的 owner、当前状态与 stopline。
3. [ ] 确认 `410 Gone` 退役解释层不被误删，也不再被误称为“可调试正式入口”。

### 5.2 Build & Runtime Slice：移除正式入口

1. [ ] 从 `Makefile` 中删除或下架 `assistant-runtime-*`、`librechat-web-*` 的正式帮助入口。
2. [ ] 停止把 `deploy/librechat/**` 与 `scripts/librechat/**` 作为现行开发默认流程的一部分。
3. [ ] 停止把 `third_party/librechat-web/**` 与 `internal/server/assets/librechat-web/**` 当作需要持续维护的正式 build artifact。

说明：

- 若这些目录短期仍需保留以便归档或一次性迁移，应在主干说明中明确标记为“historical only / not a runtime baseline”。

### 5.3 Docs & Skill Slice：统一仓库叙述

1. [ ] 更新活体文档与 README，去掉“LibreChat runtime baseline / vendored UI build”作为现行口径的表述。
2. [ ] 更新本仓 skill、脚本帮助与开发说明，改为以 `/app/cubebox` 与 `CubeBox` API 为唯一联调主链。
3. [ ] 保留必要的负向断言说明：`/app/assistant/librechat` 是退役态，不是可选入口。

### 5.4 Readiness Slice：向 380G 交接

1. [ ] 回写 `380F` readiness，登记：
   - 删除/改口/归档的资产清单；
   - 实际执行的命令与结果；
   - 剩余保留的退役解释层；
   - 交给 `380G` 的最终负向断言列表。
2. [ ] 更新 `380` 主计划与相关文档地图中的状态说明。

## 6. 验收与测试（Acceptance Criteria）

### 6.1 边界验收

1. [ ] 仓库不再通过 `Makefile`、README、skill 或 runbook 推荐 `LibreChat` 作为当前正式运行/联调路径。
2. [ ] `CubeBox` 被明确为唯一正式入口；`LibreChat` 仅作为退役负向断言或历史归档出现。
3. [ ] 仍保留的 `librechat_*`/`assistant_vendored_*` 代码与配置都能解释为“退役 contract”，而不是“可恢复的第二主链”。

### 6.2 文档与入口验收

1. [ ] `make check doc` 通过。
2. [ ] `AGENTS.md`、相关活体 dev-plan、技能说明、README 不再把 `make assistant-runtime-up`、`make librechat-web-build` 当作现行推荐流程。
3. [ ] 若某些历史目录仍保留，必须在对应 README 或计划中说明其“历史/归档”语义。

### 6.3 回归与负向断言

1. [ ] 旧路径退役断言继续成立：
   - `/app/assistant/librechat` -> `410 Gone`
   - `/assistant-ui/*` -> `410 Gone`
   - `/assets/librechat-web/**` -> `410 Gone`
2. [ ] `apps/web` 正式入口与页面测试不重新引入 LibreChat consumer。
3. [ ] `380G` 所需的最终负向断言范围已冻结并可复用。

### 6.4 Stopline

出现以下任一情况，禁止宣布 `380F` 完成：

1. [ ] `Makefile`、skill、README 或现行开发文档仍把 LibreChat runtime / vendored build 当作当前推荐入口。
2. [ ] 删除正式入口说明的同时，把必要的 `410 Gone` 退役 contract 一并误删，导致旧入口不再有稳定退役语义。
3. [ ] 以“保留目录但不说明用途”的方式继续让 `third_party/librechat-web`、`deploy/librechat` 等资产滞留主干。
4. [ ] 任何页面、测试、脚本或联调技能重新把 `/app/assistant/librechat` 或 `/assistant-ui/*` 解释为可操作正式入口。
5. [ ] `380F` 没有给 `380G` 输出清晰的资产去向清单与最终负向断言清单。

## 7. 与相邻子计划的边界（Plan Boundaries）

### 7.1 与 `380C` 的边界

- `380C` 已负责 API/DTO 与旧 `/internal/assistant/*` formal entry 的退役 contract。
- `380F` 不再重复裁决 API matrix，只消费 `380C` 的完成态，把仓内剩余 runtime/deploy/vendored 资产从正式基线中移除。

### 7.2 与 `380D` 的边界

- `380D` 继续持有文件面主链与剩余收尾。
- `380F` 只使用 `380D` 的现状结论来判断“LibreChat 资产已不再承担正式文件能力”，不接管 file-plane 本身的问题单。

### 7.3 与 `380E` 的边界

- `380E` 已负责 `apps/web` 正式前端收口。
- `380F` 不再修改页面产品逻辑，只清理与前端相关的旧 build/runtime/deploy/说明资产。

### 7.4 与 `380G` 的边界

- `380F` 负责“旧资产退役与主干入口清理”。
- `380G` 负责“最终全量回归、门禁、dev-record 与封板”。
- `380F` 完成后，`380G` 应能把问题收敛为：
  - 退役 contract 是否仍稳定；
  - 是否还存在零行为差异清理空间；
  - `380A~380F` 是否都已具备封板输入。

## 8. Readiness 要求

1. [ ] 新建 `docs/dev-records/DEV-PLAN-380F-READINESS.md`，至少记录：
   - 本轮时间与范围
   - 删除/改口/归档清单
   - 实际执行命令
   - 命中的门禁与结果
   - 剩余保留的退役解释层
   - 交接给 `380G` 的最终断言清单
2. [ ] 若修改了帮助入口、脚本、README 或技能说明，必须在 readiness 中给出修改前后口径差异摘要。
3. [ ] 若短期保留某个 LibreChat 历史目录，必须写明：
   - 为什么保留
   - 保留到哪个批次
   - 谁负责最终删除或归档

## 9. 下一步（Next Steps）

1. [ ] 先以本文为 SSOT，按“删除正式入口 / 保留退役解释层 / 归档历史证据”三类完成仓内资产清单。
2. [ ] 落地 `Makefile`、skill、README 与相关脚本的收口修改。
3. [ ] 形成 `380F` readiness，并把最终负向断言与剩余 follow-up 交给 `380G`。

## 10. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
3. `docs/dev-plans/380d-cubebox-file-plane-formalization-plan.md`
4. `docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
5. `docs/dev-plans/380g-cubebox-regression-gates-and-final-closure-plan.md`
6. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
