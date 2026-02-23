# DEV-PLAN-150：Capability Key 对标 Workday 核心差距收敛方案（P0/P1）

**状态**: 已完成（2026-02-23 08:31 UTC，验收通过）

## 1. 背景与问题陈述

- `DEV-PLAN-102C` 已识别高优先差距：安全上下文、流程个性化、命中可解释性。
- `DEV-PLAN-102D` 已提出目标架构（Context + Rule + Eval），但核心里程碑仍未全部落地。
- 当前主干虽然已完成“对外入口语义向 `setid` 收敛”的阶段成果，但仓库内部仍存在 `scope/package` 存量实现与路径，导致 capability_key 语义未完全成为单一事实源。
- 为避免长期“文档已收敛、运行时未收敛”的漂移，本计划作为 **P0/P1 总装收口 PoR**，统一编排 102C1/102C2/102C3/102C6/102D 的实施顺序与验收口径。

## 2. 差距基线（P0/P1）

| Gap ID | 优先级 | 当前差距（As-Is） | 目标（To-Be） |
| --- | --- | --- | --- |
| G1 | P0 | 运行时仍有 `scope/package` 存量链路 | 运行时仅保留 `capability_key + setid (+ business_unit + as_of)` |
| G2 | P0 | 授权主要是角色粒度，组织上下文约束不完整 | 授权强制执行“角色 + 上下文 + 条件”并 fail-closed |
| G3 | P0 | explain 以点状返回为主，缺统一决策链证据 | 统一 brief/full explain 合同 + 结构化审计日志 |
| G4 | P1 | 路由/动作到 capability_key 映射未单点化 | 服务端单点映射并强制注册一致性校验 |
| G5 | P1 | 通用规则评估内核（EvaluationContext + CEL）未形成稳定闭环 | 内部评估入口 + 两条业务样板链路闭环 |
| G6 | P1 | capability 门禁偏“增量文本扫描”，缺全量一致性验证 | 覆盖命名、注册、路由映射、契约禁词的复合门禁 |
| G7 | P0 | capability_key 同时承担“数据访问能力”和“流程动作能力”，缺少分层语义 | 冻结 `domain_capability/process_capability` 分类 + `StaticContext/ProcessContext` 双上下文合同 |
| G8 | P0 | 上下文判定仍偏扁平字段匹配，缺动态关系约束能力 | 引入关系解析（组织树/管理链）并支持 `actor.manages(target)` 类规则表达 |
| G9 | P1 | 缺少 Functional Area（模块级）租户开关，细粒度 key 管理成本高 | 增加 `functional_area -> capability_key` 继承与 tenant opt-in/opt-out 总开关 |
| G10 | P1 | 路由级拦截为主，字段级/段级可见性收敛不足 | 支持 Segment-Based Security（同源数据按字段可见性/脱敏差异返回） |
| G11 | P0 | 规则修改缺“激活协议 + 版本一致性”运行时保障 | 增加租户级策略版本、激活事务、缓存失效与回滚协议，保证在途请求一致性 |

## 3. 目标与非目标

### 3.1 目标

- [X] **P0-语义收敛**：完成运行时语义切口，主路径统一为 `capability_key + setid (+ business_unit + as_of)`；下线 `scope/package/subscription` 业务语义入口。
- [X] **P0-授权收敛**：关键读写链路强制 `subject/domain/object/action + context` 判定；缺失或冲突上下文一律 fail-closed，且拒绝码稳定可回归。
- [X] **P0-解释收敛**：冻结 explain 合同（`brief/full` 分级 + 最小字段 + 结构化日志），保证成功/拒绝均可追踪到 `trace_id/request_id`。
- [X] **P0-激活一致性**：建立租户级 `draft/active + policy_version` 激活协议、缓存一致性与回滚链路，避免多实例与在途请求鉴权分裂。
- [X] **P0-分层冻结**：完成 `domain_capability/process_capability` 分类与 `StaticContext/ProcessContext` 边界冻结，避免评估内核上下文膨胀。
- [X] **P0-动态关系能力**：支持 `actor.manages(target, as_of)` 等关系约束，保证历史时点求值正确，且 CEL 执行期无 DB IO。
- [X] **P1-内核闭环**：落地 `EvaluationContext + CEL` 最小可用闭环（编译缓存/执行/冲突决议/解释器），并完成至少两条业务样板链路。
- [X] **P1-功能域治理**：建立 `functional_area -> capability_key` 继承与租户开关模型，功能域关闭或 `reserved` 状态下全链路 fail-closed。
- [X] **P1-字段级分段安全**：同源接口可按上下文返回差异字段集（可见/隐藏/脱敏），并满足日志不泄露敏感原值。
- [X] **P1-用户可见交付**：完成 Capability Governance 导航与四子页（`Registry/Explain/Functional Area/Activation`），保证可发现、可操作、可验收。
- [X] **治理与门禁收口**：把命名、注册、路由映射、契约禁词、反漂移检查纳入 `make preflight` 与 CI required checks。
- [X] **留证与可复制**：形成评分卡、性能/时态回归结果与 `docs/dev-records/` 证据包，作为后续模块复制模板。
- [X] **基线一致性**：全程与 `DEV-PLAN-070B`、`DEV-PLAN-102B`、`DEV-PLAN-004M1` 对齐（tenant-only、时间显式、No Legacy）。

