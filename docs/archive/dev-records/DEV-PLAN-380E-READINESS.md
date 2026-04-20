# DEV-PLAN-380E Readiness

## 1. 本轮时间与范围

1. 时间：2026-04-17 05:32 CST
2. 计划：`docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
3. 本轮命中切片：
   - [x] Contract Slice
   - [x] Delivery Slice
   - [x] Test & Gates Slice
   - [x] Readiness Slice

## 2. 本轮实现结论

1. [x] `apps/web` 正式导航已收口到 CubeBox 专用语义：
   - `apps/web/src/navigation/config.tsx`
   - `apps/web/src/i18n/messages.ts`
2. [x] `cubebox/models` 导航权限已与路由权限统一为 `orgunit.read`。
3. [x] `CubeBoxPage`、`CubeBoxFilesPage`、`CubeBoxModelsPage` 已收口到统一错误提示主路径：
   - `apps/web/src/pages/cubebox/errorMessage.ts`
   - 页面不再以内联 `messageForError` 作为主错误语义入口
4. [x] `CubeBoxModelsPage` 已补齐页面级测试，并改为“模型列表失败不遮蔽运行态、运行态失败不遮蔽模型列表”的分层降级。
5. [x] `apps/web` 前端正式文案已进入统一 `en/zh` i18n 入口，未使用的历史 `nav_ai_assistant` / `nav_librechat` 导航文案 key 已删除。
6. [x] 路由 alias 与导航/路由权限一致性断言已补齐：
   - `apps/web/src/router/index.test.tsx`
   - `apps/web/src/navigation/config.test.tsx`

## 3. 用户可见结果

1. [x] `/app/cubebox` 继续保持聊天壳主入口：
   - 会话列表
   - 消息流
   - 输入框 / 发送 / 确认 / 提交
   - 附件上传入口
2. [x] `/app/cubebox/files` 明确收口为文件配套页，而不是独立治理控制台。
3. [x] `/app/cubebox/models` 明确收口为只读模型与运行态摘要页，而不是旧助手模型治理页替身。
4. [x] `/app/assistant`、`/app/assistant/models` 继续只保留 redirect alias，不承接页面组件。
5. [x] `/app/assistant/librechat` 的退役态继续由服务端 contract 承接；前端本轮未重新引入该入口。

## 4. 命名与残留收口

1. [x] `apps/web` 导航 label key 已切到 `nav_cubebox` / `nav_cubebox_models` / `nav_cubebox_files`。
2. [x] 前端环境变量已补充 `VITE_CUBEBOX_TURN_TIMEOUT_MS` 主入口，并兼容旧 `VITE_ASSISTANT_TURN_TIMEOUT_MS` 读取窗口。
3. [x] `CubeBoxPage.test.tsx` 中不再把旧 assistant 页面或旧 client 当作前端主链断言对象。
4. [x] 历史 assistant / librechat 错误码映射仍保留在 `presentApiError`，但仅服务于退役解释层，不再作为正式页面运行入口。

## 5. 验证记录

1. [x] `pnpm --dir apps/web test`
2. [x] `pnpm --dir apps/web typecheck`
3. [x] `make check tr`
4. [x] `make check error-message`
5. [x] `make generate`
   - 结果：通过（no-op placeholder）
6. [x] `pnpm --dir apps/web build`
   - 结果：通过
   - 备注：Vite 仍提示 chunk size warning，非本轮新增错误
7. [x] `pnpm --dir apps/web check`
   - 结果：通过
   - 备注：lint 阶段仍有既有 `FreeSoloDropdownField.tsx` fast-refresh warning；build 阶段仍有 Vite chunk size warning，均非本轮新增错误
8. [x] `make css`
   - 结果：通过
   - 备注：已同步服务端内嵌静态资产到 `internal/server/assets/web/assets/index-DJvRBz6k.js`
9. [ ] `git status --short` 为空
   - 当前不为空，原因是本轮代码、文档、`internal/server/assets/web` 生成物以及 E2E 证据资产尚未提交
10. [ ] `make e2e`
   - 默认端口运行结果：失败于 `127.0.0.1:8080` 已被本机既有服务占用
   - 备用端口运行结果：`E2E_BASE_URL=http://localhost:18080 E2E_SUPERADMIN_BASE_URL=http://localhost:18081 KRATOS_PUBLIC_URL=http://127.0.0.1:14433 E2E_KRATOS_ADMIN_URL=http://127.0.0.1:14434 make e2e`
   - 结果：31 个用例中 17 passed / 6 skipped / 2 did not run / 6 failed
   - 本轮前端相关失败已通过后续定向复跑关闭；剩余完整套件失败中包含 `tp290b-e2e-002` 的 `ai_plan_schema_constrained_decode_failed`，属于模型/后端链路，不属于 `380E` 前端收口范围
11. [x] `E2E_BASE_URL=http://localhost:18080 E2E_SUPERADMIN_BASE_URL=http://localhost:18081 KRATOS_PUBLIC_URL=http://127.0.0.1:14433 E2E_KRATOS_ADMIN_URL=http://127.0.0.1:14434 ./scripts/e2e/run.sh tests/tp220-assistant.spec.js tests/tp283-librechat-formal-entry-cutover.spec.js --workers=1`
   - 结果：7 passed
   - 覆盖：`/app/assistant` redirect、`/app/assistant/models` redirect、`/app/assistant/librechat` 退役态、CubeBox 正式入口
12. [x] `E2E_BASE_URL=http://localhost:18080 E2E_SUPERADMIN_BASE_URL=http://localhost:18081 KRATOS_PUBLIC_URL=http://127.0.0.1:14433 E2E_KRATOS_ADMIN_URL=http://127.0.0.1:14434 ./scripts/e2e/run.sh tests/tp060-02-master-data.spec.js --workers=1`
   - 结果：1 passed
   - 覆盖：主数据链路中对 CubeBox 入口、文件/模型入口的用户可见断言

## 6. 交接给 `380G` 的 E2E 断言范围

1. [x] 导航中存在 `CubeBox` 正式入口，且 `模型` / `文件` 为其配套页。
2. [x] `/app/assistant` -> `/app/cubebox` redirect。
3. [x] `/app/assistant/models` -> `/app/cubebox/models` redirect。
4. [x] `/app/assistant/librechat` 继续返回 `410 Gone`：
   - 现有服务端证据可引用 `internal/server/librechat_web_ui_test.go`
   - 现有服务端证据可引用 `internal/server/handler_test.go`
5. [x] 主聊天链路至少覆盖：
   - 进入 `/app/cubebox`
   - 发送消息
   - 候选确认
   - 提交任务
   - 从聊天页进入文件页 / 模型页

## 7. 待补齐项

1. [x] 将 `pnpm --dir apps/web build`、`pnpm --dir apps/web check`、`make css` 的最终结果回填到本记录。
2. [x] 记录 `git status --short` 最终状态。
3. [x] 尝试执行 `make e2e`；默认端口受本机 `8080` 占用阻塞，备用端口完整套件仍存在非 `380E` 的模型/后端链路失败。
4. [x] 通过定向 E2E 子集关闭 `380E` 前端边界：
   - `tp220-assistant.spec.js`
   - `tp283-librechat-formal-entry-cutover.spec.js`
   - `tp060-02-master-data.spec.js`
