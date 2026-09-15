"""In-memory Store implementation.

Rows live in plain lists, not dicts keyed by id, so insertion order is defined
and an ordering mutation is a proof rather than a coin flip.

Tenant scoping lives here: a target outside org_id raises NotFoundError,
identical to a missing row, so the boundary does not leak existence.
ForbiddenError is for the handler layer and is never raised here.
"""

import secrets
import threading
from dataclasses import replace

from app import seed as fixture
from app.models import (
    AccessRequest,
    Comment,
    Organization,
    Resource,
    User,
    now_rfc3339,
    parse_rfc3339,
)
from app.store import ConflictError, NotFoundError

ROLES = ("admin", "member")
DEFAULT_ROLE = "member"
DEFAULT_SENSITIVITY = "internal"


def _new_id(prefix: str) -> str:
    return prefix + secrets.token_hex(6)


def _by_created_at_then_id(rows: list) -> list:
    # created_at is compared as a parsed instant, and the id tiebreak is part
    # of the key: a tie must not fall back to insertion order.
    return sorted(rows, key=lambda row: (parse_rfc3339(row.created_at), row.id))


def _by_created_at_desc_then_id(rows: list) -> list:
    # created_at descending, id still ascending, so the two keys need
    # opposite directions and cannot share one reverse=True sort.
    ordered = sorted(rows, key=lambda row: row.id)
    ordered.sort(key=lambda row: parse_rfc3339(row.created_at), reverse=True)
    return ordered