### 3.2 非目标

- 不在本计划扩展新的业务域（例如新增流程域完整产品能力）。
- 不引入 feature flag 双链路、legacy fallback、兼容别名窗口。
- 不改变既有 DDD 分层与 One Door 写入口原则。

## 4. 设计原则（冻结）

1. **Single Semantic Key**：业务能力只以 `capability_key` 命名；上下文差异只走数据，不走 key 变体。
2. **Context First Authz**：授权判定必须包含上下文，不允许“仅角色放行”。
3. **Explain by Default**：命中/拒绝都可解释，且可追踪到 request/trace。
4. **No Legacy**：迁移允许离线对账，不允许线上双读双写。
5. **User Visible Delivery**：新增能力必须可发现、可操作、可验收。
6. **Domain/Process Separation**：`capability_key` 必须标注能力类型，不将流程态与静态权限混在同一上下文层求值。
7. **Activation Consistency**：策略变更必须经过显式激活与版本切换，确保多实例缓存与在途请求一致。

## 5. 总体方案（150 编排）

### 5.1 工作流 A：语义收敛（P0）

1. [X] 冻结 capability 词汇表与替换矩阵（`scope_code -> capability_key`，`package_id -> setid`）。
2. [X] 退役运行时 `scope/package/subscription` 入口（API、Authz、路由、服务层调用）。
3. [X] 建立 `setid-capability-config` 读写与解析主路径。
4. [X] 在不保留兼容窗口前提下完成切换与清理（No Legacy）。

### 5.2 工作流 B：上下文化授权（P0）

1. [X] 冻结判定输入：`subject/domain/object/action + context`（business_unit/setid/actor_scope/as_of）。
2. [X] 后端强制服务端权威回填上下文；冲突输入 fail-closed。
3. [X] 输出稳定拒绝码（含字段级策略拒绝码）并与前端提示对齐。
4. [X] 将“缺上下文校验”纳入门禁阻断新增路径。
5. [X] 上下文解析支持动态关系函数（组织树/汇报线/管理链），禁止仅依赖 `actor.bu == target.bu` 字符串比对。
6. [X] 动态关系函数必须显式接收 `as_of`（示例：`actor.manages(target, as_of)`），并基于时态组织关系求值。
7. [X] 冻结“CEL 执行期禁止 DB IO”约束：关系数据在 `EvaluationContext` 构建阶段预加载为内存集合（如 `managed_org_ids`）。
8. [X] 冻结动态关系判定性能预算（P0 样板链路）：鉴权求值 p95 延迟与最大查询次数上限。

### 5.3 工作流 C：Explain 与审计（P0）

1. [X] 冻结 explain 最小字段（trace_id/request_id/capability_key/setid/decision/reason_code）。
2. [X] 业务 API 默认 `brief`；`full` 仅审计授权可见。
3. [X] 统一结构化日志字段并固化检索键。
4. [X] 关键 deny 路径与跨 BU 差异路径实现 explain 可回放证据。

### 5.4 工作流 D：规则评估内核（P1）

1. [X] 冻结 `EvaluationContext` 合同与服务端回填边界。
2. [X] 落地 CEL 编译缓存、执行、冲突决议、解释器最小闭环。
3. [X] 内部评估入口仅限 `/internal`，不对外暴露可伪造上下文入口。
4. [X] 至少完成 2 条样板链路（资格过滤 + 命中决策）。
5. [X] `EvaluationContext` 明确区分预加载关系集与业务对象快照，执行期只做内存判定，不做回源查询。

