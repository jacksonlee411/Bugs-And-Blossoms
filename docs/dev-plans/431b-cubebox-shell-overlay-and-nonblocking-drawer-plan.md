# DEV-PLAN-431B：CubeBox 壳层避让与非阻断右侧悬挂方案

**状态**: 规划中（2026-04-22 15:35 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：承接 `DEV-PLAN-431` 与 `DEV-PLAN-431A`，单独冻结 CubeBox 右侧悬挂壳层在 Web Shell 中的顶栏避让、非阻断交互和响应式承载策略，避免实现阶段在“抽屉能打开”之后继续漂移为“被全局标题栏遮挡”或“打开即阻断左侧业务页面”。
- **关联模块/目录**：`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`、`docs/dev-plans/431a-cubebox-page-design-contract.md`、`docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`、`apps/web/src/layout/AppShell.tsx`、`apps/web/src/pages/cubebox/CubeBoxPanel.tsx`
- **关联计划/标准**：`DEV-PLAN-002`、`DEV-PLAN-005`、`DEV-PLAN-012`、`DEV-PLAN-431`、`DEV-PLAN-431A`、`DEV-PLAN-437`

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理 CubeBox 外层壳层布局与交互模式，不重定义 `431` 中的 conversation/turn/event/reducer 契约，也不替代 `431A` 的页面视觉合同。
2. **不变量**：CubeBox 仍然是“右侧悬挂、可拉出/收回”的正式承载面；桌面端打开后不得遮挡全局标题栏，不得默认阻断左侧业务页面继续操作。
3. **可解释**：reviewer 必须能在 5 分钟内说明为什么桌面端应采用非阻断右侧悬挂，而移动端仍允许退化为阻断式全屏/抽屉。

## 1. 背景与问题定义

当前 `apps/web/src/layout/AppShell.tsx` 已实现全局 `AppBar` 与右侧 CubeBox `Drawer`，但运行结果暴露出两个直接影响用户可见交付的问题：

1. **顶栏遮挡问题**：全局 `AppBar` 使用固定定位且层级高于 `Drawer`，左侧导航与主内容都通过 `<Toolbar />` 占位避让；但右侧 CubeBox `Drawer` 未做同类避让，导致 CubeBox 顶部标题区被标题栏覆盖。
2. **阻断式交互问题**：当前右侧 CubeBox 使用默认 `Drawer` 形态，等价于 `temporary`，打开时带有 modal/backdrop/focus trap 语义，会阻断左侧业务页面点击、滚动和继续操作。

这两个问题都属于“壳层 contract 未冻结”而不是“业务页面内容错误”：

- `431A` 已要求 CubeBox 页面结构是“顶栏 + 正文 + 底部 composer”，因此顶部被挡住属于壳层未履行页面合同。
- `431` 已定义桌面宽屏下右侧悬挂应可与主页面并行，因此桌面端继续维持阻断式 modal 语义，会破坏“边聊边操作左侧页面”的核心场景。

## 2. 目标与非目标

### 2.1 核心目标

1. 冻结 CubeBox 桌面端右侧悬挂壳层必须避让全局标题栏的正式 contract。
2. 冻结 CubeBox 桌面端打开后默认不阻断左侧业务页面的正式交互 contract。
3. 冻结桌面端与移动端的分层策略，避免未来在所有断点下一刀切使用同一种 Drawer 语义。
4. 为 `AppShell.tsx` 的实现提供单一判断标准，避免围绕 `Drawer` 的 `variant/backdrop/z-index/top/height` 出现临时拼接。

### 2.2 非目标

1. 不在本计划中改写 CubeBox timeline、composer、会话恢复或模型配置业务语义。
2. 不把 CubeBox 提升为左侧主导航独立页面，也不新增第二套页面主链来规避 Drawer 问题。
3. 不在本计划中冻结所有视觉细节，例如阴影、圆角、具体动效曲线；这些只要求服从本计划的壳层 contract。
4. 不允许通过“给内部内容加硬编码 `margin-top`”这种局部补丁替代壳层层级修正。

## 3. 现状成因分析

### 3.1 顶栏遮挡成因

- 全局 `AppBar` 为固定定位，并显式设置为高于 drawer 的层级。
- 左侧永久导航 drawer 与主内容区都在正文开始前插入 `<Toolbar />` 作为顶栏高度占位。
- 右侧 CubeBox drawer 当前未设置 `top`、`height` 或内部 `<Toolbar />` 占位，因此其 paper 从视口 `top: 0` 开始渲染。
- 结果是 CubeBox 顶栏/标题内容位于全局 `AppBar` 背后，形成“顶部被挡住”的可见缺陷。

