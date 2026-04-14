# DEV-PLAN-380F：LibreChat vendored/runtime/deploy 资产退役与收口

**状态**: 草拟中（2026-04-14 20:52 CST）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `LibreChat` vendored Web UI、runtime、部署链与相关历史资产退役的实施 SSOT。  
> `380` 负责总裁决；本文只负责仓库中旧资产、旧构建链、旧 runbook 与相关目录的退役批次。

## 1. 背景与定位

1. [ ] 当前正式 UI 已切到 `CubeBox`，但仓库内仍保留 `third_party/librechat-web`、`deploy/librechat/*`、`scripts/librechat/*` 等历史资产。
2. [ ] 只退役入口不退役资产，会继续造成文档、构建链与运行口径漂移。
3. [ ] 需要把“LibreChat 不是正式运行基线”落实到仓库目录、构建链、脚本和文档层。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 退役 `third_party/librechat-web` 的正式构建/运行责任。
2. [ ] 退役 `deploy/librechat/*`、`scripts/librechat/*` 与相关构建脚本、README、runbook。
3. [ ] 清理仓库中仍宣称 LibreChat 为正式基线的文档与状态说明。

### 2.2 非目标

1. [ ] 不在本文设计 `CubeBox` 新数据面或 API。
2. [ ] 不在本文实现 `apps/web` 页面逻辑。
3. [ ] 不在本文处理 `370` 知识 runtime contract。

## 3. 关键边界

1. [ ] 迁移期可保留历史证据或归档记录，但不得继续让旧资产承担正式运行责任。
2. [ ] 退役批次必须与文档地图、README、脚本入口保持一致。
3. [ ] 不允许用“保留但不声明”方式长期搁置旧 runtime 资产。

## 4. 实施步骤

1. [ ] 盘点旧 LibreChat 目录、构建脚本、部署说明与引用位置。
2. [ ] 逐批删除或归档旧资产，并更新文档/脚本/构建链。
3. [ ] 为退役结果补齐文档、测试或断言，证明仓库不再以 LibreChat 为正式基线。

## 5. 验收与测试

1. [ ] `make check doc`
2. [ ] 旧构建链/旧部署脚本不再被正式入口引用
3. [ ] 退役后仓库说明、路线图与 runtime-status 文档收口完成

## 6. 关联事实源

1. `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
2. `docs/dev-plans/013-docs-creation-and-governance-guide.md`