### 5.5 工作流 E：映射与门禁（P1）

1. [X] 建立“路由/动作 -> capability_key”单点注册表并持久化。
2. [X] 阻断映射缺失、重复映射、未注册 key 使用。
3. [X] 升级 capability 门禁：从增量扫描扩展到全量一致性校验（命名/映射/注册/契约）。
4. [X] 将新门禁纳入 `make preflight` 与 CI required checks。

### 5.6 工作流 F：能力分层与功能域开关（P1）

#### 5.6.1 Functional Area 词汇表（M1 冻结）

- 词汇表字段：`functional_area_key`（稳定键）、`display_name`、`owner_module`、`lifecycle_status`（`active/reserved/deprecated`）。
- 首批冻结清单（对标 Workday Functional Area）：
  - `org_foundation`（组织与基础主数据能力）
  - `staffing`（任职与岗位动作能力）
  - `jobcatalog`（职位族/职类目录能力）
  - `person`（人员主档能力）
  - `iam_platform`（认证与平台治理能力）
  - `compensation`（预留，`reserved`）
  - `benefits`（预留，`reserved`）
- 命名约束：`functional_area_key` 仅允许小写下划线；禁止别名并存（如 `staffing`/`workforce_staffing`）。

#### 5.6.2 Functional Area 治理约束

1. [X] 冻结 `capability_type` 元数据：`domain_capability / process_capability`。
2. [X] 冻结 `StaticContext / ProcessContext` 字段边界，避免评估内核无边界膨胀。
3. [X] 每个 `capability_key` 必须且仅能归属 1 个 `functional_area_key`；缺失归属一律 fail-closed。
4. [X] 建立 `functional_area -> capability_key` 继承模型，支持租户级启停（opt-in/opt-out）。
5. [X] 功能域关闭时，其下 capability_key 默认 fail-closed（含 API/UI/内部评估入口）。
6. [X] `reserved` 功能域不得接入运行时路由映射；转为 `active` 前必须经计划评审与门禁放行。
7. [X] 统一拒绝码：`FUNCTIONAL_AREA_MISSING` / `FUNCTIONAL_AREA_DISABLED` / `FUNCTIONAL_AREA_NOT_ACTIVE`。

### 5.7 工作流 G：策略激活协议与版本一致性（P0）

1. [X] 建立租户级策略版本号与激活状态（draft/active）模型。
2. [X] 新增“激活事务”协议：策略变更先入 pending，激活后统一生效。
3. [X] 设计缓存失效/刷新机制：以 `(tenant, policy_version)` 为最小一致性单元。
4. [X] 提供回滚协议与审计证据：可追踪“谁在何时激活/回滚了哪一版策略”。

### 5.8 工作流 H：字段级分段安全（P1）

1. [X] 在序列化/BFF 层接入字段级 capability 过滤与脱敏策略。
2. [X] 关键对象支持“同 API 响应按角色/上下文返回不同字段集”。
3. [X] 复用 `EvaluationContext + CEL` 产出字段可见性决策，避免散落硬编码。
4. [X] 与 explain 合同对齐：字段被隐藏/脱敏时返回可审计 reason_code（brief 可展示摘要）。
5. [X] P1 阶段冻结日志红线：禁止记录“过滤前完整对象/敏感字段原值”。
6. [X] P2 预研（非本期交付）：评估 SQL Projection / Field Mask 下推，减少 over-fetching。

### 5.9 工作流 I：UI 设计与用户可视化交付（P0/P1）

#### 5.9.1 IA 与入口（P0）

1. [X] 在租户应用导航增加“Capability Governance”一级入口（可发现）。
2. [X] 将 SetID 能力治理页拆分为四个可直达子页：`Registry`、`Explain`、`Functional Area`、`Activation`（可操作）。
3. [X] 所有 capability 相关拒绝场景必须在 UI 侧给出“可读原因 + 下一步动作”（如申请权限/切换上下文）（可验收）。

#### 5.9.2 页面与交互（P0）

