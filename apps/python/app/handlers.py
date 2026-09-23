"""One handler per operation, plus the four shared helpers.

Every handler runs the contract's check order itself: 401 authentication, then
403 admin role, then 400 body parsing and field validation, then the store's
404 or 409. The body is read with request.body() inside the handler rather than
bound as a pydantic model, because a model bound as a route parameter validates
before the role check and would answer 422 where the contract requires 403.

Every error in this app is JSON, and every response that has a body carries
application/json; charset=utf-8.
"""

import json
from urllib.parse import unquote

from everscribe.asgi import current_event
from everscribe.event.types import Target
from starlette.requests import Request
from starlette.responses import JSONResponse, Response

from app.models import User
from app.store import ConflictError, ForbiddenError, NotFoundError, Store, StoreError

ROLES = ("admin", "member")
DEFAULT_SENSITIVITY = "internal"


class JSON(JSONResponse):
    # Starlette only appends a charset to text/*; the contract wants one here.
    media_type = "application/json; charset=utf-8"


def fail(status: int, message: str) -> JSON:
    return JSON({"error": message}, status_code=status)


def fail_store(err: StoreError) -> JSON:
    if isinstance(err, NotFoundError):
        return fail(404, "not found")
    if isinstance(err, ForbiddenError):
        # Unreachable today: the store answers a cross-tenant target with
        # NotFoundError. The branch keeps the mapping total.
        return fail(403, "forbidden")
    if isinstance(err, ConflictError):
        return fail(409, "conflict")
    return fail(500, "internal error")


def require_actor(request: Request, store: Store) -> tuple[User | None, JSON | None]:
    # Missing or unresolvable X-User-ID is a 401, never a 404.
    user_id = request.headers.get("x-user-id", "")
    if not user_id:
        return None, fail(401, "unauthorized")
    try:
        return store.get_user(user_id), None
    except StoreError:
        return None, fail(401, "unauthorized")


def require_admin(user: User) -> JSON | None:
    if user.role != "admin":
        return fail(403, "admin role required")
    return None


# Body reading and field access.


def _read_json(raw: bytes) -> tuple[dict | None, JSON | None]:
    # The 400 message is not normative; only the status is.
    if not raw.strip():
        return None, fail(400, "invalid body")
    try:
        data = json.loads(raw)
    except ValueError:
        return None, fail(400, "invalid body")
    if not isinstance(data, dict):
        return None, fail(400, "invalid body")
    return data, None


def _string(data: dict, key: str) -> tuple[str, bool]:
    """Returns (value, ok). A null is a missing field, never a type error, and
    an unknown field is simply never looked up. Keys match byte for byte."""
    value = data.get(key)
    if value is None:
        return "", True
    if isinstance(value, str):
        return value, True
    return "", False


def _path_id(request: Request) -> str:
    # Routing matches the raw path; a captured id is decoded before the store
    # sees it, so r%5Facme_pg and r_acme_pg are the same request.
    return unquote(request.path_params["id"])


# Anonymous routes.


async def login(request: Request, store: Store) -> Response:
    data, err = _read_json(await request.body())
    if err:
        return err
    email, ok = _string(data, "email")
    if not ok:
        return fail(400, "invalid body")
    if not email:  # pointedly not trimmed
        return fail(400, "email required")

    # Named here, after the body checks but before the lookup, so a rejected
    # sign-in still records: the middleware fills result from the response, so
    # the 401 below lands as user.login with an error result and no tenant.
    # The 400s above return without an action and record nothing, which is the
    # intent - a malformed body reached no account.
    e = current_event()
    e.action = "user.login"

    try:
        user = store.find_user_by_email(email)
    except StoreError:
        return fail(401, "no user with that email")

    # Only known once the email resolves. A failed sign-in has no tenant to
    # attribute, by construction.
    e.target = Target(type="user", id=user.id)
    e.tenant_id = user.organization_id

    return JSON(user.to_dict())


async def health(request: Request, store: Store) -> Response:
    return JSON({"status": "ok"})


# Session.


