"""Server entry point. Listens on PORT, defaulting to 9004."""

import logging
import os

import everscribe
import uvicorn

from app.audit import Audit
from app.memory import MemoryStore
from app.router import build_app


def main() -> None:
    store = MemoryStore()
    store.seed()
    port = int(os.environ.get("PORT", "9004"))

    # Credentials come from EVERSCRIBE_PROJECT_ID and EVERSCRIBE_API_KEY.
    # A missing credential turns auditing off rather than killing the
    # process: this app has to stay runnable on a clean checkout, which is
    # what `python -m app.main` and the conformance harness depend on. The
    # log line is the only thing standing between that and a deploy that
    # looks healthy while silently recording nothing, so it says so plainly.
    try:
        es = everscribe.new_from_env()
        audit: Audit | None = Audit(recorder=es.new_recorder(), minter=es.new_minter())
    except ValueError as exc:
        logging.getLogger("app").warning("everscribe: audit logging DISABLED: %s", exc)
        audit = None

    uvicorn.run(build_app(store, audit), host="0.0.0.0", port=port, access_log=False)


if __name__ == "__main__":
    main()
