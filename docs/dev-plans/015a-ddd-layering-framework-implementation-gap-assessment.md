# DEV-PLAN-015A：DDD 分层框架履职缺口评估（承接 DEV-PLAN-015）

**状态**: 草拟中（2026-04-09 08:05 CST）

## 背景

`DEV-PLAN-015` 已为本仓定义 DDD 四层目录、Composition Root、两类落地形态（常规 Go DDD / DB Kernel + Go Facade）以及依赖规则与停止线。

但截至当前代码树，`015` 仍主要停留在“蓝图 + 骨架”层面，尚未完成对既有实现的系统性收口。为避免后续评审继续在“文档目标态”与“当前实现态”之间来回切换，需要一份独立的缺口评估文档，明确：

1. [ ] `015` 已经履行了哪些职责。
2. [ ] 当前代码树相对 `015` 的主要偏差在哪里。
3. [ ] 这些偏差属于“实现落后于蓝图”，还是“超出 015 当前范围的后续实施项”。
4. [ ] 后续若要继续承接，应优先补哪些收口计划。

## 目标与非目标

### 目标

1. [ ] 以 `DEV-PLAN-015` 为蓝图，对当前仓库实现做一次可引用的履职评估。
2. [ ] 把“目录已建成”和“分层真正落地”区分开，避免把骨架完成误判成架构完成。
3. [ ] 把当前缺口分成“已履职 / 部分履职 / 明显缺口”三类，便于后续规划承接。
4. [ ] 输出一版适合 code review、后续 dev-plan、以及 `docs/dev-records/` 引用的稳定表述。

### 非目标

1. [ ] 本文不直接重构模块边界，不修改现有 `module.go` / `links.go` / `internal/server` 代码。
2. [ ] 本文不收紧 `.gocleanarch.yml` 规则；仅评估当前门禁与 `015` 蓝图的距离。
3. [ ] 本文不把“当前偏差存在”直接定性为 `015` 失效；本次结论限定为“实现落后于蓝图”。
4. [ ] 本文不替代后续专项实施计划；如需收口，必须另立后续子计划承接。

## 评估口径

### 事实源

- 主计划：`docs/dev-plans/015-ddd-layering-framework.md`
- 仓库规则入口：`AGENTS.md`
- 分层门禁配置：`.gocleanarch.yml`
- 门禁入口：`scripts/ci/cleanarch.sh`、`make check lint`

### 评估原则

1. [ ] 区分“蓝图是否成立”与“实现是否已完全对齐蓝图”。
2. [ ] 优先依据当前代码树中的真实装配点、依赖方向、Kernel 调用点与门禁覆盖面给出判断。
3. [ ] 若 `015` 明确列为目标态、但未承诺在本计划内完成全部现有代码树重构，则记为“部分履职”或“后续承接缺口”，不记为“失效”。

## 当前判断

### 结论摘要

总体判断：`DEV-PLAN-015` 当前处于“蓝图与骨架已建立、实现收口仍明显滞后”的状态。

更准确的定性是：

- [X] `015` 已履行“定义架构蓝图、目录骨架、基本门禁入口”的职责。
- [ ] `015` 尚未带动现有实现完成系统性收口。
- [ ] 当前主要问题应表述为“实现落后于蓝图”，而不是“蓝图已失效”。

### 已履职项

1. [X] 四层目录骨架曾在主要业务模块落位；当前活体模块以 `iam/orgunit` 为准，`jobcatalog/person/staffing` 已由 `DEV-PLAN-450` 删除。
2. [X] `.gocleanarch.yml` 与 `scripts/ci/cleanarch.sh` 已接入，`make check lint` 中存在分层门禁入口。
3. [X] `DEV-PLAN-015` 已把两类落地形态（Go DDD / DB Kernel + Go Facade）与 One Door 口径写清，后续评审已有统一引用点。
4. [X] 各模块 `module.go` / `links.go` 已预留为 Composition Root 挂载位置，且目前没有演化成新的业务逻辑堆积点。

### 部分履职项

1. [ ] Composition Root 已有文件落点，但尚未真正承接组装职责；当前更像“预留骨架”而非“实际装配主入口”。
2. [ ] `CleanArchGuard` 已提供基础层级检查，但尚不足以验证 `015` 中更细颗粒度的架构目标。
3. [ ] DB Kernel 路线已经在多个模块落地，但 “services 做统一 Facade、infrastructure 提供统一 Kernel Port” 这一表达方式尚未全仓一致。

### 明显缺口

1. [ ] `internal/server` 仍承担大量默认装配职责，包括建 pool、new store、new service，未完成向模块 Composition Root 的收口。
2. [ ] `internal/server` 仍保留多处模块内 PG store / Kernel 调用实现，说明部分模块能力仍未回收到模块自己的 `services` / `infrastructure` 分层中。
3. [ ] 存在 `infrastructure -> services` 的分层回流实例，当前实现与 `015` 的分层叙事不一致。
4. [ ] 现有 `.gocleanarch.yml` 无法直接阻断“组合根未收口”“`internal/server` 继续持有模块实现”“infrastructure 反向依赖 services”这类偏差。

## 关键证据

### 1. Composition Root 仍未成为实际装配主入口

`DEV-PLAN-015` 将 `modules/{module}/module.go`、`links.go` 定义为 Composition Root，并要求其负责“依赖注入、注册路由/控制器/服务、拼装 infra 实现”。

但当前：

1. [ ] 当前活体侧的 `modules/orgunit/module.go`、`modules/iam/module.go` 及对应 `links.go` 仍未完全做到“名实相符”。
2. [ ] 默认装配仍有一部分集中在 `internal/server/handler.go`：包括创建 PG pool、决定默认 store、拼装 write service 等。

