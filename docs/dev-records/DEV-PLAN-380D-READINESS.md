# DEV-PLAN-380D Readiness

## 1. 本轮时间与范围

1. 时间：2026-04-16 14:05 CST
2. 计划：`docs/dev-plans/380d-cubebox-file-plane-formalization-plan.md`
3. 本轮命中切片：
   - [x] Contract Slice
   - [x] Persistence Slice
   - [x] Storage Slice
   - [x] Service Slice
   - [x] Delivery Slice
   - [x] Readiness Slice

## 2. 本轮实现结论

1. [x] 新增文件面 durable cleanup persistence contract：
   - `modules/iam/infrastructure/persistence/schema/00014_iam_cubebox_file_cleanup_jobs.sql`
   - `migrations/iam/20260416160000_iam_cubebox_file_cleanup_jobs.sql`
   - `migrations/iam/atlas.sum` 已更新
2. [x] `modules/cubebox/services/files.go` 已从过渡期 `legacy FileStore` 主链收口为：
   - 正式主链：`PG metadata + links + localfs object store + cleanup jobs`
   - 兼容支架：保留单参数 `NewFileService(store)` 以承接既有测试与临时 stub，但运行态正式装配已不再走该分支
3. [x] `modules/cubebox/infrastructure/local_file_store.go` 已从 `index.json` 事实源改成纯对象存储适配器：
   - 只负责对象写入 / 物理删除 / 健康检查
   - 不再承担读列表、删 metadata、维护索引
4. [x] `internal/server/handler.go` 已改为正式装配：
   - `cubeboxmodule.NewPGFileService(pgPool, cubeboxmodule.DefaultLocalFileRoot())`
5. [x] `runtime-status.file_store` 健康探针已覆盖正式主链两部分：
   - `PG metadata repo`
   - `localfs object store`
5. [x] `/internal/cubebox/files` 已开始向 `380C` 完成态字段靠拢：
   - 主字段：`file_id`、`filename`、`content_type`、`size_bytes`、`scan_status`、`created_at`、`links[]`
   - 兼容字段仍保留：`conversation_id`、`file_name`、`media_type`、`uploaded_at`

## 3. 正式语义对齐

1. [x] tenant 级列表以 `cubebox_files` 为主，`links[]` 由 `cubebox_file_links` 聚合。
2. [x] `conversation_id` 过滤已走正式单链路：`GET /internal/cubebox/files?conversation_id=...`
3. [x] 删除语义已收口为：
   - 文件不存在：`cubebox_file_not_found`
   - 文件仍被引用：`cubebox_file_delete_blocked`
   - 删除成功：`204`
4. [x] 上传非法已收口为：`cubebox_file_upload_invalid`
5. [x] cleanup durable persistence 已具备正式落点：
   - 表：`iam.cubebox_file_cleanup_jobs`
   - 原因：`metadata_write_failed` / `object_delete_failed`

## 4. 兼容窗口登记

1. 当前仍保留的兼容字段：
   - `conversation_id`
   - `file_name`
   - `media_type`
   - `uploaded_at`
2. 保留原因：
   - 当前 `380D` 只负责文件面事实源与恢复主链正式化
   - files 对外 DTO 字段名的最终删除批次仍由 `380C` 统一持有
3. 完成态主字段已切换为：
   - `filename`
   - `content_type`
   - `created_at`
   - `links[]`

## 5. `index.json` 退出状态

1. [x] 正式 list / delete / metadata 写入路径已不再依赖 `index.json`
2. [x] `index.json` 当前仅剩历史导入器语义：
   - `cmd/dbtool cubebox-import-local-files`
   - `cmd/dbtool cubebox-verify-file-import`
3. [x] `localfs` 运行态职责已改为对象正文存储，不再充当文件元数据 SoT

## 6. orphan / cleanup 结果

1. [x] `orphan file` 与 cleanup queue 已明确分离：
   - `orphan file`：无 link 但 metadata 仍在，属于正常业务状态
   - cleanup job：仅处理 metadata/object 不一致的异常补偿
2. [x] 当前已落地的 cleanup 记录入口：
   - metadata/link 写入失败后，先同步补偿删除对象；仅当对象补偿删除失败时登记 cleanup job
   - metadata 删除成功但对象删除失败时登记 cleanup job
3. [ ] cleanup worker / retry runner 尚未单独实现后台消费循环
   - 本轮已满足 `380D` stopline：durable persistence 已存在，不再只靠日志/内存
   - 后续若要补正式重试 worker，应在 `380D/380G` 后续批次继续登记

## 7. 验证记录

1. [x] `make sqlc-generate`
2. [x] `go test ./modules/cubebox/... ./internal/server`
3. [x] `pnpm --dir apps/web test -- --runInBand`
4. [x] `make iam plan`
5. [x] `make iam lint`

## 8. 结果摘要

1. [x] 文件面正式事实源已切到 `PostgreSQL metadata + links`
2. [x] 对象存储正式运行职责已切到 `localfs object store adapter`
3. [x] cleanup durable persistence 已正式落库
4. [x] `/internal/cubebox/files` 已开始向 `380C` 完成态字段收口
5. [x] 当前未检测到 `index.json` 被正式 list/delete 路径继续依赖