async def logout(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err

    e = current_event()
    e.action = "user.logout"
    e.target = Target(type="user", id=user.id)
    e.tenant_id = user.organization_id

    return Response(status_code=204)


async def me(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    return JSON(user.to_dict())


# Members.


async def list_members(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    rows = store.list_org_members(user.organization_id)
    return JSON([row.to_dict() for row in rows])


async def invite_member(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    denied = require_admin(user)
    if denied:
        return denied
    data, err = _read_json(await request.body())
    if err:
        return err
    name, name_ok = _string(data, "name")
    email, email_ok = _string(data, "email")
    role, role_ok = _string(data, "role")
    if not (name_ok and email_ok and role_ok):
        return fail(400, "invalid body")
    name, email = name.strip(), email.strip()
    if not name or not email:
        return fail(400, "name and email required")
    # Named before the store call, so a conflict (409, email already in the
    # org) records as a failed invite rather than vanishing.
    e = current_event()
    e.action = "member.invited"
    e.tenant_id = user.organization_id
    e.with_fields("email", email, "role", role)

    # role is not trimmed and anything outside the two values defaults to member.
    member = store.invite_member(user.organization_id, name, email, role)

    # The new member's id only exists after the store accepts it.
    e.target = Target(type="user", id=member.id)

    return JSON(member.to_dict(), status_code=201)


async def remove_member(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    denied = require_admin(user)
    if denied:
        return denied
    e = current_event()
    e.action = "member.removed"
    e.target = Target(type="user", id=_path_id(request))
    e.tenant_id = user.organization_id

    try:
        store.remove_member(user.organization_id, _path_id(request))
    except StoreError as exc:
        return fail_store(exc)
    return Response(status_code=204)


async def change_member_role(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    denied = require_admin(user)
    if denied:
        return denied
    data, err = _read_json(await request.body())
    if err:
        return err
    role, ok = _string(data, "role")
    if not ok or role not in ROLES:  # compared exactly, so " admin " is a 400
        return fail(400, "role must be admin or member")

    # A privilege change is the event an auditor reads first, so it is named
    # before the store call and carries the requested role even when the call
    # is refused.
    e = current_event()
    e.action = "member.role.changed"
    e.target = Target(type="user", id=_path_id(request))
    e.tenant_id = user.organization_id
    e.with_field("role", role)

    try:
        member = store.update_member_role(user.organization_id, _path_id(request), role)
    except StoreError as exc:
        return fail_store(exc)
    return JSON(member.to_dict())


# Resources.


async def list_resources(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    rows = store.list_org_resources(user.organization_id)
    return JSON([row.to_dict() for row in rows])


async def register_resource(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    denied = require_admin(user)
    if denied:
        return denied
    data, err = _read_json(await request.body())
    if err:
        return err
    name, name_ok = _string(data, "name")
    description, description_ok = _string(data, "description")
    sensitivity, sensitivity_ok = _string(data, "sensitivity")
    if not (name_ok and description_ok and sensitivity_ok):
        return fail(400, "invalid body")
    name = name.strip()
    if not name:
        return fail(400, "name required")
    e = current_event()
    e.action = "resource.registered"
    e.tenant_id = user.organization_id
    e.with_fields("name", name, "sensitivity", sensitivity.strip() or DEFAULT_SENSITIVITY)

    resource = store.register_resource(
        user.organization_id,
        name,
        description.strip(),
        sensitivity.strip() or DEFAULT_SENSITIVITY,
        user.id,
    )

    e.target = Target(type="resource", id=resource.id)

    return JSON(resource.to_dict(), status_code=201)


async def decommission_resource(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    denied = require_admin(user)
    if denied:
        return denied
    e = current_event()
    e.action = "resource.decommissioned"
    e.target = Target(type="resource", id=_path_id(request))
    e.tenant_id = user.organization_id

    try:
        resource = store.decommission_resource(user.organization_id, _path_id(request))
    except StoreError as exc:
        return fail_store(exc)
    return JSON(resource.to_dict())


# Access requests.


async def list_resource_requests(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    try:
        rows = store.list_resource_requests(user.organization_id, _path_id(request))
    except StoreError as exc:
        return fail_store(exc)
    return JSON([row.to_dict() for row in rows])


async def request_access(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    # No admin check here, so the body is read before the store is touched and
    # an unparseable body against an unknown resource is a 400, not a 404.
    data, err = _read_json(await request.body())
    if err:
        return err
    reason, ok = _string(data, "reason")
    if not ok:
        return fail(400, "invalid body")
    reason = reason.strip()
    if not reason:
        return fail(400, "reason required")
    # Target is the resource until the request exists; a refusal
    # (decommissioned resource, or a cross-tenant miss) still says what was
    # being asked for.
    e = current_event()
    e.action = "access_request.submitted"
    e.target = Target(type="resource", id=_path_id(request))
    e.tenant_id = user.organization_id
    e.with_field("reason", reason)

    try:
        created = store.request_access(
            user.organization_id, _path_id(request), user.id, reason
        )
    except StoreError as exc:
        return fail_store(exc)

    e.target = Target(type="access_request", id=created.id)

    return JSON(created.to_dict(), status_code=201)


#: Store method name -> audit action. One closure serves all three routes,
#: so the event name is derived here rather than repeated per handler.
_DECISION_ACTIONS = {
    "approve_request": "access_request.approved",
    "deny_request": "access_request.denied",
    "revoke_request": "access_request.revoked",
}


def _decide(action: str):
    audit_action = _DECISION_ACTIONS[action]

    async def handler(request: Request, store: Store) -> Response:
        user, err = require_actor(request, store)
        if err:
            return err
        denied = require_admin(user)
        if denied:
            return denied

        # Named before the store call: a refused decision (already decided,
        # wrong tenant) is as auditable as an accepted one.
        e = current_event()
        e.action = audit_action
        e.target = Target(type="access_request", id=_path_id(request))
        e.tenant_id = user.organization_id

        method = getattr(store, action)
        try:
            decided = method(user.organization_id, _path_id(request), user.id)
        except StoreError as exc:
            return fail_store(exc)
        return JSON(decided.to_dict())

    return handler


approve_request = _decide("approve_request")
deny_request = _decide("deny_request")
revoke_request = _decide("revoke_request")


# Comments.


async def list_request_comments(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    try:
        rows = store.list_request_comments(user.organization_id, _path_id(request))
    except StoreError as exc:
        return fail_store(exc)
    return JSON([row.to_dict() for row in rows])


async def add_comment(request: Request, store: Store) -> Response:
    user, err = require_actor(request, store)
    if err:
        return err
    data, err = _read_json(await request.body())
    if err:
        return err
    body, ok = _string(data, "body")
    if not ok:
        return fail(400, "invalid body")
    body = body.strip()
    if not body:
        return fail(400, "body required")
    # The comment is part of the approval trail, so the event points at the
    # access request rather than at the comment row. The body is deliberately
    # not copied into metadata: it is free text a requester typed, and the
    # audit log is not the place to duplicate it.
    e = current_event()
    e.action = "access_request.commented"
    e.target = Target(type="access_request", id=_path_id(request))
    e.tenant_id = user.organization_id

    try:
        comment = store.add_comment(user.organization_id, _path_id(request), user.id, body)
    except StoreError as exc:
        return fail_store(exc)
    return JSON(comment.to_dict(), status_code=201)
