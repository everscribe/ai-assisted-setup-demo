"""The app factory.

Registers exactly the eighteen operations contract/openapi.yaml declares, and
turns off the framework defaults that would break it:

1. No pydantic body models, so the role check runs before body parsing.
2. Starlette's {"detail": "Not Found"} becomes the contract's Error schema.
3. Same for the 405 body; its Allow header is kept and HEAD dropped from it.
4. HEAD is not routed to GET, so HEAD /api/health is a 405.
5. redirect_slashes is off, so /api/me/ is a 404 rather than a 307.
6. A RequestValidationError, if one ever escapes, renders a 400 Error.

Routing matches the raw, undecoded path: /api/%6de is a 404, not Ada's record.
Captured path parameters are still decoded, in handlers._path_id.
"""

from http import HTTPStatus
from pathlib import Path

from fastapi import FastAPI
from fastapi.exceptions import RequestValidationError
from starlette.exceptions import HTTPException as StarletteHTTPException
from starlette.requests import Request
from starlette.responses import Response
from starlette.staticfiles import StaticFiles
from starlette.types import Receive, Scope, Send

from app import handlers
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
    """Routes on the raw path and rejects empty segments before routing."""

    def __init__(self, app) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return
        path = _raw_path(scope)
        scope = dict(scope)
        scope["path"] = path
        if _under_api(path) and "//" in path:
            await fail(404, "not found")(scope, receive, send)
            return
        await self.app(scope, receive, send)


def _allowed_methods(app: FastAPI, path: str) -> list[str]:
    """Every method registered for this path, across routes. Starlette only
    remembers the first partial match, which would under-report Allow on a path
    that carries two methods. HEAD is registered on no route."""
    methods: set[str] = set()
    for route in app.routes:
        regex = getattr(route, "path_regex", None)
        if regex is not None and regex.match(path):
            methods |= set(getattr(route, "methods", None) or [])
    methods.discard("HEAD")
    return sorted(methods)


def build_app(store: Store) -> FastAPI:
    app = FastAPI(docs_url=None, redoc_url=None, openapi_url=None)
    app.router.redirect_slashes = False

    for path, methods, handler in ROUTES:
        app.add_api_route(path, _bind(handler, store), methods=methods)

    for route in app.routes:
        # Starlette adds HEAD wherever GET is registered.
        methods = getattr(route, "methods", None)
        if methods and "HEAD" in methods:
            route.methods = {m for m in methods if m != "HEAD"}

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

    async def _validation_error(request: Request, exc: RequestValidationError) -> Response:
        # No route binds a model, so this should be unreachable. A 422 with a
        # detail list would not be.
        return fail(400, _MESSAGES[400])

    app.add_exception_handler(StarletteHTTPException, _http_exception)
    app.add_exception_handler(RequestValidationError, _validation_error)
    app.add_middleware(RawPathMiddleware)
    return app


def _bind(handler, store: Store):
    async def endpoint(request: Request) -> Response:
        return await handler(request, store)

    return endpoint
