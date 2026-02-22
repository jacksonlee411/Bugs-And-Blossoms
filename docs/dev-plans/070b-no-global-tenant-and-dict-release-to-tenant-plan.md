# DEV-PLAN-070B：取消共享租户（global_tenant）并收敛为租户本地发布方案（以字典配置模块为样板）

**状态**: 进行中（2026-02-22 19:10 UTC）

## 1. 背景与上下文 (Context)
- **来源**：承接 `DEV-PLAN-070A` 的调查方向，明确从“运行时共享租户读取”转向“共享数据发布到租户本地（天然隔离）”。
- **决策门（Decision Gate）**：以 `DEV-PLAN-070A` 在 **2026-02-22** 的结论（推荐采纳选项 B）作为本计划前置输入。
- **现状**：字典配置模块已在 `DEV-PLAN-105/105B` 落地 `dict registry + dict value`，但读取口径仍含 tenant/global 组合与 fallback 语义。
- **核心问题**：`global_tenant` 作为运行时读路径，长期在合规解释、安全边界与运维治理上成本偏高。
- **目标收益**：把“共享”从运行时能力改为“发布时能力”，让业务运行时仅访问租户本地数据，形成更清晰的租户边界与审计叙事。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 在业务运行时取消 `global_tenant` 读取路径（字典模块优先，后续推广至 scope package 相关配置域）。
- [ ] 将“共享配置”改为“平台发布 -> 租户落地 -> 租户本地读取”的单链路模式。
- [ ] 保持 One Door / No Tx, No RLS / Valid Time(date) / No Legacy。
- [ ] 形成可执行迁移路径：基线导入、幂等发布、切换验证、清理收口。
- [ ] 对 `DEV-PLAN-070/071/071A/071B/105/105B` 输出条目级修订建议。

### 2.2 非目标
- 不在本计划内一次性改完所有业务模块；采用“字典先行、分域推进”。
- 不引入运行时 Feature Flag 或长期双链路回退。
- 不改变字典模块已冻结的核心语义（`as_of` 必填、有效期窗口、稳定错误码）。
- 不引入业务数据多语言（继续遵循 `DEV-PLAN-020`）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（勾选本计划命中的项）**：
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] DB 迁移 / Schema（按模块 `make <module> plan && make <module> lint && make <module> migrate up`）
  - [ ] sqlc（`make sqlc-generate`）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [ ] Routing（`make check routing`）
  - [ ] E2E（`make e2e`）
  - [x] 文档（`make check doc`）
