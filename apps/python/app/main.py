"""Server entry point. Listens on PORT, defaulting to 9004."""

import os

import uvicorn

from app.memory import MemoryStore
from app.router import build_app


def main() -> None:
    store = MemoryStore()
    store.seed()
    port = int(os.environ.get("PORT", "9004"))
    uvicorn.run(build_app(store), host="0.0.0.0", port=port, access_log=False)


if __name__ == "__main__":
    main()