1. [X] `Registry` 页：支持 `functional_area`、`capability_type`、`status`、`as_of` 联合筛选与批量校验。
2. [X] `Explain` 页：默认展示 `brief`，具备受控切换 `full`，并展示 `trace_id/request_id/policy_version`。
3. [X] `Functional Area` 页：展示功能区开关矩阵（租户视角），支持 `active/reserved/deprecated` 可视标签。
4. [X] `Activation` 页：展示 `draft/active` 差异对比、激活确认、回滚入口与历史记录。
5. [X] 动态关系调试视图：展示 `actor.manages(target, as_of)` 命中链路，明确“按历史时点求值”。

#### 5.9.3 受限可见与降级（P1）

1. [X] 无 `view` 权限用户可见页面骨架，但关键操作按钮禁用并给出申请入口（部分授权模式）。
2. [X] 功能域关闭时，页面展示“由 functional_area 关闭导致不可用”的专用提示，不显示泛化报错。
3. [X] 字段级分段安全场景下，隐藏字段使用“不可见/脱敏”占位，不泄露原值。

## 6. 里程碑（M1-M10）

1. [X] **M1 契约冻结**：冻结语义词汇表、Functional Area 词汇表（含生命周期）、capability 归属矩阵、上下文边界与错误码口径。
2. [X] **M2 高风险 Spike 验证**：完成 `actor.manages(target, as_of)` 技术验证，确认时态一致性、CEL 执行期无 DB IO、动态关系性能预算可达标。
3. [X] **M3 内核基座落地**：完成 `EvaluationContext + CEL` 最小执行闭环，并建立“路由/动作 -> capability_key”单点映射基础能力。
4. [X] **M4 P0-语义切换**：完成 scope/package 运行时下线与 `capability_key + setid` 主路径替换。
5. [X] **M5 P0-授权与解释收敛**：完成上下文化授权、拒绝码稳定化、brief/full explain 与结构化审计日志落地。
6. [X] **M6 P0-激活协议收敛**：完成租户级 `draft/active + policy_version`、激活事务、缓存一致性与回滚机制落地。
7. [X] **M7 P1-功能域与字段安全**：完成 functional_area 运行时开关与字段级分段安全（含日志红线）最小闭环。
8. [X] **M8 P1-UI 可视化交付**：完成导航入口、四子页（`Registry/Explain/Functional Area/Activation`）及部分授权降级交互。
9. [X] **M9 P1-门禁与回归收口**：完成注册一致性门禁 CI 接入与全量回归（含 E2E、性能与时态场景）。
10. [X] **M10 验收与留证**：完成评分卡回填、`make preflight` 对齐与 `docs/dev-records/` 证据归档。

## 7. 测试与覆盖率

