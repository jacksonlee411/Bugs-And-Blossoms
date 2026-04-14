# DEV-PLAN-380D：CubeBox 文件面正式化

**状态**: 草拟中（2026-04-14 20:52 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `CubeBox` 文件元数据、文件引用关系与文件存储适配器正式化的实施 SSOT。  
> `380A` 负责总数据面 contract；本文聚焦文件面本身，不扩张到完整会话主链。

## 1. 背景与定位

1. [ ] 当前 `CubeBox` 文件能力仅有本地目录 + `index.json` 的最小闭环，尚非正式业务 SoT。
2. [ ] `CubeBox` 正式产品面需要明确 `cubebox_files / cubebox_file_links`、删除保护、引用关系与可替换存储适配器。
3. [ ] 文件面收口应独立于会话/任务主链，避免把文件本体和会话附件关系混成一层。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 冻结 `cubebox_files / cubebox_file_links` 的元数据语义与引用关系。
2. [ ] 明确文件本体存储适配器、开发环境默认实现与生产可替换策略。
3. [ ] 实现文件删除、引用保护、扫描状态、会话关联的正式规则。

### 2.2 非目标

1. [ ] 不在本文实现 File Search / RAG / 向量检索。
2. [ ] 不在本文设计前端文件页 IA。
3. [ ] 不在本文承接 Prompt、Memory、Search 等历史 Mongo 模型迁移。

## 3. 关键边界

1. [ ] PostgreSQL 只存元数据与引用关系，不存大文件正文。
2. [ ] 文件删除必须遵循引用一致性与租户隔离，不允许跨租户或误删已引用资源。
3. [ ] 存储实现必须可替换，但开发环境默认仍需有本仓一方实现。

## 4. 实施步骤

1. [ ] 冻结文件元数据 schema、link schema、删除规则与扫描状态契约。
2. [ ] 实现文件 repository、storage adapter 与 service 边界。
3. [ ] 补齐上传、列出、删除、引用一致性与租户隔离测试。

## 5. 验收与测试

1. [ ] 文件 repository / storage adapter 集成测试
2. [ ] 文件 API matrix 回归
3. [ ] 退役本地 `index.json` 作为正式 SoT 后的回归验证

## 6. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/380a-cubebox-postgresql-data-plane-and-migration-contract.md`
