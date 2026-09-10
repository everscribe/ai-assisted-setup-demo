.PHONY: up down verify check-clean dev-go

up:
	docker compose up --build -d

down:
	docker compose down

# Plan 2 replaces this with the conformance harness.
verify:
	@echo "conformance harness lands in plan 2"

dev-go:
	cd apps/go/web && pnpm install && pnpm run build
	cd apps/go && PORT=9001 go run .
