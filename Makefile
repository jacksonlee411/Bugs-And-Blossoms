SHELL := bash
.SHELLFLAGS := -euo pipefail -c

.DEFAULT_GOAL := help

export ATLAS_VERSION ?= v0.38.0
export DEV_COMPOSE_PROJECT ?= bugs-and-blossoms-dev
export DEV_INFRA_ENV_FILE ?= .env.example

.PHONY: help preflight check pr-branch naming no-legacy assistant-config-single-source assistant-domain-allowlist assistant-knowledge-single-source assistant-knowledge-runtime-load assistant-knowledge-no-json-runtime assistant-no-legacy-overlay assistant-no-knowledge-literals assistant-knowledge-no-archive-ref assistant-knowledge-contract-separation assistant-no-knowledge-db no-scope-package granularity ddd-layering-p0 ddd-layering-p2 org-node-key-backflow capability-key capability-contract capability-route-map capability-catalog policy-baseline-dup request-code as-of-explicit dict-tenant-only go-version error-message fmt lint test routing e2e doc tr generate css
.PHONY: sqlc-generate sqlc-verify-schema authz-pack authz-test authz-lint
.PHONY: plan migrate up
.PHONY: iam orgunit jobcatalog staffing person
.PHONY: dev dev-up dev-down dev-reset dev-ps dev-server dev-kratos-stub
.PHONY: coverage

help:
	@printf "%s\n" \
		"常用入口：" \
		"  make preflight" \
				"  make check naming" \
					"  make check no-legacy" \
					"  make check assistant-config-single-source" \
					"  make check assistant-domain-allowlist" \
					"  make check assistant-knowledge-single-source" \
					"  make check assistant-knowledge-runtime-load" \
					"  make check assistant-knowledge-no-json-runtime" \
					"  make check assistant-no-legacy-overlay" \
					"  make check assistant-no-knowledge-literals" \
					"  make check assistant-knowledge-no-archive-ref" \
					"  make check assistant-knowledge-contract-separation" \
					"  make check assistant-no-knowledge-db" \
					"  make check no-scope-package" \
					"  make check granularity" \
					"  make check ddd-layering-p0" \
					"  make check ddd-layering-p2" \
					"  make check org-node-key-backflow" \
				"  make check capability-key" \
				"  make check capability-contract" \
				"  make check capability-route-map" \
				"  make check capability-catalog" \
				"  make check policy-baseline-dup" \
			"  make check request-code" \
				"  make check as-of-explicit" \
				"  make check dict-tenant-only" \
			"  make check go-version" \
			"  make check error-message" \
			"  make check fmt" \
		"  make check lint" \
		"  make test" \
		"  make check routing" \
		"  make e2e" \
		"开发环境：" \
		"  make dev-up" \
		"  make dev-server" \
		"  make dev-superadmin" \
		"  make dev-down" \
		"  make dev-reset" \
		"  make dev-ps" \
	"" \
	"模块级（示例）：" \
		"  make iam plan" \
		"  make iam migrate up"

preflight: ## 本地一键对齐CI（严格版：含 UI build/typecheck）
	@$(MAKE) check pr-branch
	@$(MAKE) check naming
	@$(MAKE) check no-legacy
	@$(MAKE) check assistant-config-single-source
	@$(MAKE) check assistant-domain-allowlist
	@$(MAKE) check assistant-knowledge-single-source
	@$(MAKE) check assistant-knowledge-runtime-load
	@$(MAKE) check assistant-knowledge-no-json-runtime
	@$(MAKE) check assistant-no-legacy-overlay
	@$(MAKE) check assistant-no-knowledge-literals
	@$(MAKE) check assistant-knowledge-no-archive-ref
	@$(MAKE) check assistant-knowledge-contract-separation
	@$(MAKE) check assistant-no-knowledge-db
	@$(MAKE) check no-scope-package
	@$(MAKE) check granularity
	@$(MAKE) check ddd-layering-p0
	@$(MAKE) check ddd-layering-p2
	@$(MAKE) check org-node-key-backflow
	@$(MAKE) check capability-key
	@$(MAKE) check capability-contract
	@$(MAKE) check capability-route-map
	@$(MAKE) check capability-catalog
	@$(MAKE) check policy-baseline-dup
	@$(MAKE) check request-code
	@$(MAKE) check as-of-explicit
	@$(MAKE) check dict-tenant-only
	@$(MAKE) check go-version
	@$(MAKE) check error-message
	@$(MAKE) check doc
	@$(MAKE) check fmt
	@$(MAKE) check lint
	@$(MAKE) css
	@$(MAKE) test
	@$(MAKE) check routing
	@$(MAKE) e2e