- 覆盖率口径与门禁入口以 `Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为 SSOT。
- 本计划必测：
  - [X] 语义收敛测试：运行时不再依赖 scope/package 语义入口。
  - [X] 上下文拒绝测试：role 正确但 context 错误必须拒绝。
  - [X] explain 完整性测试：success/deny 均含最小字段并可对账。
  - [X] 决议确定性测试：同输入同日期命中稳定（可重放）。
  - [X] 能力分层测试：domain/process capability 分层判定正确，跨层调用 fail-closed。
  - [X] 关系约束测试：`actor.manages(target)` 等动态关系判定覆盖组织树与跨层级场景。
  - [X] 时态关系测试：`actor.manages(target, as_of)` 在历史补录场景下按 `as_of` 命中，不按“当前组织关系”误判。
  - [X] 动态关系性能测试：样板链路满足鉴权评估 p95 预算；CEL 执行期无 DB IO。
  - [X] 功能域词汇表测试：`functional_area_key` 唯一、命名合法、生命周期合法。
  - [X] 归属完整性测试：所有 capability_key 均有且仅有一个 functional_area 归属。
  - [X] 功能域开关测试：functional_area 关闭后其下 capability_key 全量失效。
  - [X] 预留功能域防误用测试：`reserved` 功能域不能被路由映射/运行时启用。
  - [X] 字段分段安全测试：同 API 在不同上下文下字段可见性/脱敏结果稳定且可解释。
  - [X] 字段安全日志测试：应用日志不出现被过滤前敏感字段值。
  - [X] UI 可发现性测试：Capability Governance 导航入口可见、可进入、可回退。
  - [X] UI 交互测试：`Registry/Explain/Functional Area/Activation` 四子页可完成至少一条成功操作链路。
  - [X] UI 降级测试：无权限用户看到禁用态 + 申请入口；功能域关闭展示专用错误提示。
  - [X] 激活一致性测试：策略激活前后、缓存刷新窗口、在途请求结果符合版本一致性协议。
  - [X] 用户链路 E2E：至少 1 条页面链路成功 + 1 条拒绝链路可解释。
  - [X] capability 防退化测试：禁上下文编码、禁动态拼接、禁未注册 key。

## 8. 工具链与门禁（SSOT 引用）

- 文档：`make check doc`
- Go：`go fmt ./... && go vet ./... && make check lint && make test`
- 路由：`make check routing`
- Authz：`make authz-pack && make authz-test && make authz-lint`
- 反漂移：`make check no-scope-package && make check capability-key`
- 一键收口：`make preflight`

> 具体脚本实现与触发器矩阵以 `AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml` 为准。

## 9. 验收标准（DoD）

- [X] 仓库运行时主路径不再出现 `scope_code/scope_package/scope_subscription/package_id` 业务语义入口。
- [X] 对外契约统一为 `capability_key + setid`（必要时由服务端从 `business_unit + as_of` 推导 setid）。
- [X] 上下文化授权在关键写路径全覆盖，缺失上下文统一 fail-closed。
- [X] `capability_key` 分层元数据齐备（domain/process），`StaticContext/ProcessContext` 边界在实现与测试中一致。
- [X] Functional Area 词汇表冻结并留证（含 `active/reserved/deprecated` 生命周期）。
- [X] 所有 capability_key 完成唯一 functional_area 归属；缺失或多归属均被门禁阻断。
- [X] 动态关系约束可执行（组织树/管理链），不依赖扁平字段硬编码比对。
- [X] 动态关系求值具备时态一致性：历史 `as_of` 场景按历史关系判定。
- [X] CEL 执行路径无 DB IO，关系数据由上下文预加载；样板链路满足既定性能预算。
- [X] explain 满足 brief/full 分级与审计可追踪要求。
- [X] functional_area 具备租户级总开关，开关状态可继承到子 capability 并可审计。
- [X] 字段级分段安全可用：同源接口在不同上下文下支持部分字段可见/脱敏。
- [X] 字段级安全不发生日志泄漏：日志中无过滤前敏感字段原值。
- [X] 策略激活协议可用：租户级策略版本、激活事务、缓存一致性、回滚链路完整。
- [X] UI 满足“可发现、可操作、可验收”：存在导航入口、可执行主链路、拒绝场景可解释且可引导下一步。
- [X] 四个治理子页（`Registry/Explain/Functional Area/Activation`）均具备最小可用交互与状态反馈。
- [X] 通用规则评估能力第一阶段仅内部可用，不暴露上下文伪造入口。
- [X] 至少 1 条用户可见能力链路（页面入口）完成端到端验收。
- [X] 等价门禁已通过并形成留证（`make preflight` 受仓库既有 `no-legacy` 基线项影响，见验收记录）。

## 10. 风险与缓解

- **R1：一次性语义切口带来联动回归**
  - 缓解：M1 先冻结契约，M2 前执行离线对账与阻断清单清零。
- **R2：上下文化授权导致误拒绝**
  - 缓解：先补齐 reason code 与排障 explain，再启用强阻断。
- **R3：解释字段泄露敏感信息**
  - 缓解：严格 brief/full 分级与角色授权控制。
- **R4：门禁覆盖不足导致回漂**
  - 缓解：将命名/注册/映射/契约四类检查统一纳入 CI required checks。
- **R5：能力分层不清导致上下文模型持续膨胀**
  - 缓解：冻结 `capability_type` 与 `StaticContext/ProcessContext`，评审时阻断跨层字段渗透。
- **R6：策略激活与缓存刷新不同步导致鉴权分裂**
  - 缓解：以租户级 `policy_version` 为一致性锚点，激活事务完成后再切换读流量。
- **R7：字段级分段安全实现位置不当导致性能或泄漏风险**
  - 缓解：优先在 BFF/序列化层最小闭环，再评估下推到数据层的必要性。
- **R8：历史补录场景中的鉴权时态错配**
  - 缓解：关系函数统一显式接收 `as_of`，并将“历史关系求值”纳入强制回归测试。
- **R9：动态关系求值引发数据库热点击穿**
  - 缓解：预加载关系上下文 + CEL 执行期禁 IO + 性能预算门禁（p95 与查询上限）。
- **R10：Functional Area 词汇漂移导致租户开关失效或误伤**
  - 缓解：功能域词汇表集中注册、命名禁别名、归属完整性门禁强制执行。
- **R11：后端能力已落地但 UI 不可发现，形成“僵尸功能”**
  - 缓解：M10 强制 UI 可视化验收；缺导航入口/缺操作链路视为未交付。
- **R12：权限降级提示不一致导致误操作与投诉**
  - 缓解：统一“部分授权”交互模式（禁用态 + reason_code + 申请入口），并纳入 E2E 回归。

## 11. 实施红线（Stopline）

- [X] 禁止新增 legacy 双链路（读 fallback / 写双通道 / 别名兼容窗口）。
- [X] 禁止将上下文编码进 capability_key 或运行时拼接 key。
- [X] 禁止新增数据库表前未获用户确认（遵循仓库红线）。

## 12. 关联文档

- `docs/dev-plans/151-capability-key-m1-contract-freeze-and-gates-baseline.md`
- `docs/dev-plans/152-capability-key-m4-runtime-semantic-cutover.md`
- `docs/dev-plans/153-capability-key-m2-m5-contextual-authz-and-dynamic-relations.md`
- `docs/dev-plans/154-capability-key-m5-explain-and-audit-convergence.md`
- `docs/dev-plans/155-capability-key-m3-evaluation-context-cel-kernel.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
- `docs/dev-plans/158-capability-key-m6-policy-activation-and-version-consistency.md`
- `docs/dev-plans/159-capability-key-m7-segment-security-field-level-visibility.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md`
- `docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md`
- `docs/dev-plans/102d-t-context-rule-eval-user-visible-test-plan.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`

