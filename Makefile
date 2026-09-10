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

check-clean:
	@files=""; \
	for f in apps/*/go.mod apps/*/go.sum \
	         apps/*/package.json apps/*/pnpm-lock.yaml apps/*/package-lock.json \
	         apps/*/web/package.json apps/*/web/pnpm-lock.yaml \
	         apps/*/web/package-lock.json apps/*/web/yarn.lock \
	         apps/*/Cargo.toml apps/*/Cargo.lock apps/*/pyproject.toml apps/*/uv.lock \
	         apps/*/requirements.txt apps/*/requirements-dev.txt \
	         apps/*/Pipfile apps/*/Pipfile.lock apps/*/poetry.lock \
	         apps/*/setup.py apps/*/setup.cfg \
	         apps/*/web/index.html apps/*/web/dist/index.html; do \
	  [ -f "$$f" ] && files="$$files $$f"; \
	done; \
	hits=""; [ -n "$$files" ] && hits=$$(for f in $$files; do \
	  sed 's|github\.com/everscribe/ai-assisted-setup-demo||g' "$$f" | grep -nE '@everscribe/(sdk-node|components-)|everscribe/sdk-go|"everscribe[^"]*"|^[[:space:]]*everscribe[[:space:]]*=|everscribe/|^[[:space:]]*everscribe[[:space:]]*[<>=!~]' | sed "s|^|$$f:|"; \
	done); \
	src=$$(grep -rnE '@everscribe/components-' apps/*/web/src 2>/dev/null); \
	if [ -n "$$hits$$src" ]; then \
	  echo "everscribe dependency on main:"; echo "$$hits"; echo "$$src"; exit 1; \
	fi
