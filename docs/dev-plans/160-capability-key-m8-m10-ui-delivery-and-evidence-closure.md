# DEV-PLAN-160：Capability Key Phase 10 UI 可视化交付与证据收口（承接 150 M8/M10）

**状态**: 规划中（2026-02-23 04:50 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 工作流 I 与 M8/M10。
- **当前痛点**：后端能力即使落地，若缺少可见入口与可操作链路，会形成“僵尸功能”。
- **业务价值**：把 capability 治理能力转化为用户可见交付，保证上线后可发现、可操作、可验收。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 交付 Capability Governance 导航入口与四子页：`Registry/Explain/Functional Area/Activation`。
- [ ] 完成核心交互闭环：查询、校验、激活、回滚、解释查看。
- [ ] 完成部分授权降级模式（骨架可见、操作禁用、申请入口）。
- [ ] 关键拒绝场景展示“可读原因 + 下一步动作”。
- [ ] 完成 E2E、截图/录屏、日志证据并归档 `docs/dev-records/`。

### 2.2 非目标
- 不重写权限内核与规则执行引擎。
- 不新增流程编排产品能力。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（若涉及后端 API 调整）
  - [x] 前端构建与 E2E（`make e2e`）
  - [x] 路由治理（`make check routing`）
  - [x] 文档（`make check doc`）
  - [x] 总收口（`make preflight`）
- **SSOT**：`AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 UI IA 架构
- 一级入口：Capability Governance。
- 二级子页：Registry、Explain、Functional Area、Activation。
- 跨页一致：状态反馈、错误提示、trace/request 展示。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：四子页统一入口（选定）**
  - A：分散到多个业务模块页。缺点：可发现性差。
  - B（选定）：统一治理入口，降低学习成本。
- **决策 2：部分授权可见骨架（选定）**
  - A：无权限直接隐藏页面。缺点：不可发现。
  - B（选定）：可见骨架 + 禁用交互 + 申请入口。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 前端状态模型
- `current_context`：tenant/setid/as_of
- `registry_filters`：functional_area/capability_type/status
- `activation_diff`：draft vs active
- `explain_mode`：brief/full

### 4.2 约束
- 页面必须包含加载、空态、成功、失败、无权限五类状态。
- 功能域关闭场景必须展示专用提示，不使用泛化报错。

## 5. 接口契约 (API Contracts)
### 5.1 Registry
- 查询：支持 `functional_area/capability_type/status/as_of` 联合筛选。

### 5.2 Explain
- 默认 `brief`；管理员可受控切换 `full`。
- 必须展示 `trace_id/request_id/policy_version`。

### 5.3 Functional Area / Activation
- Functional Area：展示开关矩阵与生命周期标签。
- Activation：展示 `draft/active` 差异、激活确认、回滚历史。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 用户链路算法
1. [ ] 用户进入 Capability Governance。
2. [ ] 在 Registry 筛选并校验 capability 配置。
3. [ ] 在 Explain 查看命中原因与追踪标识。
4. [ ] 在 Activation 执行激活/回滚。
5. [ ] 系统反馈结果并写入审计证据。

## 7. 安全与鉴权 (Security & Authz)
- 无权限用户仅可见骨架与申请入口，不可执行关键动作。
- full explain 仅管理员可见。
- 页面不回显敏感内部实现细节。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-154`、`DEV-PLAN-157`、`DEV-PLAN-158`、`DEV-PLAN-159`。
- **里程碑**：
  1. [ ] M8.1 导航与页面骨架完成。
  2. [ ] M8.2 四子页最小操作闭环完成。
  3. [ ] M10.1 E2E 与证据归档完成。

## 9. 测试与验收标准 (Acceptance Criteria)
- [ ] 四子页均具最小可用交互。
- [ ] 至少 1 条成功链路 + 1 条拒绝链路可解释。
- [ ] 部分授权与功能域关闭场景反馈正确。
- [ ] `make preflight` 通过，证据归档完成。

## 10. 运维与监控 (Ops & Monitoring)
- 监控项：页面访问率、激活成功率、拒绝链路可解释率。
- 故障处置：UI 不可发现/不可操作视为未交付，阻断发布。
- 证据：E2E 报告、截图/录屏、操作审计归档。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/102c5-ui-design-for-setid-context-security-registry-explainability.md`
- `docs/dev-plans/102d-t-context-rule-eval-user-visible-test-plan.md`
