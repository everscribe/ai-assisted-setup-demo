// Everscribe wiring: the actor step the audit middleware resolves from,
// and the embed-token route the frontend's audit-trail component POSTs
// for a short-lived credential.
//
// expressMiddleware's resolveActor is synchronous - (req) => Actor - but
// store.getUser is async, so the lookup cannot happen inside it. This app
// also has no session middleware (requireActor runs inside each handler),
// so there was nothing on the request to read either. attachActor is that
// missing upstream step: it resolves X-User-ID once per request and hangs
// the user off req, and main.ts mounts it BEFORE the audit middleware so
// resolveActor is reading something already populated. Mounted the other
// way round, every event would record as an anonymous actor and nothing
// in the build or the tests would catch it.
import type { Request, RequestHandler, Response } from "express";
import type { Actor } from "@everscribe/sdk-node/event";
import type { Client as MinterClient } from "@everscribe/sdk-node/minter";

import type { User } from "./models.js";
import type { Store } from "./store.js";

declare global {
  namespace Express {
    interface Request {
      /** Resolved by attachActor; absent when the request is anonymous. */
      actor?: User;
    }
  }
}

/** Resolves X-User-ID into req.actor. Never rejects: authorization stays
 *  in each handler's requireActor, so the contract's check order is
 *  untouched and POST /api/login (which legitimately arrives with no
 *  X-User-ID) still reaches its handler. */
export function attachActor(store: Store): RequestHandler {
  return async function attachActorMiddleware(req, _res, next) {
    const id = req.header("X-User-ID");
    if (!id) return next();
    try {
      req.actor = await store.getUser(id);
    } catch {
      // Unresolvable id is an anonymous request, not an error here.
    }
    next();
  };
}

/** Derives the audit Actor from what attachActor resolved. Type uses the
 *  SDK's conventional values: "admin" for this app's admin role, "user"
 *  for everyone else, "anonymous" when nothing resolved. */
export function resolveActor(req: Request): Actor {
  const user = req.actor;
  if (!user) return { type: "anonymous" };
  return {
    type: user.role === "admin" ? "admin" : "user",
    id: user.id,
    displayName: user.name,
    email: user.email,
  };
}

/** Mints the short-lived credential the embeddable audit-trail component
 *  needs, scoped to the caller's organization.
 *
 *  POST, not GET: the component POSTs with no body, so a GET-only route
 *  would answer 405 and the embed would never load. Minting also issues a
 *  bearer credential and consumes quota, so it is not a safe idempotent
 *  read. Cache-Control: no-store on top, as defense in depth - a POST
 *  response is not cached without an explicit opt-in, but a bearer token
 *  sitting in a shared cache is the whole risk this route carries. */
export function embedToken(minter: MinterClient): RequestHandler {
  return async function embedTokenHandler(req: Request, res: Response): Promise<void> {
    res.set("Cache-Control", "no-store");

    const user = req.actor;
    if (!user) {
      res.status(401).json({ error: "unauthorized" });
      return;
    }

    try {
      // Scoped to the caller's own organization: the token must not let
      // the component read another tenant's trail.
      const token = await minter.mintToken({ tenantId: user.organization_id });
      res.status(200).json({ token });
    } catch {
      res.status(502).json({ error: "could not mint embed token" });
    }
  };
}
