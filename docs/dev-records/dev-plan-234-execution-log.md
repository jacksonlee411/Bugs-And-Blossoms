# DEV-PLAN-234 执行记录（MCP/Actions/Domain Allowlist 复用落地）

## 1. 记录信息
- 计划：`docs/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md`
- 记录时间：2026-03-03 16:10 UTC
- 记录人：Codex

## 2. 本次交付范围
1. 新增域名策略配置 SSOT：`config/assistant/domain-allowlist.yaml`（`default: deny` + 风险域名阻断）。
2. 新增运行时域名策略解析与能力状态回传：
   - `internal/server/assistant_domain_policy.go`
   - `internal/server/assistant_runtime_status.go`（新增 `capabilities` 字段）
3. 新增域名白名单门禁脚本并接入本地/CI：
   - `scripts/ci/check-assistant-domain-allowlist.sh`
   - `Makefile`（`make check assistant-domain-allowlist` + `preflight`）
   - `.github/workflows/quality-gates.yml`（always gate）
4. 补齐测试：
   - `internal/server/assistant_domain_policy_test.go`
   - `internal/server/assistant_runtime_status_test.go`

## 3. 验收命令与结果
- `go test ./internal/server -run 'TestAssistantDomainPolicyRepoConfigIsValid|TestValidateAssistantDomainPolicy|TestAssistantRuntimeStatus_MergesLockAndSnapshot|TestAssistantRuntimeStatus_DomainPolicyMissingFailsClosed' -count=1` ✅
- `make check assistant-domain-allowlist` ✅
- `make check assistant-config-single-source` ✅
- `make check doc` ✅

## 4. 关键结论
- Domain Allowlist 配置缺失/非法时，运行时状态接口返回 fail-closed（`assistant_oss_domain_policy_missing|assistant_oss_domain_policy_invalid`）。
- MCP/Actions 能力状态与 policy version 已进入 `/internal/assistant/runtime-status` 输出（`capabilities` 字段）。
- `assistant-domain-allowlist` 门禁已进入 `preflight` 与 CI always checks，策略漂移可阻断。
