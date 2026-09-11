# apps/node

The Node app in the four-app demo: an Express API backing a Svelte SPA.
Deliberately uninstrumented, see the root README.

## Running it

```bash
make dev-node   # from the repo root: builds the SPA and runs the server
```

Serves on `:9002` (override with the `PORT` env var).
