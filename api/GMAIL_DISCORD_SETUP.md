# Gmail / Google / Discord Configuration Guide

This document explains where Gmail/SMTP, Google OAuth, and Discord OAuth are
handled inside the current `api` project, and what must be checked before
deployment.

## Quick Summary

- If by **Gmail** you mean **sending verification emails after registration**:
  the implementation lives in `api/internal/handlers/auth.go`, inside
  `sendVerificationEmail()`.
- If by **Gmail** you actually mean **Google account sign-in**:
  the implementation also lives in `api/internal/handlers/auth.go`, under the
  `google` OAuth branch.
- If by **dc** you mean **Discord**:
  the implementation is Discord OAuth, also handled in
  `api/internal/handlers/auth.go`.
- All related configuration values come from environment variables. The central
  config loader is `api/internal/core/config/config.go`.
- When started with Docker, the `api` service reads values from `api/.env`
  (see the root `docker-compose.yml`).
- Public callback / verification URLs are controlled through `API_BASE_URL`.

---

## Key Code Locations

### 1. Environment Variable Entry Point

File:

- `api/internal/core/config/config.go`

The current config loader supports these variables:

- `API_BASE_URL`
- `FRONTEND_URL`
- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `DISCORD_CLIENT_ID`
- `DISCORD_CLIENT_SECRET`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USER`
- `SMTP_PASS`
- `SMTP_FROM`

### 2. Auth Route Entry Points

File:

- `api/cmd/server/main.go`

Registered auth-related routes include:

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/auth/verify-email`
- `GET /api/v1/auth/oauth/:provider`
- `GET /api/v1/auth/oauth/:provider/callback`

Currently supported providers:

- `google`
- `discord`

### 3. Verification Email Sending Logic

File:

- `api/internal/handlers/auth.go`

Key function:

- `sendVerificationEmail(email, token string)`

Current behavior:

- If `SMTP_HOST` is empty, no email is sent; the verification link is logged
  to the server logs.
- If `SMTP_HOST` is set, the API sends a verification email through SMTP.
- The current implementation uses explicit SMTP negotiation with
  `STARTTLS/TLS + AUTH + DATA`, not the old bare `smtp.SendMail(...)` path.

### 4. Google / Discord OAuth Logic

File:

- `api/internal/handlers/auth.go`

Key functions:

- `OAuthRedirect`
- `OAuthCallback`
- `exchangeGoogleCode`
- `fetchGoogleUserInfo`
- `exchangeDiscordCode`
- `fetchDiscordUserInfo`

### 5. Frontend Auth Callback Page

File:

- `frontend/pages/auth/callback.vue`

Current behavior:

- After successful email verification, the backend redirects to the frontend
  callback page with `?code=...`, and the frontend exchanges that one-time code
  for a JWT.
- After successful Google / Discord OAuth, the backend redirects to the same
  frontend callback page.

---

## What You Need To Configure

### A. Configure Gmail / SMTP Email Sending

If your goal is **sending verification emails after registration**, the main
files involved are:

- `api/.env`
- `api/internal/core/config/config.go`
- `api/internal/handlers/auth.go`

Example configuration in `api/.env`:

```env
API_BASE_URL=http://localhost:8080
FRONTEND_URL=http://localhost:3000

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your_account@gmail.com
SMTP_PASS=your_gmail_app_password
SMTP_FROM=your_account@gmail.com
```

Notes:

- Gmail usually does **not** allow your normal mailbox password for SMTP.
  Use a Gmail **App Password** instead.
- The current code path is designed for **587 + STARTTLS/TLS**.
- This is only about **SMTP mail delivery**, not Google OAuth login.

### B. Configure Google Sign-In

If by “gmail” you actually mean **Google account sign-in**, the main files are:

- `api/.env`
- `api/internal/core/config/config.go`
- `api/internal/handlers/auth.go`

Example environment variables:

```env
API_BASE_URL=http://localhost:8080
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
FRONTEND_URL=http://localhost:3000
```

Current backend login entrypoint:

```text
GET /api/v1/auth/oauth/google
```

The Google OAuth callback must match the backend logic exactly:

- Local development: `http://localhost:8080/api/v1/auth/oauth/google/callback`
- Production: `<API_BASE_URL>/api/v1/auth/oauth/google/callback`

### C. Configure Discord Sign-In

If `dc` means **Discord**, the main files are:

- `api/.env`
- `api/internal/core/config/config.go`
- `api/internal/handlers/auth.go`

Example environment variables:

```env
API_BASE_URL=http://localhost:8080
DISCORD_CLIENT_ID=your_discord_client_id
DISCORD_CLIENT_SECRET=your_discord_client_secret
FRONTEND_URL=http://localhost:3000
```

Current backend login entrypoint:

```text
GET /api/v1/auth/oauth/discord
```

The Discord OAuth callback must match the backend logic exactly:

- Local development: `http://localhost:8080/api/v1/auth/oauth/discord/callback`
- Production: `<API_BASE_URL>/api/v1/auth/oauth/discord/callback`

---

## Recommended `api/.env` Block

`api/.env.example` may not list every variable you need. In practice, you
should define at least the following in `api/.env`:

```env
# API
API_BASE_URL=http://localhost:8080

# Frontend
FRONTEND_URL=http://localhost:3000

# Google OAuth
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# Discord OAuth
DISCORD_CLIENT_ID=
DISCORD_CLIENT_SECRET=

# SMTP / Gmail
SMTP_HOST=
SMTP_PORT=587
SMTP_USER=
SMTP_PASS=
SMTP_FROM=noreply@clawlink.app
```

---

## Current URL Configuration Model

The base URL for both:

- email verification links
- OAuth callback links

is now controlled by:

```env
API_BASE_URL=https://your-api-domain.com
```

Related code paths:

- `oauthCallbackURL()`
- `sendVerificationEmail()`

If your production API domain is not the default, changing `API_BASE_URL` in
`api/.env` is the correct fix.

---

## How To Verify It Locally

### Verify Gmail / SMTP

- Call `POST /api/v1/auth/register`
- If `SMTP_HOST` is empty, the server logs will print the verification link
- If `SMTP_HOST` is configured correctly, you should receive a real
  verification email

### Verify Google OAuth

- Open `GET /api/v1/auth/oauth/google`
- It should redirect to Google’s consent screen
- After completion, it should return to the frontend `/auth/callback`

### Verify Discord OAuth

- Open `GET /api/v1/auth/oauth/discord`
- It should redirect to Discord’s consent screen
- After completion, it should return to the frontend `/auth/callback`

---

## Recommended Order Of Work

1. Fill in `FRONTEND_URL`, SMTP, Google, and Discord variables in `api/.env`
2. Confirm your real frontend and API domains
3. Set `API_BASE_URL`
4. Register the matching callback URLs in Google Console and Discord Developer Portal
5. Test email verification, Google login, and Discord login separately

---

## One-Line Decision Guide

- **Send Gmail/SMTP email** → focus on `sendVerificationEmail()`
- **Google login** → focus on the `google` branch in `OAuthRedirect/OAuthCallback`
- **Discord login** → focus on the `discord` branch in `OAuthRedirect/OAuthCallback`
- **Central config loader** → `config.go`
- **Actual values to edit** → `api/.env`
