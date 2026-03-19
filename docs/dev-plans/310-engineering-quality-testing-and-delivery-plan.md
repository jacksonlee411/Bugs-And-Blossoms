# DEV-PLAN-310：工程质量、测试与交付子计划

**状态**: 规划中（2026-03-17 07:23 CST）

## 1. 背景与上下文

从 `300` 蓝图进入真实实施后，若没有一个单独的工程计划，所有子计划都会默认“之后再补测试、再补 CI、再补环境”，最后会变成集体技术债。

同时，`300` 已冻结“默认部署平台 = Linux 容器平台（以 `OCI image` 作为标准发布物）”，`310` 需要把这一上层口径收敛为可执行的工程与交付基线，而不是让后续模块各自发明运行形态。

`310` 负责定义：

- 解决方案结构
- 本地开发方式
- CI/CD
- 单测/集成/E2E
- 环境与部署基线
- 观测与告警最小集

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立可持续开发的工程与测试基线。
- [ ] 明确后端、前端、数据库、E2E 的测试责任分层。
- [ ] 建立 CI/CD、以 Linux 容器平台为默认目标的环境与部署基础流程。
- [ ] 建立日志、指标、追踪的最小观测能力。
- [ ] 建立用户可见错误契约门禁基线，确保“稳定错误码 -> 明确提示 -> 可诊断追踪”可执行、可阻断。
- [ ] 冻结横切 Assistant 能力的统一门禁责任拆分：`390/395` 定义业务合同，`347` 承接结构门禁，`312` 承接测试门禁，`313` 承接 required checks 与流水线执行。

### 2.2 非目标

- [ ] 本计划不定义具体业务功能。
- [ ] 本计划不替代安全与合规治理，它们由独立子计划承接。

## 3. 范围

- Repo / solution 结构
- Local dev
- CI/CD
- Container build / deploy
- Unit / Integration / E2E
- Seed data
- Observability
- Error contract & quality gates
- Assistant cross-cutting quality gates

## 4. 关键设计决策

### 4.1 测试分层明确（选定）

- 单元测试：领域与应用逻辑
- 集成测试：数据库、仓储、外部适配与关键约束
- E2E：关键用户流程与跨模块状态衔接

### 4.2 垂直切片优先验收（选定）

- 不以模块完成率为唯一进度指标。
- 以“用户能完成一条真实链路”为主要验收单位。

### 4.3 观测从第一阶段就纳入（选定）

- 结构化日志
- 基础指标
- Trace 关联

### 4.4 部署基线采用 Linux 容器平台（选定）

- 标准发布物为 `OCI image`。
- 默认部署目标为 Linux VM + container runtime 或托管 Linux 容器服务。
- 不采用 Windows-first 作为默认交付口径。
- 不把 Kubernetes 作为第一阶段前置条件。
- 保留宿主机本地开发入口，但交付与 smoke 基线必须围绕 Linux 容器发布物建立。

### 4.5 横切 Assistant 能力必须进入统一门禁（选定）

- `390` 负责定义“全平台可问答/可读取/可操作/可回落”的业务合同，但不单独拥有全部门禁实现。
- `395` 负责冻结 Assistant 覆盖目录、支持级别、无暗面能力与变更触发矩阵。
- `347` 负责静态结构门禁；`312` 负责 contract/integration/E2E 测试门禁；`313` 负责流水线绑定与 required checks。
- 新增用户可见 capability、高价值查询面或 handoff route 时，若未同步声明 Assistant 支持级别、状态跟踪语义或 UI 回落面，不得视为“已交付”。

## 5. 交付范围

- [ ] 工程结构与脚手架
- [ ] 测试策略
- [ ] CI/CD 基线
- [ ] Linux 容器平台环境与部署方案
- [ ] 观测与告警最小集
- [ ] 错误契约门禁与质量校验入口（与前端交互层协同）
- [ ] 横切 Assistant 门禁责任矩阵（`390/395/347/312/313`）

## 6. 验收标准

- [ ] 每个实施子计划都能落到统一的工程与测试体系中。
- [ ] 关键流程具备从本地到 CI 的可复现验证路径。
- [ ] 系统能够产出可复现的 `OCI image`，并在 Linux 容器平台完成最小 smoke 验证。
- [ ] 系统具备最基本的日志、指标与追踪能力。
- [ ] effective-dated 区间冲突、跨租户访问阻断、Assistant 与审批长链路状态回执均有明确验证入口。
- [ ] 用户可见错误满足“稳定错误码 + 明确提示 + trace 关联”要求，且可通过统一门禁阻断回归。
- [ ] `390` 的横切 Assistant 合同已被拆解为可执行门禁：结构门禁、测试门禁与流水线 required checks 均有明确 owner，后续模块不得以“后补 Assistant”作为交付完成的默认前提。

## 7. 后续拆分建议

1. [ ] [DEV-PLAN-311：工程结构与本地开发基线详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/311-engineering-structure-and-local-development-baseline-detailed-design.md)
2. [ ] [DEV-PLAN-312：测试金字塔与 E2E 策略详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md)
3. [ ] [DEV-PLAN-313：CI/CD、Linux 容器平台部署与观测基线详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md)