check:
	@:

pr-branch: ## PR 固定分支门禁（只允许 wt-dev-main / wt-dev-a / wt-dev-b）
	@./scripts/ci/check-pr-fixed-branch.sh

naming: ## 命名去噪门禁（已取消：no-op）
	@./scripts/ci/check-no-version-marker.sh

no-legacy: ## 禁止 legacy 分支/回退通道（单链路原则）
	@./scripts/ci/check-no-legacy.sh

assistant-config-single-source: ## 助手配置单主源门禁（禁止第二写入口/契约回写/SSOT 漂移）
	@./scripts/ci/check-assistant-config-single-source.sh

assistant-domain-allowlist: ## 助手外域名白名单门禁（default deny + SSRF 风险域名阻断 + SSOT 接线）
	@./scripts/ci/check-assistant-domain-allowlist.sh

assistant-knowledge-single-source: ## Assistant 知识单主源门禁（仅允许 assistant_knowledge_md 为人工写入口）
	@./scripts/ci/check-assistant-knowledge-single-source.sh

assistant-knowledge-runtime-load: ## Assistant Markdown runtime load 门禁（front matter / refs / index fail-closed）
	@./scripts/ci/check-assistant-knowledge-runtime-load.sh

assistant-knowledge-no-json-runtime: ## Assistant JSON runtime 回流门禁（禁止 assistant_knowledge/*.json 与旧 loader/embed）
	@./scripts/ci/check-assistant-knowledge-no-json-runtime.sh

assistant-no-legacy-overlay: ## Assistant mixed-source / overlay 回流门禁
	@./scripts/ci/check-assistant-no-legacy-overlay.sh

assistant-no-knowledge-literals: ## Assistant prompt/runtime 邻近知识字面量回流门禁
	@./scripts/ci/check-assistant-no-knowledge-literals.sh

assistant-knowledge-no-archive-ref: ## Assistant Markdown source_refs 禁止 archive 引用
	@./scripts/ci/check-assistant-knowledge-no-archive-ref.sh

assistant-knowledge-contract-separation: ## Assistant knowledge / contract 强分离门禁
	@./scripts/ci/check-assistant-knowledge-contract-separation.sh

assistant-no-knowledge-db: ## Assistant knowledge/runtime 禁止 DB / vector / 外部知识平台依赖
	@./scripts/ci/check-assistant-no-knowledge-db.sh

no-scope-package: ## 反漂移门禁（阻断新增 scope/package 语义）
	@./scripts/ci/check-no-scope-package.sh

granularity: ## 颗粒度层次门禁（阻断 org_level/scope_type/scope_key 回流）
	@./scripts/ci/check-granularity.sh

ddd-layering-p0: ## DDD 分层 P0 反漂移门禁（阻断 internal/server 扩散与 infra->services 回流）
	@./scripts/ci/check-ddd-layering-p0.sh

ddd-layering-p2: ## DDD 分层 P2 组合根门禁（模块扩张时要求 module.go/links.go 承接职责）
	@./scripts/ci/check-ddd-layering-p2.sh

org-node-key-backflow: ## Org node key 切窗反回流门禁（阻断 org_id/org_node_key DTO 暴露、旧 resolver 与 legacy parent payload）
	@./scripts/ci/check-org-node-key-backflow.sh

capability-key: ## capability_key 命名与拼接门禁（防退化为 scope）
	@./scripts/ci/check-capability-key.sh

capability-contract: ## capability_key 契约冻结门禁（151 基线）
	@./scripts/ci/check-capability-contract.sh

capability-route-map: ## 路由动作到 capability_key 映射门禁（156 基线）
	@./scripts/ci/check-capability-route-map.sh

