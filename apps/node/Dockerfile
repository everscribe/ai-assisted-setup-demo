# syntax=docker/dockerfile:1

# Stage 1: build the SPA. The server serves web/dist as static files,
# so the frontend has to be built before the server runs.
FROM node:22-alpine AS web
WORKDIR /app/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

# Stage 2: build the server. tsc compiles src/ to dist/.
FROM node:22-alpine AS build
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY tsconfig.json ./
COPY src ./src
RUN pnpm run build

# Stage 3: thin runtime. Production deps only, plus the compiled
# server and the built SPA it serves.
FROM node:22-alpine AS runtime
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile --prod
COPY --from=build /app/dist ./dist
COPY --from=web /app/web/dist ./web/dist
ENV PORT=9002
EXPOSE 9002
CMD ["node", "dist/main.js"]
