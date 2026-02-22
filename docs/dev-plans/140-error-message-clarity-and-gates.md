# DEV-PLAN-140：全仓错误提示明确化与质量门禁

**状态**: 规划中（2026-02-22 16:42 UTC）

## 1. 背景

近期“新建组织失败仅显示泛化文案”的问题再次暴露：

1. 后端已返回稳定错误码（如 `ORG_ROOT_ALREADY_EXISTS`），但部分路径仍落成默认 message（如 `orgunit_write_failed`）。
2. 前端页面存在分散映射与缺口，导致用户看到“失败”但无法直接判断可操作步骤。
3. 当前门禁尚未覆盖“错误码 → 明确提示”的完备性，回归容易再次发生。

该问题已从 Org 模块扩展为全仓工程质量问题：**只要是用户可见错误，就必须明确、可理解、可操作，并且可被门禁阻断漂移**。  
本计划按“一次性全仓收敛”执行，不采用“先部分覆盖再长期并存”的策略。

## 2. 目标与非目标

### 2.1 目标

- [ ] 全仓用户可见错误（Web/API）统一收敛到“稳定错误码 + 明确提示”的契约。
- [ ] 前端所有写路径与关键读路径默认展示可操作提示，不再出现泛化“xx_failed”作为最终用户文案。
- [ ] 构建“错误提示完备性门禁”：映射缺失、i18n 缺失、回退到泛化文案时，CI fail。
- [ ] 保持 fail-closed：未知错误不编造业务语义，但不得静默退化为误导性提示。
- [ ] 一次性完成全仓收敛并冻结契约：合并后不允许“模块内例外名单”长期存在。

### 2.2 非目标

- 不在本计划中重写全部错误模型或引入第二套错误链路。
- 不改变既有业务规则与权限规则，仅收敛“错误呈现契约 + 门禁”。
- 不引入 legacy 兼容分支。

## 3. 范围与 SSOT