class MemoryStore:
    def __init__(self) -> None:
        self._lock = threading.RLock()
        self._organizations: list[Organization] = []
        self._users: list[User] = []
        self._resources: list[Resource] = []
        self._requests: list[AccessRequest] = []
        self._comments: list[Comment] = []

    def seed(self) -> None:
        # Replaces the contents, so seeding twice is idempotent.
        with self._lock:
            self._organizations = fixture.organizations()
            self._users = fixture.users()
            self._resources = fixture.resources()
            self._requests = fixture.access_requests()
            self._comments = fixture.comments()

    # Users and members

    def get_user(self, user_id: str) -> User:
        with self._lock:
            for user in self._users:
                if user.id == user_id:
                    return replace(user)
        raise NotFoundError("not found")

    def find_user_by_email(self, email: str) -> User:
        # Emails are not unique. Collect every match and take the first under
        # the list order, so storage order never decides the answer.
        with self._lock:
            matches = [u for u in self._users if u.email == email]
        if not matches:
            raise NotFoundError("not found")
        return replace(_by_created_at_then_id(matches)[0])

    def list_org_members(self, org_id: str) -> list[User]:
        with self._lock:
            rows = [u for u in self._users if u.organization_id == org_id]
        return [replace(u) for u in _by_created_at_then_id(rows)]

    def invite_member(self, org_id: str, name: str, email: str, role: str) -> User:
        if role not in ROLES:
            role = DEFAULT_ROLE
        user = User(
            id=_new_id("u_"),
            name=name,
            email=email,
            role=role,
            organization_id=org_id,
            created_at=now_rfc3339(),
        )
        with self._lock:
            self._users.append(user)
        return replace(user)

    def remove_member(self, org_id: str, user_id: str) -> None:
        # Removal does not cascade: requests and comments keep naming the id.
        with self._lock:
            user = self._find_user(org_id, user_id)
            self._users.remove(user)

    def update_member_role(self, org_id: str, user_id: str, role: str) -> User:
        with self._lock:
            user = self._find_user(org_id, user_id)
            user.role = role
            return replace(user)

    # Resources

    def list_org_resources(self, org_id: str) -> list[Resource]:
        # Includes decommissioned resources.
        with self._lock:
            rows = [r for r in self._resources if r.organization_id == org_id]
        return [replace(r) for r in _by_created_at_then_id(rows)]

    def get_resource(self, org_id: str, resource_id: str) -> Resource:
        with self._lock:
            return replace(self._find_resource(org_id, resource_id))

    def register_resource(
        self,
        org_id: str,
        name: str,
        description: str,
        sensitivity: str,
        registered_by: str,
        *,
        _id: str | None = None,
        _at: str | None = None,
    ) -> Resource:
        # _id and _at are test seams for planting rows; production never passes them.
        resource = Resource(
            id=_id or _new_id("r_"),
            name=name,
            description=description,
            sensitivity=sensitivity or DEFAULT_SENSITIVITY,
            organization_id=org_id,
            registered_by=registered_by,
            created_at=_at or now_rfc3339(),
        )
        with self._lock:
            self._resources.append(resource)
        return replace(resource)

    def decommission_resource(self, org_id: str, resource_id: str) -> Resource:
        # Idempotent: a second call re-stamps rather than conflicting.
        with self._lock:
            resource = self._find_resource(org_id, resource_id)
            resource.decommission_at = now_rfc3339()
            return replace(resource)

    # Access requests

    def list_resource_requests(self, org_id: str, resource_id: str) -> list[AccessRequest]:
        with self._lock:
            self._find_resource(org_id, resource_id)
            rows = [
                r for r in self._requests
                if r.organization_id == org_id and r.resource_id == resource_id
            ]
        return [replace(r) for r in _by_created_at_desc_then_id(rows)]

    def get_request(self, org_id: str, request_id: str) -> AccessRequest:
        with self._lock:
            return replace(self._find_request(org_id, request_id))

    def request_access(self, org_id: str, resource_id: str, requested_by: str, reason: str) -> AccessRequest:
        with self._lock:
            resource = self._find_resource(org_id, resource_id)
            if resource.decommission_at is not None:
                raise ConflictError("conflict")
            request = AccessRequest(
                id=_new_id("req_"),
                resource_id=resource_id,
                organization_id=org_id,
                requested_by=requested_by,
                reason=reason,
                status="pending",
                created_at=now_rfc3339(),
            )
            self._requests.append(request)
            return replace(request)

    def approve_request(self, org_id: str, request_id: str, approved_by: str) -> AccessRequest:
        return self._decide(org_id, request_id, approved_by, "approved")

    def deny_request(self, org_id: str, request_id: str, denied_by: str) -> AccessRequest:
        return self._decide(org_id, request_id, denied_by, "denied")

    def revoke_request(self, org_id: str, request_id: str, revoked_by: str) -> AccessRequest:
        with self._lock:
            request = self._find_request(org_id, request_id)
            if request.status != "approved":
                raise ConflictError("conflict")
            # decided_by and decided_at stay as the approval wrote them.
            request.status = "revoked"
            request.revoked_by = revoked_by
            request.revoked_at = now_rfc3339()
            return replace(request)

    # Comments

    def list_request_comments(self, org_id: str, request_id: str) -> list[Comment]:
        with self._lock:
            self._find_request(org_id, request_id)
            rows = [
                c for c in self._comments
                if c.organization_id == org_id and c.request_id == request_id
            ]
        return [replace(c) for c in _by_created_at_then_id(rows)]

    def add_comment(self, org_id: str, request_id: str, author_id: str, body: str) -> Comment:
        with self._lock:
            self._find_request(org_id, request_id)
            comment = Comment(
                id=_new_id("c_"),
                request_id=request_id,
                organization_id=org_id,
                author_id=author_id,
                body=body,
                created_at=now_rfc3339(),
            )
            self._comments.append(comment)
            return replace(comment)

    # Organizations. No route reaches these; the tests do.

    def list_organizations(self) -> list[Organization]:
        with self._lock:
            rows = list(self._organizations)
        return [replace(o) for o in _by_created_at_then_id(rows)]

    def get_organization(self, org_id: str) -> Organization:
        with self._lock:
            for org in self._organizations:
                if org.id == org_id:
                    return replace(org)
        raise NotFoundError("not found")

    # Internals. Each returns the live row and holds the tenant check, so a
    # cross-tenant target is indistinguishable from a missing one.

    def _find_user(self, org_id: str, user_id: str) -> User:
        for user in self._users:
            if user.id == user_id and user.organization_id == org_id:
                return user
        raise NotFoundError("not found")

    def _find_resource(self, org_id: str, resource_id: str) -> Resource:
        for resource in self._resources:
            if resource.id == resource_id and resource.organization_id == org_id:
                return resource
        raise NotFoundError("not found")

    def _find_request(self, org_id: str, request_id: str) -> AccessRequest:
        for request in self._requests:
            if request.id == request_id and request.organization_id == org_id:
                return request
        raise NotFoundError("not found")

    def _decide(self, org_id: str, request_id: str, decided_by: str, status: str) -> AccessRequest:
        with self._lock:
            request = self._find_request(org_id, request_id)
            if request.status != "pending":
                raise ConflictError("conflict")
            request.status = status
            request.decided_by = decided_by
            request.decided_at = now_rfc3339()
            return replace(request)
