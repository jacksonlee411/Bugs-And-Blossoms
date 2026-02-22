# DEV-PLAN-073A：组织架构树运行态问题记录（Shoelace 资源加载失败）

**状态**: 已解决（2026-02-05 11:15 UTC）

## 背景
- 本记录用于补充 DEV-PLAN-073 的运行态验证结果，聚焦组织架构树 UI 的实际可用性。

## 问题描述
- `/org/nodes?tree_as_of=2026-02-05` 页面能渲染 `sl-tree` 的 HTML 结构，但 Shoelace Web Components 未注册。
- 浏览器控制台报错：`Failed to resolve module specifier "lit/directives/class-map.js". Relative references must start with either "/", "./", or "../".`
- 结果：树的展开/折叠与懒加载事件（`sl-lazy-load`）无法正常工作，导致子节点无法通过 UI 交互加载。

## 补充问题（已确认）
- Shoelace 资源加载修复后，点击“飞虫与鲜花”节点的展开按钮仍一直显示加载中。
- 事件监听使用 `event.detail.item` 获取节点，但 `sl-lazy-load` 的 `detail.item` 在运行时为 `null`，导致 `loadChildren` 未执行，前端请求未发出。


## 影响范围
- 组织架构树的“展开/折叠/懒加载”交互不可用。
- 依赖树交互的“搜索定位逐级展开”流程无法完成。

## 复现步骤
1. 登录：`http://localhost:8080/login`（账号 `admin@localhost` / `admin123`）。
2. 打开：`http://localhost:8080/org/nodes?tree_as_of=2026-02-05`。
3. 打开浏览器控制台，出现 Shoelace 依赖加载错误；展开节点无效。

## 证据（本地验证）
- 页面加载后 `customElements.get('sl-tree-item') === false`（组件未注册）。
- 控制台错误：`Failed to resolve module specifier "lit/directives/class-map.js"...`。
- 资源可达性：`GET /assets/shoelace/shoelace.js` 返回 200，但其依赖的 `lit/*` 模块在运行时解析失败。
- 后端接口可用：`GET /org/nodes/children?...` 返回 `<sl-tree-item ...>` 片段，说明数据层正常。
- 监听到 `sl-lazy-load` 事件时：`event.detail.item === null`，但 `event.target` 为对应的 `sl-tree-item`（org_id=10000000）。
- 未修复时：前端无 `/org/nodes/children` 请求发出，节点 `loading` 长期为 `true`。

## 初步判断
- Shoelace 资源包中存在裸模块引用（`lit/directives/*`），未被改写为可被浏览器直接解析的路径，也未提供 import map。
- 该错误阻断了 Shoelace 组件注册，从而导致树交互失败。
- `sl-lazy-load` 事件不保证 `detail.item` 有值；应优先使用 `event.target` / `event.composedPath()` 获取触发的 `sl-tree-item`。

## 修复与验证记录
### 修复 1：Shoelace 资源加载（方案 A）
- 在 `apps/web/src/pages/index.astro` 注入 import map，补齐 `lit/*`、`@floating-ui/*`、`@ctrl/tinycolor` 等依赖映射。
- 在 `scripts/ui/build-astro.sh` 增加 vendor 依赖拷贝（同步到 `internal/server/assets/shoelace/vendor/`）。
- 结果：`customElements.get('sl-tree-item') === true`，树组件可正常注册。

### 修复 2：懒加载事件取不到 item
- `sl-lazy-load` 监听改为 `(event.detail && event.detail.item) || event.target` 获取节点。
- `sl-selection-change` 改为读取 `tree.selectedItems[0]`。
- 子节点 HTML 增加 `slot="children"`，确保 Shoelace tree 识别子层级。

### 修复 3：展开后节点标题被覆盖
- 原因：`fetch` 后直接写入 `innerHTML` 会覆盖当前 `sl-tree-item` 的 label，仅留下子节点，导致展开后父节点文本变为空。
- 修复：将 swap 改为 `beforeend`，仅追加子节点，保留父节点 label。

### 验证结果
- 打开：`/org/nodes?tree_as_of=2026-02-05`。
- 点击“飞虫与鲜花”（org_id=10000000）展开：
  - `GET /org/nodes/children?parent_id=10000000&as_of=2026-02-05` 返回 200。
  - 2 秒内 `lazy=false`、`loading=false`，子节点渲染数量 `children=3`。
  - 展开/折叠可重复执行。

## 关联文档
- `docs/dev-plans/073-orgunit-crud-implementation-status.md`
