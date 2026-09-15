import { describe, expect, it, beforeAll } from "vitest";
import request from "supertest";
import { makeApp } from "./main.js";

let app: ReturnType<typeof makeApp>;
beforeAll(async () => { app = await makeApp(); });

describe("router", () => {
  it("401s without an actor, as JSON", async () => {
    const r = await request(app).get("/api/me");
    expect(r.status).toBe(401);
    expect(r.headers["content-type"]).toMatch(/application\/json/);
    expect(r.body).toEqual({ error: "unauthorized" });
  });

  it("405s on a method the path does not serve", async () => {
    const r = await request(app).get("/api/login");
    expect(r.status).toBe(405);
    expect(r.body).toEqual({ error: "method not allowed" });
    expect(r.headers["allow"]).toContain("POST");
  });

  it("404s a trailing slash rather than redirecting", async () => {
    const r = await request(app).get("/api/me/").set("X-User-ID", "u_ada");
    expect(r.status).toBe(404);
    expect(r.body).toEqual({ error: "not found" });
  });

  it("does not decode a percent-encoded octet before routing", async () => {
    // Express matches the raw path, so /api/%6de is an unregistered
    // path. Gin decoded it and served /api/me until the contract
    // pinned this.
    for (const path of ["/api/%6de", "/api/%72esources"]) {
      const r = await request(app).get(path).set("X-User-ID", "u_ada");
      expect(r.status, path).toBe(404);
      expect(r.body).toEqual({ error: "not found" });
    }

    // Raw routing is not raw path parameters: a captured id is decoded.
    const encoded = await request(app)
      .get("/api/resources/r%5Facme_pg/requests").set("X-User-ID", "u_ada");
    const plain = await request(app)
      .get("/api/resources/r_acme_pg/requests").set("X-User-ID", "u_ada");
    expect(encoded.status).toBe(200);
    expect(encoded.body).toEqual(plain.body);
  });

  it("returns an identical 404 for an unknown path and a cross-tenant miss", async () => {
    const unknown = await request(app).get("/api/nope").set("X-User-ID", "u_ada");
    const cross = await request(app)
      .get("/api/resources/r_globex_k8s/requests").set("X-User-ID", "u_ada");
    expect(unknown.status).toBe(cross.status);
    expect(unknown.body).toEqual(cross.body);
  });

  it("204s with no content-type and no body on logout", async () => {
    const r = await request(app).post("/api/logout").set("X-User-ID", "u_ada");
    expect(r.status).toBe(204);
    expect(r.headers["content-type"]).toBeUndefined();
    expect(r.text).toBe("");
  });

  it("400s a non-string role on invite, rather than coercing it", async () => {
    const r = await request(app)
      .post("/api/members")
      .set("X-User-ID", "u_ada")
      .send({ name: "Grace", email: "grace2@acme.test", role: 123 });
    expect(r.status).toBe(400);
    expect(r.body).toEqual({ error: "invalid body" });
  });

  it("400s a non-string role on role change, rather than coercing it", async () => {
    const r = await request(app)
      .put("/api/members/u_linus/role")
      .set("X-User-ID", "u_ada")
      .send({ role: 123 });
    expect(r.status).toBe(400);
    expect(r.body).toEqual({ error: "invalid body" });
  });

  // null is equivalent to omitting the field, per the contract, so an
  // optional field set to null takes its documented default.
  it("treats a null role on invite as omitted, defaulting to member", async () => {
    const r = await request(app)
      .post("/api/members")
      .set("X-User-ID", "u_ada")
      .send({ name: "Null Role", email: "null-role@acme.test", role: null });
    expect(r.status).toBe(201);
    expect(r.body.role).toBe("member");
  });

  it("treats a null description and sensitivity as omitted", async () => {
    const r = await request(app)
      .post("/api/resources")
      .set("X-User-ID", "u_ada")
      .send({ name: "null-optionals", description: null, sensitivity: null });
    expect(r.status).toBe(201);
    expect(r.body.description).toBe("");
    expect(r.body.sensitivity).toBe("internal");
  });

  // A required field set to null is a missing field, not a wrong type,
  // so it gets the route's own "required" message.
  it("treats a null required field as missing, not as a wrong type", async () => {
    const member = await request(app)
      .post("/api/members")
      .set("X-User-ID", "u_ada")
      .send({ name: null, email: "null-name@acme.test" });
    expect(member.status).toBe(400);
    expect(member.body).toEqual({ error: "name and email required" });

    const login = await request(app).post("/api/login").send({ email: null });
    expect(login.status).toBe(400);
    expect(login.body).toEqual({ error: "email required" });

    const role = await request(app)
      .put("/api/members/u_linus/role")
      .set("X-User-ID", "u_ada")
      .send({ role: null });
    expect(role.status).toBe(400);
    expect(role.body).toEqual({ error: "role must be admin or member" });

    const reason = await request(app)
      .post("/api/resources/r_acme_pg/requests")
      .set("X-User-ID", "u_linus")
      .send({ reason: null });
    expect(reason.status).toBe(400);
    expect(reason.body).toEqual({ error: "reason required" });

    const comment = await request(app)
      .post("/api/requests/req_acme_1/comments")
      .set("X-User-ID", "u_linus")
      .send({ body: null });
    expect(comment.status).toBe(400);
    expect(comment.body).toEqual({ error: "body required" });
  });

  // Unknown request fields are ignored, per the contract.
  it("ignores unknown request fields rather than rejecting them", async () => {
    const r = await request(app)
      .post("/api/members")
      .set("X-User-ID", "u_ada")
      .send({ name: "Extra Fields", email: "extra-fields@acme.test", bogus: 1 });
    expect(r.status).toBe(201);
    expect(r.body).not.toHaveProperty("bogus");
  });

  // A mixed-case spelling of a registered path must be as anonymous as
  // an unknown one, or it becomes an existence oracle.
  it("404s a mixed-case path identically to an unknown path", async () => {
    const unknown = await request(app).get("/api/nope").set("X-User-ID", "u_ada");
    for (const path of ["/api/ME", "/api/Members", "/api/RESOURCES"]) {
      const r = await request(app).get(path).set("X-User-ID", "u_ada");
      expect(r.status, path).toBe(unknown.status);
      expect(r.body, path).toEqual(unknown.body);
    }
  });

  it("404s a mixed-case /api mount", async () => {
    const r = await request(app).get("/API/me").set("X-User-ID", "u_ada");
    expect(r.status).toBe(404);
  });

  // Check precedence on every route that reads a body: 401 first, then
  // 403, then the parse. Nothing else in this file asserted a 403 at all.
  // The malformed body is what makes each row discriminate: a valid body
  // answers 403 whichever order the handler runs its checks in, so only
  // an unparseable one shows a role check that has slipped behind the
  // parse. express.text in router.ts is what keeps that order available;
  // express.json would have answered 400 before any handler ran.
  it("checks auth and the role before parsing the body, on every body route", async () => {
    const send = (method: "post" | "put", path: string, actor?: string) => {
      const req = request(app)[method](path).set("Content-Type", "application/json");
      if (actor) req.set("X-User-ID", actor);
      return req.send("{");
    };

    const routes = [
      { name: "POST /api/members", method: "post", path: "/api/members", adminOnly: true },
      { name: "PUT /api/members/:id/role", method: "put", path: "/api/members/u_linus/role", adminOnly: true },
      { name: "POST /api/resources", method: "post", path: "/api/resources", adminOnly: true },
      { name: "POST /api/resources/:id/requests", method: "post", path: "/api/resources/r_acme_pg/requests", adminOnly: false },
      { name: "POST /api/requests/:id/comments", method: "post", path: "/api/requests/req_acme_1/comments", adminOnly: false },
    ] as const;

    for (const r of routes) {
      const anon = await send(r.method, r.path);
      expect(anon.status, `${r.name}: no actor`).toBe(401);

      const member = await send(r.method, r.path, "u_linus");
      expect(member.status, `${r.name}: member`).toBe(r.adminOnly ? 403 : 400);
      if (r.adminOnly) {
        expect(member.body, r.name).toEqual({ error: "admin role required" });
      }

      // Positive control: the same body from an admin reaches the parse,
      // so the row above is about order and not a route that refuses
      // everyone.
      const admin = await send(r.method, r.path, "u_ada");
      expect(admin.status, `${r.name}: admin`).toBe(400);
    }
  });

  it("still reaches the API's own JSON 404 on a bare /api", async () => {
    const r = await request(app).get("/api").set("X-User-ID", "u_ada");
    expect(r.status).toBe(404);
    expect(r.body).toEqual({ error: "not found" });
  });
});