capability-catalog: ## capability catalog 一致性门禁（对象/意图目录）
	@./scripts/ci/check-capability-catalog.sh

policy-baseline-dup: ## baseline + intent override 冗余覆盖门禁
	@./scripts/ci/check-policy-baseline-dup.sh

request-code: ## 业务幂等字段命名收敛（统一 request_id；阻断 request_code 与 tracing 场景 request_id/X-Request-ID）
	@./scripts/ci/check-request-code.sh --full

as-of-explicit: ## 时间参数显式化门禁（禁止 as_of/effective_date 默认 today）
	@./scripts/ci/check-as-of-explicit.sh

dict-tenant-only: ## 字典链路租户本地化门禁（禁止 runtime global fallback）
	@./scripts/ci/check-dict-tenant-only.sh

go-version: ## Go 版本门禁（禁止 go.mod/.tool-versions 回退到非 1.26）
	@./scripts/ci/check-go-version.sh

error-message: ## 错误提示收敛门禁（禁止泛化失败文案直出）
	@./scripts/ci/check-error-message.sh

fmt: ## 格式化/格式检查（按项目能力渐进接入）
	@if [[ -f go.mod ]]; then \
		echo "[fmt] go fmt ./..."; \
		go fmt ./...; \
	else \
		echo "[fmt] no go.mod; no-op"; \
	fi

lint: ## 静态检查（按项目能力渐进接入）
	@if [[ -n "$(MODULE)" ]]; then \
		./scripts/db/lint.sh "$(MODULE)"; \
	elif [[ -f go.mod ]]; then \
		echo "[lint] go vet ./..."; \
		go vet ./...; \
		echo "[lint] go-cleanarch"; \
		./scripts/ci/cleanarch.sh; \
		echo "[lint] ddd-layering-p0"; \
		./scripts/ci/check-ddd-layering-p0.sh; \
		echo "[lint] ddd-layering-p2"; \
		./scripts/ci/check-ddd-layering-p2.sh; \
	else \
		echo "[lint] no go.mod; no-op"; \
	fi

test: ## 单元/集成测试
	@if [[ -f go.mod ]]; then \
		./scripts/ci/test.sh; \
	else \
		echo "[test] no go.mod; no-op"; \
	fi

dev: dev-up dev-server

dev-up:
	docker compose -p "$(DEV_COMPOSE_PROJECT)" --env-file "$(DEV_INFRA_ENV_FILE)" -f compose.dev.yml up -d

dev-down:
	docker compose -p "$(DEV_COMPOSE_PROJECT)" --env-file "$(DEV_INFRA_ENV_FILE)" -f compose.dev.yml down

dev-reset:
	docker compose -p "$(DEV_COMPOSE_PROJECT)" --env-file "$(DEV_INFRA_ENV_FILE)" -f compose.dev.yml down -v

dev-ps:
	docker compose -p "$(DEV_COMPOSE_PROJECT)" --env-file "$(DEV_INFRA_ENV_FILE)" -f compose.dev.yml ps

dev-server:
	@env_file="$(DEV_SERVER_ENV_FILE)"; \
	if [[ -z "$$env_file" ]]; then \
		if [[ -f ".env.local" ]]; then env_file=".env.local"; fi; \
		if [[ -z "$$env_file" && -f "env.local" ]]; then env_file="env.local"; fi; \
		if [[ -z "$$env_file" && -f ".env" ]]; then env_file=".env"; fi; \
		if [[ -z "$$env_file" && -f ".env.example" ]]; then env_file=".env.example"; fi; \
	fi; \
	if [[ -n "$$env_file" ]]; then \
		set -a; . "$$env_file"; set +a; \
	fi; \
	if [[ -n "$(DEV_SERVER_HTTP_ADDR)" ]]; then \
		export HTTP_ADDR="$(DEV_SERVER_HTTP_ADDR)"; \
	fi; \
	go run ./cmd/server

dev-kratos-stub:
	go run ./cmd/kratosstub