### 3.2 左侧被阻断成因

- 当前右侧 CubeBox drawer 未显式指定 `variant`，按 MUI 默认行为等价于 `temporary`。
- `temporary` drawer 自带 `Modal`、`Backdrop` 与焦点锁定语义。
- 这类语义适合“必须先完成当前抽屉动作”的场景，不适合“边查看对话边继续操作左侧业务页面”的桌面助手场景。

### 3.3 为什么不能只做局部补丁

以下做法都不应作为正式方案：

- 只在 `CubeBoxPanel` 内部增加 `margin-top/padding-top` 去躲开标题栏。
- 继续保留 `temporary Drawer`，只把 backdrop 透明化或隐藏。
- 通过降低 `AppBar` 的 z-index 让 CubeBox 覆盖顶栏。

原因：

- 这些做法会把全局壳层问题下沉到业务组件，造成职责漂移。
- modal 语义与非阻断交互目标天然冲突，隐藏 backdrop 不等于消除阻断。
- 全局标题栏是 Shell SSOT，不能为了一个右侧助手面板破坏全局层级。

## 4. 方案备选

### 4.1 方案 A：保留 temporary Drawer，仅修正顶部偏移

做法：

- 为右侧 drawer 增加顶栏高度避让。
- 继续保留 `temporary` 的 modal/backdrop 行为。

优点：

- 改动小，最容易落地。

缺点：

- 只能解决“被遮挡”，不能解决“阻断左侧页面”。
- 与 `431` 的桌面端并行使用目标不一致。

结论：

- **不采纳作为正式桌面方案**；最多可作为短期排障过渡，不得写入长期 contract。

### 4.2 方案 B：桌面端改为 persistent 右侧 Drawer，移动端保留 temporary

做法：

- `md` 及以上使用 `persistent` 右侧 drawer。
- 为 drawer paper 增加全局标题栏避让：`top` 与 `height` 按 `theme.mixins.toolbar` 计算。
- `xs/sm` 继续使用 `temporary`，允许全屏或近全屏承载。

优点：

- 同时解决顶栏遮挡与桌面端阻断问题。
- 仍保留“右侧滑出/收回”的认知模型，和当前实现最连续。
- 语义上最接近“桌面助手侧挂面板”。

缺点：

- 需要处理桌面端打开后是否压盖主内容、以及主内容是否让位。
- 响应式逻辑比单一 variant 稍复杂。

结论：

- **推荐采纳**，作为本计划正式 owner 方案。

### 4.3 方案 C：自定义右侧浮动 Panel，放弃 MUI Drawer

做法：

- 自行实现固定定位右侧 panel，独立控制过渡动画、遮罩、焦点与尺寸。

优点：

- 控制力最强，后续可扩展拖拽、可调宽度、分栏模式。

缺点：

- 重复造壳层轮子，超出当前阶段“简单优先”的范围。
- 需要重新补齐一整套无障碍、焦点管理和响应式行为。

结论：

- **当前不采纳**；仅在 future iteration 需要可调宽度或 IDE 级复杂壳层时再评估。

## 5. 正式决策

### 5.1 壳层承载决策

CubeBox 壳层正式冻结为：

1. **桌面端（`md` 及以上）**：右侧 `persistent` 悬挂 drawer。
2. **窄屏/移动端（`sm` 及以下）**：右侧 `temporary` 或全屏抽屉。
3. **所有断点**：仍保持“右侧拉出/收回”的单一产品语义，不新增第二套页面主链。

### 5.2 顶栏避让决策

桌面端与移动端的 CubeBox 壳层都必须避让全局标题栏：

- 避让必须在 `AppShell` 的 drawer/paper 层实现。
- 推荐实现口径：
  - `top: theme.mixins.toolbar.minHeight`
  - `height: calc(100% - toolbarHeight)`
- 若实现上改为在 drawer 内首行放置 `<Toolbar />` 占位，也必须由壳层组件统一承接，不得下沉到 `CubeBoxPanel`。

### 5.3 非阻断交互决策

桌面端打开 CubeBox 后，左侧页面必须继续具备以下能力：

- 可点击按钮、链接、表格、树和表单。
- 可滚动主页面内容。
- 可切换路由而不强制关闭 CubeBox。
- 可在保留 CubeBox 打开状态的同时继续读取或编辑左侧业务内容。

移动端不强制满足上述条件，允许因为屏幕限制而退化为阻断式承载。

