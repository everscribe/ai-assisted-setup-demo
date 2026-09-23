"""The app factory.

Registers exactly the eighteen operations contract/openapi.yaml declares, and
turns off the framework defaults that would break it:

1. No pydantic body models, so the role check runs before body parsing.
2. Starlette's {"detail": "Not Found"} becomes the contract's Error schema.
3. Same for the 405 body, whose Allow header lists every method registered for
   the path rather than only the first matching route's.
4. redirect_slashes is off, so /api/me/ is a 404 rather than a 307.

Two more contract rules come from framework behaviour this file relies on
rather than overrides, so tests/test_router.py pins each instead:

5. HEAD is not routed to GET, because FastAPI's add_api_route registers exactly
   the methods it is given. Starlette's own Route adds HEAD wherever GET
   appears; APIRoute does not.
6. A malformed body is the handler's own 400 rather than FastAPI's 422, because
   no route binds a request model, so RequestValidationError never fires.

Routing matches the raw, undecoded path: /api/%6de is a 404, not Ada's record.
Captured path parameters are still decoded, in handlers._path_id.
"""

from http import HTTPStatus
from pathlib import Path

from everscribe.asgi import EverscribeMiddleware
from fastapi import FastAPI
from starlette.exceptions import HTTPException as StarletteHTTPException
from starlette.requests import Request
from starlette.responses import Response
from starlette.staticfiles import StaticFiles
from starlette.types import Receive, Scope, Send

from app import handlers
from app.audit import Audit, make_embed_token, make_resolve_actor
from app.handlers import JSON, fail
from app.store import Store

API_PREFIX = "/api"
DIST = Path(__file__).resolve().parent.parent / "web" / "dist"

# path, methods, handler. Eighteen operations over fourteen paths.
ROUTES = [
    ("/api/login", ["POST"], handlers.login),
    ("/api/logout", ["POST"], handlers.logout),
    ("/api/me", ["GET"], handlers.me),
    ("/api/members", ["GET"], handlers.list_members),
    ("/api/members", ["POST"], handlers.invite_member),
    ("/api/members/{id}", ["DELETE"], handlers.remove_member),
    ("/api/members/{id}/role", ["PUT"], handlers.change_member_role),
    ("/api/resources", ["GET"], handlers.list_resources),
    ("/api/resources", ["POST"], handlers.register_resource),
    ("/api/resources/{id}/decommission", ["POST"], handlers.decommission_resource),
    ("/api/resources/{id}/requests", ["GET"], handlers.list_resource_requests),
    ("/api/resources/{id}/requests", ["POST"], handlers.request_access),
    ("/api/requests/{id}/approve", ["POST"], handlers.approve_request),
    ("/api/requests/{id}/deny", ["POST"], handlers.deny_request),
    ("/api/requests/{id}/revoke", ["POST"], handlers.revoke_request),
    ("/api/requests/{id}/comments", ["GET"], handlers.list_request_comments),
    ("/api/requests/{id}/comments", ["POST"], handlers.add_comment),
    ("/api/health", ["GET"], handlers.health),
]

_MESSAGES = {
    400: "invalid body",
    401: "unauthorized",
    403: "forbidden",
    404: "not found",
    405: "method not allowed",
    409: "conflict",
}


def _under_api(path: str) -> bool:
    # Byte exact and case sensitive: /API/me and /%61pi/me are not the API.
    return path == API_PREFIX or path.startswith(API_PREFIX + "/")


def _raw_path(scope: Scope) -> str:
    """The path exactly as it arrived, before percent-decoding.

    ASGI servers put the decoded path in scope["path"] and the bytes in
    raw_path, so routing on scope["path"] would serve /api/%6de as /api/me.
    latin-1 is a byte-preserving decode, which is what byte-for-byte matching
    wants; every registered path is ASCII anyway.
    """
    raw = scope.get("raw_path")
    if raw is None:
        return scope.get("path", "/")
    return raw.split(b"?", 1)[0].decode("latin-1")


class RawPathMiddleware:
    """Routes on the raw path, so a percent-encoded octet is not decoded first.

    An empty segment needs no guard here: no registered path carries one and a
    path parameter never matches across a slash, so /api//me falls through to
    the JSON 404 on its own. tests/test_router.py pins that.
    """

    def __init__(self, app) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return
        scope = dict(scope)
        scope["path"] = _raw_path(scope)
        await self.app(scope, receive, send)


def _allowed_methods(app: FastAPI, path: str) -> list[str]:
    """Every method registered for this path, across routes. Starlette only
    remembers the first partial match, which would under-report Allow on a path
    that carries two methods.

    Nothing filters HEAD out: add_api_route registers no HEAD, so there is none
    to filter. tests/test_router.py pins that.
    """
    methods: set[str] = set()
    for route in app.routes:
        regex = getattr(route, "path_regex", None)
        if regex is not None and regex.match(path):
            methods |= set(getattr(route, "methods", None) or [])
    return sorted(methods)


def build_app(store: Store, audit: Audit | None = None) -> FastAPI:
    app = FastAPI(docs_url=None, redoc_url=None, openapi_url=None)
    app.router.redirect_slashes = False

    for path, methods, handler in ROUTES:
        app.add_api_route(path, _bind(handler, store), methods=methods)

    if audit is not None:
        # Outside /api deliberately: this route is not in
        # contract/openapi.yaml, and the conformance harness compares the
        # four apps over the /api surface only.
        app.add_api_route(
            "/everscribe/embed-token",
            make_embed_token(audit.minter, store),
            methods=["POST"],
        )

    # check_dir=False lets this construct before task 4 builds web/dist;
    # StaticFiles still raises on the first request, so the directory is
    # checked per request instead of a placeholder dist being invented.
    spa = StaticFiles(directory=str(DIST), html=True, check_dir=False)

    async def _default(scope: Scope, receive: Receive, send: Send) -> None:
        # Nothing matched. Under /api that is the API's own JSON 404; anywhere
        # else it is the SPA, whose 404 body the contract does not specify.
        if _under_api(scope["path"]):
            await fail(404, "not found")(scope, receive, send)
            return
        if not DIST.is_dir():
            await fail(404, "not found")(scope, receive, send)
            return
        await spa(scope, receive, send)

    app.router.default = _default

    async def _http_exception(request: Request, exc: StarletteHTTPException) -> Response:
        if exc.status_code == 405:
            allow = _allowed_methods(app, request.scope["path"])
            return JSON(
                {"error": _MESSAGES[405]},
                status_code=405,
                headers={"Allow": ", ".join(allow)},
            )
        message = _MESSAGES.get(exc.status_code) or HTTPStatus(exc.status_code).phrase.lower()
        return fail(exc.status_code, message)

    app.add_exception_handler(StarletteHTTPException, _http_exception)

    if audit is not None:
        app.add_middleware(
            EverscribeMiddleware,
            recorder=audit.recorder,
            resolve_actor=make_resolve_actor(store),
        )

    # Added last, so it runs first: Starlette's add_middleware prepends, and
    # the last-added middleware ends up outermost. RawPathMiddleware has to
    # stay outermost because it rewrites scope["path"] before routing, which
    # puts the audit middleware inside it - the right place for it anyway.
    app.add_middleware(RawPathMiddleware)
    return app


def _bind(handler, store: Store):
    async def endpoint(request: Request) -> Response:
        return await handler(request, store)

    return endpoint
