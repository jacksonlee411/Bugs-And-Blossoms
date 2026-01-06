SHELL := bash
.SHELLFLAGS := -euo pipefail -c

.DEFAULT_GOAL := help

export ATLAS_VERSION ?= v0.38.0
export GOOSE_VERSION ?= v3.26.0
export SQLC_VERSION ?= v1.28.0
export GOIMPORTS_VERSION ?= v0.26.0

.PHONY: help preflight check naming fmt lint test routing e2e doc tr generate css
.PHONY: sqlc-generate authz-pack authz-test authz-lint
.PHONY: plan migrate up
.PHONY: iam orgunit jobcatalog staffing person
.PHONY: dev dev-up dev-down dev-server
.PHONY: coverage

help:
	@printf "%s\n" \
		"常用入口：" \
		"  make preflight" \
		"  make check naming" \
		"  make check fmt" \
		"  make check lint" \
		"  make test" \
		"  make check routing" \
		"  make e2e" \
		"" \
		"开发环境：" \
		"  make dev-up" \
		"  make dev-server" \
		"  make dev-down" \
		"" \
		"模块级（示例）：" \
		"  make iam plan" \
		"  make iam migrate up"

preflight: ## 本地一键对齐CI
	@$(MAKE) check naming
	@$(MAKE) check doc
	@$(MAKE) check fmt
	@$(MAKE) check lint
	@$(MAKE) test
	@$(MAKE) check routing
	@$(MAKE) e2e

check:
	@:

naming: ## 命名去噪门禁（禁止版本标记再次进入仓库）
	@./scripts/ci/check-no-version-marker.sh

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
	@env_file=".env"; \
	if [[ -f ".env.local" ]]; then env_file=".env.local"; fi; \
	if [[ -f "env.local" ]]; then env_file="env.local"; fi; \
	docker compose --env-file "$$env_file" -f compose.dev.yml up -d

dev-down:
	@env_file=".env"; \
	if [[ -f ".env.local" ]]; then env_file=".env.local"; fi; \
	if [[ -f "env.local" ]]; then env_file="env.local"; fi; \
	docker compose --env-file "$$env_file" -f compose.dev.yml down -v

dev-server:
	@env_file=""; \
	if [[ -f ".env.local" ]]; then env_file=".env.local"; fi; \
	if [[ -z "$$env_file" && -f "env.local" ]]; then env_file="env.local"; fi; \
	if [[ -z "$$env_file" && -f ".env" ]]; then env_file=".env"; fi; \
	if [[ -n "$$env_file" ]]; then \
		set -a; . "$$env_file"; set +a; \
	fi; \
	go run ./cmd/server

routing: ## 路由门禁（allowlist/entrypoint key 等）
	@./scripts/routing/check-allowlist.sh

e2e: ## E2E smoke（按项目能力渐进接入）
	@if [[ -d apps/web ]]; then \
		echo "[e2e] apps/web exists; no-op (placeholder)"; \
	else \
		echo "[e2e] no apps/web; no-op"; \
	fi

doc: ## 文档门禁（按项目能力渐进接入）
	@./scripts/doc/check.sh

coverage:
	@./scripts/ci/coverage.sh

tr: ## i18n（en/zh）门禁（按项目能力渐进接入）
	@echo "[tr] no-op (placeholder)"

generate: ## templ/生成物（按项目能力渐进接入）
	@echo "[generate] no-op (placeholder)"

css: ## Tailwind/Astro CSS（按项目能力渐进接入）
	@echo "[css] no-op (placeholder)"

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

MODULE := $(firstword $(filter-out preflight check fmt lint test routing e2e doc tr generate css sqlc-generate authz-pack authz-test authz-lint plan migrate up dev dev-up dev-down dev-server,$(MAKECMDGOALS)))
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
