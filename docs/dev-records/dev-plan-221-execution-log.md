# DEV-PLAN-221 执行日志

## 2026-03-02（UTC）

- 2026-03-02 07:10 UTC：完成 `internal/server/assistant_api.go` 收口实现：
  - 状态机新增终态 `canceled` / `expired`，对 `confirm`/`commit` 统一返回 `conversation_state_invalid`。
  - `commit` 增加 `policy/composition/mapping` 版本漂移检测；命中漂移时原子回退 `validated` 并返回 `conversation_confirmation_required`。
  - `confirm` 增加候选固化：`confirmed` 后允许同候选幂等确认，拒绝改写为不同候选。
  - 新增 strict decode / boundary 拒绝链路：`ai_plan_schema_constrained_decode_failed`、`ai_plan_boundary_violation`。
- 2026-03-02 07:10 UTC：完成测试补齐（`internal/server/assistant_api_coverage_test.go`）：覆盖 TC-220-BE-003/004/006/008/015 对应分支。
- 2026-03-02 07:10 UTC：完成错误目录与前端映射同步：
  - `config/errors/catalog.yaml` 新增 3 个错误码。
  - `apps/web/src/errors/presentApiError.ts` 新增对应中英文提示。
  - `apps/web/src/errors/presentApiError.test.ts` 增加映射断言。

## 220A 缺口关闭映射（221 负责项）

| 220A Blocker | 关闭结果 | 证据 |
| --- | --- | --- |
| 状态机终态缺失（`canceled/expired`） | 已关闭 | `internal/server/assistant_api.go`、`internal/server/assistant_api_coverage_test.go` |
| 版本漂移回退缺失 | 已关闭 | `internal/server/assistant_api.go`、`internal/server/assistant_api_coverage_test.go` |
| 候选确认固化不足（可静默改写） | 已关闭 | `internal/server/assistant_api.go`、`internal/server/assistant_api_coverage_test.go` |
| strict decode / boundary 错误码缺失 | 已关闭 | `internal/server/assistant_api.go`、`config/errors/catalog.yaml`、`apps/web/src/errors/presentApiError.ts` |

## TC-220-BE 责任闭环（221）

| 测试项 | 状态 | 说明 |
| --- | --- | --- |
| TC-220-BE-003 schema 违约拒绝 | 已自动化 | 缺必填场景返回 `ai_plan_schema_constrained_decode_failed` |
| TC-220-BE-004 边界违约拒绝 | 已自动化 | SQL/越界输入返回 `ai_plan_boundary_violation` |
| TC-220-BE-006 终态提交拒绝 | 已自动化 | `canceled/expired -> commit` 返回 `conversation_state_invalid` |
| TC-220-BE-008 版本漂移回退 | 已自动化 | 漂移触发 `confirmed -> validated` 回退并要求重确认 |
| TC-220-BE-015 候选确认后会话固化 | 已自动化 | 同 turn 二次确认不可改写候选 |

## 本地验证记录

- [X] `go test ./internal/server -run Assistant -count=1`
- [X] `make check error-message`
- [X] `make check routing`
- [X] `make check capability-route-map`
- [X] `make authz-pack && make authz-test && make authz-lint`
- [X] `make check fmt`
- [X] `make check no-legacy`
- [X] `go vet ./...`
- [X] `make check lint`
- [X] `make test`
