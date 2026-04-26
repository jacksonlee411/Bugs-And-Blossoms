---
name: bugs-and-blossoms-dev-login
description: Start and verify the Bugs-And-Blossoms local development runtime, including Postgres/Redis, migrations, authz policy packing, KratosStub login seeding, the Go server, and CubeBox model configuration. Use this when the user asks to start the project locally, prepare a browser-testable dev environment, seed login identities, or validate CubeBox startup.
---

# Bugs-And-Blossoms Dev Runtime

## What This Skill Does

Use this skill for local startup and browser verification of this repository. The canonical script is:

```bash
tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh
```

It performs the local dev startup sequence:

1. Starts Docker infra with `make dev-up`.
2. Waits for Postgres.
3. Runs `iam` and `orgunit` migrations with the admin DB user.
4. Runs `make authz-pack`.
5. Starts `make dev-kratos-stub`.
6. Starts `make dev-server`.
7. Seeds default KratosStub identities.
8. Logs in as the local tenant admin.
9. Optionally initializes CubeBox model settings to the current project baseline.

Default app URL: `http://localhost:8080/app`

Default login:

```text
email: admin@localhost
password: admin123
tenant: localhost / 00000000-0000-0000-0000-000000000001
```

## Environment Rule

Keep real secrets in `.env.local` or exported shell environment. Do not put real keys in tracked files.

Runtime logs and pid files are written under `.local/runtime/` by default. Override with `DEV_RUNTIME_DIR` only for local debugging.

For CubeBox, the DB stores `secret_ref=env://OPENAI_API_KEY`. The server resolves that at runtime through its environment. Removing a duplicate key from `.env` does not break CubeBox as long as `OPENAI_API_KEY` remains in `.env.local` or is exported when `make dev-server` starts.

The startup script never prints, moves, or deletes secrets. It only warns if `OPENAI_API_KEY` appears duplicated between `.env` and `.env.local`.

## Common Commands

Start the default dev runtime:

```bash
tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh
```

Start and rebuild embedded web assets first:

```bash
tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh --build-ui
```

Start and verify the active CubeBox provider after model setup:

```bash
tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh --verify-cubebox
```

Stop locally started Go processes:

```bash
tools/codex/skills/bugs-and-blossoms-dev-login/scripts/stop_dev_runtime.sh
```

## Notes

- `make dev-server` loads `.env.local`, then `env.local`, then `.env`, then `.env.example`, choosing the first existing file unless `DEV_SERVER_ENV_FILE` is set.
- `make dev-up` uses `DEV_INFRA_ENV_FILE`, defaulting to `.env.example`.
- If DB roles are missing because an old Docker volume predates `scripts/dev/postgres-init`, run `make dev-reset` manually, then start again.
- The current CubeBox baseline is `provider_id=openai-compatible`, `provider_type=codex`, `base_url=https://code2026.pumpkinai.vip/v1`, `secret_ref=env://OPENAI_API_KEY`, `model_slug=gpt-5.2`.
