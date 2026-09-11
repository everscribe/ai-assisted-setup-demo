// The eighteen routes contract/openapi.yaml declares, and nothing else.
// strict routing means a trailing slash never matches a registered
// path (it falls to the 404 below); router.all per path means every
// method, including HEAD and OPTIONS, reaches our own dispatch instead
// of Express's built-in HEAD-to-GET or auto-OPTIONS behaviour.
import express, { type Request, type Response, type Router } from "express";
import * as h from "./handlers.js";
import { fail } from "./handlers.js";
import type { Store } from "./store.js";

type Method = "GET" | "POST" | "PUT" | "DELETE";
type RouteHandler = (req: Request, res: Response, store: Store) => Promise<void> | void;

interface RouteDef {
  path: string;
  handlers: Partial<Record<Method, RouteHandler>>;
}

const routes: RouteDef[] = [
  { path: "/login", handlers: { POST: h.handleLogin } },
  { path: "/logout", handlers: { POST: h.handleLogout } },
  { path: "/me", handlers: { GET: h.handleMe } },
  { path: "/members", handlers: { GET: h.handleListMembers, POST: h.handleInviteMember } },
  { path: "/members/:id", handlers: { DELETE: h.handleRemoveMember } },
  { path: "/members/:id/role", handlers: { PUT: h.handleUpdateMemberRole } },
  { path: "/resources", handlers: { GET: h.handleListResources, POST: h.handleRegisterResource } },
  { path: "/resources/:id/decommission", handlers: { POST: h.handleDecommissionResource } },
  {
    path: "/resources/:id/requests",
    handlers: { GET: h.handleListResourceRequests, POST: h.handleRequestAccess },
  },
  { path: "/requests/:id/approve", handlers: { POST: h.handleApproveRequest } },
  { path: "/requests/:id/deny", handlers: { POST: h.handleDenyRequest } },
  { path: "/requests/:id/revoke", handlers: { POST: h.handleRevokeRequest } },
  {
    path: "/requests/:id/comments",
    handlers: { GET: h.handleListRequestComments, POST: h.handleAddComment },
  },
  { path: "/health", handlers: { GET: h.handleHealth } },
];

export function buildRouter(store: Store): Router {
  const router = express.Router({ strict: true, caseSensitive: false });

  // Raw text, every content type, and never parsed here: parsing and
  // validation happen inside each handler, after its auth/role checks,
  // so a bad body cannot preempt a 401 or 403 (see the contract's check
  // order).
  router.use(express.text({ type: () => true }));

  for (const route of routes) {
    const allow = (Object.keys(route.handlers) as Method[]).join(", ");
    router.all(route.path, async (req, res) => {
      const handler = route.handlers[req.method as Method];
      if (!handler) {
        res.set("Allow", allow);
        fail(res, 405, "method not allowed");
        return;
      }
      await handler(req, res, store);
    });
  }

  // Unknown path, or a trailing slash on a real one (strict routing
  // means it never matched above): the identical JSON 404 a
  // cross-tenant miss also produces.
  router.use((_req, res) => {
    fail(res, 404, "not found");
  });

  return router;
}
