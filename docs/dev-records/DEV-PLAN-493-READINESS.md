# DEV-PLAN-493 Readiness

## 说明

- 本文件记录 `DEV-PLAN-493` 的实施结果与验证证据。
- 本次实现范围：组织架构页搜索与筛选入口收敛、显式 `include_descendants` 范围列表契约、页面 URL 状态收敛，以及 `designs/493.pen` 首期信息架构落地。

## 2026-05-05 实施记录

### 已完成

- `/org/api/org-units` list/grid query parser 已接入三态 `include_descendants`：
  - 未传参保持既有 direct-children/root/list 语义。
  - `include_descendants=false` 返回当前范围锚点本身及直接下级。
  - `include_descendants=true` 返回当前范围锚点本身及全部各级可见下级。
- `modules/orgunit/services.OrgUnitReadService.List` 已承接范围列表业务规则：
  - 先 resolve 并校验锚点可见性。
  - 下级结果按当前 principal scope 裁剪。
  - keyword/status filter 只作用于锚点之后的下级结果。
  - 锚点固定第一页第一行，分页 total 以锚点加过滤后的下级结果为准。
- 组织架构页 UI 已按 `designs/493.pen` 首期信息架构收敛：
  - 首屏保留一个主搜索入口，支持防抖刷新列表、Enter/搜索图标定位组织。
  - 新增“包含各级下级”开关并置于“包含无效”左侧，页面请求和 URL 均显式写出 `include_descendants=true/false`。
  - “历史视图”入口替换为“生效日期”日期框；默认显示当天，用户修改合法日期后写入 `as_of`。
  - 删除首屏状态下拉、首屏应用按钮、树内搜索框和扩展字段查询区。
  - 旧页面链接中的扩展字段查询/排序参数进入组织架构页时加载期清理，不再作为隐藏条件继续影响列表。
  - 树节点只显示组织名称；右侧列表第一行显示当前范围锚点组织。
  - 旧 `status=active/inactive/disabled` deep link 仅作为可见 shim 生效，并提供清除动作；任一新主筛选更新会清理该参数。
  - 多候选选择会解析候选路径并展开树父链，避免深层候选选中态不可见。
  - 表格 toolbar 显式关闭 MUI X quick filter，避免和页面主搜索形成第二搜索入口。
- CubeBox `orgunit.list` tool schema / knowledge pack / planner prompt 首期未新增 `include_descendants`。

### 已验证

- `go fmt ./...`
- `go test ./modules/orgunit/services ./internal/server`
- `go test ./...`
- `go vet ./...`
- `make check lint`
- `pnpm --dir apps/web typecheck`
- `pnpm --dir apps/web test -- OrgUnitsPage.test.tsx`
- `pnpm --dir apps/web test -- OrgUnitsPage.test.tsx orgUnitListExtQuery.test.ts`
- `pnpm --dir apps/web lint`
  - 通过；存在既有 9 个 warning，位于 `FreeSoloDropdownField.tsx`、`AuthzCatalogPage.tsx`、`CubeBoxProvider.tsx`。
- `pnpm --dir apps/web build`
  - 通过；存在 Vite 既有大 chunk 提示。
- `make authz-pack && make authz-test && make authz-lint`
- `make check routing`
- `make check no-legacy`
- `make check cubebox-api-first`
- `make check tr`
- `make check doc`
- 本地浏览器验证：
  - 使用 `tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh --build-ui` 启动并登录成功。
  - 打开 `http://localhost:8080/app/org/units` 后页面自动规范化为 `?include_descendants=false`。
  - DOM 文本确认首屏包含“搜索组织 / 包含各级下级 / 包含无效 / 生效日期”，不再出现“查看历史 / 应用筛选 / 组织状态 / 扩展字段查询入口”。
  - 组织树节点显示“飞虫与鲜花”，未显示括号编码；右侧列表第一行显示当前锚点 `100000 / 飞虫与鲜花`。

### 2026-05-05 评审意见复核补充

- 已确认并修复旧 `status=disabled` 页面 deep link 未继续生效的问题；前端 shim 现在按后端 alias 口径映射为 `inactive`，并保留可见清理提示。
- 已确认并修复多候选选择后深层路径不可展开的问题；候选 DTO 类型允许携带 `path_org_codes`，缺省时通过现有 `searchOrgUnit` 成功响应解析路径，不新增后端 route。
- 已确认 MUI X `@mui/x-data-grid@8.27.0` 默认 toolbar 的 `showQuickFilter=true`；组织架构页显式关闭 quick filter，`DataGridPage` 在启用 toolbar 时也默认关闭 quick filter，调用方仍可显式开启。
- 已修正文档中页面测试覆盖描述，并补充页面测试覆盖 `status=disabled` shim、多候选路径展开、包含各级下级自动应用、旧扩展字段参数清理和表格 quick filter 关闭。
