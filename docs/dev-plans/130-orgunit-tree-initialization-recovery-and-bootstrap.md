# DEV-PLAN-130：Org 组织树初始化问题收敛与自举修复方案

**状态**: 已完成（2026-02-22 13:37 UTC）

## 1. 背景与现象

在本地租户（`localhost -> 00000000-0000-0000-0000-000000000001`）执行“新建组织”时，前端提示：

- `追加动作不可用：ORG_TREE_NOT_INITIALIZED`

同时间窗口内确认：

1. [X] `write-capabilities(create_org)` 返回 `deny_reasons=["ORG_TREE_NOT_INITIALIZED"]`。  
2. [X] `orgunit.org_trees` 中该租户无 `root_org_id`（组织树未初始化）。  
3. [X] `org_code` 策略为 `maintainable=false + default_rule_expr=next_org_code("",8)`，与服务解析约束存在冲突。  

该问题会阻断“首次建树”的用户路径。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. [X] 空树租户具备可达、可重复的首个 root 自举路径。  
2. [X] 保持 One Door（写入仍走 `submit_*_event`），不引入第二写入口。  
3. [X] UI 在空树场景提供可操作引导，而非仅暴露错误码。  
4. [X] 补齐 API/服务/UI 回归测试与门禁验证。  
5. [X] 计划与执行证据纳入 SSOT 文档地图。  

### 2.2 非目标（Stopline）

1. 不引入 legacy 双链路/回退通道。  
2. 不通过手工改库作为常态修复方式。  
3. 不扩展 Org 其他业务功能范围。  

## 3. 根因结论

1. `create_org` 写能力判定在 `tree=false` 且非 `ROOT` 代码时直接拒绝，导致空树租户无法通过 UI 完成首个 root 创建。  
2. 空树是 fail-closed 的合理状态，但缺少“同链路自举引导”。  
3. `next_org_code("",8)` 为现存策略数据，但服务曾拒绝空前缀表达式，放大首次创建失败概率。  

## 4. 方案与实施结果

### 4.1 方案收敛（已实施）

1. [X] 后端能力判定放宽：`create_org` 不再因 `tree_not_initialized` 直接 deny。  
2. [X] 能力接口返回 `tree_initialized`，前端据此识别“空树自举态”。  
3. [X] 前端空树态强约束：锁定父组织为空、强制 `is_business_unit=true`、展示 bootstrap 提示。  
4. [X] 修复默认规则解析：允许 `next_org_code("", N)` 空前缀。  

### 4.2 One Door 与语义说明

- 本次不新增独立 bootstrap API；首个 root 仍通过既有 `create_org` 写链路落库，满足 One Door 和审计一致性要求。  

## 5. 实施步骤完成情况

1. [X] **Phase 0（证据冻结）**：完成问题复现与数据确认。  
2. [X] **Phase 1（后端）**：完成能力判定与能力响应扩展。  
3. [X] **Phase 2（前端）**：完成空树引导与表单约束。  
4. [X] **Phase 3（策略）**：完成 `next_org_code` 空前缀兼容。  
5. [X] **Phase 4（验证）**：完成 `go fmt/go vet/make check lint/make test/make check routing/make css`。  
6. [X] **Phase 5（收口）**：补齐执行日志与 Doc Map 链接。  

## 6. 验收标准（DoD）

1. [X] 空树租户可进入并完成首个 root 创建路径（不再被 `ORG_TREE_NOT_INITIALIZED` 能力预检阻断）。  
2. [X] 初始化后可回到常规“新建组织”流程。  
3. [X] 相关服务/API/UI 改动已通过仓库门禁测试。  
4. [X] 文档与执行记录可追溯。  

## 7. 关联 SSOT

- `AGENTS.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/111-frontend-error-message-accuracy-and-field-level-hints.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-records/dev-plan-130-execution-log.md`
