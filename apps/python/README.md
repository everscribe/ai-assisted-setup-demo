# apps/python

The Python app in the four-app demo: a FastAPI API backing a Vue SPA.
Deliberately uninstrumented, see the root README.

## Running it

```bash
make dev-python   # from the repo root: builds and runs this app's container
```

Serves on `:9004` (override with the `PORT` env var).

Dependencies are pip and `requirements.txt`; `requirements-dev.txt` adds
pytest and httpx for `python -m pytest tests`.
