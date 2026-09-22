"""Domain records, shaped by the schemas in contract/openapi.yaml.

Optional fields are None when absent and are dropped by to_dict, because the
contract's response schemas set additionalProperties: false and an absent
field must not serialize as null.
"""

from dataclasses import dataclass
from datetime import datetime, timezone


def now_rfc3339() -> str:
    # isoformat gives +00:00; the contract requires a literal Z.
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def parse_rfc3339(value: str) -> datetime:
    # created_at is ordered as a parsed instant, never as a string.
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def _compact(pairs: list[tuple[str, str | None]]) -> dict[str, str]:
    return {key: value for key, value in pairs if value is not None}


@dataclass
class Organization:
    id: str
    name: str
    created_at: str

    def to_dict(self) -> dict[str, str]:
        return _compact([
            ("id", self.id),
            ("name", self.name),
            ("created_at", self.created_at),
        ])


@dataclass
class User:
    id: str
    name: str
    email: str
    role: str
    organization_id: str
    created_at: str

    def to_dict(self) -> dict[str, str]:
        return _compact([
            ("id", self.id),
            ("name", self.name),
            ("email", self.email),
            ("role", self.role),
            ("organization_id", self.organization_id),
            ("created_at", self.created_at),
        ])


@dataclass
class Resource:
    id: str
    name: str
    description: str
    sensitivity: str
    organization_id: str
    registered_by: str
    created_at: str
    decommission_at: str | None = None

    def to_dict(self) -> dict[str, str]:
        return _compact([
            ("id", self.id),
            ("name", self.name),
            ("description", self.description),
            ("sensitivity", self.sensitivity),
            ("organization_id", self.organization_id),
            ("registered_by", self.registered_by),
            ("created_at", self.created_at),
            ("decommission_at", self.decommission_at),
        ])


@dataclass
class AccessRequest:
    id: str
    resource_id: str
    organization_id: str
    requested_by: str
    reason: str
    status: str
    created_at: str
    decided_by: str | None = None
    decided_at: str | None = None
    revoked_by: str | None = None
    revoked_at: str | None = None

    def to_dict(self) -> dict[str, str]:
        # Schema order, which puts created_at last.
        return _compact([
            ("id", self.id),
            ("resource_id", self.resource_id),
            ("organization_id", self.organization_id),
            ("requested_by", self.requested_by),
            ("reason", self.reason),
            ("status", self.status),
            ("decided_by", self.decided_by),
            ("decided_at", self.decided_at),
            ("revoked_by", self.revoked_by),
            ("revoked_at", self.revoked_at),
            ("created_at", self.created_at),
        ])


@dataclass
class Comment:
    id: str
    request_id: str
    organization_id: str
    author_id: str
    body: str
    created_at: str

    def to_dict(self) -> dict[str, str]:
        return _compact([
            ("id", self.id),
            ("request_id", self.request_id),
            ("organization_id", self.organization_id),
            ("author_id", self.author_id),
            ("body", self.body),
            ("created_at", self.created_at),
        ])
