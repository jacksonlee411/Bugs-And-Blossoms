# DEV-PLAN-350：前端产品壳与交互系统子计划

**状态**: 规划中（2026-03-17 07:23 CST）

## 1. 背景与上下文

`340` 提供平台壳与会话上下文，`360/370/380/390` 都会产生大量页面，但目前还没有一个计划真正拥有前端产品系统本身。

`350` 负责冻结前端的统一交付语言：

- 信息架构
- 导航与路由
- 列表/详情/历史模式
- 表单系统
- MUI 组件规范
- 权限感知 UI

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立统一前端信息架构和导航模式。
- [ ] 建立列表、详情、历史三件套页面模式。
- [ ] 建立表单、校验、错误反馈、空状态的统一规范。
- [ ] 建立权限感知的 UI 展示约定。

### 2.2 非目标

- [ ] 本计划不承接具体业务规则。
- [ ] 本计划不替代设计系统工具链，但会定义产品交互规范。

## 3. 范围

- Product Shell
- Routing & Navigation
- Page Patterns
- Form Patterns
- Grid / Tree / Timeline 模式
- Permission-aware UI

## 4. 关键设计决策

### 4.1 页面模式优先统一（选定）

- 列表页
- 详情页
- 历史页
- 对话/工作台页

### 4.2 生效日期是 UI 一级概念（选定）

- 任何支持历史的业务对象，都必须在 UI 上显式暴露 effective date 视角。

### 4.3 不把复杂交互藏进零散弹窗（选定）

- 复杂对象优先使用详情页或双栏页。
- Dialog 只用于短事务。

## 5. 功能拆分

### 5.1 M1：Product Shell

- [ ] 导航
- [ ] 路由分组
- [ ] 顶栏/侧栏
- [ ] 租户与用户上下文

### 5.2 M2：Page Patterns

- [ ] 列表页模板
- [ ] 详情页模板
- [ ] 历史页模板
- [ ] 工作台模板

### 5.3 M3：表单与交互规范

- [ ] 表单布局
- [ ] 校验反馈
- [ ] 错误/空状态
- [ ] 提交确认

## 6. 验收标准

- [ ] 后续所有业务页面都能复用统一的前端模式，而不是各自设计页面骨架。
- [ ] 有效期历史、详情和编辑交互已经有清晰统一的呈现方式。
- [ ] 权限差异在 UI 上有一致的行为表达。

## 7. 后续拆分建议

1. [ ] [DEV-PLAN-351：Product Shell 与路由信息架构详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/351-product-shell-and-route-information-architecture-detailed-design.md)
2. [ ] [DEV-PLAN-352：列表/详情/历史页面模式详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md)
3. [ ] [DEV-PLAN-353：表单与权限感知交互详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md)