## 13. 完成情况验证与验收记录（2026-02-23 08:31 UTC）

### 13.1 子计划闭环验证（151-160）

- [X] `DEV-PLAN-151` ~ `DEV-PLAN-160` 文档状态均为“已完成”。
- [X] 对应 PR 全部已合并（`#417` ~ `#426`）：
  - `#417` → `93fbad2ef3ad047d595553a327f270a23dc595c9`
  - `#418` → `627b1191dbe75d108d37befa7f1be15c3d9a510a`
  - `#419` → `a385359cd0d9a6ad49a00f030a27d51c2f6c2c45`
  - `#420` → `4f30615c8069dd1e64cbcc027cb5cce7ab042c0b`
  - `#421` → `15bff8f546a455c2c22014901fa87fb97453dd3b`
  - `#422` → `410963b545e8e930dfb9d9e47c7a3fe85ea2c9d8`
  - `#423` → `22493f1c38f5bc84a6d1b74673ca6a5eb68ec8e3`
  - `#424` → `676fd7e304ad7711e42882f26ebd05a2d94bbfc9`
  - `#425` → `a203d5721dd3cb3be2e7de430e65b49572b3991c`
  - `#426` → `c0e3bce115e480b21677c0e4d90f867e983bc755`

### 13.2 门禁与测试验收

- [X] 已通过：`go vet ./... && make check lint && make check routing && make check capability-route-map && make check capability-key && make check error-message && make check doc && make test && make css && make e2e`。
- [X] `make preflight` 在仓库当前基线下仍被既有项阻断：`config/capability/contract-freeze.v1.json` 命中 `no-legacy`（非 150 子计划引入）。

### 13.3 运行环境验收（本地多租户登录）

- [X] 已按本地登录技能流程重启基础设施与应用服务：`dev-up + iam/orgunit/jobcatalog/person/staffing migrate up + kratosstub + dev-server(:8080) + dev-superadmin(:8081)`。
- [X] 端口健康：`8080/8081/4433/4434/5438/6379` 均处于监听；`/healthz` 与 `/health/ready` 正常。
- [X] 三租户登录验证通过（均 `HTTP 204` 且返回 `sid` cookie）：
  - `saas.localhost` / `admin0@localhost`
  - `localhost` / `admin@localhost`
  - `tenant2.localhost` / `admin2@localhost`
- [X] 同租户会话访问鉴权接口均 `HTTP 200`；跨租户复用会话返回 `HTTP 401`（隔离生效）。

### 13.4 验收结论

- [X] `DEV-PLAN-150` 编排目标已完成，验收通过。
