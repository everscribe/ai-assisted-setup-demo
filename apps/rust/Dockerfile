# syntax=docker/dockerfile:1

# Stage 1: build the SPA. The server serves web/dist through ServeDir,
# so the frontend has to be built before the server runs.
FROM node:22-alpine AS web
WORKDIR /app/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

# Stage 2: build the server. The alpine image targets musl, whose
# crt-static default links the binary statically, so stage 3 can be the
# same distroless/static base apps/go uses. musl-dev supplies the libc
# the linker needs. The manifests are copied ahead of src/ against a
# placeholder main so the dependency build lands in its own layer and
# survives a source edit.
FROM rust:1.96-alpine AS build
RUN apk add --no-cache musl-dev
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo 'fn main() {}' > src/main.rs && \
    cargo build --release --locked && \
    rm -rf src
COPY src ./src
RUN touch src/main.rs && cargo build --release --locked

# Stage 3: minimal runtime. The binary is static, but unlike apps/go it
# reads the SPA from disk rather than embedding it, so web/dist ships
# alongside it and STATIC_DIR names where it landed.
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /app/target/release/ai-assisted-setup-demo-rust /app/server
COPY --from=web /app/web/dist /app/web/dist
ENV STATIC_DIR=/app/web/dist
ENV PORT=9003
EXPOSE 9003
ENTRYPOINT ["/app/server"]
