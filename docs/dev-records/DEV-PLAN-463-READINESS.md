# DEV-PLAN-463 Readiness

## 状态

- `463`：P0 缺陷已修复，待页面复验回填
- 首次调查日期：`2026-04-24`
- 本次修复日期：`2026-04-24`
- 关联 owner：`463`、`464`

## 结论

`DEV-PLAN-463` 的两个直接问题已经拆开处理：

1. 根列表分页遗漏 `has_children` 的 P0 缺陷，已在 `internal/server/orgunit_field_metadata_store.go` 修复。
2. 页面回答层把缺失值模板化渲染为“未知”的旧表现，已在 `DEV-PLAN-464` 下通过 narrator 主切和输出约束 fail-closed 修复。

当前仍保留的产品边界只有一项：`“组织树”` 仍默认返回当前租户一级组织入口，而不是整棵树递归展开；这属于现有查询能力边界，不再视为本轮缺陷。

## 历史页面失败证据

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

### 3. 当时页面实际回答

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

- 当时用户可见面确实出现 `有下级：未知`
- 当时页面回答与 `463` 文档中的缺陷描述一致

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

## 修复落实

### 1. 根列表分页已补齐 `has_children`

当前实现位于：

- [internal/server/orgunit_field_metadata_store.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_store.go)

修复结果：

- 根列表与子列表现在统一查询 `has_children`
- 根列表扫描现在也会给 `item.HasChildren` 赋值
- 相关回归测试已同步更新，根列表路径不再允许 `HasChildren=nil`

### 2. 回答层旧模板路径已退出 query 命中主链

当前回答层位于：

- [internal/server/cubebox_query_flow.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go)

修复结果：

- query 命中结果叙述已经切到 narrator
- narrator 输出新增模板化结果拦截，旧式 `已完成只读查询 / 本次关注 / step-1 / 详情如下` 会 fail-closed
- 当前 query 主链不再依赖 capability-specific 模板摘要

## 当前剩余项

以下事项当前仍待补齐：

1. 如需封板 `463` 计划文档，再同步勾选对应验收项

## 本轮完成内容

- 根列表 `has_children` 修复落地
- 查询结果叙述已切回 narrator，并增加输出约束 fail-closed
- 相关单元测试/定向测试已更新到新 owner 口径
- `463` 现已从“问题仍存在的调查记录”转为“缺陷已修复，待页面复验”

## 最新页面复验证据

### 1. 真实页面入口

- 入口：`http://localhost:8080/app/login`
- 登录：`admin@localhost / admin123`
- 验证位置：主壳层右侧 `CubeBox` 抽屉

### 2. `查询今天的组织树`

页面最新可见回答：

```text
截至 2026-04-24，你的组织树一级节点只有 1 个：100000「飞虫与鲜花」，当前为启用状态，并且还有下级组织。
```

结论：

- 已不再出现 `有下级：未知`
- 已不再出现 `已完成只读查询 / 本次关注 / step-1` 这类模板化结果
- 根列表 `has_children` 缺失导致的旧页面症状已消失

### 3. `查一下 100000 在 2026-04-24 的组织详情`

页面最新可见回答：

```text
截至 2026-04-24，组织 100000 名称是“飞虫与鲜花”，当前为启用状态，且属于业务单元。其全路径为“飞虫与鲜花”。系统中未记录它的上级组织和负责人信息，也没有扩展字段记录。
```

结论：

- query 命中结果已稳定走 narrator
- 页面已不再回落到 capability-specific 模板摘要

## 未纳入本轮范围

- 不把“组织树”扩成自动递归展开全部层级
- 不引入新的跨步参数引用 DSL
- 不建设新的本地解释平台
