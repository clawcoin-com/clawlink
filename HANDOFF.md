# ClawLink — Session Handoff

> Paste or reference this file at the start of the next session so Sisyphus can
> pick up without re-reading all the chat history.
>
> **Authoritative rule set lives in `docs/final-rules.md`. Read that first.**

---

## Repo layout

- Main repo: `D:\work\clawcoin-com\clawlink` (Go API + Nuxt frontend + docs + deploy)
- Sibling CLI repo: `D:\work\clawcoin-com\clcli` (official CLI client)

Both must stay aligned with `docs/final-rules.md`.

---

## Current state (as of this handoff)

### Done and verified
- Auth model finalized: `register-agent` accepts only `username+password` OR
  `wallet+signature`; `email` field is rejected; email-verify gate applies
  only to non-agent accounts.
- Login returns JWT via one-time code exchange (`/auth/exchange`).
- OAuth auto-link only on provider-verified email.
- `clcli` is a full client (not pure agent). `auth register-agent`,
  `auth login`, `wallet *`, `agent *`, `submolt list`, `config`, `version`
  are wired; `agent reply` automatically uses queue; `--force` retries once
  on `INVALID_TOKEN`.
- Heartbeat returns `recent_notifications` with actor_username / display_name
  / message / type.
- SMTP: explicit STARTTLS + AUTH + DATA; multipart text/html emails with
  brand logo and CTA button.
- Public skill docs exposed at `/skill.md` and `/api/v1/skill/docs`; skillDoc
  rewrites URLs dynamically with `www.clawlink.net` fallback.
- Branding: emoji logos removed; image logo used everywhere; favicons /
  apple-touch-icon / manifest wired.
- `clcli config init` writes mainnet defaults by default; testnet is opt-in.
- Git author rewrite script at `scripts/rewrite-git-author.ps1`.
- Release pipeline: `release.yml` builds multi-platform binaries;
  `publish-npm.yml` is separate and triggers on `workflow_run: [release]
  completed` with tag `v*`. `npm install -g npm@latest` has been removed.
- `docker-compose.yml` no longer has obsolete `version:` field.
- Frontend Dockerfile copies `/public` so static files (skill.md, icons,
  manifest) are served at runtime.
- Deprecated direct-reply endpoint **removed**:
  - Route `sk.POST("/posts/:id/reply", ...)` deleted from `api/cmd/server/main.go`.
  - `Reply()` handler deleted from `api/internal/skill/handler.go`.
  - skillDoc no longer mentions it in endpoints table, step text, or best
    practices.
  - `readme.md`, `docs/feature-summary.md`, `frontend/public/skill.md`,
    `frontend/pages/docs.vue` all cleaned.
  - `clcli` `--force` retry logic now only checks `INVALID_TOKEN`; the
    `QUEUE_REQUIRED` branch has been removed.

### Not yet verified in this handoff (but code is already changed)
- API container has NOT been rebuilt `--no-cache` after the latest skillDoc
  / route cleanup. Live `/api/v1/skill/docs` may still serve the cached
  older doc until a no-cache rebuild.
- `clcli` binary in `D:\work\clawcoin-com\clcli\build\clcli.exe` has NOT been
  rebuilt after the `--force` dead-code cleanup.
- No final regression grep has been run to prove the whole repo is free of
  `Deprecated` / `QUEUE_REQUIRED` mentions and residual `posts/:id/reply` as
  a documented valid endpoint.

---

## Immediate next steps (copy-paste runnable)

### 1. Rebuild API with no-cache, restart container
```powershell
cd D:\work\clawcoin-com\clawlink
docker-compose build --no-cache api
docker-compose up -d --force-recreate api
```

### 2. Verify deprecated route returns 404, not 409
```powershell
curl.exe -i -s -X POST http://localhost:8080/api/v1/skill/posts/any/reply `
  -H "X-API-Key: does-not-matter" `
  -H "Content-Type: application/json" `
  -d '{"content":"should 404 now"}'
```
Expect: `HTTP 404`. If it returns `409 QUEUE_REQUIRED`, the old image is
still cached. Rerun step 1 with `--no-cache`.

### 3. Verify live skillDoc is queue-only, zero deprecated language
```powershell
$resp = curl.exe -s http://localhost:8080/api/v1/skill/docs
$resp | Select-String -Pattern 'Deprecated|QUEUE_REQUIRED|Direct reply|posts/:id/reply'
# expected: NO matches
```

### 4. Full-repo regression grep
```powershell
cd D:\work\clawcoin-com\clawlink
Select-String -Path @("readme.md","docs/*.md","frontend/public/skill.md","frontend/pages/docs.vue","api/internal/skill/handler.go","api/cmd/server/main.go") -Pattern 'Deprecated|QUEUE_REQUIRED|/skill/posts/:id/reply'
# expected: NO matches (direct-reply endpoint is fully gone)
```

### 5. Rebuild clcli
```powershell
cd D:\work\clawcoin-com\clcli
if (Test-Path 'build/clcli.exe') { Remove-Item 'build/clcli.exe' -Force }
go build -o build/clcli.exe ./cmd/clcli
./build/clcli.exe agent reply --help
# expected: --force flag is still listed; Short text still "via ordered queue"
```

### 6. Optional — full loop smoke test
Run A→post→B→queue reply→A heartbeat and confirm `recent_notifications[0]`
contains B's username. Procedure already documented in the last successful
run on 2026-04-22 (see chat history of that date) — just replay it with
fresh usernames. Expected outcome matches `docs/final-rules.md` §4.

---

## Known traps for next session

- **Docker build cache can silently serve old skillDoc.** Always use
  `docker-compose build --no-cache api` when changing skillDoc text.
- **`clcli build/clcli.exe` can be stale.** If `agent post --help` misses a
  flag, rebuild first before suspecting source code.
- **Nuxt runtimeConfig env names.** Only `NUXT_API_BASE` and
  `NUXT_PUBLIC_API_BASE` are valid. `NUXT_PUBLIC_API_URL` is silently
  ignored by Nuxt and falls back to the built-in default.
- **`register-agent` must reject email.** If it ever accepts `{email, password}`
  again, that is a regression.

---

## Agent hygiene for the next session

- Read short slices with `Read_tool` using `offset` + `limit`; avoid
  full-file reads.
- `Grep_tool` with `head_limit` and narrow patterns; avoid `output_mode=content`
  on broad queries.
- When delegating with `Task`, demand a PASS/FAIL + exact file paths only,
  not pasted source.
- When asked "what did we do so far", refer to this file and
  `docs/final-rules.md` instead of re-listing history.
- Prefer doing one unit of work, confirming, then the next — not batching
  ten changes into one message.

---

## How to continue in a fresh session

Open the new chat and say:

> Read `docs/final-rules.md` and `HANDOFF.md`. Then run immediate next
> steps 1–5 and report PASS/FAIL with one-line evidence each.

That is enough context for Sisyphus to resume cleanly.
