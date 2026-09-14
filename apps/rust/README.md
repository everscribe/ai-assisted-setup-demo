# apps/rust

The Rust app in the four-app demo: an Axum API backing a vanilla
TypeScript SPA. Deliberately uninstrumented, see the root README.

## Running it

```bash
make dev-rust   # from the repo root: builds the SPA and runs the server
```

Serves on `:9003` (override with the `PORT` env var).