- **SSOT**：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile`、`.github/workflows/quality-gates.yml`。

### 2.4 与 070A 的前后关系（冻结）
- `070A` 负责“是否转向发布到租户”的方案评估与取舍。
- `070B` 负责“如何分阶段实施”的工程化落地与验收。
- 若后续出现新证据导致 `070A` 结论变化，必须先更新 `070A` 再调整 `070B`（避免决策与实施倒挂）。

### 2.5 与 102B 的前后关系（冻结）
- `102B` 是 070/071 时间口径收敛 PoR；`070B` 必须继承其显式时间契约（`as_of/effective_date` 必填、禁止 default today）。
- `070B` 的回填、对账、切流验收以 `102B` 的“跨天重放一致性”标准为准，不再定义并行口径。
- `070B` 进入切流阶段（PR-070B-5/6）前，要求 `102B` 至少完成 M2（API 收口）+ M3（Kernel 收口）+ M5（门禁落地）。

## 3. 架构原则与关键决策 (Architecture & Decisions)
### 3.1 原则冻结
1. **运行时 tenant-only**：业务读写链路只允许当前租户数据；不跨租户读取。
2. **共享改发布**：共享基线由平台发布，不再通过运行时 `allow_share_read` 暴露。
3. **审计连续性**：发布动作也必须走 One Door 事件写入，可追溯“谁、何时、向哪个租户发布了什么”。
4. **失败即阻断**：目标租户缺少基线时 fail-closed，不回退到 global。
5. **无 legacy 回退**：切换后禁止“空结果回退 global”或兼容旁路。

### 3.2 决策摘要（ADR）
- **决策 A（选定）**：取消 `global_tenant` 作为运行时读路径。
- **决策 B（选定）**：保留“平台维护基线”能力，但只通过发布任务写入目标租户。
- **决策 C（选定）**：以字典模块为首个样板；`scope_package` 路径后续按同口径改造为“租户本地包”。
- **决策 D（选定）**：回滚策略仅允许“环境级停写 + 修复后重试”，不恢复双链路。

## 4. 目标态设计（字典模块优先）
### 4.1 读写语义（目标态）
- `GET /iam/api/dicts`：只读当前租户的 `iam.dicts`（`as_of` 必填保持不变）。
- `GET /iam/api/dicts/values`：只读当前租户的 `iam.dict_value_segments`。
- `ResolveValueLabel/ListOptions`：tenant-only；tenant 无数据即明确错误，不回退 global。
- 字典写入（create/disable/correct/value event）仍走 `submit_*_event(...)`，保持 One Door。

### 4.2 发布机制（新增能力）
- 平台维护“标准字典基线（release）”，以发布任务向目标租户写入 dict/dict values。
- 发布写入必须：
  - 使用独立 `request_code` 幂等；
  - 按 `enabled_on/disabled_on` 保留 Valid Time 语义；
  - 记录发布来源（release_id / operator / tx_time）。
- 新租户开通前置：必须完成字典基线导入，未导入则相关写请求 fail-closed。

### 4.3 与 071/071B 的对齐
- `071B` 中 `tenant_global` 默认模式调整为 `tenant_only`（命名与语义同步收敛）。
- `scope_package` 保留“按 scope 解析”的治理能力，但包数据来源收敛为租户本地，不依赖 global owner 运行时读取。
- `ResolveScopePackage` 目标态输出仅允许租户本地 package 命中；跨租户 owner 作为迁移期数据治理输入，不作为运行时读取路径。

## 5. 数据与迁移策略 (Data & Migration)
> 如需新增表/迁移（例如 release 元数据表、发布状态表），执行前必须按仓库红线获得用户手工确认。

### 5.1 迁移分期
1. [ ] **Phase 0（契约冻结）**
   - 冻结 API/Store/SQL 不变量：禁止 runtime global fallback。
   - 新增防漂移检查：阻断新代码引入 `global_tenant` 读取分支。
2. [ ] **Phase 1（发布基座）**
   - 实现“基线 -> 租户”幂等发布能力（可先从字典模块落地）。
   - 建立发布审计字段与失败重试语义。
3. [ ] **Phase 2（历史回填）**
   - 基于 `as_of` 导出当前共享基线，回填到各租户本地字典。
   - 对“租户已覆盖”场景执行冲突检测并产出处理清单。
4. [ ] **Phase 3（切流）**
   - 停写窗口内执行最终增量发布与一致性校验。
   - 部署 tenant-only 读路径并移除 global fallback。
5. [ ] **Phase 4（收口）**
   - 清理 `allow_share_read` 在字典链路的使用。
   - 下线/封存不再被运行时使用的 global 读函数与查询分支。

### 5.2 一致性校验（最小集合）
- [ ] 租户维度 `dict_code` 数量与基线预期一致。
- [ ] 关键 dict_code（首批 `org_type`）按 `as_of` 抽样比对一致。
- [ ] 发布日志可追溯到 request_code / release_id / operator。
- [ ] 切流后任何 API 不再访问 global 字典路径。

## 6. API/错误码/权限调整点
### 6.1 API 行为调整
- `GET /iam/api/dicts?as_of=...`：删除 global fallback 语义，返回 tenant-only 视图。
- `GET /iam/api/dicts/values?...`：删除“tenant 不存在时回退 global”语义。
- 可新增发布控制面 API（仅管理端），业务面 API 不新增跨租户读取参数。

### 6.2 错误码（新增/收敛）
- [ ] 新增：`dict_baseline_not_ready`（租户未完成基线导入）。
- [ ] 收敛：`dict_not_found` 明确为“当前租户未命中”，不再隐含 global 查询。
- [ ] 对齐 `STD-002`：读接口缺失/非法 `as_of` 统一 `400 invalid_as_of`（message：`as_of required`）。
- [ ] 对齐 `STD-002`：写接口缺失/非法业务生效日统一 `400 invalid_effective_date`（message：`effective_date required`）。

### 6.3 权限模型
- 业务面保持：`dict.read` / `dict.admin`。
- 新增发布能力建议独立权限（如 `dict.release.admin`），避免复用业务租户管理员权限。

## 7. 测试与覆盖率 (Testing & Coverage)
- 覆盖率口径与阈值遵循仓库 SSOT（`Makefile` + CI workflow）；本计划不引入豁免。
- 必须新增/更新测试：
  - [ ] tenant-only 读取测试（不存在 global fallback）。
  - [ ] 发布幂等测试（同 request_code 同 payload 幂等、不同 payload 冲突）。
  - [ ] 新租户未导入基线时 fail-closed 测试。
  - [ ] 历史回放测试（`as_of` 结果与发布后租户本地数据一致）。

## 8. 风险与缓解 (Risks & Mitigations)
- **R1：发布链路复杂度上升**
  - 缓解：先做字典样板、限制首批 dict_code、严格幂等与审计。
- **R2：迁移窗口失败导致租户不可写**
  - 缓解：停写窗口 + 分批演练 + 快速重试脚本；不回退双链路。
- **R3：租户已有自定义覆盖与基线冲突**
  - 缓解：迁移前输出冲突清单并人工确认处理策略（保留租户覆盖优先）。
- **R4：范围蔓延**
  - 缓解：本计划先聚焦字典模块，scope_package 仅定义衔接原则，实施另起子计划。

## 9. 里程碑与交付物 (Milestones & Deliverables)
1. [ ] **M1 契约与门禁**：完成 070B 文档冻结 + 防漂移检查设计。
2. [ ] **M2 字典发布样板**：完成字典基线发布链路与审计。
3. [ ] **M3 租户回填与切流**：完成迁移、切流、校验证据。
4. [ ] **M4 收口**：清理 global 读分支，更新关联计划状态。

**交付物**：
- 本计划文档（`070B`）
- 后续实施子计划（建议 `070B1` 字典落地、`070B2` scope package 落地）
- 执行证据记录（`docs/dev-records/dev-plan-070b-execution-log.md`）

### 9.1 详细实施拆解（按 PR 主轴）
> 原则：单 PR 单主轴、可验证、可回滚（仅环境级停写+修复后重试），禁止引入 legacy 双链路。

#### PR-070B-1：契约冻结与防漂移门禁（Docs + Guard）
- [ ] 冻结“运行时 tenant-only、共享改发布、禁止 global fallback”契约文字（070B + 受影响 P0 文档）。
- [ ] 新增防漂移检查：阻断新代码新增 `global_tenant` 字典读取分支。
- [ ] 固化迁移期错误码与术语：`dict_baseline_not_ready`、`tenant_only`、`release`。
- [ ] 产出：契约差异清单（旧口径 -> 新口径）并落档到 `docs/dev-records/dev-plan-070b-execution-log.md`。

#### PR-070B-2：发布基座数据模型与 Kernel 写入口（DB + One Door）
- [ ] 设计并落地“发布元数据 + 发布任务状态”最小模型（如需新增表，先获得用户手工确认）。
- [ ] 发布写入全部走 iam kernel 事件入口，禁止控制器直写字典表。
- [ ] 增加幂等/冲突约束：同 `(tenant, request_code)` 幂等，不同 payload 冲突拒绝。
- [ ] 增加审计字段：`release_id`、`operator`、`tx_time`、`initiator`。
- [ ] 产出：Schema/迁移/函数变更与测试证据。

#### PR-070B-3：发布服务与控制面 API（Service + API）
- [ ] 新增“基线发布到租户”服务编排：创建任务、分批执行、重试与终态收敛。
- [ ] 新增管理端发布 API（仅控制面，非业务读 API），并接入独立权限（建议 `dict.release.admin`）。
- [ ] 新租户开通流程接入“先导入基线后开放写入”的前置校验。
- [ ] 产出：发布 API 合约样例、权限策略、失败路径测试。

#### PR-070B-4：字典读链路 tenant-only 改造（Store + Handler + pkg/dict）
- [ ] `GET /iam/api/dicts` / `GET /iam/api/dicts/values` 移除 global fallback 逻辑。
- [ ] `ResolveValueLabel/ListOptions` 改为 tenant-only；未命中返回稳定错误码。
- [ ] 清理 `allow_share_read` 在字典读取路径的依赖与测试夹具。
- [ ] 产出：行为回归测试（含“tenant 空结果不回退 global”）。

#### PR-070B-5：历史数据回填与对账工具（Migration Tooling）
- [ ] 实现“按 `as_of` 导出共享基线 -> 导入租户本地”脚本/任务（幂等可重放）。
- [ ] 增加冲突检测：租户已有覆盖值与基线冲突时输出人工确认清单。
- [ ] 增加对账：`dict_code` 数量、关键 code/label、窗口边界（`enabled_on/disabled_on`）抽样比对。
- [ ] 产出：回填报告与对账报告（附执行时间与环境）。

#### PR-070B-6：切流执行与停写窗口 Runbook（Release Cutover）
- [ ] 预演至少 1 次（预发布环境），验证停写、增量发布、切流、验收、恢复写入全流程。
- [ ] 生产切流按 runbook 执行：停写 -> 最终增量发布 -> tenant-only 版本发布 -> 验收 -> 开写。
- [ ] 切流后 24h 内完成重点租户抽样核验与异常闭环。
- [ ] 产出：切流执行记录与验收记录。

#### PR-070B-7：收口清理与文档对齐（Cleanup）
- [ ] 清理不再使用的 global 字典读取函数/分支/测试桩。
- [ ] 完成 P0/P1 受影响文档修订与状态更新。
- [ ] 复核 `No Legacy`：代码、路由、策略中不存在 fallback/兼容别名窗口。
- [ ] 产出：收口清单与最终验收结论。

### 9.2 执行节奏与依赖顺序（冻结）
1. [ ] 完成 PR-070B-1（不通过不允许进入代码实施）。
2. [ ] PR-070B-2 与 PR-070B-3 串行推进（先基座后服务）。
3. [ ] PR-070B-4 在 PR-070B-3 可用后落地（确保基线导入能力先可用）。
4. [ ] `DEV-PLAN-102B` 至少完成 M2+M3+M5，并在 `docs/dev-records/dev-plan-102b-execution-log.md` 留证。
5. [ ] PR-070B-5 在切流前完成并输出冲突清单与对账报告。
6. [ ] PR-070B-6 切流完成后，立即推进 PR-070B-7 收口，关闭迁移窗口。

### 9.3 切流 Runbook（最小步骤）
- [ ] **T-7 ~ T-3**：完成预演、冻结发布脚本版本、确认回填与对账通过。
- [ ] **T-1**：发布窗口评审（变更范围、回滚口径、责任人、通讯录）并发公告。
- [ ] **T0（停写窗口）**：停写 -> 最终增量发布 -> 部署 tenant-only 代码 -> 执行验收清单 -> 恢复写入。
- [ ] **T+1**：抽样租户业务回归、错误码与审计链巡检、关闭变更窗口。

### 9.4 验证与证据模板（执行期必填）
- [ ] 命中触发器与门禁结果（按 `AGENTS.md` 触发器矩阵记录）。
- [ ] 关键 SQL/API 验证输出（tenant-only 命中、无 global fallback）。
- [ ] 发布任务审计证据（release_id/request_code/operator/tx_time）。
- [ ] 异常与修复记录（失败原因、重试次数、最终结果）。
- [ ] 102B 对齐证据（`invalid_as_of` / `invalid_effective_date`、`as_of/effective_date` 显式必填、跨天回放一致）。

## 10. 验收标准 (Acceptance Criteria)
- [ ] 运行时业务链路不再依赖 `global_tenant` 字典读取。
- [ ] 任一字典读取失败不再触发 global 回退。
- [ ] 新租户未导入基线时，相关写入口稳定 fail-closed。
- [ ] 审计可回答“某租户某字典值来自哪个发布、何时落地、由谁触发”。
- [ ] 与仓库不变量无冲突（One Door / No Tx, No RLS / No Legacy / Valid Time）。
- [ ] 时间参数口径与 `STD-002`/`DEV-PLAN-102B` 一致（缺失/非法日期统一 `invalid_*`，无 default today）。

### 10.1 070A 评估维度到 070B 验收映射
| 070A 评估维度 | 070B 对应验收项 | 关键证据 |
| --- | --- | --- |
| 隔离强度 | “运行时业务链路不再依赖 `global_tenant`” | store/SQL 路径审计、集成测试 |
| 合规可解释性 | “不再触发 global 回退” + “审计可回答发布来源” | 错误码稳定性、发布审计日志 |
| 安全稳健性 | “fail-closed + 无 legacy 回退” | 权限用例、异常路径测试 |
| 可扩展性 | “发布到租户本地后可重放一致” | as_of 回放测试、对账报告 |
| 运维复杂度 | “切流 runbook 可执行且可回溯” | 停写窗口记录、执行日志 |
| 迁移成本 | “分阶段里程碑按证据推进” | PR 证据链、里程碑勾选记录 |

## 11. 受影响历史开发计划文档与修订建议
### 11.1 受影响文档清单（历史计划）
| DEV-PLAN | 文档 | 影响级别 | 影响点摘要 |
| --- | --- | --- | --- |
| DEV-PLAN-028 | `docs/dev-plans/028-setid-management.md` | 高 | 历史口径仍含旧 SetID 语义；需补充“070B 后不再存在运行时共享租户读取”说明。 |
| DEV-PLAN-070 | `docs/dev-plans/070-setid-orgunit-binding-redesign.md` | 高 | 共享层与租户层边界需更新为“运行时 tenant-only + 发布式共享”。 |
| DEV-PLAN-070A | `docs/dev-plans/070a-setid-global-share-vs-tenant-native-isolation-investigation.md` | 高 | 候选路线应收敛为 070B 主路径，并补齐决策结论与取舍依据。 |
| DEV-PLAN-071 | `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md` | 高 | scope package 的 shared-only 描述需从“运行时共享读”改为“发布后本地读”。 |
| DEV-PLAN-071A | `docs/dev-plans/071a-package-selection-ownership-and-subscription.md` | 中 | package 编辑归属不变，但订阅消费路径需去除 global 运行时依赖。 |
| DEV-PLAN-071B | `docs/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md` | 高 | `tenant_global` 需收敛为 `tenant_only`，并补发布治理与 fail-closed 口径。 |
| DEV-PLAN-102B | `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md` | 高 | 作为 070/071 时间契约 PoR：`as_of/effective_date` 显式必填、`invalid_*` 错误码冻结、跨天回放一致性门禁。 |
| DEV-PLAN-105 | `docs/dev-plans/105-dict-config-platform-module.md` | 高 | 字典读取口径需移除 tenant/global fallback，改为 tenant-only。 |
| DEV-PLAN-105A | `docs/dev-plans/105a-dict-config-validation-issues-investigation.md` | 中 | 校验问题需补“基线未导入”与“不回退 global”错误路径。 |
| DEV-PLAN-105B | `docs/dev-plans/105b-dict-code-management-and-governance.md` | 高 | tenant/global 冲突与 fallback 条款需更新为“仅租户本地 + 发布同步”。 |
| DEV-PLAN-106 | `docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md` | 中 | 字典候选来源仍是 dict registry，但需补“租户基线就绪”前置条件。 |
| DEV-PLAN-106A | `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md` | 中 | `d_<dict_code>` 候选生成需明确 tenant-only 来源与基线未就绪失败语义。 |

### 11.2 条目级修订建议（最小集）
- `DEV-PLAN-070A`：将“选项 B（发布到租户）”从候选提升为推荐主路径。
- `DEV-PLAN-071`：共享包读取描述改为“发布到租户本地后解析”，弱化/移除运行时共享读。
- `DEV-PLAN-071A/071B`：`tenant_global` 命名与语义收敛为 `tenant_only`，并补充发布治理条款。
- `DEV-PLAN-105/105B`：补充“取消 global fallback”后的字典读取与迁移验收条目。
- `DEV-PLAN-102B`：将 M2/M3/M5 明确为 070B 切流前置门；070B 不再定义独立时间错误码分支。

### 11.3 同步优先级分组（P0/P1）
> 口径：P0=070B 进入“准备就绪”前必须完成同步；P1=可在 070B 实施期内后置同步，但不得晚于 070B 收口验收。

- **P0（必须同步修订）**
  - `DEV-PLAN-070`（SetID 主方案边界）
  - `DEV-PLAN-070A`（调查结论收敛）
  - `DEV-PLAN-071`（scope package 读取主路径）
  - `DEV-PLAN-071A`（订阅/所有权与消费路径）
  - `DEV-PLAN-071B`（`tenant_global -> tenant_only` 契约）
  - `DEV-PLAN-102B`（as_of 回放稳定性口径）
  - `DEV-PLAN-105`（字典读取契约）
  - `DEV-PLAN-105B`（dict registry/fallback 规则）

- **P1（可后置修订）**
  - `DEV-PLAN-028`（历史方案说明补注）
  - `DEV-PLAN-105A`（验证问题与错误路径补充）
  - `DEV-PLAN-106`（字段启用前置条件补充）
  - `DEV-PLAN-106A`（`d_<dict_code>` 候选口径补充）

## 12. 关联文档
- `AGENTS.md`
- `docs/dev-plans/070a-setid-global-share-vs-tenant-native-isolation-investigation.md`
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/105-dict-config-platform-module.md`
- `docs/dev-plans/105b-dict-code-management-and-governance.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/012-ci-quality-gates.md`
