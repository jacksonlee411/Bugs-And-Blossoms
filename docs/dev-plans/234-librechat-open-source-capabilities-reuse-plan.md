# DEV-PLAN-234：LibreChat 开源能力复用落地实施计划（MCP/Actions/Allowlist）

**状态**: 草拟中（2026-03-03 13:35 UTC）

## 1. 背景
- 承接 `DEV-PLAN-230` 的 PR-230-03（开源能力复用部分）。
- 目标是把“复用判定”落成代码与验收，而不是停留在矩阵描述。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 复用 MCP Servers 与远程域名限制能力。
2. [ ] 复用 Actions 与 `actions.allowedDomains`。
3. [ ] 复用 Domain Allowlist，建立本仓 fail-closed 校验与审计留痕。
4. [ ] 明确 Agents 暂不复用边界与复评触发条件。

### 2.2 非目标
1. [ ] 不开放 Agents 自动执行业务写动作。
2. [ ] 不自建第二套 MCP/Actions 配置中心。

## 3. 输入与输出契约
### 3.1 输入
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `https://raw.githubusercontent.com/danny-avila/LibreChat/main/librechat.example.yaml`

### 3.2 输出
1. [ ] LibreChat 能力配置落地（MCP/Actions/Allowlist）。
2. [ ] 本仓校验规则：白名单外域名阻断、审计记录可追溯。
3. [ ] 测试用例：正向连通 + 负向阻断（SSRF/越权）。

## 4. 实施步骤（直接落地）
1. [ ] MCP 复用落地
   - [ ] 注册与调用链路接通。
   - [ ] `mcpSettings.allowedDomains` 与本仓出口策略一致化。
2. [ ] Actions 复用落地
   - [ ] Actions 能力接通并限定域名。
   - [ ] 保留 One Door 边界：不可直写业务路由。
3. [ ] Domain Allowlist 收口
   - [ ] 非白名单域名请求 fail-closed。
   - [ ] 审计日志含 tenant/request_id/blocked_domain。
4. [ ] Agents 边界
   - [ ] 在配置与文档中冻结“本阶段禁自动写动作”。

## 5. 验收与门禁
1. [ ] MCP 正向场景通过，白名单外场景被阻断。
2. [ ] Actions 正向场景通过，非白名单域名被阻断。
3. [ ] `make e2e` 增加至少 1 条 SSRF 负测。
4. [ ] `make check routing` 与 `make check capability-route-map` 通过。

## 6. 风险与缓解
1. [ ] 风险：开源能力默认配置过宽。  
   缓解：默认最小白名单，增量放开需审批。
2. [ ] 风险：能力复用后遗漏审计字段。  
   缓解：无 tenant/request_id 不允许入库。

## 7. SSOT 引用
- `AGENTS.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/017-routing-strategy.md`
- `Makefile`