### 5.4 主内容让位策略

桌面端默认允许以下两种承载策略，但必须在实现前二选一并保持一致：

1. **覆盖式悬挂**：CubeBox 悬挂在主内容之上，但不阻断主内容交互。
2. **让位式悬挂**：CubeBox 打开时主内容区减少右侧可用宽度，避免被覆盖。

当前推荐顺序：

1. 先实现 **覆盖式悬挂**，因为改动更小且不影响现有主布局。
2. 若后续验证发现大量表格/表单场景被遮挡影响可用性，再升级为 **让位式悬挂**。

## 6. 实施约束

1. [ ] `AppShell` 必须成为右侧 CubeBox 壳层的单一 owner，负责 `variant`、`top`、`height`、`width`、`open/close` 与响应式分支。
2. [ ] `CubeBoxPanel` 只负责 header/timeline/composer 等内容层，不得承担对全局标题栏高度、drawer 类型或 backdrop 语义的判断。
3. [ ] 不得通过手写常量把标题栏高度硬编码为多个互相独立的 magic number；应统一从 theme 或单一常量派生。
4. [ ] 不得在桌面端继续保留 backdrop 阻断左侧交互。
5. [ ] 不得因为引入桌面端 `persistent` 而新增第二套 CubeBox store、第二套路由或第二套 API client。

## 7. 验收标准

### 7.1 顶栏避让验收

- [ ] 打开 CubeBox 后，顶部标题与图标区完整可见，不被全局 `AppBar` 覆盖。
- [ ] 切换中英文、深浅色、不同会话标题长度时，顶部区域仍完整可见。
- [ ] 窗口高度变化后，CubeBox timeline 与 composer 的可见区域仍正确计算，不出现双重遮挡。

### 7.2 桌面端非阻断验收

- [ ] 桌面宽度下打开 CubeBox 后，左侧页面按钮、链接、表格行、树节点和表单仍可交互。
- [ ] 左侧页面滚动不被 backdrop 阻止。
- [ ] 左侧路由切换后，CubeBox 可按既有 contract 保持或恢复打开状态，不因 modal 生命周期被意外销毁。

### 7.3 响应式验收

- [ ] `md` 及以上断点采用非阻断右侧悬挂。
- [ ] `sm` 及以下断点允许退化为阻断式抽屉或全屏对话承载。
- [ ] 从窄屏切回宽屏时，不出现 drawer 状态错乱、位置跳变或内容重复挂载。

## 8. Stopline

- 不得把 CubeBox 改成独立主页面来回避右侧悬挂问题。
- 不得通过在 `CubeBoxPanel` 内部增加硬编码顶边距来修正全局壳层遮挡。
- 不得在桌面端继续使用默认 `temporary + backdrop` 作为长期正式方案。
- 不得为了让 CubeBox 盖住标题栏而下调全局 `AppBar` 层级。

## 9. 与现有计划的关系

- `431` 持有 CubeBox 的协议、状态机、timeline reducer 与右侧悬挂壳层的总原则；`431B` 是其下钻的 Shell 布局/交互子方案。
- `431A` 持有页面层视觉与语义合同；`431B` 负责保证这些页面元素不会因为壳层实现错误而被顶栏遮挡或被 modal 语义破坏。
- `437` 的“抽屉壳层与页面视觉合同并行推进”在本计划中进一步具体化为：可以并行，但壳层实现必须消费 `431A/431B`，不得各自解释“右侧悬挂”的含义。

## 10. 实施步骤

1. [ ] 在 `AppShell` 中为 CubeBox 壳层引入断点分支：桌面端 `persistent`，移动端 `temporary`。
2. [ ] 在 drawer paper 层补齐顶栏避让与高度计算，移除被标题栏遮挡的问题。
3. [ ] 桌面端验证左侧主内容可继续点击、滚动与路由切换。
4. [ ] 评估桌面端是否需要“主内容右侧让位”；如无需立即落地，至少在 PR 说明中明确当前采用“覆盖式悬挂”。
5. [ ] 补齐对应组件测试或 E2E 验证，覆盖桌面端非阻断与移动端退化行为。

## 11. 本地必跑与门禁

- 文档变更：`make check doc`
- 命中 `apps/web` 实现时：`pnpm --dir apps/web check`
- 若补充页面交互验证：按 `DEV-PLAN-012`/`DEV-PLAN-437` 补充对应 E2E 证据到 `docs/dev-records/`

## 12. 关联文档

- `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- `docs/dev-plans/431a-cubebox-page-design-contract.md`
- `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`
