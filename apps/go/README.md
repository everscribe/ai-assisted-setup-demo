# apps/go

The Go app in the four-app demo: a Gin API backing a React SPA, both served
from a single binary. Deliberately uninstrumented, see the root README.

## Running it

```bash
make dev-go   # from the repo root: builds the SPA and runs the server
```

Serves on `:9001` (override with the `PORT` env var).
