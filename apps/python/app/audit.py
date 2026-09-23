"""Everscribe wiring: the actor resolver the audit middleware calls per
request, and the embed-token endpoint the frontend's audit-trail component
POSTs for a short-lived credential.

``resolve_actor`` is synchronous and receives the Starlette ``Request``, and
this app's store is synchronous too, so the X-User-ID lookup happens directly
in the resolver - no upstream step is needed the way the Go and Node apps need
one. It is deliberately the same lookup ``handlers.require_actor`` performs:
this app has no session middleware, so without it every event would record as
an anonymous actor and nothing in the tests would catch it.

Named ``audit`` rather than ``everscribe`` so the module cannot shadow the
installed SDK package it imports from.
"""

from __future__ import annotations

from dataclasses import dataclass

from everscribe.event.types import Actor
from everscribe.minter import Client as MinterClient
from everscribe.minter import TokenOptions
from everscribe.recorder import Recorder
from starlette.requests import Request
from starlette.responses import Response

from app.handlers import JSON, fail
from app.models import User
from app.store import Store, StoreError


@dataclass
class Audit:
    """The Everscribe surfaces the server wires in. Optional throughout, so
    the router tests - which call ``build_app(store)`` with no audit - need
    neither credentials nor the network."""

    recorder: Recorder
    minter: MinterClient


def _lookup(request: Request, store: Store) -> User | None:
    """Resolve X-User-ID, or None. Never raises: an unresolvable id is an
    anonymous request here, and authorization stays in each handler's
    require_actor so the contract's check order is untouched."""
    user_id = request.headers.get("x-user-id", "")
    if not user_id:
        return None
    try:
        return store.get_user(user_id)
    except StoreError:
        return None


def make_resolve_actor(store: Store):
    """Build the resolver the ASGI middleware calls once per request.

    Actor.type uses the SDK's conventional values: "admin" for this app's
    admin role, "user" for everyone else, "anonymous" when nothing resolved.
    POST /api/login is the case that matters - it arrives with no X-User-ID at
    all, and its event names itself before the email lookup so a failed
    sign-in is still auditable.
    """

    def resolve_actor(request: Request) -> Actor:
        user = _lookup(request, store)
        if user is None:
            return Actor(type="anonymous")
        return Actor(
            type="admin" if user.role == "admin" else "user",
            id=user.id,
            display_name=user.name,
            email=user.email,
        )

    return resolve_actor


def make_embed_token(minter: MinterClient, store: Store):
    """Build the endpoint that mints the credential the embeddable
    audit-trail component needs, scoped to the caller's organization.

    POST, not GET: the component POSTs with no body, so a GET-only route
    would answer 405 and the embed would never load. Minting also issues a
    bearer credential and consumes quota, so it is not a safe idempotent read.
    Cache-Control: no-store on top, as defense in depth - a POST response is
    not cached without an explicit opt-in, but a bearer token sitting in a
    shared cache is the whole risk this endpoint carries.
    """

    async def embed_token(request: Request) -> Response:
        user = _lookup(request, store)
        if user is None:
            response: Response = fail(401, "unauthorized")
            response.headers["Cache-Control"] = "no-store"
            return response

        try:
            # Scoped to the caller's own organization: the token must not
            # let the component read another tenant's trail.
            token = minter.mint_token(TokenOptions(tenant_id=user.organization_id))
        except Exception:
            response = fail(502, "could not mint embed token")
            response.headers["Cache-Control"] = "no-store"
            return response

        response = JSON({"token": token})
        response.headers["Cache-Control"] = "no-store"
        return response

    return embed_token
