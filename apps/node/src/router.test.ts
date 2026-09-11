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
});
