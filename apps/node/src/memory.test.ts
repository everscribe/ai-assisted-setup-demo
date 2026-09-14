import { describe, expect, it } from "vitest";
import { MemoryStore } from "./memory.js";
import type { User } from "./models.js";
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

  // The duplicate-email rule. Over HTTP a duplicate can only be minted
  // after the user it duplicates, so insertion order and created_at order
  // always agree and no conformance check can tell the sort from a plain
  // map walk. Planting an older duplicate behind the seeded user is what
  // makes the sort load-bearing: without it this returns u_ada.
  it("resolves a duplicate email to the earliest user, not the first stored", async () => {
    const s = new MemoryStore();
    await s.seed();

    // Positive control: the lookup answers before any duplicate exists.
    expect((await s.findUserByEmail("ada@acme.test")).id).toBe("u_ada");

    const users = (s as unknown as { users: Map<string, User> }).users;
    const older: User = {
      id: "u_older",
      name: "Ada Elder",
      email: "ada@acme.test",
      role: "member",
      organization_id: "org_acme",
      created_at: "2020-01-01T00:00:00Z",
    };
    users.set(older.id, older);
    expect((await s.findUserByEmail("ada@acme.test")).id).toBe("u_older");

    // Tied created_at, stored last, so only the id tiebreak can win it.
    users.set("u_aaa", { ...older, id: "u_aaa", name: "Ada Tie" });
    expect((await s.findUserByEmail("ada@acme.test")).id).toBe("u_aaa");

    await expect(s.findUserByEmail("nobody@acme.test")).rejects.toBe(ErrNotFound);
  });

  it("decommissions idempotently, re-stamping decommission_at", async () => {
    const s = new MemoryStore();
    await s.seed();
    const first = await s.decommissionResource("org_acme", "r_acme_pg");
    expect(first.decommission_at).toBeTruthy();
    const second = await s.decommissionResource("org_acme", "r_acme_pg");
    expect(second.decommission_at).toBeTruthy();
  });
});
