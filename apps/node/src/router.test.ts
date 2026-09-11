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

});
