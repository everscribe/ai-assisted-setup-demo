import { describe, expect, it } from "vitest";
import { MemoryStore } from "./memory.js";
import { ErrNotFound } from "./store.js";

describe("MemoryStore", () => {
  it("seeds the fixture the contract declares", async () => {
    const s = new MemoryStore();
    await s.seed();
    const u = await s.getUser("u_ada");
    expect(u.email).toBe("ada@acme.test");
    expect(u.role).toBe("admin");
  });

  it("returns not-found for a cross-tenant resource", async () => {
    const s = new MemoryStore();
    await s.seed();
    // r_globex_k8s belongs to org_globex, not org_acme.
    await expect(s.getResource("org_acme", "r_globex_k8s")).rejects.toBe(ErrNotFound);
  });

  it("orders resources by created_at then id", async () => {
    const s = new MemoryStore();
    await s.seed();
    const rows = await s.listOrgResources("org_acme");
    expect(rows.map((r) => r.id)).toEqual(["r_acme_pg", "r_acme_s3", "r_acme_stripe"]);
  });
});
