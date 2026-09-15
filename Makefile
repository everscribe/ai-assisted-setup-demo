.PHONY: up down verify test-node check-clean dev-go dev-node dev-rust dev-python

up:
	docker compose up --build -d

down:
	docker compose down

# Readiness polls every host port docker-compose.yml declares, derived
# from the file rather than hardcoded, so plans 4 and 5 adding the node,
# rust and python ports need no edit here.
#
# The same derivation gives the harness the number of apps it should
# find, as CONFORMANCE_EXPECT_APPS. Without it the harness only fails
# when zero apps answer, so an app the roster lists but compose never
# starts is silently untested and verify still exits 0. Set the variable
# in the environment to override the derived count, which is how the
# guard itself is exercised.
#
# Teardown runs on every exit path, including a compose failure and a
# readiness timeout, so a failed verify never leaves containers up.
verify:
	@docker compose up --build -d; up=$$?; \
	if [ $$up -ne 0 ]; then \
	  echo "FAIL: docker compose up failed"; \
	  docker compose down; \
	  exit $$up; \
	fi; \
	ports="$$(grep -oE '\"[0-9]{4}:[0-9]{4}\"' docker-compose.yml | cut -d: -f1 | tr -d '\"')"; \
	echo "waiting for apps to answer /api/health on: $$ports"; \
	ready=0; \
	for i in $$(seq 1 60); do \
	  allup=1; \
	  for p in $$ports; do \
	    curl -fsS localhost:$$p/api/health >/dev/null 2>&1 || allup=0; \
	  done; \
	  if [ $$allup -eq 1 ]; then ready=1; break; fi; \
	  sleep 1; \
	done; \
	if [ $$ready -ne 1 ]; then \
	  echo "FAIL: not every app answered /api/health within 60s (ports: $$ports)"; \
	  docker compose logs --tail 50; \
	  docker compose down; \
	  exit 1; \
	fi; \
	expect="$${CONFORMANCE_EXPECT_APPS:-$$(echo "$$ports" | wc -w | tr -d ' ')}"; \
	echo "expecting $$expect app(s) to answer the conformance suite"; \
	( cd contract/conformance && CONFORMANCE_REQUIRE_APPS=1 CONFORMANCE_EXPECT_APPS=$$expect \
	  go test ./... -count=1 -v ); status=$$?; \
	docker compose down; \
	exit $$status

# apps/node's unit tests. CI's conformance job runs this, so they are no
# longer a suite that only runs when someone remembers it exists. It is
# not folded into `verify`: verify needs docker and nothing else, and a
# pnpm install on that path would make the acceptance command depend on a
# node toolchain being present.
test-node:
	cd apps/node && pnpm install --frozen-lockfile && pnpm test

dev-go:
	cd apps/go/web && pnpm install --frozen-lockfile && pnpm run build
	cd apps/go && PORT=9001 go run .

dev-node:
	cd apps/node/web && pnpm install --frozen-lockfile && pnpm run build
	cd apps/node && pnpm install --frozen-lockfile && PORT=9002 pnpm run dev

dev-rust:
	cd apps/rust/web && pnpm install --frozen-lockfile && pnpm run build
	cd apps/rust && PORT=9003 cargo run --release

# The venv step is this app's `pnpm install --frozen-lockfile`: creating
# a venv that already exists is a no-op, so this is safe to re-run.
dev-python:
	cd apps/python/web && pnpm install --frozen-lockfile && pnpm run build
	cd apps/python && python3 -m venv .venv && .venv/bin/pip install -q -r requirements.txt
	cd apps/python && PORT=9004 .venv/bin/python -m app.main

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