- 后端：`internal/server/**`（ErrorEnvelope 产出、稳定码映射、默认 message 策略）。
- 前端：`apps/web/src/**`（统一错误翻译入口、页面接入、字段级提示）。
- 门禁：`Makefile`、`.github/workflows/quality-gates.yml`、`scripts/ci/**`。
- 文档与规范：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/111-frontend-error-message-accuracy-and-field-level-hints.md`。

## 3.1 标准对齐（DEV-PLAN-005 / STD-XXX）

本计划命中并强制对齐以下标准条目：

1. [ ] `STD-001`：错误返回与前端展示必须保留 `trace_id` 语义，不得将 tracing 与业务幂等字段混用。
2. [ ] `STD-004`：禁止把实现细节/泛化失败文案暴露为对外契约语义（例如将 `*_failed` 作为最终用户提示）。
3. [ ] `STD-005`：Tenant App 单链路（`/app/**`）下的错误展示口径唯一，禁止页面级双轨错误处理分支。
4. [ ] `STD-006`：未登录/失效会话错误语义按 `route_class` 冻结（UI 302、API 401 JSON），不得在本计划中被破坏。

## 4. 方案设计

### 4.1 错误契约 SSOT（Error Catalog）

- [ ] 新增错误目录清单（建议：`config/errors/catalog.yaml`），作为“可见错误码”唯一来源。
- [ ] 目录字段至少包含：`code`、`http_status`、`user_message_key`、`module`、`severity`、`field_hint(optional)`。
- [ ] 新增或调整稳定错误码时，必须同步更新 catalog 与中英文文案。
- [ ] catalog 覆盖范围为**全仓用户可见错误码全集**（非“高频子集”）。

### 4.2 后端输出收敛

- [ ] 统一策略：已识别稳定码必须输出明确 message（禁止最终回落 `*_failed` 作为用户文案）。
- [ ] 未识别错误可保持 fail-closed，但必须输出可诊断 message（保留 trace_id）。
- [ ] 将“稳定码映射完整性”纳入后端单测（按 catalog 逐项校验）。

### 4.3 前端展示收敛

- [ ] 统一错误呈现入口（单点模块），页面不得各自维护散落 `switch(code)`。
- [ ] 按 catalog 映射至 i18n key；支持字段级提示（`field_hint`/映射规则）。
- [ ] 默认展示规则：
  1) 命中稳定码 → 明确提示；
  2) 未命中稳定码 → 展示后端 message（非泛化固定文案）；
  3) 仅技术异常才展示通用兜底提示。

### 4.4 门禁设计（新增）

新增门禁目标（命名冻结）：

- [ ] `make check error-message`：校验错误码映射与文案完备性。

门禁最小规则：

1. [ ] catalog 中每个 `user_message_key` 必须在 `en/zh` 同时存在。
2. [ ] catalog 中每个 `code` 必须有后端 message 映射（或明确声明为“后端 message 直出”类型）。
3. [ ] 前端统一错误翻译入口必须覆盖全部 catalog code（缺失即 fail）。
4. [ ] 禁止将 `*_failed`、`invalid_request` 等泛化文案直接作为最终用户提示（允许仅作内部默认码）。
5. [ ] 任一“用户可见稳定错误码”若未登记到 catalog，直接 fail（禁止隐式漏网）。

CI 接入要求：

- [ ] 将 `make check error-message` 纳入 `make preflight`。
- [ ] 将 `make check error-message` 纳入 `Quality Gates` required check（对齐 `DEV-PLAN-012`）。

### 4.5 门禁执行语义冻结（对齐 DEV-PLAN-012）

1. [ ] `Quality Gates` 的 required check 名称冻结，不因本计划实施而改名。
2. [ ] `error-message` 门禁在 required check 中必须产出稳定结论；禁止通过 job-level `if` 变成 `skipped`。
3. [ ] 未命中路径触发器时可 no-op，但必须输出明确 no-op 证据；命中触发器时必须真实执行并可阻断。
4. [ ] `make preflight` 与 CI workflow 仅通过 `Makefile` 单一入口调用，禁止在 CI YAML 内复制第二套检查逻辑。

### 4.6 故障处置与回滚（No-Legacy）

当全仓收敛后出现线上/联调问题，处置路径冻结为：

1. [ ] 环境级保护（只读/停写）；
2. [ ] 定位并修复错误映射或文案；
3. [ ] 重试/重放验证；
4. [ ] 恢复写入；
5. [ ] 形成执行证据。

禁止事项（Stopline）：

- [ ] 禁止通过恢复旧错误链路、双映射、legacy 分支进行“快速回滚”。
- [ ] 禁止引入“临时例外名单”长期绕过 `check error-message`。

## 5. 实施步骤

1. [ ] 盘点全仓用户可见错误来源（server 输出码、web 展示路径）。
2. [ ] 建立 `error catalog`（覆盖全仓用户可见错误码全集，不留模块例外）。
3. [ ] 后端补齐稳定码 message 映射，移除可见路径中的泛化默认文案。
4. [ ] 前端接入统一错误呈现模块，替换页面散落映射。
5. [ ] 增补 i18n 文案（`en/zh` 同步）并校验 key 完备性。
6. [ ] 实现 `make check error-message`（本地）+ `Quality Gates`（CI）门禁并冻结执行语义。
7. [ ] 补齐测试：Go 单测 + Web 单测 + E2E 失败提示断言（覆盖全仓模块入口）。
8. [ ] 执行一次 No-Legacy 故障处置演练并固化证据（只读/停写→修复→重试/重放→恢复）。
9. [ ] 更新 `AGENTS.md` 触发器矩阵与文档地图，确保门禁可发现。

## 6. 验收标准（DoD）

- [ ] 全仓用户可见路径（Org/Person/Staffing/Dict/SetID/JobCatalog/IAM）不再出现“仅泛化失败”提示。
- [ ] 同一错误码在后端与前端展示语义一致（中英文一致）。
- [ ] `make check error-message` 本地与 CI 可复现，并能阻断映射缺失。
- [ ] `make preflight` 已包含该门禁，且 PR required checks 强制执行。
- [ ] `error catalog` 与后端/前端/i18n 三端一致性由自动化校验保证，无人工白名单豁免。

## 7. 质量门禁与执行记录（待实施）

- [ ] `make check error-message`
- [ ] `go test ./internal/server/...`
- [ ] `pnpm -C apps/web test`
- [ ] `make check tr`
- [ ] `make check no-legacy`
- [ ] `make preflight`

## 8. 风险与回滚

- 风险：一次性全仓收敛范围大，短期可能出现漏映射。
  - 对策：一次性收敛仍按“清单驱动 + 自动校验”执行，不以“分批上线”替代全仓完成定义。
- 风险：过度“翻译”导致语义偏差。
  - 对策：未知码坚持 fail-closed，优先展示后端 message，不做猜测。
- 回滚：允许临时保留后端 message 直出，不允许恢复 legacy 双链路。

## 9. 关联计划

- `docs/dev-plans/111-frontend-error-message-accuracy-and-field-level-hints.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
