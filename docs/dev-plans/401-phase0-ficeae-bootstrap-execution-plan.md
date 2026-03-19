# DEV-PLAN-401：Phase 0 新线（Ficeae）起步与硬切换执行计划

**状态**: 进行中（2026-03-19 15:35 CST）

## 1. 背景与定位

本计划承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-400](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/400-implementation-roadmap-and-vertical-slice-plan.md)
- “停止旧线维护，干净进行新线开发”的决策

`401` 的职责是把“方向共识”变成“可执行切换”：

- 旧线硬冻结为只读维护态；
- 新线以独立仓库（`Ficeae`）/独立交付链路启动；
- 8 周内完成 `310/320/330` 的 Phase 0 最小可运行闭环。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 完成旧线硬冻结（默认停止功能开发，仅保留极端安全热修入口）。
- [ ] 建立 `Ficeae` 独立代码仓与独立 CI/CD，不复用旧线构建门禁脚本。
- [ ] 完成 Phase 0 横切基线：工程结构、测试金字塔、容器发布、最小观测、安全边界。
- [ ] 形成首批 `Ficeae` 垂直切片起步能力：`登录 -> 壳层 -> 组织列表（只读）`。
- [ ] 建立“规则矿 -> `Ficeae` 验收”可追溯机制（承接 `401A`）。

### 2.2 非目标

- [ ] 本计划不在当前 Go 仓库内做“大规模重写”。
- [ ] 本计划不迁移旧线实现代码、SQL、路由或门禁脚本。
- [ ] 本计划不在 Phase 0 交付完整 `360/370/380/390` 全能力。

## 3. 强制决策（Hard Decisions）

### 3.1 旧线冻结策略

- [ ] 旧线仓库进入“只读维护态”，停止新功能需求接入。
- [ ] 仅允许“安全热修/合规阻断/生产事故修复”三类例外变更。
- [ ] 例外变更必须记录审批人、原因、影响面与过期时间。

### 3.2 新线独立策略

- [ ] `Ficeae` 作为新线独立仓库。
- [ ] `Ficeae` 独立分支治理、独立 CI required checks、独立发布流水线。
- [ ] `Ficeae` 不继承旧线 `setid/package_uuid` 治理语义。

### 3.3 实现输入策略

- [ ] 旧线只作为“规则矿（业务规则+测试样例）”证据源。
- [ ] 禁止复制旧线实现代码到 `Ficeae`（包括 Go 服务层、SQL Kernel、旧门禁脚本）。
- [ ] 允许迁移：规则描述、验收样例、错误码语义、测试数据结构。

## 4. 8 周里程碑（Phase 0）

### Week 1-2：治理与仓库起步

- [ ] 创建 `Ficeae` 仓库与基础目录（`backend/`, `web/`, `infra/`, `docs/`）。
- [ ] 建立 `ADR` 与 `DEV-PLAN` 入口，冻结 `300/400/401/401A` 映射关系。
- [ ] 完成 `310/320/330` 对应骨架文档与 owner 分工。
- [ ] 接入最小 CI：编译、单测、lint、合同校验占位。

### Week 3-4：工程与运行基线

- [ ] 后端骨架：`ASP.NET Core` + 模块化单体壳 + 健康检查。
- [ ] 前端骨架：`React + TS + MUI` + 应用壳 + 路由壳层。
- [ ] 数据访问基线：`EF Core + Dapper` 双轨约定样例。
- [ ] 容器发布基线：`OCI image` 构建、smoke、回滚脚本。
- [ ] 最小观测：日志/指标/trace 贯通（OpenTelemetry）。

### Week 5-6：第一条垂直切片（Slice 1）

- [ ] Tenancy/AuthN 最小闭环（登录、会话、租户上下文）。
- [ ] 前端壳层接线（`/login`, `/app`, 权限守卫与失败态）。
- [ ] Org 只读列表 API + 页面 + 合同测试。
- [ ] 错误契约与 trace 关联在 UI 与 API 双侧可见。

### Week 7-8：Phase 0 验收封板

- [ ] `310/320/330` 的 stopline 验收（工程、共享建模、安全治理）。
- [ ] 完成“规则矿首批 20 条”在 `Ficeae` 的自动化映射（承接 `401A`）。
- [ ] 完成 `402` 启动前的 readiness 评审与风险清单。

## 5. 交付物清单

- [ ] `Ficeae` 仓库初始化完成并可拉起本地开发环境。
- [ ] Phase 0 门禁清单（编译/测试/合同/容器/观测）可在 CI 阻断。
- [ ] Slice 1 演示路径可稳定复现（登录 -> 壳层 -> 组织列表）。
- [ ] 规则矿台账与 `Ficeae` 验收映射台账可追溯。

## 6. 与 `401A` 的协同

- [ ] `401A` 定义规则卡片模板与首批 20 条规则基线。
- [ ] `401` 负责把这些规则映射为 `Ficeae` 自动化验收。
- [ ] Phase 0 结束前，首批 20 条规则至少落地 1 条自动化证据（unit/integration/e2e 三选一）。

## 7. 风险与缓解

- [ ] 风险：团队继续把旧线当主线开发。  
缓解：旧线仓库开启分支保护与变更审批标签。
- [ ] 风险：`Ficeae` 早期为了速度复制旧代码。  
缓解：CI 增加“禁止旧线实现复制”静态扫描与人工抽检。
- [ ] 风险：`348C` 在实现中退化成口号。  
缓解：Week 3 前冻结 `OrgContext` 最小词汇表并接门禁。
- [ ] 风险：Phase 0 范围膨胀。  
缓解：严格以 Slice 1 为封板标准，不提前承诺 Phase 1 功能。

## 8. 验收标准

- [ ] 旧线已进入只读维护态，且例外流程可审计。
- [ ] `Ficeae` 已具备独立仓库、独立 CI、独立容器发布链路。
- [ ] `Ficeae` 完成 Phase 0 横切基线并通过 smoke 验证。
- [ ] Slice 1 全链路可演示且可自动化复验。
- [ ] 规则矿机制已接入，首批 20 条规则可追溯到旧线证据并映射到 `Ficeae` 验收。

## 9. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md)
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md)
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md)
- [DEV-PLAN-348C](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348c-workday-reference-key-governance-candidate-plan.md)
- [DEV-PLAN-400](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/400-implementation-roadmap-and-vertical-slice-plan.md)
- [DEV-PLAN-401A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/401a-rule-mining-template-and-ficeae-acceptance-baseline.md)
