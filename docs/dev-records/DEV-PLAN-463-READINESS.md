# DEV-PLAN-463 Readiness

## 状态

- `463`：调查完成，待修复
- 调查日期：`2026-04-24`
- 关联 owner：`463`、`461`

## 结论

`DEV-PLAN-463` 记录的问题在当前页面真链路中仍然存在，并非历史描述或已失效结论。

当前 `CubeBox` 在真实页面中对提示词 `查询今天的组织树` 的回答仍会落到：

- “当前租户一级组织”
- 根组织 `100000 | 飞虫与鲜花`
- `有下级：未知`

因此，`463` 中关于“根列表分页遗漏 `has_children`，导致回答层将其渲染为 `未知`”的 P0 结论，当前仍成立。

同时，`463` 中关于“组织树”当前默认只交付“一级组织入口”，而不是整棵树递归展开的能力边界，当前页面回答也仍然成立；这不是本轮新增发现，而是已有边界尚未进一步产品化外显。

## 页面复验证据

### 1. 真实页面入口已打通

- 入口：`http://localhost:8080/app/login`
- 关键前提：
  - 当前租户解析走 `Host=localhost`
  - `iam` 迁移已为租户 `00000000-0000-0000-0000-000000000001` 写入 `tenant_domains.hostname=localhost`
  - `kratosstub` 已写入测试身份：
    - `tenant_uuid=00000000-0000-0000-0000-000000000001`
    - `email=admin@localhost`
    - `password=admin123`

### 2. 真实页面操作路径

- 登录：`admin@localhost / admin123`
- 进入主壳层后打开右侧 `CubeBox` 抽屉
- 点击“新建对话”
- 发送用户消息：`查询今天的组织树`

### 3. 页面实际回答

页面中 `CubeBox` 可见文本为：

```text
已完成只读查询。
本次关注：当前租户一级组织、状态、是否还有下级。
step-1（orgunit.list）
as_of：2026-04-24
include_disabled：false
组织列表：1 条 / total null
- 100000 | 飞虫与鲜花 | 状态：active | 业务单元：是 | 有下级：未知
```

结论：

- 当前用户可见面仍然出现 `有下级：未知`
- 页面回答与 `463` 文档中的缺陷描述一致

### 4. 同次页面网络证据

本轮真实页面交互中已确认以下请求成功发生：

- `POST /iam/api/sessions` -> `204`
- `POST /internal/cubebox/conversations` -> `201`
- `POST /internal/cubebox/turns:stream` -> `200`

其中查询请求体为：

```json
{"conversation_id":"conv_881846163bc249e2b074d43d2b166056","prompt":"查询今天的组织树","next_sequence":2}
```

结论：

- 本轮不是前端假数据、静态 mock 或旧历史消息回放
- 实际命中了当前服务端 `CubeBox` 查询主链

## 代码级复核

### 1. 根列表分页仍未查询 `has_children`

当前实现位于：

- [internal/server/orgunit_field_metadata_store.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_store.go)

复核结果：

- 根列表分支的 `SELECT` 只返回：
  - `org_code`
  - `name`
  - `status`
  - `is_business_unit`
- 仅在 `parentOrgNodeKey != ""` 的子列表分支才额外查询 `has_children`
- 根列表扫描也没有给 `item.HasChildren` 赋值

因此 `463` 的一级根因当前仍成立。

### 2. 回答层仍会把空值渲染为“未知”

当前回答层位于：

- [internal/server/cubebox_query_flow.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go)

复核结果：

- `renderQueryOptionalBoolCN(...)` 对 `nil` 固定输出 `未知`
- `summarizeOrgUnitListQueryResult(...)` 仍使用：
  - `业务单元：` + `renderQueryOptionalBoolCN(item.IsBusinessUnit)`
  - `有下级：` + `renderQueryOptionalBoolCN(item.HasChildren)`

因此根列表 `HasChildren=nil` 时，页面回答必然继续显示 `有下级：未知`。

## 当前未完成项

以下事项当前仍未完成：

1. 根列表分页查询补齐 `has_children`
2. `CubeBox` 回归测试覆盖根列表 `有下级：是/否`
3. `DEV-PLAN-463` 文档状态从“规划中”提升到与当前事实一致的状态
4. `docs/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md` 中验收清单第 5 项打勾

## 本轮仅完成内容

- 补齐真实页面复验证据
- 确认 `463` 当前问题仍存在
- 确认当前阻塞属于实现缺陷而非环境误报
- 明确当前结论可直接支撑后续 P0 修复 PR

## 未纳入本轮范围

- 不修改 `orgunit` 根列表分页实现
- 不修改 `CubeBox` 摘要渲染逻辑
- 不新增 `parent_org_code_from`
- 不对“组织树”做自动展开一层的能力补全
- 不补测试