dev-superadmin:
	@env_file="$(DEV_SUPERADMIN_ENV_FILE)"; \
	if [[ -z "$$env_file" ]]; then \
		if [[ -f ".env.local" ]]; then env_file=".env.local"; fi; \
		if [[ -z "$$env_file" && -f "env.local" ]]; then env_file="env.local"; fi; \
		if [[ -z "$$env_file" && -f ".env" ]]; then env_file=".env"; fi; \
		if [[ -z "$$env_file" && -f ".env.example" ]]; then env_file=".env.example"; fi; \
	fi; \
	if [[ -n "$$env_file" ]]; then \
		set -a; . "$$env_file"; set +a; \
	fi; \
	if [[ -n "$(DEV_SUPERADMIN_HTTP_ADDR)" ]]; then \
		export SUPERADMIN_HTTP_ADDR="$(DEV_SUPERADMIN_HTTP_ADDR)"; \
	fi; \
	export SUPERADMIN_DATABASE_URL="$${SUPERADMIN_DATABASE_URL:-postgres://superadmin_runtime:$${DB_PASSWORD:-app}@$${DB_HOST:-127.0.0.1}:$${DB_PORT:-5438}/$${DB_NAME:-bugs_and_blossoms}?sslmode=$${DB_SSLMODE:-disable}}"; \
	export SUPERADMIN_BASIC_AUTH_USER="$${SUPERADMIN_BASIC_AUTH_USER:-admin}"; \
	export SUPERADMIN_BASIC_AUTH_PASS="$${SUPERADMIN_BASIC_AUTH_PASS:-admin}"; \
	go run ./cmd/superadmin

routing: ## 路由门禁（allowlist/entrypoint key 等）
	@./scripts/routing/check-allowlist.sh

e2e: ## E2E smoke（按项目能力渐进接入）
	@./scripts/e2e/run.sh

doc: ## 文档门禁（按项目能力渐进接入）
	@./scripts/doc/check.sh

coverage:
	@./scripts/ci/coverage.sh

tr: ## i18n（en/zh）门禁（按项目能力渐进接入）
	@echo "[tr] no-op (placeholder)"

generate: ## templ/生成物（按项目能力渐进接入）
	@echo "[generate] no-op (placeholder)"

css: ## UI Build（MUI；产物入仓 + go:embed）
	@./scripts/ui/build-web.sh

sqlc-generate:
	@./scripts/sqlc/generate.sh

sqlc-verify-schema:
	@./scripts/sqlc/verify-schema-consistency.sh

authz-pack:
	@./scripts/authz/pack.sh

authz-test:
	@./scripts/authz/test.sh

authz-lint:
	@./scripts/authz/lint.sh

iam:
	@:
orgunit:
	@:
jobcatalog:
	@:
staffing:
	@:
person:
	@:

MODULE := $(firstword $(filter-out preflight check fmt lint test routing e2e doc tr generate css sqlc-generate sqlc-verify-schema authz-pack authz-test authz-lint no-legacy assistant-config-single-source assistant-domain-allowlist assistant-knowledge-single-source assistant-knowledge-runtime-load assistant-knowledge-no-json-runtime assistant-no-legacy-overlay assistant-no-knowledge-literals assistant-knowledge-no-archive-ref assistant-knowledge-contract-separation assistant-no-knowledge-db no-scope-package granularity ddd-layering-p0 ddd-layering-p2 org-node-key-backflow capability-key capability-contract capability-route-map capability-catalog policy-baseline-dup request-code as-of-explicit dict-tenant-only go-version error-message plan migrate up dev dev-up dev-down dev-reset dev-ps dev-server,$(MAKECMDGOALS)))
MIGRATE_DIR := $(lastword $(filter up down,$(MAKECMDGOALS)))

plan:
	@if [[ -z "$(MODULE)" ]]; then \
		echo "用法：make <module> plan（例如：make iam plan）" >&2; \
		exit 2; \
	fi
	@./scripts/db/plan.sh "$(MODULE)"

migrate:
	@if [[ -z "$(MODULE)" ]]; then \
		echo "用法：make <module> migrate up|down（例如：make iam migrate up）" >&2; \
		exit 2; \
	fi
	@if [[ -z "$(MIGRATE_DIR)" ]]; then \
		echo "缺少方向：up 或 down（例如：make iam migrate up）" >&2; \
		exit 2; \
	fi
	@./scripts/db/migrate.sh "$(MODULE)" "$(MIGRATE_DIR)"

up:
	@:
