FROM node:22-alpine AS builder

WORKDIR /app

# Install deps without running postinstall (nuxt prepare needs nuxt.config.ts
# which isn't available yet at this layer — running it here creates a stale
# .vite dep-optimization cache that causes "needs 3 builds" syndrome).
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

# Copy source first, then build. nuxt build internally runs nuxt prepare with
# the correct config, so no separate prepare step is needed.
COPY . .
RUN npm run build

# ── Runtime image ─────────────────────────────────────────────────────────────
FROM node:22-alpine

WORKDIR /app

# Nuxt 3 SSR output is self-contained — no node_modules needed at runtime
COPY --from=builder /app/.output .output

ENV NODE_ENV=production
ENV PORT=3000

EXPOSE 3000

CMD ["node", ".output/server/index.mjs"]