判断：

- [ ] 这说明 Composition Root 已“有位”，但未“履职到位”。
- [ ] 该问题更适合定性为“实现未完成向蓝图收口”，而不是“015 蓝图错误”。

### 2. `internal/server` 仍偏胖，尚未完成模块装配下沉

当前代码树中，`internal/server` 不仅承担 HTTP/路由适配，还继续承担部分模块内部装配与 persistence 组织工作。例如：

1. [ ] 历史上 `internal/server/staffing.go` / `internal/server/jobcatalog.go` 曾直接持有模块内 PG store 与 Kernel 调用实现；这些例子已随 `DEV-PLAN-450` 退出当前执行面。
2. [ ] `internal/server/setid.go` 曾直接定义 `setidPGStore` 并调用 `orgunit.submit_setid_event(...)`、`submit_setid_binding_event(...)` 等；但当前 SetID 主链也已由 `DEV-PLAN-440` 收口删除。
3. [ ] 当前更值得关注的是：现存 `internal/server` 是否仍保留不应继续存在的模块内装配残余，而不是继续围绕已删除模块评估。

判断：

- [ ] 该现状与 `015` 所描述的“services 做 Facade、infrastructure 提供 Kernel Port 实现”之间仍有明显距离。
- [ ] 问题核心不是 DB Kernel 路线不存在，而是该路线尚未在 Go 侧形成统一、稳定的分层表达。

### 3. 存在 `infrastructure -> services` 的反向依赖

历史上 `modules/staffing/infrastructure/persistence/assignment_pg_store.go` 曾直接 import `modules/staffing/services`，并调用 `PrepareUpsertPrimaryAssignment(...)`、`PrepareCorrectAssignmentEvent(...)` 等准备逻辑；该例已随 `DEV-PLAN-450` 退出当前执行面。

判断：

1. [ ] 这说明部分输入规范化/命令准备逻辑仍位于 `services`，却被 `infrastructure` 反向调用。
2. [ ] 从 `015` 的层次叙事看，这属于明显的分层回流。
3. [ ] 该问题说明当前 lint 通过并不等价于 `015` 所要求的架构已真正对齐。

### 4. 门禁已接线，但强度仍停留在基础层

当前 `.gocleanarch.yml` 仅声明：

1. [X] `domain: domain`
2. [X] `application: services`
3. [X] `infrastructure: infrastructure`
4. [X] `interfaces: presentation`
5. [X] `ignore_tests: true`

并且 `./scripts/ci/cleanarch.sh` 当前可通过。

判断：

1. [ ] 这证明仓库已有基础的目录分层门禁。
2. [ ] 但它尚不能验证 `015` 中更细的目标，例如：
   - [ ] Composition Root 是否真正承接组装职责；
   - [ ] `internal/server` 是否还在直接持有模块实现；
   - [ ] `infrastructure` 是否出现回流依赖 `services`。
3. [ ] 因此更稳妥的表述是：`015` 已提出 lint 应逐步承接的目标，但当前门禁仍停留在基础层级，尚未覆盖这些更细规则。

## 正式结论

`DEV-PLAN-015` 当前不是“失效”或“被证明错误”，而是：

1. [X] 已完成架构蓝图、骨架路径、基本门禁入口的定义。
2. [ ] 尚未完成对现有实现的系统性收口。
3. [ ] 其主要缺口表现为：模块组合根未实际承接装配、`internal/server` 仍承担大量模块内部实现、以及 lint 尚不足以验证更细颗粒度目标。

因此，本次评估的正式定性应固定为：

**`015` 当前属于“蓝图已建立、实现落后于蓝图”的状态。**

## 后续承接建议

### 建议作为后续子计划处理的主题

1. [ ] Composition Root 收口：把默认 store/service 装配从 `internal/server` 迁入模块级 `module.go` / `links.go`。
2. [ ] `internal/server` 瘦身：把模块内部 PG store 与 Kernel 访问逐步回收到模块自己的 `infrastructure` / `services`。
3. [ ] 分层回流修复：清理 `infrastructure -> services` 依赖，把输入规范化/命令准备逻辑下沉到更稳定的 domain/pkg/helper seam。
4. [ ] 门禁增强：在不引入第二套规则源的前提下，逐步让 lint 能覆盖更细颗粒度的架构偏差。

### 不建议在本文件内直接做的事

1. [ ] 不在 `015A` 中直接实施代码重构。
2. [ ] 不在 `015A` 中直接扩写 `.gocleanarch.yml` 细则。
3. [ ] 不把当前所有偏差一次性归责给 `015` 本身。

## 测试与覆盖率

覆盖率与门禁口径遵循仓库 SSOT：

- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`

本计划当前为文档评估类变更：

1. [ ] 不修改生产代码，不单独引入新的覆盖率口径。
2. [ ] 不通过降低阈值、扩大排除项或弱化门禁来回避架构缺口。
3. [ ] 后续若进入代码实施，应按命中触发器执行对应 `go fmt ./...`、`go vet ./...`、`make check lint`、`make test` 等门禁。

## 验收标准

1. [ ] `015A` 已形成稳定、可引用的评估结论。
2. [ ] 文档明确区分“已履职 / 部分履职 / 明显缺口”。
3. [ ] 文档明确把结论固定为“实现落后于蓝图”，避免误写为“015 失效”。
4. [ ] 文档已加入 `AGENTS.md` 的 Doc Map，可被后续计划与评审直接引用。
