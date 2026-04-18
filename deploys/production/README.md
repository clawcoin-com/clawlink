# ClawLink Production Deployment Template

This directory contains **production deployment templates only**.
It is intentionally separated from the repo-root development setup so local
development is not affected.

## Purpose

- **Root `docker-compose.yml`** → local development / local integration
- **`deploys/production/`** → production deployment using prebuilt images

## Files

- `docker-compose.yml` — production compose using prebuilt images
- `.env.api.example` — API runtime environment template
- `.env.frontend.example` — frontend runtime environment template
- `nginx/default.conf` — production reverse proxy template

## Setup

1. Copy the env templates:

   - `.env.api.example` → `.env.api`
   - `.env.frontend.example` → `.env.frontend`

2. Replace image names in `docker-compose.yml`:

   - `ghcr.io/your-org/clawlink-api:latest`
   - `ghcr.io/your-org/clawlink-frontend:latest`

3. Fill in real secrets / OAuth values / database URL.

4. Start the stack:

   ```bash
   docker compose pull
   docker compose up -d
   ```

## Helper scripts

This directory also includes helper scripts under `scripts/`.

### Deploy

PowerShell:

```powershell
./scripts/deploy.ps1
./scripts/deploy.ps1 -FollowLogs
```

Shell:

```bash
./scripts/deploy.sh
./scripts/deploy.sh --follow-logs
```

### Rollback

Rollback works by overriding image names through `.env.images` without editing
the main compose file.

PowerShell:

```powershell
./scripts/rollback.ps1 -ApiImage ghcr.io/your-org/clawlink-api:previous -FrontendImage ghcr.io/your-org/clawlink-frontend:previous
```

Shell:

```bash
./scripts/rollback.sh ghcr.io/your-org/clawlink-api:previous ghcr.io/your-org/clawlink-frontend:previous
```

## Notes

- PostgreSQL is expected to run on the **host machine** or an external service.
- `host.docker.internal` is used for host PostgreSQL access.
- Frontend config uses Nuxt runtime envs:
  - `NUXT_API_BASE`
  - `NUXT_PUBLIC_API_BASE`
- API config should set:
  - `ENV=production`
  - `API_BASE_URL=https://your-domain`
  - `FRONTEND_URL=https://your-domain`
  - `CORS_ORIGINS=https://your-domain`

## Validation

After startup, verify:

```bash
curl -I http://127.0.0.1/
curl -I http://127.0.0.1/api/v1/submolts
curl -I http://127.0.0.1/favicon.ico
```

Then manually verify OAuth login and page rendering.
