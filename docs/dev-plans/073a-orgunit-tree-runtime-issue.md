# DEV-PLAN-073A：组织架构树运行态问题记录（Shoelace 资源加载失败）

**状态**: 进行中（2026-02-05 00:48 UTC）

## 背景
- 本记录用于补充 DEV-PLAN-073 的运行态验证结果，聚焦组织架构树 UI 的实际可用性。

## 问题描述
- `/org/nodes?as_of=2026-02-05` 页面能渲染 `sl-tree` 的 HTML 结构，但 Shoelace Web Components 未注册。
- 浏览器控制台报错：`Failed to resolve module specifier "lit/directives/class-map.js". Relative references must start with either "/", "./", or "../".`
- 结果：树的展开/折叠与懒加载事件（`sl-lazy-load`）无法正常工作，导致子节点无法通过 UI 交互加载。

## 影响范围
- 组织架构树的“展开/折叠/懒加载”交互不可用。
- 依赖树交互的“搜索定位逐级展开”流程无法完成。

## 复现步骤
1. 登录：`http://localhost:8080/login`（账号 `admin@localhost` / `admin123`）。
2. 打开：`http://localhost:8080/org/nodes?as_of=2026-02-05`。
3. 打开浏览器控制台，出现 Shoelace 依赖加载错误；展开节点无效。

## 证据（本地验证）
- 页面加载后 `customElements.get('sl-tree-item') === false`（组件未注册）。
- 控制台错误：`Failed to resolve module specifier "lit/directives/class-map.js"...`。
- 资源可达性：`GET /assets/shoelace/shoelace.js` 返回 200，但其依赖的 `lit/*` 模块在运行时解析失败。
- 后端接口可用：`GET /org/nodes/children?...` 返回 `<sl-tree-item ...>` 片段，说明数据层正常。

## 初步判断
- Shoelace 资源包中存在裸模块引用（`lit/directives/*`），未被改写为可被浏览器直接解析的路径，也未提供 import map。
- 该错误阻断了 Shoelace 组件注册，从而导致树交互失败。

## 待办事项
1. [ ] 确认 Shoelace 资源的打包方式与 import map 是否缺失。
2. [ ] 修复 Shoelace 资源加载（确保 `lit` 依赖可解析）。
3. [ ] 重新验证组织树的展开/折叠/懒加载。

## 关联文档
- `docs/dev-plans/073-orgunit-crud-implementation-status.md`
