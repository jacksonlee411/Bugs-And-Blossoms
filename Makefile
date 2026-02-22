SHELL := bash
.SHELLFLAGS := -euo pipefail -c

.DEFAULT_GOAL := help

export ATLAS_VERSION ?= v0.38.0
export DEV_COMPOSE_PROJECT ?= bugs-and-blossoms-dev
export DEV_INFRA_ENV_FILE ?= .env.example

.PHONY: help preflight check pr-branch naming no-legacy request-code as-of-explicit dict-tenant-only fmt lint test routing e2e doc tr generate css
.PHONY: sqlc-generate authz-pack authz-test authz-lint
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
			"  make check request-code" \
			"  make check as-of-explicit" \
			"  make check dict-tenant-only" \
			"  make check fmt" \
		"  make check lint" \
		"  make test" \
		"  make check routing" \
		"  make e2e" \
	"" \
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
	@$(MAKE) check request-code
	@$(MAKE) check as-of-explicit
	@$(MAKE) check dict-tenant-only
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

request-code: ## 业务幂等字段命名收敛（统一 request_id；阻断 request_code 与 tracing 场景 request_id/X-Request-ID）
	@./scripts/ci/check-request-code.sh --full

as-of-explicit: ## 时间参数显式化门禁（禁止 as_of/effective_date 默认 today）
	@./scripts/ci/check-as-of-explicit.sh

dict-tenant-only: ## 字典链路租户本地化门禁（禁止 runtime global fallback）
	@./scripts/ci/check-dict-tenant-only.sh

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
	@env_file=""; \
	if [[ -f ".env.local" ]]; then env_file=".env.local"; fi; \
	if [[ -z "$$env_file" && -f "env.local" ]]; then env_file="env.local"; fi; \
	if [[ -z "$$env_file" && -f ".env" ]]; then env_file=".env"; fi; \
	if [[ -z "$$env_file" && -f ".env.example" ]]; then env_file=".env.example"; fi; \
	if [[ -n "$$env_file" ]]; then \
		set -a; . "$$env_file"; set +a; \
	fi; \
	go run ./cmd/server

dev-kratos-stub:
	go run ./cmd/kratosstub

dev-superadmin:
	@env_file=""; \
	if [[ -f ".env.local" ]]; then env_file=".env.local"; fi; \
	if [[ -z "$$env_file" && -f "env.local" ]]; then env_file="env.local"; fi; \
	if [[ -z "$$env_file" && -f ".env" ]]; then env_file=".env"; fi; \
	if [[ -z "$$env_file" && -f ".env.example" ]]; then env_file=".env.example"; fi; \
	if [[ -n "$$env_file" ]]; then \
		set -a; . "$$env_file"; set +a; \
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

MODULE := $(firstword $(filter-out preflight check fmt lint test routing e2e doc tr generate css sqlc-generate authz-pack authz-test authz-lint request-code as-of-explicit dict-tenant-only plan migrate up dev dev-up dev-down dev-reset dev-ps dev-server,$(MAKECMDGOALS)))
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
