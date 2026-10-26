# apps/node

The Node app in the four-app demo: an Express API backing a Svelte SPA.
Deliberately uninstrumented, see the root README.

## Running it

```bash
make dev-node   # from the repo root: builds and runs this app's container
```

Serves on `:9002` (override with the `PORT` env var).
