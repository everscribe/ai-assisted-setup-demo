# syntax=docker/dockerfile:1

# Stage 1: build the SPA. The server serves web/dist through StaticFiles,
# so the frontend has to be built before the server runs.
FROM node:22-alpine AS web
WORKDIR /app/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

# Stage 2: install the runtime dependencies into a virtualenv. A venv is
# one directory stage 3 can copy whole, which a --user or system install
# is not. requirements.txt is copied before app/ so editing a handler
# does not invalidate this layer.
FROM python:3.12-slim AS deps
ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
COPY requirements.txt ./
RUN pip install -r requirements.txt

# Stage 3: runtime. The same slim base, carrying the venv, the app and
# the built SPA. Not the distroless/static base apps/go and apps/rust
# end on: CPython is not a static binary, so the runtime needs the
# interpreter and its shared libraries whatever else it drops.
FROM python:3.12-slim
WORKDIR /app
COPY --from=deps /opt/venv /opt/venv
COPY app ./app
COPY --from=web /app/web/dist ./web/dist
ENV PATH="/opt/venv/bin:$PATH" \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PORT=9004
EXPOSE 9004
CMD ["python", "-m", "app.main"]
