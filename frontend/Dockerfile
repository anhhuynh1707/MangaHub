# ── Stage 1: Build ────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

# .npmrc carries legacy-peer-deps so npm ci tolerates the openapi-typescript
# peer range lagging behind TypeScript 6.
COPY package*.json .npmrc ./
RUN npm ci

COPY . .

# VITE_API_URL is baked into the bundle at build time.
# Default: browser calls the backend directly on localhost:8080.
# Override with: docker build --build-arg VITE_API_URL=https://api.example.com
ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=$VITE_API_URL

RUN npm run build

# ── Stage 2: Serve ────────────────────────────────────────────────
FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
