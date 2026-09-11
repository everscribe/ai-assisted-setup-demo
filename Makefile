.PHONY: up down verify check-clean dev-go

up:
	docker compose up --build -d

down:
	docker compose down

verify:
	docker compose up --build -d
	@echo "waiting for apps to answer /api/health"
	@for i in $$(seq 1 30); do \
	  curl -fsS localhost:9001/api/health >/dev/null 2>&1 && break; \
	  sleep 1; \
	done
	@cd contract/conformance && go test ./... -count=1 -v; status=$$?; \
	 cd ../.. && docker compose down; \
	 exit $$status

dev-go:
	cd apps/go/web && pnpm install --frozen-lockfile && pnpm run build
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
	  LC_ALL=C sed 's|github\.com/everscribe/ai-assisted-setup-demo||g' "$$f" | LC_ALL=C grep -naE '@everscribe/(sdk-node|components-)|everscribe/sdk-go|"everscribe[^"]*"|^[[:space:]]*everscribe[[:space:]]*=|everscribe/|^[[:space:]]*everscribe[[:space:]]*[<>=!~]' | LC_ALL=C sed "s|^|$$f:|"; \
	done); \
	src=$$(LC_ALL=C grep -ranE '@everscribe/components-' apps/*/web/src 2>/dev/null); \
	if [ -n "$$hits$$src" ]; then \
	  echo "everscribe dependency on main:"; echo "$$hits"; echo "$$src"; exit 1; \
	fi
