# DEV-PLAN-291 R4：Runtime 版本锁一致性检查

- 执行时间：2026-03-08T18:12:21Z
- 输入锁文件：`deploy/librechat/versions.lock.yaml`
- 比对源：`docker compose -p bugs-and-blossoms-librechat --env-file deploy/librechat/.env -f deploy/librechat/docker-compose.upstream.yaml -f deploy/librechat/docker-compose.overlay.yaml config --format json`
- 结论：通过

| 服务 | compose image | lock tag | lock digest | 结论 |
| --- | --- | --- | --- | --- |
| `api` | `ghcr.io/danny-avila/librechat:v0.8.0` | `v0.8.0` | `sha256:1111111111111111111111111111111111111111111111111111111111111111` | `pass` |
| `mongodb` | `mongo:7.0` | `7.0` | `sha256:2222222222222222222222222222222222222222222222222222222222222222` | `pass` |
| `meilisearch` | `getmeili/meilisearch:v1.12.0` | `v1.12.0` | `sha256:3333333333333333333333333333333333333333333333333333333333333333` | `pass` |
| `rag_api` | `ghcr.io/danny-avila/librechat-rag-api-dev-lite@sha256:201958505e21a1334234df6538713bac204b10d98a72d239b5318ce11f40f20b` | `latest` | `sha256:201958505e21a1334234df6538713bac204b10d98a72d239b5318ce11f40f20b` | `pass` |
| `vectordb` | `qdrant/qdrant:v1.13.4` | `v1.13.4` | `sha256:5555555555555555555555555555555555555555555555555555555555555555` | `pass` |
