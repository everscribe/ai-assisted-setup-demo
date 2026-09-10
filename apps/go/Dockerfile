# syntax=docker/dockerfile:1

# Stage 1: build the SPA. The Go server embeds web/dist via go:embed,
# so the frontend has to be built before the Go binary.
FROM node:20-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: build the static Go server with the SPA embedded.
FROM golang:1.25-alpine AS build
WORKDIR /app
COPY . .
COPY --from=web /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /server .

# Stage 3: minimal runtime. The binary is a self-contained static build
# (embedded SPA, in-memory store), so distroless/static is enough.
FROM gcr.io/distroless/static-debian12
COPY --from=build /server /server
ENV PORT=9001
EXPOSE 9001
ENTRYPOINT ["/server"]
