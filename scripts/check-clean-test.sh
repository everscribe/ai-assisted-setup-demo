#!/usr/bin/env bash
# Proves make check-clean catches a violation in every scanned file type
# and passes on a clean tree. Run from the repo root.
set -u
MAKEFILE="$PWD/Makefile"
fails=0

run() { # name, setup-command
  D=$(mktemp -d)
  cp "$MAKEFILE" "$D/Makefile"
  ( cd "$D"
    mkdir -p apps/go/web/dist apps/node/web/src apps/python apps/rust
    printf 'module github.com/everscribe/ai-assisted-setup-demo/apps/go\n' > apps/go/go.mod
    printf '<html></html>\n' > apps/go/web/dist/index.html
    eval "$2"
    if make check-clean >/dev/null 2>&1; then echo "  MISSED  $1"; exit 1; fi
    echo "  caught  $1" )
  [ $? -ne 0 ] && fails=$((fails+1))
  rm -rf "$D"
}

run "go.mod"            'echo "require github.com/everscribe/sdk-go v0.1.0" >> apps/go/go.mod'
run "go.sum"            'echo "github.com/everscribe/sdk-go v0.1.0 h1:x" > apps/go/go.sum'
run "package.json"      'echo "{\"dependencies\":{\"@everscribe/sdk-node\":\"^0.1.0\"}}" > apps/node/package.json'
run "pnpm-lock.yaml"    'echo "  '\''@everscribe/sdk-node'\'':" > apps/node/pnpm-lock.yaml'
run "package-lock.json" 'echo "{\"packages\":{\"node_modules/@everscribe/components-react\":{}}}" > apps/go/web/package-lock.json'
run "yarn.lock"         'echo "\"@everscribe/components-element@^0.1.0\":" > apps/node/web/yarn.lock'
run "web/package.json"  'echo "{\"dependencies\":{\"@everscribe/components-element\":\"^0.1\"}}" > apps/node/web/package.json'
run "CDN tag in dist"   'echo "<script src=\"//cdn/@everscribe/components-element\"></script>" > apps/go/web/dist/index.html'
run "import in web/src" 'echo "import \"@everscribe/components-react\";" > apps/node/web/src/main.ts'
run "pyproject.toml"    'printf "[project]\ndependencies = [\n    \"everscribe>=0.1\",\n]\n" > apps/python/pyproject.toml'
run "uv.lock"           'printf "[[package]]\nname = \"everscribe\"\n" > apps/python/uv.lock'
run "Cargo.toml"        'printf "[dependencies]\neverscribe = \"0.1\"\n" > apps/rust/Cargo.toml'
run "Cargo.lock"        'printf "[[package]]\nname = \"everscribe\"\n" > apps/rust/Cargo.lock'

D=$(mktemp -d); cp "$MAKEFILE" "$D/Makefile"
( cd "$D"; mkdir -p apps/go
  printf 'module github.com/everscribe/ai-assisted-setup-demo/apps/go\n' > apps/go/go.mod
  make check-clean >/dev/null 2>&1 ) \
  && echo "  clean tree exits 0" \
  || { echo "  FALSE POSITIVE on a clean tree"; fails=$((fails+1)); }
rm -rf "$D"

[ "$fails" -eq 0 ] && echo "check-clean: all cases pass" || { echo "check-clean: $fails failures"; exit 1; }
