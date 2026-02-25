# DEV-PLAN-004M1：禁止 legacy（单链路原则）——清理、门禁与迁移策略

**状态**: 已实施（2026-01-06 00:00 UTC）

> 本文是仓库级“**不引入 legacy**”原则的合同化落地：明确什么算 legacy、为什么禁止、当前仓库遗留点如何清理、以及如何用门禁阻断再次引入。  
> 本文不承担“存量系统迁移/兼容”的方案（Greenfield 仓库默认不做历史兼容包袱）。

## 1. 背景与问题陈述

本仓库为 Greenfield implementation repo，但在实现过程中出现了“legacy”形态的代码与契约（例如 UI 通过 query param 选择 `read=legacy`、在失败时自动回退到另一套读路径/数据模型）。

这类做法会把复杂度从“显性设计决策”转移为“隐性分支与双权威表达”，短期看似降低风险，长期会造成：
- 分支矩阵膨胀（测试、文案、排障口径、接口契约都需要双份维护）。
- 数据/契约漂移风险（两套读写模型无法保证永远一致）。
- “僵尸后门”风险（过渡期入口被长期保留、被误用或被依赖）。

## 2. 原则（No Legacy / Single Mainline）

### 2.1 定义：什么算 legacy（本仓库口径）

以下任一命中即视为“引入 legacy”：
- **对外/对内暴露第二条链路**：例如 `read=legacy`、`use_legacy`、`legacy_mode`、`*_legacy*` 的入口/参数/路由。
- **回退到另一套事实源/读模型**：主链路失败时自动切换到“旧表/旧函数/旧查询/旧投射”的兜底行为。
- **兼容别名窗口**：为历史 URL/API/命名提供“长期 alias”，却没有严格时间盒与删除条件（Greenfield 默认直接拒绝）。
- **双写/双读的长期共存**：两套实现都被认为“可用”，但没有明确的退场策略与门禁约束。

### 2.2 允许的回滚/降级方式（替代 legacy）

只允许以下方式实现“可回滚/可恢复”，不得以 legacy 分支替代：
- 环境级保护（例如临时下线入口、提高保护级别、限制写入）。
- 明确的“只读/停写”模式（运维策略或配置层面），并提供可审计的恢复步骤。
- 修复配置/数据/迁移后重试（而不是走另一套旧实现）。

## 3. 现状与清理范围（本仓库）

### 3.1 已发现的 legacy 形态（示例）

- OrgUnit 读取链路曾提供 `read=legacy` 并在 `current` 失败/为空时回退到 legacy 读取路径。
- DB schema/迁移中存在用于 legacy 读写的 baseline 表/函数（与 current 的事件 SoT + 同步投射读模型并存）。

### 3.2 清理目标（Done 口径）

- [X] 仓库内不再存在 `read=legacy` 或等价入口；UI/HTTP 只保留单一主链路（current）。
- [X] 不再存在“回退到另一套数据模型/读路径”的行为；失败应显式报错并引导修复/重试。
- [X] 移除为 legacy 链路服务的表/函数/查询与测试分支。
- [X] 质量门禁加入 `no-legacy`，并在 `make preflight` 与 CI required checks 中阻断再次引入。

## 4. 门禁（No-Legacy Gate）

### 4.1 唯一入口

- `make check no-legacy`
- `make preflight` 必须包含该门禁

### 4.2 阻断口径（建议实现）

门禁至少应覆盖：
- Go/SQL/迁移/脚本中出现 `read=legacy` 或“legacy 分支”标识符的新增。
- 对外暴露的 legacy query param/route/alias。

门禁必须避免误报文档讨论（docs 允许出现“legacy”一词用于阐述禁止原则），但不得放过运行时代码与 DB 事实源。

## 5. 与其他合同的关系（SSOT 引用）

- 触发器矩阵与门禁入口：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`
- Simple > Easy 评审 stopline：`docs/dev-plans/003-simple-not-easy-review-guide.md`
- 路由治理：`docs/dev-plans/017-routing-strategy.md`
- Tenancy/AuthN 的“主链路唯一”：`docs/dev-plans/019-tenant-and-authn.md`
- 控制面的“可回滚但不引入 legacy”：`docs/dev-plans/023-superadmin-authn.md`

## 6. 实施结果（本次落地的最小证据）

- 门禁入口：`make check no-legacy`（实现：`scripts/ci/check-no-legacy.sh`），已接入 `make preflight` 与 CI required checks。
- OrgUnit（`/org/units` + `/org/api/org-units*`）：移除 legacy 读路径与回退行为；仅保留 current 主链路（`as_of` 仍作为快照时间点参数）。
- DB：移除为 legacy 链路服务的 baseline schema/迁移与相关 sqlc 模型；保留 current 的事件 SoT + 同步投射读模型。
