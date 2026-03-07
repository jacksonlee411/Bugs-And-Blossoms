# DEV-PLAN-240C：Assistant ActionInterceptor 与风险门左移计划（承接 240-M4）

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] `required_checks` 与鉴权规则仍有编译阶段硬编码残留，缺少统一执行前拦截器。
2. [ ] 需将 `auth_object/auth_action/risk_tier/required_checks` 收敛到单点 gate，避免策略漂移。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 落地 `ActionInterceptor`，统一执行鉴权、风险分级与必需检查。
2. [ ] 实现 “No Auth Pass, No DryRun” 与 “No Capability, No Plan” 的运行时硬约束。
3. [ ] 固定失败路径错误码，避免泛化失败提示直出。

### 2.2 非目标
1. [ ] 不在本计划内调整 Casbin 领域模型口径。
2. [ ] 不在本计划内引入异步耐久执行默认切换（由 `240D` 承接）。

## 3. 实施步骤
1. [ ] 定义拦截链：capability 校验 -> authz 校验 -> risk gate -> required_checks。
2. [ ] 在 plan/confirm/commit 前统一接入拦截链并记录审计字段。
3. [ ] 将历史散落校验迁移到拦截链并删除重复逻辑。
4. [ ] 补齐错误码映射与错误消息契约测试。
5. [ ] 为高风险动作保留人工确认门（与 `240D` 接轨）。

## 4. 停止线（Fail-Closed）
1. [ ] 若未经过拦截链仍可执行 commit，则本计划失败。
2. [ ] 若 capability/authz 拒绝未被稳定错误码表达，则本计划失败。
3. [ ] 若同一动作在不同入口判定不一致，则本计划失败。

## 5. 验收标准
1. [ ] plan/confirm/commit 均通过统一拦截链。
2. [ ] `auth_object/auth_action/risk_tier/required_checks` 在审计中可追踪。
3. [ ] 拒绝路径错误码稳定且与错误文案契约一致。

## 6. 门禁与命令（SSOT 引用）
1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] `make authz-pack && make authz-test && make authz-lint`
3. [ ] `make check capability-key`
4. [ ] `make check capability-route-map`
5. [ ] `make check error-message`
6. [ ] `make check doc`

## 7. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `AGENTS.md`
