# DEV-PLAN-240E：Assistant MCP 写能力准入与治理计划（承接 240-M6）

**状态**: 规划中（2026-03-08 CST；属于 `271-S5` 的结构性尾项，当前尚未形成“写能力默认 fail-closed”的运行时与门禁闭环）

## 1. 背景
1. [ ] `240` 已冻结“MCP 默认只读，写能力必须显式注册并受审计门控制”。
2. [ ] 当前需将该策略从文档口径落到可执行门禁与运行时准入。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 固化 MCP/外部工具默认只读策略。
2. [ ] 落地写能力准入：`capability_key` 显式映射 + adapter allowlist + 审计。
3. [ ] 为写能力引入审批/确认钩子，防止模型侧直达写门。

### 2.2 非目标
1. [ ] 不在本计划内扩容 MCP 工具生态。
2. [ ] 不在本计划内接管 LibreChat runtime 配置主源（沿用既有主源策略）。

## 3. 实施步骤
1. [ ] 盘点现有 MCP/工具动作，标注只读/可写分类。
2. [ ] 对可写动作建立显式 capability 映射清单与注册校验。
3. [ ] 在执行链加入审批/确认钩子与审计字段。
4. [ ] 将未注册可写动作统一 fail-closed。
5. [ ] 补齐测试：未映射阻断、审批拒绝、审计字段完整性。

## 3.1 当前推进口径（2026-03-08）
1. [ ] 本计划不属于 `288` 的最近战术阻塞，但属于 `271-S5` 必须清零的治理尾项。
2. [ ] 当前目标是尽快把“默认只读、显式注册、可审计”从文档口径转成运行时与门禁口径，避免在 `240F/285` 阶段暴露写能力旁路问题。
3. [ ] 在 `240E` 未完成前，不得把 MCP/外部工具接入视为已满足 `240` 的封板前置。

## 4. 停止线（Fail-Closed）
1. [ ] 发现未注册可写工具仍可触发业务写入，则本计划失败。
2. [ ] 发现可写动作缺失 capability 映射或审计字段，则本计划失败。
3. [ ] 发现只读工具可旁路升级为写能力，则本计划失败。

## 5. 验收标准
1. [ ] MCP 写能力全部显式注册、映射、审计。
2. [ ] 拒绝路径稳定返回错误码并可追踪。
3. [ ] 外部工具接入不突破 One Door 与租户边界。

## 6. 门禁与命令（SSOT 引用）
1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] `make check capability-key`
3. [ ] `make check capability-route-map`
4. [ ] `make check assistant-config-single-source`
5. [ ] `make check no-legacy`
6. [ ] `make check doc`

## 7. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `AGENTS.md`
