# ── Stage 1: Build ────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

# .npmrc carries legacy-peer-deps so npm ci tolerates the openapi-typescript
# peer range lagging behind TypeScript 6.
COPY package*.json .npmrc ./
RUN npm ci

COPY . .

# VITE_API_URL is baked into the bundle. Production uses the edge proxy's /api
# path so SPA routes such as /manga/:id do not collide with backend routes.
# The local root Compose file explicitly overrides this with localhost:8080.
ARG VITE_API_URL=/api
ENV VITE_API_URL=$VITE_API_URL

RUN npm run build

# ── Stage 2: Serve ────────────────────────────────────────────────
FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
